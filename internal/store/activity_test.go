package store_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/regutierrez/traicr/internal/domain"
	"github.com/regutierrez/traicr/internal/store"
)

func TestActivityCountsEachSessionOnceAndMatchesTheSessionList(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	imports := []struct {
		harness, id, parent, revision string
		events                        []domain.Event
	}{
		// Two revisions record the same Event with different models, and a second
		// Event on another day: one session, under both models.
		{"amp", "a", "", "a-1", []domain.Event{
			{Key: "reply", Kind: "message", Role: "assistant", Model: "opus", Text: "first", Timestamp: "2026-11-03T15:00:00Z"},
		}},
		{"amp", "a", "", "a-2", []domain.Event{
			{Key: "reply", Kind: "message", Role: "assistant", Model: "sonnet", Text: "retried", Timestamp: "2026-11-03T15:00:00Z"},
			{Key: "later", Kind: "message", Role: "assistant", Model: "sonnet", Text: "later", Timestamp: "2026-11-04T15:00:00Z"},
		}},
		// A subagent counts with its parent, under the model it used.
		{"amp", "a-child", "a", "child-1", []domain.Event{
			{Key: "child", Kind: "message", Role: "assistant", Model: "haiku", Text: "subagent", Timestamp: "2026-11-03T16:00:00Z"},
		}},
		// 04:30 UTC on Nov 2 is still Nov 1 in New York, where daylight saving
		// ended that morning; a fixed summer offset would say Nov 2.
		{"codex", "b", "", "b-1", []domain.Event{
			{Key: "prompt", Kind: "message", Role: "user", Text: "late", Timestamp: "2026-11-02T04:30:00Z"},
			{Key: "reply", Kind: "message", Role: "assistant", Model: "opus", Text: "late reply", Timestamp: "2026-11-02T04:31:00Z"},
		}},
		// No model recorded in the range; its model is from before the range.
		{"codex", "c", "", "c-1", []domain.Event{
			{Key: "old", Kind: "message", Role: "assistant", Model: "old-model", Text: "old", Timestamp: "2026-09-01T12:00:00Z"},
			{Key: "prompt", Kind: "message", Role: "user", Text: "recent", Timestamp: "2026-11-04T12:00:00Z"},
		}},
		// A child of an uncollected parent is its own session.
		{"codex", "orphan", "missing", "orphan-1", []domain.Event{
			{Key: "reply", Kind: "message", Role: "assistant", Model: "opus", Text: "orphan", Timestamp: "2026-10-01T12:00:00Z"},
		}},
		// No Events at all: only an unbounded range counts it.
		{"pi", "empty", "", "empty-1", nil},
	}
	for _, trace := range imports {
		input := traceInput("machine", trace.revision, "2026-11-05T00:00:00Z", trace.id, trace.events)
		input.manifest.Traces[0].Harness = trace.harness
		input.manifest.Traces[0].NativeTraceID = trace.id
		input.manifest.Traces[0].ParentNativeTraceID = trace.parent
		if _, err := s.Import(ctx, input.manifest, input.files, normalizer(trace.events, "normalized", 1), nil); err != nil {
			t.Fatal(err)
		}
	}
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}

	recent, err := s.Activity(ctx, "2026-11-01", "", newYork)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(recent.Sessions, recent.Harnesses, recent.Models, recent.WithoutModel, recent.Days); got !=
		"3 [{codex 2} {amp 1}] [{opus 2} {haiku 1} {sonnet 1}] 1 [{2026-11-01 1} {2026-11-03 1} {2026-11-04 2}]" {
		t.Fatalf("recent activity = %s", got)
	}
	all, err := s.Activity(ctx, "", "", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(all.Sessions, all.Harnesses, all.Models, all.WithoutModel); got !=
		"5 [{codex 3} {amp 1} {pi 1}] [{opus 3} {haiku 1} {old-model 1} {sonnet 1}] 1" {
		t.Fatalf("all activity = %s", got)
	}

	// Each row filters the session list to exactly the sessions it counts.
	for _, check := range []struct {
		activity store.Activity
		after    string
	}{{recent, "2026-11-01"}, {all, ""}} {
		for _, row := range check.activity.Harnesses {
			page, err := s.TranscriptCards(ctx, store.SearchQuery{Harness: row.Name, After: check.after, Limit: 200})
			if err != nil || int64(len(page.Cards)) != row.Sessions {
				t.Errorf("after %q harness %s: list has %d cards, activity %d (%v)", check.after, row.Name, len(page.Cards), row.Sessions, err)
			}
		}
		for _, row := range check.activity.Models {
			page, err := s.TranscriptCards(ctx, store.SearchQuery{Model: row.Name, After: check.after, Limit: 200})
			if err != nil || int64(len(page.Cards)) != row.Sessions {
				t.Errorf("after %q model %s: list has %d cards, activity %d (%v)", check.after, row.Name, len(page.Cards), row.Sessions, err)
			}
		}
	}

	if _, err := s.Activity(ctx, "yesterday", "", time.UTC); !errors.Is(err, store.ErrInvalidQuery) {
		t.Fatalf("invalid date: %v", err)
	}
}
