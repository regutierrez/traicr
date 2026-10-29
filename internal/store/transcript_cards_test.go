package store_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/regutierrez/traicr/internal/domain"
	"github.com/regutierrez/traicr/internal/store"
)

func TestTranscriptCardsDeduplicateMatchesAndPageBySession(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	for i := 0; i < 3; i++ {
		events := []domain.Event{}
		if i < 2 {
			events = []domain.Event{
				{Key: "message:user", Kind: "message", Role: "user", Text: "Build a transcript viewer", Timestamp: "2026-09-06T10:00:00Z"},
				{Key: "call:1", Kind: "tool_call", Role: "assistant", Tool: "bash", Text: "needle first", Timestamp: "2026-09-06T10:01:00Z"},
				{Key: "call:2", Kind: "tool_call", Role: "assistant", Tool: "bash", Text: "needle second", Timestamp: "2026-09-06T10:02:00Z"},
			}
		}
		input := traceInput("machine", fmt.Sprint(i), fmt.Sprintf("2026-09-0%dT12:00:00Z", i+1), fmt.Sprintf("Session %d", i), events)
		input.manifest.Traces[0].NativeTraceID = fmt.Sprint(i)
		if _, err := s.Import(ctx, input.manifest, input.files, normalizer(events, "normalized", 1), nil); err != nil {
			t.Fatal(err)
		}
	}
	browse, err := s.TranscriptCards(ctx, store.SearchQuery{})
	if err != nil || len(browse.Cards) != 3 {
		t.Fatalf("browse: %+v, %v", browse, err)
	}
	if browse.Cards[0].NativeTraceID != "2" || browse.Cards[0].EventCount != 0 {
		t.Fatalf("missing empty transcript or wrong recency: %+v", browse.Cards)
	}
	if browse.Cards[1].Preview != "Build a transcript viewer" || browse.Cards[1].EventCount != 3 || browse.Cards[1].RevisionCount != 1 {
		t.Fatalf("session metadata: %+v", browse.Cards[1])
	}
	for _, mode := range []string{"fulltext", "exact", "regex"} {
		query := store.SearchQuery{Query: "needle", Mode: mode, Limit: 1}
		first, err := s.TranscriptCards(ctx, query)
		if err != nil || len(first.Cards) != 1 || first.NextCursor == "" {
			t.Fatalf("%s first: %+v %v", mode, first, err)
		}
		query.Cursor = first.NextCursor
		second, err := s.TranscriptCards(ctx, query)
		if err != nil || len(second.Cards) != 1 || second.NextCursor != "" || second.Cards[0].ID == first.Cards[0].ID {
			t.Fatalf("%s second: %+v %v", mode, second, err)
		}
		query.Cursor = ""
		query.Role = "user"
		filtered, err := s.TranscriptCards(ctx, query)
		if err != nil || len(filtered.Cards) != 0 {
			t.Fatalf("cross-event filter leak: %+v %v", filtered, err)
		}
	}
	if _, err := s.TranscriptCards(ctx, store.SearchQuery{Cursor: "invalid"}); !errors.Is(err, store.ErrInvalidQuery) {
		t.Fatalf("invalid cursor: %v", err)
	}
	if _, err := s.TranscriptCards(ctx, store.SearchQuery{Query: "[", Mode: "regex"}); !errors.Is(err, store.ErrInvalidQuery) {
		t.Fatalf("invalid regex: %v", err)
	}
}

func TestTranscriptCardsListChildTracesUnderTheirTopAncestor(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	// Imported newest first so a child's own updated_at would sort it above its parent.
	for i, trace := range []struct{ id, parent, title, text string }{
		{"root", "", "Plan the release", "write the changelog"},
		{"child", "root", "Review subagent", "check the migration order"},
		{"grandchild", "child", "", "nested needle"},
		{"orphan", "never-collected", "Lost subagent", "orphan needle"},
		{"other", "", "Unrelated", "nothing here"},
	} {
		events := []domain.Event{{Key: "message:" + trace.id, Kind: "message", Role: "user", Text: trace.text, Timestamp: "2026-09-06T10:00:00Z"}}
		input := traceInput("machine", trace.id, fmt.Sprintf("2026-09-0%dT12:00:00Z", 9-i), trace.title, events)
		input.manifest.Traces[0].NativeTraceID = trace.id
		input.manifest.Traces[0].ParentNativeTraceID = trace.parent
		if _, err := s.Import(ctx, input.manifest, input.files, normalizer(events, "normalized", 1), nil); err != nil {
			t.Fatal(err)
		}
	}
	titles := func(page store.TranscriptPage) []string {
		var found []string
		for _, card := range page.Cards {
			found = append(found, card.NativeTraceID+">"+card.MatchedChildTitle)
		}
		return found
	}
	browse, err := s.TranscriptCards(ctx, store.SearchQuery{})
	if got := fmt.Sprint(titles(browse)); err != nil || got != "[root> orphan> other>]" {
		t.Fatalf("browse = %s, %v; want top-level traces and the orphan", got, err)
	}
	for query, want := range map[string]string{
		"migration": "[root>Review subagent]", // a child match shows the parent card
		"nested":    "[root>]",                // grandchild has no title of any kind
		"changelog": "[root>]",                // a match in the parent itself names no child
		"orphan":    "[orphan>]",              // an uncollected parent leaves the child its own card
	} {
		page, err := s.TranscriptCards(ctx, store.SearchQuery{Query: query})
		if got := fmt.Sprint(titles(page)); err != nil || got != want {
			t.Errorf("search %q = %s, %v; want %s", query, got, err, want)
		}
	}
	page, err := s.TranscriptCards(ctx, store.SearchQuery{Query: "needle"})
	if err != nil || len(page.Cards) != 2 || page.Cards[0].NativeTraceID != "root" || page.Cards[0].MatchedChildID == 0 {
		t.Fatalf("grandchild match = %+v, %v; want the root card pointing at the grandchild", page.Cards, err)
	}
}
