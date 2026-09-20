package normalize

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/regutierrez/traicr/internal/amp"
	"github.com/regutierrez/traicr/internal/domain"
)

func TestAmpMissingHostedImagesProduceRebuildableDiagnostics(t *testing.T) {
	const data = `{"messages":[{"id":1,"role":"user","content":[
 {"type":"image","url":"https://ampcode.com/user-content/attachments/missing.png"},
 {"type":"image","url":"https://ampcode.com/user-content/attachments/missing.png#width=20"},
 {"type":"image","url":"https://example.com/external.png"},
 {"type":"image","source":{"type":"base64","media_type":"image/png","data":"aGVsbG8="}}
 ]}]}`
	_, path := amp.AttachmentLocation("https://ampcode.com/user-content/attachments/missing.png")
	for _, archived := range []bool{false, true} {
		descriptor := domain.Descriptor{Harness: "amp"}
		if archived {
			descriptor.Files = []domain.File{{Path: path, Size: 12}}
		}
		result, err := Run(context.Background(), descriptor, fstest.MapFS{"source/export.json": {Data: []byte(data)}})
		if err != nil {
			t.Fatal(err)
		}
		if archived {
			if len(result.Warnings) != 0 || result.Status != "normalized" {
				t.Fatalf("archived images still reported missing: %+v", result)
			}
		} else if len(result.Warnings) != 1 || result.Warnings[0].Code != "amp_image_unavailable" || !strings.Contains(result.Warnings[0].Message, "/messages/0/content/0") || result.Status != "partially_parsed" {
			t.Fatalf("missing hosted image diagnostic: %+v", result.Warnings)
		}
	}
}

func TestAmpAcceptsExportsAboveOldSourceLimit(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "source"), 0700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(filepath.Join(root, "source/export.json"))
	if err != nil {
		t.Fatal(err)
	}
	padding := strings.Repeat(" ", 1<<20)
	for range 65 {
		if _, err := file.WriteString(padding); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := file.WriteString(`{"messages":[{"messageId":1,"role":"user","content":[{"type":"text","text":"beyond 64 MiB"}]}]}`); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), domain.Descriptor{Harness: "amp"}, os.DirFS(root))
	if err != nil || len(result.Events) != 1 || result.Events[0].Text != "beyond 64 MiB" {
		t.Fatalf("large Amp export: %+v %v", result, err)
	}
}

func TestAmpNumericMessageIdentitySurvivesReadAndContentChanges(t *testing.T) {
	for _, id := range []string{"0", "17", `"17"`} {
		var previous domain.Event
		for _, text := range []string{"draft", "final"} {
			data := `{"messages":[{"messageId":` + id + `,"parentId":0,"readAt":"` + text + `","role":"assistant","content":[{"type":"text","text":"` + text + `"}]}]}`
			result, err := Run(context.Background(), domain.Descriptor{Harness: "amp"}, fstest.MapFS{"source/export.json": {Data: []byte(data)}})
			if err != nil || len(result.Events) != 1 {
				t.Fatalf("normalize: %+v %v", result, err)
			}
			event := result.Events[0]
			if strings.Contains(event.Key, "sha256:") || previous.Key != "" && previous.Key != event.Key {
				t.Fatalf("native identity lost: %q -> %q", previous.Key, event.Key)
			}
			if event.ParentKey != "message:0" || event.Text != text {
				t.Fatalf("native parent or updated content lost: %+v", event)
			}
			previous = event
		}
	}
}

func TestAmpPreservesResultShapesAndExecutionDetails(t *testing.T) {
	for _, test := range []struct{ run, text string }{
		{`{"status":"done","result":"Oracle answer"}`, "Oracle answer"},
		{`{"status":"done","result":[{"type":"text","text":"First"},{"type":"text","text":"Second"}]}`, "First\nSecond"},
		{`{"status":"done","result":{"content":[{"type":"text","text":"Skill instructions"}]}}`, "Skill instructions"},
		{`{"status":"error","error":{"message":"Permission denied"}}`, "Permission denied"},
		{`{"status":"done","result":{"output":"Failure output","exitCode":2}}`, "Failure output"},
		{`{"status":"done","result":{"runners":[{"name":"runner-a"}]}}`, `{"runners":[{"name":"runner-a"}]}`},
	} {
		data := `{"messages":[{"messageId":3,"role":"user","content":[{"type":"tool_result","toolUseID":"TU-1","run":` + test.run + `}]}]}`
		result, err := Run(context.Background(), domain.Descriptor{Harness: "amp"}, fstest.MapFS{"source/export.json": {Data: []byte(data)}})
		if err != nil || len(result.Events) != 1 || result.Events[0].Text != test.text {
			t.Fatalf("run=%s: %+v %v", test.run, result, err)
		}
		var metadata map[string]any
		if err := json.Unmarshal(result.Events[0].Metadata, &metadata); err != nil {
			t.Fatal(err)
		}
		if metadata["run"] == nil {
			t.Fatalf("execution details discarded: %s", result.Events[0].Metadata)
		}
	}
}

func TestAmpRetainsTranscriptDetailsAndUnknownContent(t *testing.T) {
	result, err := Run(context.Background(), domain.Descriptor{Harness: "amp"}, exportFS(ampRichExport))
	if err != nil {
		t.Fatal(err)
	}
	events := map[string]domain.Event{}
	for _, event := range result.Events {
		events[event.Key] = event
	}
	if events["compaction:5:0"].Text != "The build fails because the upper bound is exclusive. Preserve the boundary case." {
		t.Fatal("summary disappeared")
	}
	image := events["attachment:0:1"].Attachments
	if len(image) != 1 || image[0].URL != "https://ampcode.com/attachments/fixture.png" || image[0].Path != "screenshot.png" {
		t.Fatalf("image source lost: %+v", image)
	}
	if event := events["reasoning:1:0"]; event.Timestamp != "2026-09-16T10:00:01Z" || event.Provider != "example-provider" {
		t.Fatalf("timing/provider lost: %+v", event)
	}
	var meta map[string]any
	if err := json.Unmarshal(events["reasoning:1:0"].Metadata, &meta); err != nil {
		t.Fatal(err)
	}
	if object(meta["usage"])["outputTokens"] != float64(7) || object(meta["message_meta"])["openAIResponsePhase"] != "commentary" || meta["start_time"] != "2026-09-16T10:00:01Z" {
		t.Fatalf("message details lost: %+v", meta)
	}
	if !strings.Contains(events["unknown:15:0"].Text, "Unknown content stays inspectable") || !strings.Contains(string(events["unknown:15:0"].Metadata), "/messages/15/content/0") {
		t.Fatal("unknown block and source position lost")
	}
	if !strings.Contains(string(events["session_info:T-amp-rich-fixture"].Metadata), "fixture-commit") {
		t.Fatal("exported repository details lost")
	}
	data, _ := json.Marshal(result)
	if strings.Contains(string(data), "PRIVATE_") {
		t.Fatal("binary payload or reasoning signature escaped source storage")
	}
}

func TestAmpSourcePointersIncludeLegacyTextAndToolPreviews(t *testing.T) {
	data := `{"messages":[{"messageId":1,"role":"user","content":"plain prompt"},{"messageId":2,"role":"user","content":[{"type":"tool_result","toolUseID":"TU-image","run":{"status":"done","progress":{"displayImages":[{"type":"image","url":"https://example.com/preview.png","mimeType":"image/png"}]}}}]}]}`
	result, err := Run(context.Background(), domain.Descriptor{Harness: "amp"}, fstest.MapFS{"source/export.json": {Data: []byte(data)}})
	if err != nil || len(result.Events) != 2 {
		t.Fatalf("normalize: %+v %v", result, err)
	}
	var details map[string]any
	if err := json.Unmarshal(result.Events[0].Metadata, &details); err != nil {
		t.Fatal(err)
	}
	if details["source_pointer"] != "/messages/0/content" {
		t.Fatalf("legacy content pointer: %s", result.Events[0].Metadata)
	}
	images := result.Events[1].Attachments
	if len(images) != 1 || images[0].URL != "https://example.com/preview.png" || images[0].SourcePointer != "/messages/1/content/0/run/progress/displayImages/0" {
		t.Fatalf("tool preview lost: %+v", images)
	}
}

func TestAmpPairsNativeToolIDsBeforeProviderIDs(t *testing.T) {
	source := fstest.MapFS{"source/export.json": {Data: []byte(`{"messages":[
 {"id":"m1","role":"assistant","content":[{"type":"tool_use","id":"TU-native","providerToolUseId":"call-provider","name":"bash","input":{"command":"pwd"}}]},
 {"id":"m2","role":"user","content":[{"type":"tool_result","toolUseID":"TU-native","content":"/repo"}]}
 ]}`)}}
	result, err := Run(context.Background(), domain.Descriptor{Harness: "amp"}, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 2 || result.Events[0].CallID != "TU-native" || result.Events[1].CallID != result.Events[0].CallID {
		t.Fatalf("unpaired Amp tool result: %+v", result.Events)
	}
	if result.Version != 4 {
		t.Fatalf("Amp normalizer must rebuild stored ID mappings: version %d", result.Version)
	}
}

func TestAmpPreservesNativeMessageOrderWithoutIDsOrTimestamps(t *testing.T) {
	source := fstest.MapFS{"source/export.json": {Data: []byte(`{"messages":[
 {"role":"user","meta":{"sentAt":1787140168649},"content":[{"type":"text","text":"First prompt"}]},
 {"role":"assistant","content":[{"type":"thinking","thinking":"Reasoning"},{"type":"text","text":"Response"}]}
 ]}`)}}
	result, err := Run(context.Background(), domain.Descriptor{Harness: "amp"}, source)
	if err != nil || len(result.Events) != 3 {
		t.Fatalf("Amp normalization: %+v %v", result, err)
	}
	var previous string
	for i, event := range result.Events {
		var meta struct {
			Message string `json:"transcript_message"`
			Order   int    `json:"transcript_order"`
			Block   int    `json:"transcript_block"`
		}
		if err := json.Unmarshal(event.Metadata, &meta); err != nil {
			t.Fatal(err)
		}
		if meta.Message == "" || (i == 0 && meta.Order != 0) || (i > 0 && meta.Order != 1) {
			t.Fatalf("native order lost: %+v", meta)
		}
		if i == 2 && (meta.Message != previous || meta.Block != 1) {
			t.Fatalf("assistant content blocks split: %+v", meta)
		}
		previous = meta.Message
	}
}
