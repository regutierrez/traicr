package store_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"
	"unicode/utf8"

	"github.com/regutierrez/traicr/internal/domain"
	"github.com/regutierrez/traicr/internal/store"
)

func TestImportMergesIdenticalRepeatedEventsInsteadOfFailing(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	retry := domain.Event{Key: "message:sha256:retry", Kind: "message", Role: "user", Text: "run smoke test", Sources: []domain.SourceRef{{Path: "source/records.jsonl", Line: 3}}}
	again := retry
	again.Sources = []domain.SourceRef{{Path: "source/records.jsonl", Line: 9}}
	other := domain.Event{Key: "message:sha256:other", Kind: "message", Role: "assistant", Text: "done", Sources: []domain.SourceRef{{Path: "source/records.jsonl", Line: 4}}}
	input := traceInput("machine-a", "retries", "2026-08-22T12:00:00Z", "", []domain.Event{retry, other, again})
	report, err := s.Import(ctx, input.manifest, input.files, normalizer(input.events, "normalized", 1), nil)
	if err != nil || report.Imported != 1 || report.Failed != 0 {
		t.Fatalf("import: %+v, %v", report, err)
	}
	outcome := report.Traces[0]
	if len(outcome.Warnings) != 1 || outcome.Warnings[0].Code != "duplicate_event" {
		t.Fatalf("warnings: %+v", outcome.Warnings)
	}
	page, err := s.Events(ctx, outcome.TraceID, "", 10)
	if err != nil || len(page.Events) != 2 {
		t.Fatalf("events: %+v, %v", page, err)
	}
	for _, event := range page.Events {
		sources, err := s.Sources(ctx, event.ID)
		if err != nil {
			t.Fatal(err)
		}
		var lines []int
		for _, source := range sources {
			lines = append(lines, source.Line)
		}
		want := "[4]"
		if event.Key == retry.Key {
			want = "[3 9]"
		}
		if fmt.Sprint(lines) != want || event.ObservationCount != 1 {
			t.Fatalf("event %s sources %v (observations %d), want lines %s", event.Key, lines, event.ObservationCount, want)
		}
	}
	// Re-importing the same revision must stay idempotent.
	report, err = s.Import(ctx, input.manifest, input.files, normalizer(input.events, "normalized", 1), nil)
	if err != nil || report.Unchanged != 1 {
		t.Fatalf("retry import: %+v, %v", report, err)
	}
}

func TestImportBackfillsMissingTitleWithoutNewRevision(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name, originalTitle, incomingTitle, newerTitle, wantTitle string
		updated, newerRevision                                    bool
	}{
		{name: "missing title", incomingTitle: "Repair authentication", wantTitle: "Repair authentication", updated: true},
		{name: "preserve existing", originalTitle: "Existing", incomingTitle: "Replacement", wantTitle: "Existing"},
		{name: "unnamed retry"},
		{name: "stale retry", incomingTitle: "Old name", newerTitle: "New name", wantTitle: "New name", newerRevision: true},
		{name: "stale name after clear", incomingTitle: "Old name", newerRevision: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := openStore(t)
			input := traceInput("machine-a", "same-revision", "2026-08-22T12:00:00Z", test.originalTitle, []domain.Event{{Key: "message-1", Kind: "message", Role: "user", Text: "Hello"}})
			report, err := s.Import(ctx, input.manifest, input.files, normalizer(input.events, "normalized", 1), nil)
			if err != nil || report.Imported != 1 {
				t.Fatalf("initial import: %+v, %v", report, err)
			}
			revisions := 1
			if test.newerRevision {
				newer := traceInput("machine-a", "newer-revision", "2026-08-23T12:00:00Z", test.newerTitle, input.events)
				if _, err := s.Import(ctx, newer.manifest, newer.files, normalizer(newer.events, "normalized", 1), nil); err != nil {
					t.Fatal(err)
				}
				revisions++
			}
			input.manifest.Traces[0].Title = test.incomingTitle
			for attempt := 0; attempt < 2; attempt++ {
				report, err = s.Import(ctx, input.manifest, input.files, func(context.Context, domain.Descriptor, fs.FS) (domain.Normalization, error) {
					t.Fatal("duplicate revision must not be normalized again")
					return domain.Normalization{}, nil
				}, nil)
				if err != nil || report.Failed != 0 || (test.updated && attempt == 0 && report.Updated != 1) || ((!test.updated || attempt > 0) && report.Unchanged != 1) {
					t.Fatalf("retry: %+v, %v", report, err)
				}
			}
			trace, err := s.Trace(ctx, report.Traces[0].TraceID)
			if err != nil || trace.Title != test.wantTitle || len(trace.Revisions) != revisions {
				t.Fatalf("trace: %+v, %v", trace, err)
			}
			cards, err := s.TranscriptCards(ctx, store.SearchQuery{})
			if err != nil || len(cards.Cards) != 1 || cards.Cards[0].Title != test.wantTitle {
				t.Fatalf("transcript cards: %+v, %v", cards, err)
			}
			if test.updated {
				for _, mode := range []string{"fulltext", "exact", "regex"} {
					page, err := s.Search(ctx, store.SearchQuery{Query: "authentication", Mode: mode})
					if err != nil || len(page.Results) != 1 || page.Results[0].TraceTitle != test.wantTitle {
						t.Fatalf("%s title search: %+v, %v", mode, page, err)
					}
				}
			}
		})
	}
}

func TestImportBackfillsTitleForPartiallyParsedRevision(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	input := traceInput("machine-a", "partial-revision", "2026-08-22T12:00:00Z", "", []domain.Event{{Key: "message-1", Kind: "message", Role: "user", Text: "Hello"}})
	report, err := s.Import(ctx, input.manifest, input.files, normalizer(input.events, "partially_parsed", 1), nil)
	if err != nil || report.Partial != 1 {
		t.Fatalf("initial partial import: %+v, %v", report, err)
	}
	input.manifest.Traces[0].Title = "Repair authentication"
	for attempt := 0; attempt < 2; attempt++ {
		report, err = s.Import(ctx, input.manifest, input.files, normalizer(input.events, "partially_parsed", 1), nil)
		if err != nil || report.Partial != 1 {
			t.Fatalf("partial retry: %+v, %v", report, err)
		}
		trace, err := s.Trace(ctx, report.Traces[0].TraceID)
		if err != nil || trace.Title != "Repair authentication" || len(trace.Revisions) != 1 {
			t.Fatalf("partial trace title: %+v, %v", trace, err)
		}
		for _, mode := range []string{"fulltext", "exact", "regex"} {
			page, err := s.Search(ctx, store.SearchQuery{Query: "authentication", Mode: mode})
			if err != nil || len(page.Results) != 1 || page.Results[0].TraceTitle != "Repair authentication" {
				t.Fatalf("%s partial title search: %+v, %v", mode, page, err)
			}
		}
	}
}

func TestImportMergesRevisionsMachinesAndConflicts(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	newer := traceInput("machine-a", "revision-new", "2026-08-22T12:00:00Z", "New title", []domain.Event{
		{Key: "message-1", Kind: "message", Role: "assistant", Timestamp: "2026-08-22T11:00:00Z", Text: "new observation", Sources: []domain.SourceRef{{Path: "source/records.jsonl", Line: 1}}},
	})
	report, err := s.Import(ctx, newer.manifest, newer.files, normalizer(newer.events, "normalized", 1), nil)
	if err != nil || report.Imported != 1 {
		t.Fatalf("first import: report=%+v err=%v", report, err)
	}
	retry := newer
	retry.manifest.SourceMachine.ID = "machine-b"
	retry.manifest.SourceMachine.Hostname = "other"
	report, err = s.Import(ctx, retry.manifest, retry.files, normalizer(retry.events, "normalized", 1), nil)
	if err != nil || report.Unchanged != 1 {
		t.Fatalf("cross-machine retry: report=%+v err=%v", report, err)
	}
	stale := traceInput("machine-c", "revision-old", "2026-08-20T12:00:00Z", "Old title", []domain.Event{
		{Key: "message-1", Kind: "message", Role: "assistant", Timestamp: "2026-08-20T11:00:00Z", Text: "old conflicting observation"},
		{Key: "message-0", Kind: "message", Role: "user", Timestamp: "2026-08-20T10:00:00Z", Text: "event only seen in stale revision"},
	})
	report, err = s.Import(ctx, stale.manifest, stale.files, normalizer(stale.events, "normalized", 1), nil)
	if err != nil || report.Updated != 1 {
		t.Fatalf("stale import: report=%+v err=%v", report, err)
	}
	identical := traceInput("machine-d", "revision-identical", "2026-08-23T12:00:00Z", "New title", newer.events)
	identical.events[0].Sources = []domain.SourceRef{{Path: "source/records.jsonl", Line: 99}}
	report, err = s.Import(ctx, identical.manifest, identical.files, normalizer(identical.events, "normalized", 1), nil)
	if err != nil || report.Updated != 1 {
		t.Fatalf("identical observation revision: report=%+v err=%v", report, err)
	}
	if len(report.Traces[0].Warnings) != 1 || report.Traces[0].Warnings[0].Code != "conflicting_observation" {
		t.Fatalf("true conflict diagnostic missing: %+v", report.Traces[0].Warnings)
	}
	trace, err := s.Trace(ctx, report.Traces[0].TraceID)
	if err != nil {
		t.Fatal(err)
	}
	if trace.Title != "New title" || len(trace.Revisions) != 3 {
		t.Fatalf("stale metadata replaced newer state: %+v", trace)
	}
	page, err := s.Events(ctx, trace.ID, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 2 {
		t.Fatalf("events=%+v", page.Events)
	}
	if page.Events[0].Key != "message-0" || page.Events[1].Key != "message-1" {
		t.Fatalf("events not in native chronology: %+v", page.Events)
	}
	var conflicting store.Event
	for _, event := range page.Events {
		if event.Key == "message-1" {
			conflicting = event
		}
	}
	if conflicting.Text != "new observation" || conflicting.ObservationCount != 2 {
		t.Fatalf("preferred conflict=%+v", conflicting)
	}
	fromEvent, err := s.EventsFrom(ctx, trace.ID, conflicting.ID, 10)
	if err != nil || len(fromEvent.Events) != 1 || fromEvent.Events[0].ID != conflicting.ID {
		t.Fatalf("events from ID=%+v err=%v", fromEvent, err)
	}
	fromKey, err := s.EventsFromKey(ctx, trace.ID, conflicting.Key, 10)
	if err != nil || len(fromKey.Events) != 1 || fromKey.Events[0].ID != conflicting.ID {
		t.Fatalf("events from key=%+v err=%v", fromKey, err)
	}
	sources, err := s.Sources(ctx, conflicting.ID)
	if err != nil || len(sources) != 3 {
		t.Fatalf("sources=%+v err=%v", sources, err)
	}
	foundLine := false
	for _, source := range sources {
		foundLine = foundLine || source.Line == 99
	}
	if !foundLine {
		t.Fatalf("revision-specific source line lost: %+v", sources)
	}
	machines, err := s.Machines(ctx)
	if err != nil || len(machines) != 4 {
		t.Fatalf("machines=%+v err=%v", machines, err)
	}
	storedReport, err := s.ImportReport(ctx, report.ID)
	if err != nil || storedReport.ID != report.ID {
		t.Fatalf("stored report=%+v err=%v", storedReport, err)
	}
	imports, err := s.Imports(ctx, "", 2)
	if err != nil || len(imports.Imports) != 2 || imports.NextCursor == "" {
		t.Fatalf("imports=%+v err=%v", imports, err)
	}
}

func TestSourceRetentionStreamingDeletionAndSharedObjects(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	first := traceInput("machine-a", "revision-a", "2026-08-22T10:00:00Z", "Unsupported", nil)
	report, err := s.Import(ctx, first.manifest, first.files, normalizer(nil, "unsupported", 1), nil)
	if err != nil || report.Unsupported != 1 {
		t.Fatalf("unsupported import: report=%+v err=%v", report, err)
	}
	traceID := report.Traces[0].TraceID
	trace, err := s.Trace(ctx, traceID)
	if err != nil {
		t.Fatal(err)
	}
	objects, err := s.RevisionSources(ctx, trace.Revisions[0].ID)
	if err != nil || len(objects) != 1 || objects[0].Path != "source/records.jsonl" {
		t.Fatalf("objects=%+v err=%v", objects, err)
	}
	reader, err := s.OpenSource(ctx, objects[0].RevisionID, objects[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || string(body) != "shared source record" {
		t.Fatalf("body=%q err=%v", body, err)
	}
	second := traceInput("machine-a", "revision-b", "2026-08-22T11:00:00Z", "Second", []domain.Event{{Key: "e", Kind: "message", Text: "second"}})
	second.manifest.Traces[0].NativeTraceID = "trace-2"
	report2, err := s.Import(ctx, second.manifest, second.files, normalizer(second.events, "normalized", 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	trace2, _ := s.Trace(ctx, report2.Traces[0].TraceID)
	if err = s.DeleteTrace(ctx, traceID); err != nil {
		t.Fatal(err)
	}
	reader, err = s.OpenSource(ctx, trace2.Revisions[0].ID, "source/records.jsonl")
	if err != nil {
		t.Fatalf("shared object was deleted: %v", err)
	}
	reader.Close()
	if _, err = s.Trace(ctx, traceID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted trace error=%v", err)
	}
	if err = s.DeleteTrace(ctx, report2.Traces[0].TraceID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.OpenSource(ctx, trace2.Revisions[0].ID, "source/records.jsonl"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted source error=%v", err)
	}
}

func TestSourceLocationsDoNotCreateObservationConflicts(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	event := domain.Event{Key: "event", Kind: "message", Text: "same semantic event", Sources: []domain.SourceRef{{Path: "source/records.jsonl", Line: 1}}}
	first := traceInput("machine-a", "source-one", "2026-08-22T10:00:00Z", "Sources", []domain.Event{event})
	report, err := s.Import(ctx, first.manifest, first.files, normalizer(first.events, "normalized", 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	second := traceInput("machine-b", "source-two", "2026-08-23T10:00:00Z", "Sources", []domain.Event{event})
	second.events[0].Sources = []domain.SourceRef{{Path: "source/revised.jsonl", Line: 200}}
	second.manifest.Traces[0].Files[0].Path = "source/revised.jsonl"
	second.files["traces/000001/source/revised.jsonl"] = second.files["traces/000001/source/records.jsonl"]
	delete(second.files, "traces/000001/source/records.jsonl")
	report, err = s.Import(ctx, second.manifest, second.files, normalizer(second.events, "normalized", 1), nil)
	if err != nil || len(report.Traces[0].Warnings) != 0 {
		t.Fatalf("source-only change reported conflict: %+v err=%v", report, err)
	}
	page, err := s.Events(ctx, report.Traces[0].TraceID, "", 10)
	if err != nil || len(page.Events) != 1 || page.Events[0].ObservationCount != 1 {
		t.Fatalf("semantic observations=%+v err=%v", page.Events, err)
	}
	sources, err := s.Sources(ctx, page.Events[0].ID)
	if err != nil || len(sources) != 2 || sources[0].Line == sources[1].Line {
		t.Fatalf("source provenance=%+v err=%v", sources, err)
	}
	for _, query := range []store.SearchQuery{
		{Query: `"source/revised.jsonl"`, Mode: "fulltext"},
		{Query: "source/revised.jsonl", Mode: "exact"},
		{Query: `source/revised[.]jsonl`, Mode: "regex"},
		{Query: `(?s).*source/revised[.]jsonl`, Mode: "regex"},
	} {
		query.Machine = "machine-b"
		found, err := s.Search(ctx, query)
		if err != nil || len(found.Results) != 1 || !strings.Contains(found.Results[0].Snippet, "source/revised.jsonl") {
			t.Fatalf("revised source query=%+v page=%+v err=%v", query, found, err)
		}
		query.Machine = "machine-a"
		found, err = s.Search(ctx, query)
		if err != nil || len(found.Results) != 0 {
			t.Fatalf("source matched wrong revision: query=%+v page=%+v err=%v", query, found, err)
		}
	}
}

func TestSearchCandidateKeepsUTF8Boundary(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	text := strings.Repeat("x", 511) + "東京駅"
	input := traceInput("machine", "unicode-boundary", "2026-08-22T10:00:00Z", "Unicode", []domain.Event{{Key: "event", Kind: "message", Text: text}})
	if _, err := s.Import(ctx, input.manifest, input.files, normalizer(input.events, "normalized", 1), nil); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"exact", "regex"} {
		page, err := s.Search(ctx, store.SearchQuery{Query: text, Mode: mode})
		if err != nil || len(page.Results) != 1 || !utf8.ValidString(page.Results[0].Snippet) {
			t.Fatalf("UTF-8 candidate mode=%s page=%+v err=%v", mode, page, err)
		}
	}
}

func TestSearchModesFiltersPaginationAndSnapshot(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	inputs := []struct {
		native string
		event  domain.Event
	}{
		{"one", domain.Event{Key: "one", Kind: "tool_result", Role: "assistant", Model: "model-a", Tool: "read", Timestamp: "2026-08-20T10:00:00Z", Text: "alpha\nbeta path /src/a.go"}},
		{"two", domain.Event{Key: "two", Kind: "message", Role: "user", Model: "model-b", Timestamp: "2026-08-21T10:00:00Z", Text: "quoted phrase and function_name"}},
		{"three", domain.Event{Key: "three", Kind: "message", Role: "assistant", Model: "model-a", Timestamp: "2026-08-22T10:00:00Z", Text: strings.Repeat("x", 17000) + "cross-boundary-token"}},
		{"four", domain.Event{Key: "four", Kind: "message", Role: "assistant", Model: "model-a", Timestamp: "2026-08-22T23:59:59Z", Text: strings.Repeat("padding ", 50) + "東京駅 quoted words"}},
	}
	for i, item := range inputs {
		input := traceInput("machine-a", "search-revision-"+item.native, item.event.Timestamp, item.native, []domain.Event{item.event})
		input.manifest.Traces[0].NativeTraceID = item.native
		input.manifest.Traces[0].Repository.Remote = "github.com/example/repo"
		if _, err := s.Import(ctx, input.manifest, input.files, normalizer(input.events, "normalized", 1), nil); err != nil {
			t.Fatalf("import %d: %v", i, err)
		}
	}
	checks := []store.SearchQuery{
		{Query: `"quoted phrase"`, Mode: "fulltext"},
		{Query: "/src/a.go", Mode: "exact"},
		{Query: `alpha\nbeta`, Mode: "regex"},
		{Query: "cross-boundary-token", Mode: "exact", Model: "model-a", Role: "assistant", Kind: "message", Repository: "github.com/example/repo", After: "2026-08-22T00:00:00Z", Before: "2026-08-23T00:00:00Z", Machine: "machine-a"},
		{Query: strings.Repeat("x", 16500), Mode: "exact"},
		{Query: "東京", Mode: "exact"},
		{Query: "東京駅", Mode: "exact"},
		{Query: `quoted (phrase|nothing)`, Mode: "regex"},
		{Query: `(?:missing )?quoted words`, Mode: "regex"},
	}
	recent, err := s.Search(ctx, store.SearchQuery{Before: "2026-08-20"})
	if err != nil || len(recent.Results) != 1 || recent.Results[0].TraceTitle != "one" {
		t.Fatalf("empty date browse=%+v err=%v", recent, err)
	}
	fullDay, err := s.Search(ctx, store.SearchQuery{Query: "message", Before: "2026-08-22"})
	if err != nil || len(fullDay.Results) != 3 {
		t.Fatalf("inclusive before date=%+v err=%v", fullDay, err)
	}
	regexContext, err := s.Search(ctx, store.SearchQuery{Query: `quoted words`, Mode: "regex"})
	if err != nil || !strings.Contains(regexContext.Results[0].Snippet, "quoted words") || strings.HasPrefix(regexContext.Results[0].Snippet, "padding") {
		t.Fatalf("regex context=%+v err=%v", regexContext, err)
	}
	for _, query := range checks {
		page, err := s.Search(ctx, query)
		if err != nil || len(page.Results) != 1 {
			t.Fatalf("query=%+v results=%+v err=%v", query, page.Results, err)
		}
	}
	page, err := s.Search(ctx, store.SearchQuery{Query: "message", Mode: "fulltext", Limit: 1})
	if err != nil || len(page.Results) != 1 || page.NextCursor == "" {
		t.Fatalf("first page=%+v err=%v", page, err)
	}
	late := traceInput("machine-a", "late", "2026-08-23T10:00:00Z", "late", []domain.Event{{Key: "late", Kind: "message", Text: "message imported later"}})
	late.manifest.Traces[0].NativeTraceID = "late"
	if _, err = s.Import(ctx, late.manifest, late.files, normalizer(late.events, "normalized", 1), nil); err != nil {
		t.Fatal(err)
	}
	second, err := s.Search(ctx, store.SearchQuery{Query: "message", Mode: "fulltext", Limit: 1, Cursor: page.NextCursor})
	if err != nil || len(second.Results) != 1 || second.Results[0].TraceTitle == "late" {
		t.Fatalf("snapshot second page=%+v err=%v", second, err)
	}
	terminal, err := s.Search(ctx, store.SearchQuery{Query: "function_name", Mode: "fulltext", Limit: 1})
	if err != nil || len(terminal.Results) != 1 || terminal.NextCursor != "" {
		t.Fatalf("terminal page=%+v err=%v", terminal, err)
	}
	for _, invalid := range []store.SearchQuery{{Query: "[", Mode: "regex"}, {Query: "x", Cursor: "bad"}, {Query: "x", After: "yesterday"}, {Query: "(", Mode: "fulltext"}, {Query: "foo:bar", Mode: "fulltext"}} {
		if _, err = s.Search(ctx, invalid); !errors.Is(err, store.ErrInvalidQuery) {
			t.Fatalf("invalid query=%+v err=%v", invalid, err)
		}
	}
}

func TestSearchSnippetsBoundLargeMatchesAndKeepUnicodeContext(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	text := strings.Repeat("İ", 400) + " needle " + strings.Repeat("large code ", 2000)
	input := traceInput("machine-a", "large-event", "2026-08-22T10:00:00Z", "Large event", []domain.Event{{Key: "event", Kind: "message", Text: text}})
	if _, err := s.Import(ctx, input.manifest, input.files, normalizer(input.events, "normalized", 1), nil); err != nil {
		t.Fatal(err)
	}
	for _, query := range []store.SearchQuery{
		{Mode: "fulltext", Query: "NEEDLE"},
		{Mode: "exact", Query: "needle"},
		{Mode: "exact", Query: "needle " + strings.Repeat("large code ", 2000)},
		{Mode: "regex", Query: "needle.*"},
	} {
		page, err := s.Search(ctx, query)
		if err != nil || len(page.Results) != 1 {
			t.Fatalf("%s search: results=%d err=%v", query.Mode, len(page.Results), err)
		}
		snippet := page.Results[0].Snippet
		if len(snippet) > 480 || !utf8.ValidString(snippet) || !strings.Contains(snippet, "needle") {
			t.Fatalf("%s snippet: bytes=%d valid_utf8=%t contains_match=%t", query.Mode, len(snippet), utf8.ValidString(snippet), strings.Contains(snippet, "needle"))
		}
	}
}

func TestRenormalizeKeepsOldEventsOnFailureThenReplacesAtomically(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	input := traceInput("machine-a", "revision", "2026-08-22T10:00:00Z", "Rebuild", []domain.Event{{Key: "event", Kind: "message", Text: "old searchable text"}})
	report, err := s.Import(ctx, input.manifest, input.files, normalizer(input.events, "normalized", 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Renormalize(ctx, func(string) int { return 2 }, func(context.Context, domain.Descriptor, fs.FS) (domain.Normalization, error) {
		return domain.Normalization{}, errors.New("broken normalizer")
	})
	if err != nil {
		t.Fatal(err)
	}
	page, err := s.Search(ctx, store.SearchQuery{Query: "old searchable", Mode: "exact"})
	if err != nil || len(page.Results) != 1 {
		t.Fatalf("old events not retained: %+v %v", page, err)
	}
	trace, _ := s.Trace(ctx, report.Traces[0].TraceID)
	if trace.Revisions[0].NormalizerVersion != 1 || len(trace.Revisions[0].Diagnostics) != 1 {
		t.Fatalf("failed rebuild diagnostics=%+v", trace.Revisions[0])
	}
	err = s.Renormalize(ctx, func(string) int { return 2 }, func(context.Context, domain.Descriptor, fs.FS) (domain.Normalization, error) {
		return domain.Normalization{Version: 2, Status: "unsupported", Warnings: []domain.Warning{{Code: "unsupported", Message: "not supported"}}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	page, err = s.Search(ctx, store.SearchQuery{Query: "old searchable", Mode: "exact"})
	if err != nil || len(page.Results) != 1 {
		t.Fatalf("unsupported rebuild replaced old events: %+v %v", page, err)
	}
	err = s.Renormalize(ctx, func(string) int { return 2 }, func(_ context.Context, _ domain.Descriptor, source fs.FS) (domain.Normalization, error) {
		if _, readErr := fs.ReadFile(source, "source/records.jsonl"); readErr != nil {
			return domain.Normalization{}, readErr
		}
		return domain.Normalization{Version: 2, Status: "normalized", Events: []domain.Event{{Key: "event", Kind: "message", Text: "new searchable text"}}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	oldPage, _ := s.Search(ctx, store.SearchQuery{Query: "old searchable", Mode: "exact"})
	newPage, _ := s.Search(ctx, store.SearchQuery{Query: "new searchable", Mode: "exact"})
	if len(oldPage.Results) != 0 || len(newPage.Results) != 1 {
		t.Fatalf("old=%+v new=%+v", oldPage, newPage)
	}
	trace, _ = s.Trace(ctx, report.Traces[0].TraceID)
	if trace.Revisions[0].NormalizerVersion != 2 {
		t.Fatalf("revision=%+v", trace.Revisions[0])
	}
}

func TestUnsuccessfulRevisionRetriesAreNotAcknowledgedAsUnchanged(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	statuses := []string{"failed", "unsupported", "partially_parsed"}
	for i, status := range statuses {
		input := traceInput("machine-a", "retry-"+status, "2026-08-22T10:00:00Z", status, nil)
		input.manifest.Traces[0].NativeTraceID = status
		input.manifest.Traces[0].Warnings = []domain.Warning{{Code: "descriptor_warning", Message: "collected with warning"}}
		normalization := domain.Normalization{Version: 1, Status: status, Warnings: []domain.Warning{{Code: "normalizer_warning", Message: "normalizer warning"}}}
		run := func(context.Context, domain.Descriptor, fs.FS) (domain.Normalization, error) {
			return normalization, nil
		}
		first, err := s.Import(ctx, input.manifest, input.files, run, nil)
		if err != nil || first.Traces[0].TraceID == 0 || len(first.Traces[0].Warnings) != 2 {
			t.Fatalf("first %s retry=%+v err=%v", status, first, err)
		}
		retry, err := s.Import(ctx, input.manifest, input.files, run, nil)
		if err != nil || retry.Unchanged != 0 || retry.Traces[0].Status != status || retry.Traces[0].TraceID != first.Traces[0].TraceID {
			t.Fatalf("retry %d %s=%+v err=%v", i, status, retry, err)
		}
	}
	input := traceInput("machine-a", "retry-unsupported", "2026-08-22T10:00:00Z", "unsupported", []domain.Event{{Key: "event", Kind: "message", Text: "recovered"}})
	input.manifest.Traces[0].NativeTraceID = "unsupported"
	recovered, err := s.Import(ctx, input.manifest, input.files, normalizer(input.events, "normalized", 2), nil)
	if err != nil || recovered.Updated != 1 || recovered.Traces[0].TraceID == 0 {
		t.Fatalf("recovered retry=%+v err=%v", recovered, err)
	}
}

func TestSearchContinuesWhileAnImportNormalizes(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	initial := traceInput("machine-a", "initial", "2026-08-22T10:00:00Z", "Initial", []domain.Event{{Key: "event", Kind: "message", Text: "already searchable"}})
	if _, err := s.Import(ctx, initial.manifest, initial.files, normalizer(initial.events, "normalized", 1), nil); err != nil {
		t.Fatal(err)
	}
	next := traceInput("machine-a", "next", "2026-08-23T10:00:00Z", "Next", []domain.Event{{Key: "next", Kind: "message", Text: "next"}})
	next.manifest.Traces[0].NativeTraceID = "next"
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := s.Import(ctx, next.manifest, next.files, func(context.Context, domain.Descriptor, fs.FS) (domain.Normalization, error) {
			close(started)
			<-release
			return domain.Normalization{Version: 1, Status: "normalized", Events: next.events}, nil
		}, nil)
		done <- err
	}()
	<-started
	page, err := s.Search(ctx, store.SearchQuery{Query: "already searchable", Mode: "exact"})
	close(release)
	if importErr := <-done; importErr != nil {
		t.Fatal(importErr)
	}
	if err != nil || len(page.Results) != 1 {
		t.Fatalf("WAL read during import=%+v err=%v", page, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = s.Search(cancelled, store.SearchQuery{Query: "searchable", Mode: "regex"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled search error=%v", err)
	}
}

func TestCancelledObjectCopyLeavesReadableInterruptedImport(t *testing.T) {
	s := openStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	content := []byte(strings.Repeat("source", 10000))
	sum := sha256.Sum256(content)
	descriptor := domain.Descriptor{Path: "traces/000001", Harness: "synthetic", Adapter: "test", NativeTraceID: "cancelled", RevisionDigest: "cancelled", Files: []domain.File{{Path: "source/records.jsonl", Size: int64(len(content)), SHA256: hex.EncodeToString(sum[:])}}}
	manifest := domain.Manifest{CreatedAt: "2026-08-22T10:00:00Z", SourceMachine: domain.Machine{ID: "machine-a", Hostname: "host", OS: "linux", Arch: "amd64"}, Traces: []domain.Descriptor{descriptor}}
	report, err := s.Import(ctx, manifest, cancellingFS{content: content, cancel: cancel}, normalizer(nil, "normalized", 1), nil)
	if !errors.Is(err, context.Canceled) || report.ID == 0 {
		t.Fatalf("cancelled import=%+v err=%v", report, err)
	}
	interrupted, err := s.ImportReport(context.Background(), report.ID)
	if err != nil || interrupted.ID != report.ID || interrupted.Machine.ID != "machine-a" || interrupted.CreatedAt == "" {
		t.Fatalf("interrupted report=%+v err=%v", interrupted, err)
	}
}

type input struct {
	manifest domain.Manifest
	files    fstest.MapFS
	events   []domain.Event
}

func traceInput(machineID, revision, updated, title string, events []domain.Event) input {
	content := []byte("shared source record")
	sum := sha256.Sum256(content)
	descriptor := domain.Descriptor{
		Path: "traces/000001", Harness: "synthetic", Adapter: "test", NativeTraceID: "trace-1", NativeUpdatedAt: updated,
		RevisionDigest: revision, Title: title, Files: []domain.File{{Path: "source/records.jsonl", Size: int64(len(content)), SHA256: hex.EncodeToString(sum[:])}},
	}
	return input{
		manifest: domain.Manifest{FormatVersion: 1, CreatedAt: updated, SourceMachine: domain.Machine{ID: machineID, Hostname: machineID, OS: "linux", Arch: "amd64"}, Traces: []domain.Descriptor{descriptor}},
		files:    fstest.MapFS{"traces/000001/source/records.jsonl": &fstest.MapFile{Data: content}},
		events:   events,
	}
}

func normalizer(events []domain.Event, status string, version int) func(context.Context, domain.Descriptor, fs.FS) (domain.Normalization, error) {
	return func(context.Context, domain.Descriptor, fs.FS) (domain.Normalization, error) {
		return domain.Normalization{Version: version, Status: status, Events: events}, nil
	}
}

func openStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

type cancellingFS struct {
	content []byte
	cancel  context.CancelFunc
}

func (f cancellingFS) Open(name string) (fs.File, error) {
	if name != "traces/000001/source/records.jsonl" {
		return nil, fs.ErrNotExist
	}
	return &cancellingFile{Reader: strings.NewReader(string(f.content)), size: int64(len(f.content)), cancel: f.cancel}, nil
}

type cancellingFile struct {
	*strings.Reader
	size   int64
	cancel context.CancelFunc
}

func (f *cancellingFile) Read(buffer []byte) (int, error) {
	if len(buffer) > 1 {
		buffer = buffer[:1]
	}
	n, err := f.Reader.Read(buffer)
	f.cancel()
	return n, err
}

func (f *cancellingFile) Close() error { return nil }
func (f *cancellingFile) Stat() (fs.FileInfo, error) {
	return cancellingFileInfo{size: f.size}, nil
}

type cancellingFileInfo struct{ size int64 }

func (f cancellingFileInfo) Name() string       { return "records.jsonl" }
func (f cancellingFileInfo) Size() int64        { return f.size }
func (f cancellingFileInfo) Mode() fs.FileMode  { return 0 }
func (f cancellingFileInfo) ModTime() time.Time { return time.Time{} }
func (f cancellingFileInfo) IsDir() bool        { return false }
func (f cancellingFileInfo) Sys() any           { return nil }
