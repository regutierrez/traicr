package store_test

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/regutierrez/traicr/internal/domain"
	"github.com/regutierrez/traicr/internal/store"
)

func TestRepeatedImportBackfillsMissingRepositoryWithoutReplacingNewerMetadata(t *testing.T) {
	for _, repository := range []domain.Repository{{Remote: "https://example.invalid/metadata-repo", Root: "/work/metadata-repo"}, {Root: "/work/path-only"}} {
		t.Run(repository.Root, func(t *testing.T) {
			ctx := context.Background()
			database := openStore(t)
			input := traceInput("machine", "same-bytes", "2026-09-16T10:00:00Z", "", []domain.Event{{Key: "event", Kind: "message", Text: "repository lookup"}})
			report, err := database.Import(ctx, input.manifest, input.files, normalizer(input.events, "normalized", 1), nil)
			if err != nil {
				t.Fatal(err)
			}
			traceID := report.Traces[0].TraceID
			input.manifest.Traces[0].Repository = repository
			report, err = database.Import(ctx, input.manifest, input.files, normalizer(input.events, "normalized", 1), nil)
			if err != nil || report.Updated != 1 {
				t.Fatalf("repository backfill: %+v %v", report, err)
			}
			trace, err := database.Trace(ctx, traceID)
			if err != nil || trace.Repository != repository.Remote || len(trace.Revisions) != 1 {
				t.Fatalf("metadata created a revision or lost repository: %+v %v", trace, err)
			}
			page, err := database.Search(ctx, store.SearchQuery{Query: "repository lookup", Repository: repository.Root})
			if err != nil || len(page.Results) != 1 {
				t.Fatalf("repository path filter missing: %+v %v", page, err)
			}
			page, err = database.Search(ctx, store.SearchQuery{Query: repository.Root, Mode: "exact"})
			if err != nil || len(page.Results) != 1 {
				t.Fatalf("repository metadata not indexed: %+v %v", page, err)
			}
			input.manifest.Traces[0].Repository = domain.Repository{Remote: "https://example.invalid/wrong", Root: "/wrong"}
			report, err = database.Import(ctx, input.manifest, input.files, normalizer(input.events, "normalized", 1), nil)
			if err != nil || report.Unchanged != 1 {
				t.Fatalf("existing repository replaced: %+v %v", report, err)
			}
			input.manifest.Traces[0].RevisionDigest = "newer"
			input.manifest.Traces[0].NativeUpdatedAt = "2026-09-17T10:00:00Z"
			input.manifest.Traces[0].Repository = domain.Repository{}
			if _, err := database.Import(ctx, input.manifest, input.files, normalizer(input.events, "normalized", 1), nil); err != nil {
				t.Fatal(err)
			}
			input.manifest.Traces[0].RevisionDigest = "same-bytes"
			input.manifest.Traces[0].NativeUpdatedAt = "2026-09-16T10:00:00Z"
			input.manifest.Traces[0].Repository = repository
			report, err = database.Import(ctx, input.manifest, input.files, normalizer(input.events, "normalized", 1), nil)
			if err != nil || report.Unchanged != 1 {
				t.Fatalf("stale repository restored: %+v %v", report, err)
			}
		})
	}
}

func TestImportReportsEveryOutcomeAndCompletedProgress(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	sample := traceInput("machine", "revision", "2026-08-22T10:00:00Z", "Mixed import", nil)
	manifest := sample.manifest
	manifest.Traces = nil
	files := fstest.MapFS{}
	for _, status := range []string{"normalized", "partially_parsed", "unsupported", "failed"} {
		descriptor := sample.manifest.Traces[0]
		descriptor.NativeTraceID = status
		descriptor.Path = "traces/" + status
		manifest.Traces = append(manifest.Traces, descriptor)
		files[descriptor.Path+"/source/records.jsonl"] = sample.files["traces/000001/source/records.jsonl"]
	}
	var updates []domain.Progress
	report, err := s.Import(ctx, manifest, files, func(_ context.Context, descriptor domain.Descriptor, _ fs.FS) (domain.Normalization, error) {
		if descriptor.NativeTraceID == "failed" {
			return domain.Normalization{}, errors.New("cannot parse this trace")
		}
		return domain.Normalization{Status: descriptor.NativeTraceID, Version: 1}, nil
	}, func(progress domain.Progress) { updates = append(updates, progress) })
	if err != nil || report.Imported != 1 || report.Partial != 1 || report.Unsupported != 1 || report.Failed != 1 || len(report.Traces) != 4 {
		t.Fatalf("mixed report=%+v err=%v", report, err)
	}
	completed := 0
	for _, update := range updates {
		if update.Outcome != nil {
			completed++
			if update.Completed != completed || update.Total != 4 {
				t.Fatalf("progress=%+v, want completed=%d total=4", update, completed)
			}
		}
	}
	if len(updates) == 0 || completed != 4 {
		t.Fatalf("missing progress: updates=%d completed=%d", len(updates), completed)
	}
	last := updates[len(updates)-1]
	if last.Phase != "complete" || last.Completed != 4 || last.Total != 4 || last.Report == nil || last.Report.ID != report.ID {
		t.Fatalf("incomplete final progress: %+v", last)
	}
	stored, err := s.ImportReport(ctx, report.ID)
	if err != nil || stored.Partial != 1 || stored.Failed != 1 || len(stored.Traces) != 4 {
		t.Fatalf("stored report=%+v err=%v", stored, err)
	}
}

func TestImportRetainsRepositoryPathsAndUsesMissingNativeTimeFallback(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	started := time.Now().UTC()
	for _, root := range []string{"/home/alice/repo", "/work/repo"} {
		input := traceInput(root, root, "", "Repository paths", []domain.Event{{Key: "event", Kind: "message", Text: "searchable"}})
		input.manifest.Traces[0].Repository = domain.Repository{Remote: "example.invalid/team/repo", Root: root}
		report, err := s.Import(ctx, input.manifest, input.files, normalizer(input.events, "normalized", 1), nil)
		if err != nil {
			t.Fatal(err)
		}
		trace, err := s.Trace(ctx, report.Traces[0].TraceID)
		if err != nil {
			t.Fatal(err)
		}
		updated, err := time.Parse(time.RFC3339Nano, trace.UpdatedAt)
		if err != nil || updated.Before(started) || updated.After(time.Now().UTC()) {
			t.Fatalf("missing native timestamp fallback=%q err=%v", trace.UpdatedAt, err)
		}
	}
	for _, repository := range []string{"/home/alice/repo", "/work/repo", "example.invalid/team/repo"} {
		page, err := s.Search(ctx, store.SearchQuery{Query: "searchable", Repository: repository})
		if err != nil || len(page.Results) != 1 {
			t.Fatalf("repository=%q results=%d err=%v", repository, len(page.Results), err)
		}
	}
}

func TestSearchDateOnlyBeforeExcludesNextMidnight(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	input := traceInput("machine", "day-boundary", "2026-08-22T10:00:00Z", "Day boundary", []domain.Event{
		{Key: "last-in-day", Kind: "message", Timestamp: "2026-08-22T23:59:59.999999999Z", Text: "boundary"},
		{Key: "next-midnight", Kind: "message", Timestamp: "2026-08-23T00:00:00Z", Text: "boundary"},
	})
	if _, err := s.Import(ctx, input.manifest, input.files, normalizer(input.events, "normalized", 1), nil); err != nil {
		t.Fatal(err)
	}
	for before, want := range map[string]int{"2026-08-22": 1, "2026-08-23T00:00:00Z": 2} {
		page, err := s.Search(ctx, store.SearchQuery{Query: "boundary", Before: before})
		if err != nil || len(page.Results) != want {
			t.Fatalf("before=%q results=%d want=%d err=%v", before, len(page.Results), want, err)
		}
	}
}

func TestOpenRejectsInvalidDatabase(t *testing.T) {
	for _, state := range []string{"corrupt", "newer", "partial"} {
		t.Run(state, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "traicr.db")
			if state == "corrupt" {
				if err := os.WriteFile(path, []byte("not a SQLite database"), 0o600); err != nil {
					t.Fatal(err)
				}
			} else {
				db, err := sql.Open("sqlite", path)
				if err != nil {
					t.Fatal(err)
				}
				statement := "PRAGMA user_version=2"
				if state == "partial" {
					statement = "CREATE TABLE events(id INTEGER)"
				}
				_, err = db.Exec(statement)
				db.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			database, err := store.Open(directory)
			if database != nil {
				database.Close()
			}
			if err == nil {
				t.Fatalf("opened %s database", state)
			}
			if state == "partial" {
				db, err := sql.Open("sqlite", path)
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				var created int
				if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_schema WHERE name='source_machines'").Scan(&created); err != nil || created != 0 {
					t.Fatalf("failed migration was not atomic: created=%d err=%v", created, err)
				}
			}
		})
	}
}
