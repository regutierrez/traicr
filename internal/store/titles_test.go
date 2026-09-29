package store_test

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/regutierrez/traicr/internal/domain"
	"github.com/regutierrez/traicr/internal/store"
)

func TestTitleOverrideSurvivesImportsAndIsSearchable(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	first := domain.Event{Key: "message-1", Kind: "message", Role: "user", Text: "Hello", Sources: []domain.SourceRef{{Path: "source/records.jsonl", Line: 1}}}
	second := domain.Event{Key: "message-2", Kind: "message", Role: "assistant", Text: "Done", Sources: []domain.SourceRef{{Path: "source/records.jsonl", Line: 2}}}
	original := traceInput("machine-a", "revision-1", "2026-08-22T12:00:00Z", "Collected name", []domain.Event{first})
	report, err := s.Import(ctx, original.manifest, original.files, normalizer(original.events, "normalized", 1), nil)
	if err != nil || report.Imported != 1 {
		t.Fatalf("import: %+v, %v", report, err)
	}
	id := report.Traces[0].TraceID

	searchCount := func(query, mode string) int {
		t.Helper()
		page, err := s.Search(ctx, store.SearchQuery{Query: query, Mode: mode})
		if err != nil {
			t.Fatalf("search %q: %v", query, err)
		}
		return len(page.Results)
	}
	requireTitle := func(want, native, override string) {
		t.Helper()
		trace, err := s.Trace(ctx, id)
		if err != nil || trace.Title != want || trace.NativeTitle != native || trace.TitleOverride != override {
			t.Fatalf("trace titles: %+v, %v; want title=%q native=%q override=%q", trace, err, want, native, override)
		}
		cards, err := s.TranscriptCards(ctx, store.SearchQuery{})
		if err != nil || len(cards.Cards) != 1 || cards.Cards[0].Title != want {
			t.Fatalf("cards: %+v, %v", cards, err)
		}
	}

	trace, err := s.SetTitleOverride(ctx, id, "  Refactor auth flow  ")
	if err != nil || trace.Title != "Refactor auth flow" {
		t.Fatalf("rename: %+v, %v", trace, err)
	}
	requireTitle("Refactor auth flow", "Collected name", "Refactor auth flow")
	for _, mode := range []string{"fulltext", "exact", "regex"} {
		page, err := s.Search(ctx, store.SearchQuery{Query: "Refactor", Mode: mode})
		if err != nil || len(page.Results) != 1 || page.Results[0].TraceTitle != "Refactor auth flow" {
			t.Fatalf("%s override search: %+v, %v", mode, page, err)
		}
	}
	if searchCount("Collected", "exact") != 1 || searchCount("source/records.jsonl", "exact") != 1 {
		t.Fatal("rename dropped the collected title or source paths from search text")
	}

	// Imports only write the collected title; the override and its search text stay.
	newer := traceInput("machine-a", "revision-2", "2026-08-23T12:00:00Z", "Newer collected name", []domain.Event{first, second})
	if _, err := s.Import(ctx, newer.manifest, newer.files, normalizer(newer.events, "normalized", 1), nil); err != nil {
		t.Fatal(err)
	}
	requireTitle("Refactor auth flow", "Newer collected name", "Refactor auth flow")
	if got := searchCount("Refactor", "exact"); got != 2 {
		t.Fatalf("override search after new revision = %d events, want 2", got)
	}
	if err := s.Renormalize(ctx, func(string) int { return 2 }, normalizer(newer.events, "normalized", 2)); err != nil {
		t.Fatal(err)
	}
	if got := searchCount("Refactor", "exact"); got != 2 {
		t.Fatalf("override search after renormalize = %d events, want 2", got)
	}

	if _, err := s.SetTitleOverride(ctx, id, "Final name"); err != nil {
		t.Fatal(err)
	}
	if searchCount("Refactor", "exact") != 0 || searchCount("Final name", "exact") != 2 {
		t.Fatal("second rename did not replace the previous override in search")
	}
	if _, err := s.SetTitleOverride(ctx, id, ""); err != nil {
		t.Fatal(err)
	}
	requireTitle("Newer collected name", "Newer collected name", "")
	if searchCount("Final name", "exact") != 0 {
		t.Fatal("cleared override is still searchable")
	}

	for _, invalid := range []string{"two\nlines", "tab\tseparated", strings.Repeat("x", 201)} {
		if _, err := s.SetTitleOverride(ctx, id, invalid); !errors.Is(err, store.ErrInvalidTitle) {
			t.Fatalf("title %q: err = %v, want ErrInvalidTitle", invalid, err)
		}
	}
	if _, err := s.SetTitleOverride(ctx, id+1, "Missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing trace: err = %v, want sql.ErrNoRows", err)
	}
}

func TestGeneratedTitlesFillOnlyUntitledIdleTraces(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	importTrace := func(nativeID, revision, updated, title string) int64 {
		t.Helper()
		input := traceInput("machine-a", revision, updated, title, []domain.Event{{Key: "m-" + revision, Kind: "message", Role: "user", Text: "Please fix the login redirect loop"}})
		input.manifest.Traces[0].NativeTraceID = nativeID
		report, err := s.Import(ctx, input.manifest, input.files, normalizer(input.events, "normalized", 1), nil)
		if err != nil || report.Failed != 0 {
			t.Fatalf("import %s: %+v, %v", nativeID, report, err)
		}
		return report.Traces[0].TraceID
	}
	untitled := importTrace("untitled", "u-1", "2026-08-22T10:00:00Z", "")
	collected := importTrace("collected", "c-1", "2026-08-22T10:00:00Z", "Pi session name")
	renamed := importTrace("renamed", "r-1", "2026-08-22T10:00:00Z", "")
	if _, err := s.SetTitleOverride(ctx, renamed, "Manual name"); err != nil {
		t.Fatal(err)
	}
	live := importTrace("live", "l-1", "2026-08-22T11:50:00Z", "")
	idleBefore := time.Date(2026, 8, 22, 11, 30, 0, 0, time.UTC)

	candidateIDs := func() []int64 {
		t.Helper()
		candidates, err := s.TitleCandidates(ctx, idleBefore, 10)
		if err != nil {
			t.Fatal(err)
		}
		var ids []int64
		for _, candidate := range candidates {
			ids = append(ids, candidate.TraceID)
		}
		return ids
	}
	candidates, err := s.TitleCandidates(ctx, idleBefore, 10)
	if err != nil || len(candidates) != 1 || candidates[0].TraceID != untitled {
		t.Fatalf("candidates = %+v, %v; want only the idle untitled trace %d (not %d, %d, %d)", candidates, err, untitled, collected, renamed, live)
	}
	source, err := s.TitleSource(ctx, untitled)
	if err != nil || source.Harness != "synthetic" || len(source.UserMessages) != 1 || source.UserMessages[0] != "Please fix the login redirect loop" {
		t.Fatalf("title source: %+v, %v", source, err)
	}

	if err := s.SetGeneratedTitle(ctx, untitled, candidates[0].RevisionID, "Fix login redirect loop"); err != nil {
		t.Fatal(err)
	}
	trace, err := s.Trace(ctx, untitled)
	if err != nil || trace.Title != "Fix login redirect loop" || trace.GeneratedTitle != "Fix login redirect loop" || trace.NativeTitle != "" {
		t.Fatalf("generated title not shown: %+v, %v", trace, err)
	}
	page, err := s.Search(ctx, store.SearchQuery{Query: "Fix login redirect loop", Mode: "exact"})
	if err != nil || len(page.Results) != 1 || page.Results[0].TraceID != untitled || page.Results[0].TraceTitle != "Fix login redirect loop" {
		t.Fatalf("generated title search: %+v, %v", page, err)
	}
	if ids := candidateIDs(); len(ids) != 0 {
		t.Fatalf("titled revision offered again: %v", ids)
	}
	// Stale results for an older revision never replace a newer one.
	if err := s.SetGeneratedTitle(ctx, untitled, candidates[0].RevisionID-1, "Stale"); err != nil {
		t.Fatal(err)
	}

	// A new revision makes the trace a candidate again; the old name stays until replaced.
	importTrace("untitled", "u-2", "2026-08-22T10:30:00Z", "")
	if ids := candidateIDs(); len(ids) != 1 || ids[0] != untitled {
		t.Fatalf("new revision candidates = %v", ids)
	}
	if trace, _ := s.Trace(ctx, untitled); trace.Title != "Fix login redirect loop" {
		t.Fatalf("title after new revision = %q", trace.Title)
	}
	found, err := s.Search(ctx, store.SearchQuery{Query: "Fix login redirect loop", Mode: "exact"})
	if err != nil || len(found.Results) != 2 {
		t.Fatalf("generated title missing from new revision's search text: %+v, %v", found, err)
	}

	// A collected title arriving later wins over the generated one.
	importTrace("untitled", "u-3", "2026-08-22T10:40:00Z", "Named in Pi")
	if trace, _ := s.Trace(ctx, untitled); trace.Title != "Named in Pi" || trace.GeneratedTitle != "Fix login redirect loop" {
		t.Fatalf("collected title did not win: %+v", trace)
	}
	if ids := candidateIDs(); len(ids) != 0 {
		t.Fatalf("collected trace still a candidate: %v", ids)
	}
	if err := s.SetGeneratedTitle(ctx, untitled, 1<<40, "bad\ntitle"); !errors.Is(err, store.ErrInvalidTitle) {
		t.Fatalf("multi-line generated title: err = %v", err)
	}
}

func TestRenormalizeKeepsARenameThatLandsMidRebuild(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	importOne := func(nativeID string) int64 {
		t.Helper()
		input := traceInput("machine-a", nativeID, "2026-08-22T10:00:00Z", "", []domain.Event{{Key: "m-" + nativeID, Kind: "message", Role: "user", Text: "hello " + nativeID}})
		input.manifest.Traces[0].NativeTraceID = nativeID
		report, err := s.Import(ctx, input.manifest, input.files, normalizer(input.events, "normalized", 1), nil)
		if err != nil || report.Imported != 1 {
			t.Fatalf("import %s: %+v, %v", nativeID, report, err)
		}
		return report.Traces[0].TraceID
	}
	first, second := importOne("first"), importOne("second")
	renamed := make(chan error, 1)
	started := false
	rebuild := func(_ context.Context, descriptor domain.Descriptor, _ fs.FS) (domain.Normalization, error) {
		if descriptor.NativeTraceID == "first" && !started {
			started = true
			// The rename waits for the writer lock that this rebuild holds, so it
			// commits after the batch was read and before the second trace is rebuilt.
			go func() {
				_, err := s.SetTitleOverride(ctx, second, "Renamed during rebuild")
				renamed <- err
			}()
			time.Sleep(100 * time.Millisecond)
		}
		return domain.Normalization{Version: 2, Status: "normalized", Events: []domain.Event{{Key: "m-" + descriptor.NativeTraceID, Kind: "message", Role: "user", Text: "hello " + descriptor.NativeTraceID}}}, nil
	}
	if err := s.Renormalize(ctx, func(string) int { return 2 }, rebuild); err != nil {
		t.Fatal(err)
	}
	if err := <-renamed; err != nil {
		t.Fatal(err)
	}
	page, err := s.Search(ctx, store.SearchQuery{Query: "Renamed during rebuild", Mode: "exact"})
	if err != nil || len(page.Results) != 1 || page.Results[0].TraceID != second {
		t.Fatalf("rename lost by the rebuild (first=%d second=%d): %+v, %v", first, second, page, err)
	}
}

func TestRenormalizedRevisionIsOfferedForNamingAgain(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	input := traceInput("machine-a", "revision-1", "2026-08-22T10:00:00Z", "", nil)
	report, err := s.Import(ctx, input.manifest, input.files, normalizer(nil, "unsupported", 1), nil)
	if err != nil || report.Unsupported != 1 {
		t.Fatalf("import: %+v, %v", report, err)
	}
	idle := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	candidates, err := s.TitleCandidates(ctx, idle, 10)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("candidates: %+v, %v", candidates, err)
	}
	// Nothing to name yet: the worker records an empty title for this revision.
	if err := s.SetGeneratedTitle(ctx, candidates[0].TraceID, candidates[0].RevisionID, ""); err != nil {
		t.Fatal(err)
	}
	if again, _ := s.TitleCandidates(ctx, idle, 10); len(again) != 0 {
		t.Fatalf("empty revision offered again before it changed: %+v", again)
	}
	events := []domain.Event{{Key: "m1", Kind: "message", Role: "user", Text: "now there is a request"}}
	if err := s.Renormalize(ctx, func(string) int { return 2 }, normalizer(events, "normalized", 2)); err != nil {
		t.Fatal(err)
	}
	if again, err := s.TitleCandidates(ctx, idle, 10); err != nil || len(again) != 1 || again[0].RevisionID != candidates[0].RevisionID {
		t.Fatalf("recovered revision not offered: %+v, %v", again, err)
	}
}

func TestTitleSourceFollowsTranscriptOrderWithoutTimestamps(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	// Timestamp-less rows get content-hash keys, so key order is not source order.
	line := func(n int) []domain.SourceRef { return []domain.SourceRef{{Path: "source/records.jsonl", Line: n}} }
	events := []domain.Event{
		{Key: "message:sha256:f", Kind: "message", Role: "user", Text: "first request", Sources: line(1)},
		{Key: "message:sha256:a", Kind: "message", Role: "assistant", Text: "early answer", Sources: line(2)},
		{Key: "message:sha256:e", Kind: "message", Role: "user", Text: "follow-up", Sources: line(3)},
		{Key: "message:sha256:b", Kind: "message", Role: "assistant", Text: "final answer", Sources: line(4)},
	}
	input := traceInput("machine-a", "revision-1", "2026-08-22T10:00:00Z", "", events)
	report, err := s.Import(ctx, input.manifest, input.files, normalizer(events, "normalized", 1), nil)
	if err != nil || report.Imported != 1 {
		t.Fatalf("import: %+v, %v", report, err)
	}
	source, err := s.TitleSource(ctx, report.Traces[0].TraceID)
	if err != nil || strings.Join(source.UserMessages, "|") != "first request|follow-up" || source.LastAssistantMessage != "final answer" {
		t.Fatalf("title source out of transcript order: %+v, %v", source, err)
	}
}
