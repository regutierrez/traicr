package store_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/regutierrez/traicr/internal/domain"
	"github.com/regutierrez/traicr/internal/store"
)

func TestTranscriptCursorRejectsRebuiltHistory(t *testing.T) {
	database := openStore(t)
	ctx := context.Background()
	var old, updated []domain.Event
	for i := range 205 {
		key := fmt.Sprintf("message:legacy-%03d", i)
		old = append(old, domain.Event{Key: key, Kind: "message", Text: fmt.Sprint(i)})
		updated = append(updated, domain.Event{Key: fmt.Sprintf("message:%d:0", i), Kind: "message", Text: fmt.Sprint(i), LegacyKeys: []string{key}, Metadata: []byte(fmt.Sprintf(`{"transcript_order":%d}`, i))})
	}
	input := traceInput("machine", "legacy", "2026-09-16T10:00:00Z", "History", old)
	report, err := database.Import(ctx, input.manifest, input.files, normalizer(old, "normalized", 2), nil)
	if err != nil {
		t.Fatal(err)
	}
	traceID := report.Traces[0].TraceID
	first, err := database.TranscriptEvents(ctx, traceID, 0, "", 200)
	if err != nil || len(first.Events) != 200 || first.NextCursor == "" {
		t.Fatalf("first page: %d events, %v", len(first.Events), err)
	}
	if err := database.Renormalize(ctx, func(string) int { return 3 }, normalizer(updated, "normalized", 3)); err != nil {
		t.Fatal(err)
	}
	if _, err := database.TranscriptEvents(ctx, traceID, 0, first.NextCursor, 200); !errors.Is(err, store.ErrTranscriptChanged) {
		t.Fatalf("stale cursor did not request reload: %v", err)
	}
	first, err = database.TranscriptEvents(ctx, traceID, 0, "", 200)
	if err != nil || len(first.Events) != 200 || first.Events[0].Text != "0" {
		t.Fatalf("reloaded first page: %d events, %v", len(first.Events), err)
	}
	last, err := database.TranscriptEvents(ctx, traceID, 0, first.NextCursor, 200)
	if err != nil || len(last.Events) != 5 || last.Events[4].Text != "204" || last.NextCursor != "" {
		t.Fatalf("reloaded last page: %+v %v", last, err)
	}
}

func TestMergedTranscriptPrefersArchivedImagesOnTimestampTies(t *testing.T) {
	for _, order := range [][]string{{"links", "images"}, {"images", "links"}} {
		t.Run(order[0], func(t *testing.T) {
			database := openStore(t)
			ctx := context.Background()
			link := domain.Attachment{URL: "https://ampcode.com/user-content/attachments/fixture.png", SourcePointer: "/messages/0/content/0"}
			image := link
			image.ArchivedPath = "source/attachments/582a1f5b341842222df56057a4c2851406c193ab079e6c4f1237d3518fc46e9c"
			versions := map[string][]domain.Event{
				"links":  {{Key: "attachment:1:0", Kind: "attachment", Attachments: []domain.Attachment{link}}},
				"images": {{Key: "attachment:1:0", Kind: "attachment", Attachments: []domain.Attachment{image}}},
			}
			var traceID, linksRevision int64
			for _, revision := range order {
				input := traceInput("machine", revision, "2026-09-16T10:00:00Z", "Images", versions[revision])
				input.manifest.Traces[0].Harness = "amp"
				report, err := database.Import(ctx, input.manifest, input.files, normalizer(input.events, "normalized", 3), nil)
				if err != nil {
					t.Fatal(err)
				}
				traceID = report.Traces[0].TraceID
				if revision == "links" {
					trace, _ := database.Trace(ctx, traceID)
					for _, item := range trace.Revisions {
						if item.Digest == "links" {
							linksRevision = item.ID
						}
					}
				}
			}
			for _, rebuild := range []bool{false, true} {
				if rebuild {
					err := database.Renormalize(ctx, func(string) int { return 4 }, func(_ context.Context, descriptor domain.Descriptor, _ fs.FS) (domain.Normalization, error) {
						return domain.Normalization{Version: 4, Status: "normalized", Events: versions[descriptor.RevisionDigest]}, nil
					})
					if err != nil {
						t.Fatal(err)
					}
				}
				page, err := database.TranscriptEvents(ctx, traceID, 0, "", 10)
				if err != nil || len(page.Events) != 1 || page.Events[0].Attachments[0].ArchivedPath != image.ArchivedPath {
					t.Fatalf("archived image lost after rebuild=%v: %+v %v", rebuild, page, err)
				}
			}
			selected, err := database.TranscriptEvents(ctx, traceID, linksRevision, "", 10)
			if err != nil || selected.Events[0].Attachments[0].ArchivedPath != "" {
				t.Fatalf("selected revision changed: %+v %v", selected, err)
			}
			input := traceInput("machine", "changed", "2026-09-16T11:00:00Z", "Images", versions["links"])
			input.manifest.Traces[0].Harness = "amp"
			if _, err := database.Import(ctx, input.manifest, input.files, normalizer(input.events, "normalized", 4), nil); err != nil {
				t.Fatal(err)
			}
			page, err := database.TranscriptEvents(ctx, traceID, 0, "", 10)
			if err != nil || page.Events[0].Attachments[0].ArchivedPath != "" {
				t.Fatalf("image preference overrode newer native timestamp: %+v %v", page, err)
			}
		})
	}
}

func TestTranscriptPagesPreserveNativeOrderAndSelectedRevision(t *testing.T) {
	database := openStore(t)
	ctx := context.Background()
	events := []domain.Event{
		{Key: "message:0:0", Kind: "message", Text: "first", Timestamp: "2026-09-16T10:00:00Z", Metadata: []byte(`{"transcript_order":0,"transcript_block":0}`)},
		{Key: "message:1:0", Kind: "message", Text: "old answer", Metadata: []byte(`{"transcript_order":1,"transcript_block":0}`)},
	}
	input := traceInput("machine", "older", "2026-09-16T10:00:00Z", "History", events)
	report, err := database.Import(ctx, input.manifest, input.files, normalizer(events, "normalized", 3), nil)
	if err != nil {
		t.Fatal(err)
	}
	traceID := report.Traces[0].TraceID
	trace, _ := database.Trace(ctx, traceID)
	oldRevision := trace.Revisions[0].ID
	events[1].Text = "new answer"
	input = traceInput("machine", "newer", "2026-09-16T11:00:00Z", "History", events)
	if _, err := database.Import(ctx, input.manifest, input.files, normalizer(events, "normalized", 3), nil); err != nil {
		t.Fatal(err)
	}
	first, err := database.TranscriptEvents(ctx, traceID, 0, "", 1)
	if err != nil || len(first.Events) != 1 || first.Events[0].Text != "first" || first.NextCursor == "" {
		t.Fatalf("first page: %+v %v", first, err)
	}
	second, err := database.TranscriptEvents(ctx, traceID, 0, first.NextCursor, 1)
	if err != nil || len(second.Events) != 1 || second.Events[0].Text != "new answer" || second.NextCursor != "" {
		t.Fatalf("second page: %+v %v", second, err)
	}
	old, err := database.TranscriptEvents(ctx, traceID, oldRevision, "", 10)
	if err != nil || len(old.Events) != 2 || old.Events[1].Text != "old answer" || old.Events[1].RevisionID != oldRevision {
		t.Fatalf("selected revision: %+v %v", old, err)
	}
	if _, err := database.TranscriptEvents(ctx, traceID, oldRevision, first.NextCursor, 10); err == nil {
		t.Fatal("accepted a cursor for a different revision")
	}
	if _, err := database.TranscriptEvents(ctx, traceID, 98765, "", 10); err == nil {
		t.Fatal("accepted a revision outside this trace")
	}
}
