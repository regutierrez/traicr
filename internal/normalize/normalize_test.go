package normalize

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/regutierrez/traicr/internal/domain"
)

func TestRunSanitizedFixtures(t *testing.T) {
	tests := []struct {
		name        string
		harness     string
		adapter     string
		wantKind    string
		wantCallID  string
		wantModel   string
		minimumSize int
	}{
		{"pi", "pi", "pi-jsonl", "compaction", "call-1", "example-model", 7},
		{"claude", "claude-code", "claude-code-jsonl", "reasoning", "tool-1", "example-model", 5},
		{"cursor-agent", "cursor-agent", "cursor-agent-jsonl", "tool_call", "call-1", "example-model", 3},
		{"cursor", "cursor", "cursor-sqlite-rows", "message", "", "example-model", 2},
		{"amp", "amp", "amp-thread-export", "tool_result", "call-1", "example-model", 4},
		{"opencode", "opencode", "opencode-export", "tool_result", "call-1", "example-model", 3},
		{"codex", "codex", "codex-app-server", "reasoning", "item-tool", "", 4},
		{"codex-rollout", "codex", "codex-rollout-jsonl", "model_change", "call-1", "example-model", 4},
		{"grok", "grok-build", "grok-native-session", "tool_result", "call-1", "", 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, err := findFixtureRoot()
			if err != nil {
				t.Fatal(err)
			}
			result, err := Run(context.Background(), domain.Descriptor{Harness: test.harness, Adapter: test.adapter}, os.DirFS(root+"/"+fixtureDirectory(test.name)))
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != "normalized" {
				t.Fatalf("status = %q, warnings = %#v", result.Status, result.Warnings)
			}
			if len(result.Events) < test.minimumSize {
				t.Fatalf("got %d events, want at least %d", len(result.Events), test.minimumSize)
			}
			assertFields(t, result.Events, test.wantKind, test.wantCallID, test.wantModel)
			for _, event := range result.Events {
				if event.Key == "" || len(event.Sources) == 0 {
					t.Fatalf("event lacks identity or source: %#v", event)
				}
			}
		})
	}
}

func TestAttachmentIndexesMetadataWithoutPayload(t *testing.T) {
	root, err := findFixtureRoot()
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), domain.Descriptor{Harness: "claude-code", Adapter: "claude-code-jsonl"}, os.DirFS(root+"/claude-code"))
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range result.Events {
		if event.Kind != "attachment" {
			continue
		}
		if event.Text != "" || len(event.Attachments) != 1 || event.Attachments[0].MediaType != "image/png" {
			t.Fatalf("unexpected attachment event: %#v", event)
		}
		return
	}
	t.Fatal("attachment event not found")
}

func TestRunLinksChildrenToFirstEmittedParentEvent(t *testing.T) {
	tests := []struct {
		name      string
		harness   string
		adapter   string
		data      string
		wantLinks map[string]string
	}{
		{
			name:    "Pi",
			harness: "pi",
			adapter: "pi-jsonl",
			data: strings.Join([]string{
				`{"type":"session","version":3,"id":"fixture"}`,
				`{"type":"message","id":"image-parent","parentId":null,"message":{"role":"user","content":[{"type":"image","name":"fixture.png","mimeType":"image/png"},{"type":"text","text":"after image"}]}}`,
				`{"type":"message","id":"image-child","parentId":"image-parent","message":{"role":"assistant","content":"image child"}}`,
				`{"type":"message","id":"unknown-parent","parentId":null,"message":{"role":"user","content":[{"type":"future"},{"type":"text","text":"after unknown"}]}}`,
				`{"type":"message","id":"unknown-child","parentId":"unknown-parent","message":{"role":"assistant","content":"unknown child"}}`,
				`{"type":"future","id":"empty-parent","parentId":null}`,
				`{"type":"message","id":"empty-child","parentId":"empty-parent","message":{"role":"assistant","content":"empty child"}}`,
			}, "\n") + "\n",
			wantLinks: map[string]string{
				"message:image-child":   "attachment:image-parent:0",
				"message:unknown-child": "message:unknown-parent:1",
				"message:empty-child":   "",
			},
		},
		{
			name:    "Claude",
			harness: "claude-code",
			adapter: "claude-code-jsonl",
			data: strings.Join([]string{
				`{"uuid":"image-parent","message":{"role":"user","content":[{"type":"image","name":"fixture.png","source":{"media_type":"image/png"}},{"type":"text","text":"after image"}]}}`,
				`{"uuid":"image-child","parentUuid":"image-parent","message":{"role":"assistant","content":"image child"}}`,
				`{"uuid":"unknown-parent","message":{"role":"user","content":[{"type":"future"},{"type":"text","text":"after unknown"}]}}`,
				`{"uuid":"unknown-child","parentUuid":"unknown-parent","message":{"role":"assistant","content":"unknown child"}}`,
				`{"uuid":"empty-parent","message":{"role":"user","content":[{"type":"future"}]}}`,
				`{"uuid":"empty-child","parentUuid":"empty-parent","message":{"role":"assistant","content":"empty child"}}`,
			}, "\n") + "\n",
			wantLinks: map[string]string{
				"message:image-child:0":   "attachment:image-parent:0",
				"message:unknown-child:0": "message:unknown-parent:1",
				"message:empty-child:0":   "",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := "source/records.jsonl"
			result, err := Run(context.Background(), domain.Descriptor{Harness: test.harness, Adapter: test.adapter}, fstest.MapFS{path: {Data: []byte(test.data)}})
			if err != nil {
				t.Fatal(err)
			}
			events := make(map[string]domain.Event)
			for _, event := range result.Events {
				events[event.Key] = event
			}
			for child, parent := range test.wantLinks {
				event, exists := events[child]
				if !exists || event.ParentKey != parent {
					t.Fatalf("%s exists=%t parent=%q, want %q", child, exists, event.ParentKey, parent)
				}
			}
		})
	}
}

func TestRunRetainsLargeToolTextAndFiltersEmbeddedPayload(t *testing.T) {
	code := strings.Repeat("const fixture = 1;\n", 20_000)
	payload := strings.Repeat("PRIVATEPAYLOAD", 2_000)
	export := map[string]any{"messages": []any{map[string]any{
		"messageId": "message-1",
		"role":      "assistant",
		"content": []any{map[string]any{
			"type":      "tool_use",
			"toolUseID": "call-1",
			"name":      "write",
			"input": map[string]any{
				"code":       code,
				"attachment": map[string]any{"type": "base64", "media_type": "image/png", "data": payload},
			},
		}},
	}}}
	data, err := json.Marshal(export)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), domain.Descriptor{Harness: "amp", Adapter: "amp-thread-export"}, fstest.MapFS{"source/export.json": {Data: data}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 1 {
		t.Fatalf("events = %d, want 1", len(result.Events))
	}
	text := result.Events[0].Text
	var indexed map[string]any
	if err := json.Unmarshal([]byte(text), &indexed); err != nil {
		t.Fatal(err)
	}
	attachment := object(indexed["attachment"])
	if stringValue(indexed["code"]) != code || stringValue(attachment["media_type"]) != "image/png" {
		t.Fatal("large tool code or attachment metadata was dropped")
	}
	if attachment["data"] != nil || strings.Contains(text, "PRIVATEPAYLOAD") {
		t.Fatal("embedded attachment payload was indexed")
	}
}

func TestRunKeepsAllHumanTextFields(t *testing.T) {
	data := []byte(`{"thread":{"turns":[{"id":"turn-1","items":[{"id":"result-1","type":"functionCallOutput","callId":"call-1","output":{"text":"first field","summary":"second field"}}]}]}}`)
	result, err := Run(context.Background(), domain.Descriptor{Harness: "codex", Adapter: "codex-app-server"}, fstest.MapFS{"source/thread.json": {Data: data}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 1 || result.Events[0].Text != "first field\nsecond field" {
		t.Fatalf("human text fields were lost: %#v", result.Events)
	}
}

func TestPiUsesParentGraphOrder(t *testing.T) {
	root, err := findFixtureRoot()
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), domain.Descriptor{Harness: "pi", Adapter: "pi-jsonl"}, os.DirFS(root+"/pi-v3"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Events[0].Text != "Read the fixture." {
		t.Fatalf("first event text = %q", result.Events[0].Text)
	}
	if result.Events[1].ParentKey != result.Events[0].Key {
		t.Fatalf("child parent key = %q, want %q", result.Events[1].ParentKey, result.Events[0].Key)
	}
}

func TestPiPropagatesBranchModelAndLinksArrayToolResult(t *testing.T) {
	data := strings.Join([]string{
		`{"type":"session","version":3,"id":"fixture"}`,
		`{"type":"message","id":"child","parentId":"result","message":{"role":"user","content":"continue"}}`,
		`{"type":"message","id":"branch","parentId":"model","message":{"role":"assistant","content":"branch response"}}`,
		`{"type":"message","id":"result","parentId":"call","message":{"role":"toolResult","toolCallId":"tool-1","toolName":"read","content":[{"type":"text","text":"fixture result"}]}}`,
		`{"type":"message","id":"call","parentId":"model","message":{"role":"assistant","content":[{"type":"toolCall","id":"tool-1","name":"read","arguments":{"path":"fixture.txt"}}]}}`,
		`{"type":"model_change","id":"model","parentId":null,"provider":"example-provider","modelId":"example-model"}`,
	}, "\n") + "\n"
	result, err := Run(context.Background(), domain.Descriptor{Harness: "pi", Adapter: "pi-jsonl"}, fstest.MapFS{"source/records.jsonl": {Data: []byte(data)}})
	if err != nil {
		t.Fatal(err)
	}
	events := make(map[string]domain.Event)
	for _, event := range result.Events {
		events[event.Key] = event
	}
	resultEvent := events["tool_result:result:0"]
	if resultEvent.ParentKey != "tool_call:call:0" {
		t.Fatalf("tool result parent = %q", resultEvent.ParentKey)
	}
	if events["message:child"].ParentKey != resultEvent.Key {
		t.Fatalf("child parent = %q, want %q", events["message:child"].ParentKey, resultEvent.Key)
	}
	for _, key := range []string{"tool_call:call:0", "tool_result:result:0", "message:child", "message:branch"} {
		if events[key].Model != "example-model" || events[key].Provider != "example-provider" {
			t.Fatalf("%s model context = %q/%q", key, events[key].Provider, events[key].Model)
		}
	}
}

func TestPiCustomSidecarIsNormalized(t *testing.T) {
	data := strings.Join([]string{
		`{"type":"session","version":3,"id":"fixture"}`,
		`{"type":"message","id":"prompt","parentId":null,"message":{"role":"user","content":"hello"}}`,
		`{"type":"custom","id":"sidecar","parentId":"prompt","customType":"plannotator","data":{"tps-stats":{"tokens":1}}}`,
	}, "\n") + "\n"
	result, err := Run(context.Background(), domain.Descriptor{Harness: "pi", Adapter: "pi-jsonl"}, fstest.MapFS{"source/records.jsonl": {Data: []byte(data)}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "normalized" || len(result.Warnings) != 0 {
		t.Fatalf("status = %q warnings = %#v", result.Status, result.Warnings)
	}
	var foundCustom, foundUser bool
	for _, event := range result.Events {
		if event.Kind == "custom" {
			foundCustom = true
			if event.Text != "plannotator" {
				t.Fatalf("custom text = %q", event.Text)
			}
		}
		if event.Kind == "message" && event.Role == "user" {
			foundUser = true
		}
	}
	if !foundCustom || !foundUser {
		t.Fatalf("missing custom or user event: %#v", result.Events)
	}
}

func TestPiWarnsAndKeepsFirstDuplicateNativeID(t *testing.T) {
	data := "{\"type\":\"session\",\"version\":3,\"id\":\"fixture\"}\n" +
		"{\"type\":\"message\",\"id\":\"same\",\"parentId\":null,\"message\":{\"role\":\"user\",\"content\":\"first\"}}\n" +
		"{\"type\":\"message\",\"id\":\"same\",\"parentId\":null,\"message\":{\"role\":\"user\",\"content\":\"second\"}}\n"
	result, err := Run(context.Background(), domain.Descriptor{Harness: "pi", Adapter: "pi-jsonl"}, fstest.MapFS{"source/records.jsonl": {Data: []byte(data)}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "partially_parsed" || len(result.Events) != 1 || result.Events[0].Text != "first" || result.Warnings[0].Code != "duplicate_native_id" {
		t.Fatalf("unexpected duplicate result: %#v", result)
	}
}

func TestUnknownVersionIsUnsupported(t *testing.T) {
	source := fstest.MapFS{"source/records.jsonl": {Data: []byte("{\"type\":\"session\",\"version\":4,\"id\":\"fixture\"}\n")}}
	result, err := Run(context.Background(), domain.Descriptor{Harness: "pi", Adapter: "pi-jsonl"}, source)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "unsupported" || len(result.Warnings) != 1 || result.Warnings[0].Code != "unsupported_version" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestMalformedAndUnknownRecordsArePartial(t *testing.T) {
	data := "{\"id\":\"one\",\"role\":\"user\",\"content\":\"fixture text\"}\nnot-json\n{\"type\":\"future_record\"}\n"
	result, err := Run(context.Background(), domain.Descriptor{Harness: "cursor-agent", Adapter: "cursor-agent-jsonl"}, fstest.MapFS{"source/records.jsonl": {Data: []byte(data)}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "partially_parsed" || len(result.Events) != 1 || len(result.Warnings) != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestContentFingerprintDoesNotDependOnLineNumber(t *testing.T) {
	first := fstest.MapFS{"source/records.jsonl": {Data: []byte("{\"role\":\"user\",\"content\":\"stable fixture\"}\n")}}
	second := fstest.MapFS{"source/records.jsonl": {Data: []byte("\n{\"role\":\"user\",\"content\":\"stable fixture\"}\n")}}
	descriptor := domain.Descriptor{Harness: "cursor-agent", Adapter: "cursor-agent-jsonl"}
	a, err := Run(context.Background(), descriptor, first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Run(context.Background(), descriptor, second)
	if err != nil {
		t.Fatal(err)
	}
	if a.Events[0].Key != b.Events[0].Key {
		t.Fatalf("fingerprint changed with source line: %q != %q", a.Events[0].Key, b.Events[0].Key)
	}
}

func TestMissingNativeIDsUseContentFingerprints(t *testing.T) {
	claude := "{\"message\":{\"role\":\"user\",\"content\":[{\"type\":\"text\",\"text\":\"same text\"}]}}\n" +
		"{\"message\":{\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"same text\"}]}}\n"
	result, err := Run(context.Background(), domain.Descriptor{Harness: "claude-code", Adapter: "claude-code-jsonl"}, fstest.MapFS{"source/records.jsonl": {Data: []byte(claude)}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Events[0].Key == result.Events[1].Key || !strings.HasPrefix(result.Events[0].Key, "message:sha256:") {
		t.Fatalf("missing native IDs did not use distinct fingerprints: %#v", result.Events)
	}

	cursor, err := Run(context.Background(), domain.Descriptor{Harness: "cursor-agent", Adapter: "cursor-agent-jsonl"}, fstest.MapFS{"source/records.jsonl": {Data: []byte("{\"role\":\"user\",\"content\":\"root\"}\n")}})
	if err != nil {
		t.Fatal(err)
	}
	if cursor.Events[0].ParentKey != "" {
		t.Fatalf("root Cursor Agent parent = %q", cursor.Events[0].ParentKey)
	}
}

func TestRunEnforcesTotalSourceLimit(t *testing.T) {
	t.Run("JSON", func(t *testing.T) {
		root := t.TempDir()
		writeSparseSource(t, root, "export.json", domain.MaxAmpExportBytes+1)
		_, err := Run(context.Background(), domain.Descriptor{Harness: "amp", Adapter: "amp-thread-export"}, os.DirFS(root))
		if !errors.Is(err, errSourceLimit) {
			t.Fatalf("error = %v, want source limit", err)
		}
	})
	t.Run("total JSONL budget", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, "source"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "source", "summary.json"), []byte(`{"chat_format_version":1}`), 0o600); err != nil {
			t.Fatal(err)
		}
		writeSparseSource(t, root, "updates.jsonl", maxSourceBytes)
		_, err := Run(context.Background(), domain.Descriptor{Harness: "grok-build", Adapter: "grok-native-session"}, os.DirFS(root))
		if !errors.Is(err, errSourceLimit) {
			t.Fatalf("error = %v, want source limit", err)
		}
	})
}

func writeSparseSource(t *testing.T, root, name string, size int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "source"), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(filepath.Join(root, "source", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(size); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNativeIdentitySurvivesRevisionChanges(t *testing.T) {
	before := fstest.MapFS{"source/records.jsonl": {Data: []byte("{\"id\":\"message-1\",\"role\":\"assistant\",\"content\":\"draft fixture\"}\n")}}
	after := fstest.MapFS{"source/records.jsonl": {Data: []byte("{\"id\":\"message-1\",\"role\":\"assistant\",\"content\":\"final fixture\"}\n")}}
	descriptor := domain.Descriptor{Harness: "cursor-agent", Adapter: "cursor-agent-jsonl"}
	a, err := Run(context.Background(), descriptor, before)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Run(context.Background(), descriptor, after)
	if err != nil {
		t.Fatal(err)
	}
	if a.Events[0].Key != b.Events[0].Key {
		t.Fatalf("native identity changed across revisions: %q != %q", a.Events[0].Key, b.Events[0].Key)
	}
}

func TestRunHonorsCancellationAndReturnsUnsupportedHarness(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(ctx, domain.Descriptor{Harness: "pi"}, fstest.MapFS{}); err == nil {
		t.Fatal("expected cancellation error")
	}
	result, err := Run(context.Background(), domain.Descriptor{Harness: "future"}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "unsupported" || len(result.Warnings) != 1 || result.Warnings[0].Code != "unsupported_harness" {
		t.Fatalf("unexpected unsupported result: %#v", result)
	}
	if Version("future") != 0 {
		t.Fatalf("unknown harness version = %d", Version("future"))
	}
}

func TestRunStopsReadingAfterContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	source := cancelAfterReadFS{
		FS:     fstest.MapFS{"source/export.json": {Data: []byte(`{"messages":[]}`)}},
		cancel: cancel,
	}
	_, err := Run(ctx, domain.Descriptor{Harness: "amp", Adapter: "amp-thread-export"}, source)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
}

type cancelAfterReadFS struct {
	fs.FS
	cancel context.CancelFunc
}

func (source cancelAfterReadFS) Open(name string) (fs.File, error) {
	file, err := source.FS.Open(name)
	if err != nil {
		return nil, err
	}
	return &cancelAfterReadFile{File: file, cancel: source.cancel}, nil
}

type cancelAfterReadFile struct {
	fs.File
	cancel context.CancelFunc
	done   bool
}

func (file *cancelAfterReadFile) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	n, err := file.File.Read(p)
	if !file.done {
		file.done = true
		file.cancel()
	}
	return n, err
}

func assertFields(t *testing.T, events []domain.Event, kind, callID, model string) {
	t.Helper()
	foundKind, foundCall, foundModel := false, callID == "", model == ""
	for _, event := range events {
		foundKind = foundKind || event.Kind == kind
		foundCall = foundCall || event.CallID == callID
		foundModel = foundModel || event.Model == model
	}
	if !foundKind || !foundCall || !foundModel {
		t.Fatalf("missing expected fields kind=%q call=%q model=%q in %#v", kind, callID, model, events)
	}
}

func fixtureDirectory(name string) string {
	switch name {
	case "pi":
		return "pi-v3"
	case "opencode":
		return "opencode-v2"
	case "codex":
		return "codex-app-server"
	case "claude":
		return "claude-code"
	case "grok":
		return "grok-build"
	default:
		return name
	}
}

func findFixtureRoot() (string, error) {
	for _, path := range []string{"testdata/harnesses", "../../testdata/harnesses"} {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", os.ErrNotExist
}
