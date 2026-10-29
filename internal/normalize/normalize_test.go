package normalize

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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
			source, err := fixtureSource(test.name)
			if err != nil {
				t.Fatal(err)
			}
			result, err := Run(context.Background(), domain.Descriptor{Harness: test.harness, Adapter: test.adapter}, source)
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

func TestCursorMCPToolResultStaysWithItsCall(t *testing.T) {
	composerID := "ddf4e792-18a5-4ad4-9004-8b7e6d5008ee"
	params := `{"server":"user-doop","toolName":"get_guide"}`
	resultText := `{"content":"{\"tool\":\"get_guide\",\"description\":\"Read the Doop agent guide\"}"}`
	rows := strings.Join([]string{
		cursorDiskRow("composerData:"+composerID, map[string]any{
			"composerId": composerID,
			"fullConversationHeadersOnly": []any{
				map[string]any{"bubbleId": "bubble-guide", "type": 2},
				map[string]any{"bubbleId": "bubble-pending", "type": 2},
				map[string]any{"bubbleId": "bubble-legacy", "type": 2},
			},
		}),
		cursorDiskRow("bubbleId:"+composerID+":bubble-guide", map[string]any{
			"bubbleId": "bubble-guide",
			"type":     2,
			"text":     "I'll pull the Doop guide and inspect that canvas so we can draw the Linear redesign there for collaboration.",
			"toolFormerData": map[string]any{
				"tool":       19,
				"toolCallId": "toolu_guide",
				"name":       "GetMcpTools",
				"status":     "completed",
				"params":     params,
				"rawArgs":    params,
				"result":     resultText,
			},
		}),
		cursorDiskRow("bubbleId:"+composerID+":bubble-pending", map[string]any{
			"bubbleId": "bubble-pending",
			"type":     2,
			"toolFormerData": map[string]any{
				"tool":       19,
				"toolCallId": "toolu_pending",
				"name":       "GetMcpTools",
				"status":     "loading",
				"params":     `{"server":"user-doop","toolName":"get_canvas"}`,
			},
		}),
		cursorDiskRow("bubbleId:"+composerID+":bubble-legacy", map[string]any{
			"bubbleId": "bubble-legacy",
			"type":     2,
			"toolFormerData": map[string]any{
				"name":   "GetMcpTools",
				"params": `{"server":"user-doop"}`,
				"result": "listed tools",
			},
		}),
	}, "\n") + "\n"
	result, err := Run(context.Background(), domain.Descriptor{Harness: "cursor", Adapter: "cursor-sqlite-rows"}, fstest.MapFS{
		"source/rows.jsonl": {Data: []byte(rows)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Version != 4 {
		t.Fatalf("cursor normalizer version = %d", result.Version)
	}
	var guideCall, guideResult, pendingCall, legacyCall, legacyResult *domain.Event
	for index := range result.Events {
		event := &result.Events[index]
		switch event.Key {
		case "tool_call:bubble-guide":
			guideCall = event
		case "tool_result:bubble-guide":
			guideResult = event
		case "tool_call:bubble-pending":
			pendingCall = event
		case "tool_call:bubble-legacy":
			legacyCall = event
		case "tool_result:bubble-legacy":
			legacyResult = event
		}
	}
	if guideCall == nil || guideResult == nil || pendingCall == nil || legacyCall == nil || legacyResult == nil {
		t.Fatalf("missing tool events: %#v", result.Events)
	}
	if guideCall.CallID != "toolu_guide" || guideResult.CallID != guideCall.CallID || guideCall.Tool != "GetMcpTools" {
		t.Fatalf("guide call was not linked: call=%+v result=%+v", guideCall, guideResult)
	}
	if !strings.Contains(guideCall.Text, `"server":"user-doop"`) || !strings.Contains(guideCall.Text, `"toolName":"get_guide"`) || strings.Contains(guideCall.Text, `\"server\"`) {
		t.Fatalf("params stayed a JSON string: %s", guideCall.Text)
	}
	if strings.Contains(guideResult.Text, `\n`) || strings.Contains(guideResult.Text, `\"tool\"`) || !strings.Contains(guideResult.Text, `"description": "Read the Doop agent guide"`) {
		t.Fatalf("result text = %q", guideResult.Text)
	}
	if pendingCall.CallID != "toolu_pending" {
		t.Fatalf("pending call id = %q", pendingCall.CallID)
	}
	for _, event := range result.Events {
		if event.Kind == "tool_result" && event.CallID == "toolu_pending" {
			t.Fatalf("loading call invented a result: %+v", event)
		}
	}
	if legacyCall.CallID != "bubble-legacy" || legacyResult.CallID != "bubble-legacy" {
		t.Fatalf("legacy bubble ids did not pair the result: call=%q result=%q", legacyCall.CallID, legacyResult.CallID)
	}
}

func TestCursorToolResultTextUnwrapsJSONEnvelopes(t *testing.T) {
	composerID := "cursor-results"
	shellResult := `{"output":"ok\n","exitCode":0}`
	readResult := `{"contents":"package main\n","relativeWorkspacePath":"main.go"}`
	mcpResult := "{\"content\":[{\"type\":\"text\",\"text\":\"Call with topic `doop-instructions` once.\"}]}"
	rows := strings.Join([]string{
		cursorDiskRow("composerData:"+composerID, map[string]any{
			"composerId": composerID,
			"fullConversationHeadersOnly": []any{
				map[string]any{"bubbleId": "shell", "type": 2},
				map[string]any{"bubbleId": "read", "type": 2},
				map[string]any{"bubbleId": "mcp", "type": 2},
			},
		}),
		cursorDiskRow("bubbleId:"+composerID+":shell", map[string]any{
			"bubbleId": "shell", "type": 2,
			"toolFormerData": map[string]any{"toolCallId": "shell-1", "name": "run_terminal_command_v2", "status": "completed", "params": `{"command":"echo ok"}`, "result": shellResult},
		}),
		cursorDiskRow("bubbleId:"+composerID+":read", map[string]any{
			"bubbleId": "read", "type": 2,
			"toolFormerData": map[string]any{"toolCallId": "read-1", "name": "read_file_v2", "status": "completed", "params": `{"targetFile":"main.go"}`, "result": readResult},
		}),
		cursorDiskRow("bubbleId:"+composerID+":mcp", map[string]any{
			"bubbleId": "mcp", "type": 2,
			"toolFormerData": map[string]any{"toolCallId": "mcp-1", "name": "mcp_user-doop_get_guide", "status": "completed", "params": `{"topic":"doop-instructions"}`, "result": mcpResult},
		}),
	}, "\n") + "\n"
	result, err := Run(context.Background(), domain.Descriptor{Harness: "cursor", Adapter: "cursor-sqlite-rows"}, fstest.MapFS{
		"source/rows.jsonl": {Data: []byte(rows)},
	})
	if err != nil {
		t.Fatal(err)
	}
	texts := map[string]domain.Event{}
	for _, event := range result.Events {
		if event.Kind == "tool_result" {
			texts[event.CallID] = event
		}
	}
	if texts["shell-1"].Text != "ok" {
		t.Fatalf("shell output = %q", texts["shell-1"].Text)
	}
	if !strings.Contains(string(texts["shell-1"].Metadata), `"exitCode":0`) {
		t.Fatalf("shell metadata = %s", texts["shell-1"].Metadata)
	}
	if texts["read-1"].Text != "package main" {
		t.Fatalf("read output = %q", texts["read-1"].Text)
	}
	if texts["mcp-1"].Text != "Call with topic `doop-instructions` once." {
		t.Fatalf("mcp output = %q", texts["mcp-1"].Text)
	}
}

func cursorDiskRow(key string, value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	row, err := json.Marshal(map[string]string{"table": "cursorDiskKV", "key": key, "value": string(encoded)})
	if err != nil {
		panic(err)
	}
	return string(row)
}

func TestAttachmentIndexesMetadataWithoutPayload(t *testing.T) {
	result, err := Run(context.Background(), domain.Descriptor{Harness: "claude-code", Adapter: "claude-code-jsonl"}, recordsFS(claudeCodeRecords))
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

func TestCodexShowsCommandsSearchesAndFileChangesAsTools(t *testing.T) {
	data := []byte(`{"thread":{"turns":[{"id":"turn-1","items":[
		{"type":"commandExecution","id":"exec-1","command":"/usr/bin/zsh -lc 'go test ./...'","cwd":"/repo","status":"completed","aggregatedOutput":"FAIL store","exitCode":1},
		{"type":"webSearch","id":"ws-1","query":"codex usage","action":{"type":"search","query":"codex usage"},"results":null},
		{"type":"fileChange","id":"patch-1","changes":[{"path":"/repo/a.go","kind":{"type":"add"},"diff":"package a"}],"status":"completed"}
	]}]}}`)
	result, err := Run(context.Background(), domain.Descriptor{Harness: "codex", Adapter: "codex-app-server"}, fstest.MapFS{"source/thread.json": {Data: data}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 || len(result.Events) != 4 {
		t.Fatalf("warnings = %#v events = %#v", result.Warnings, result.Events)
	}
	command, output, search, patch := result.Events[0], result.Events[1], result.Events[2], result.Events[3]
	if command.Tool != "shell" || command.Text != `{"command":"/usr/bin/zsh -lc 'go test ./...'","cwd":"/repo"}` {
		t.Fatalf("command = %#v", command)
	}
	if output.Kind != "tool_result" || output.CallID != command.CallID || output.Text != "FAIL store" || string(output.Metadata) != `{"run":{"result":{"exitCode":1},"status":"error"}}` {
		t.Fatalf("command output = %#v metadata %s", output, output.Metadata)
	}
	if search.Tool != "web_search" || search.Text != `{"query":"codex usage","type":"search"}` {
		t.Fatalf("search = %#v", search)
	}
	if patch.Kind != "tool_call" || patch.Tool != "apply_patch" || !strings.Contains(patch.Text, `"path":"/repo/a.go"`) || !slices.Equal(patch.LegacyKeys, []string{"file_change:patch-1"}) {
		t.Fatalf("patch = %#v", patch)
	}
	// Items have no timestamps; without a parent chain a renamed item would sort last.
	if command.ParentKey != "" || output.ParentKey != command.Key || search.ParentKey != output.Key || patch.ParentKey != search.Key {
		t.Fatalf("parents = %q %q %q %q, want each item under the one before it", command.ParentKey, output.ParentKey, search.ParentKey, patch.ParentKey)
	}
}

func TestPiUsesParentGraphOrder(t *testing.T) {
	result, err := Run(context.Background(), domain.Descriptor{Harness: "pi", Adapter: "pi-jsonl"}, recordsFS(piV3Records))
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

func TestPiNormalizesUserCommandsAndSectionedSystemPrompt(t *testing.T) {
	data := strings.Join([]string{
		`{"type":"session","version":3,"id":"fixture"}`,
		`{"type":"session","version":3,"id":"fixture"}`,
		`{"type":"message","id":"system","parentId":null,"message":{"role":"system","content":"","sections":{"zeta":"<zeta>z</zeta>","cwd":"<cwd>/repo</cwd>","alpha":"<alpha>a</alpha>","preamble":"You are a fixture.","tools":" "},"toolsAdded":[{"name":"read","description":"Read a file"}],"toolsRemoved":[{"name":"grep"}]}}`,
		`{"type":"message","id":"bang","parentId":"system","message":{"role":"bashExecution","command":"make test","output":"FAIL fixture","exitCode":2,"cancelled":false,"truncated":false,"timestamp":1}}`,
		`{"type":"message","id":"after","parentId":"bang","message":{"role":"user","content":"why did it fail?"}}`,
		`{"type":"message","id":"failed","parentId":"after","message":{"role":"assistant","content":[],"stopReason":"error","errorMessage":"Request timed out."}}`,
		`{"type":"context_edit","id":"drop","parentId":"failed","targetId":"failed","replacement":null}`,
		`{"type":"context_edit","id":"swap","parentId":"drop","targetId":"after","replacement":[{"type":"text","text":"[collapsed]"}]}`,
	}, "\n") + "\n"
	result, err := Run(context.Background(), domain.Descriptor{Harness: "pi", Adapter: "pi-jsonl"}, fstest.MapFS{"source/records.jsonl": {Data: []byte(data)}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("warnings = %#v", result.Warnings)
	}
	events := make(map[string]domain.Event)
	for _, event := range result.Events {
		events[event.Key] = event
	}
	system := events["message:system"]
	if want := "You are a fixture.\n\n<cwd>/repo</cwd>\n\n<alpha>a</alpha>\n\n<zeta>z</zeta>\n\nCleared sections: tools\n\nTools added: read\n\nTools removed: grep"; system.Role != "system" || system.Text != want {
		t.Fatalf("system = %q %q, want %q", system.Role, system.Text, want)
	}
	call, output := events["tool_call:bang:0"], events["tool_result:bang:1"]
	if call.Role != "user" || call.Tool != "bash" || call.Text != `{"command":"make test"}` || !slices.Equal(call.LegacyKeys, []string{"message:bang"}) {
		t.Fatalf("command call = %#v", call)
	}
	if output.CallID != call.CallID || output.Text != "FAIL fixture" || string(output.Metadata) != `{"run":{"result":{"exitCode":2},"status":"error"}}` {
		t.Fatalf("command result = %#v metadata %s", output, output.Metadata)
	}
	if events["message:after"].ParentKey != call.Key {
		t.Fatalf("next prompt parent = %q, want %q", events["message:after"].ParentKey, call.Key)
	}
	if failed := events["message:failed:0"]; failed.Role != "assistant" || string(failed.Metadata) != `{"state":{"errorMessage":"Request timed out.","stopReason":"error"}}` {
		t.Fatalf("failed reply = %#v metadata %s", failed, failed.Metadata)
	}
	if drop, swap := events["context_edit:drop"], events["context_edit:swap"]; drop.Text != "" || drop.ParentKey != "message:failed:0" || swap.Text != "[collapsed]" || swap.ParentKey != drop.Key {
		t.Fatalf("context edits = %#v %#v", drop, swap)
	}
}

func TestClaudeKeepsOneConversationChainThroughBookkeepingRecords(t *testing.T) {
	data := strings.Join([]string{
		`{"type":"ai-title","aiTitle":"Generated name"}`,
		`{"type":"user","uuid":"u1","parentUuid":null,"timestamp":"2026-09-06T10:00:00Z","message":{"role":"user","content":"Fix the build"}}`,
		`{"type":"attachment","uuid":"att","parentUuid":"u1","attachment":{"type":"total_tokens_reminder"}}`,
		`{"type":"api-request-blob","hash":"h1","message":{"role":"user","content":[{"type":"text","text":"Fix the build"}]}}`,
		`{"type":"api-request-blob","hash":"h2","message":{"role":"system","content":[{"type":"text","text":"You are Claude Code."}]}}`,
		`{"type":"api-request","id":"req"}`,
		`{"type":"assistant","uuid":"a1","parentUuid":"att","effort":"high","message":{"role":"assistant","content":[{"type":"tool_use","id":"call-a","name":"Bash","input":{"command":"make"}}]}}`,
		`{"type":"assistant","uuid":"a2","parentUuid":"a1","effort":"high","message":{"role":"assistant","content":[{"type":"tool_use","id":"call-b","name":"Bash","input":{"command":"go vet"}}]}}`,
		`{"type":"user","uuid":"r2","parentUuid":"a2","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"call-b","content":"vet ok"}]}}`,
		`{"type":"user","uuid":"r1","parentUuid":"a1","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"call-a","content":"build failed"}]}}`,
		`{"type":"api-request-blob","hash":"h2","message":{"role":"system","content":[{"type":"text","text":"You are Claude Code."}]}}`,
		`{"type":"user","uuid":"meta","parentUuid":"r1","isMeta":true,"message":{"role":"user","content":"Base directory for this skill"}}`,
		`{"type":"system","subtype":"api_error","uuid":"err","parentUuid":"meta","error":{"formatted":"529 overloaded"}}`,
		`{"type":"system","subtype":"turn_duration","uuid":"dur","parentUuid":"err","durationMs":5}`,
		`{"type":"assistant","uuid":"a3","parentUuid":"dur","effort":"low","message":{"role":"assistant","content":[{"type":"text","text":"The build fails in make."}]}}`,
		`{"type":"system","subtype":"compact_boundary","uuid":"cb","parentUuid":null,"logicalParentUuid":"a3","content":"Conversation compacted"}`,
		`{"type":"custom-title","customTitle":"Build fix"}`,
		`{"type":"brand-new-record","uuid":"x"}`,
	}, "\n") + "\n"
	result, err := Run(context.Background(), domain.Descriptor{Harness: "claude-code", Adapter: "claude-code-jsonl"}, fstest.MapFS{"source/records.jsonl": {Data: []byte(data)}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Title != "Build fix" {
		t.Fatalf("title = %q, want the custom title over the generated one", result.Title)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0].Message, "line 18: unknown Claude record brand-new-record") {
		t.Fatalf("warnings = %#v, want only the unknown record", result.Warnings)
	}
	// Each event's parent is the event before it: one chain, no side branches, no copies.
	want := []struct{ key, kind, role, text string }{
		{"message:u1:0", "message", "user", "Fix the build"},
		{"message:prompt:h2", "message", "system", "You are Claude Code."},
		{"thinking_level_change:effort:a1", "thinking_level_change", "", "high"},
		{"tool_call:a1:0", "tool_call", "assistant", `{"command":"make"}`},
		{"tool_call:a2:0", "tool_call", "assistant", `{"command":"go vet"}`},
		{"tool_result:r2:0", "tool_result", "user", "vet ok"},
		{"tool_result:r1:0", "tool_result", "user", "build failed"},
		{"message:meta:0", "message", "system", "Base directory for this skill"},
		{"message:err:0", "message", "system", "API error: 529 overloaded"},
		{"thinking_level_change:effort:a3", "thinking_level_change", "", "low"},
		{"message:a3:0", "message", "assistant", "The build fails in make."},
		{"compaction:cb:0", "compaction", "", "Conversation compacted"},
	}
	if len(result.Events) != len(want) {
		t.Fatalf("got %d events, want %d: %#v", len(result.Events), len(want), result.Events)
	}
	parent := ""
	for index, event := range result.Events {
		w := want[index]
		if event.Key != w.key || event.Kind != w.kind || event.Role != w.role || event.Text != w.text || event.ParentKey != parent {
			t.Fatalf("event %d = %q %s/%s %q parent %q, want %q %s/%s %q parent %q", index, event.Key, event.Kind, event.Role, event.Text, event.ParentKey, w.key, w.kind, w.role, w.text, parent)
		}
		parent = event.Key
	}
}

func TestCursorAgentShowsStoppedTurnsInOrder(t *testing.T) {
	data := strings.Join([]string{
		`{"role":"user","message":{"content":[{"type":"text","text":"rename the flag"}]}}`,
		`{"role":"assistant","message":{"content":[{"type":"text","text":"Renaming it."},{"type":"tool_use","name":"Edit","input":{"path":"flags.go"}}]}}`,
		`{"type":"turn_ended","status":"success"}`,
		`{"role":"user","message":{"content":[{"type":"text","text":"also the docs"}]}}`,
		`{"role":"user","message":{"content":[{"type":"text","text":"<available_subagent_types>"}]}}`,
		`{"role":"user","message":{"content":[{"type":"text","text":"also the docs"}]}}`,
		`{"role":"assistant","message":{"content":[{"type":"text","text":"Updating the docs."}]}}`,
		`{"type":"turn_ended","status":"aborted","error":"User aborted/interrupted manually."}`,
	}, "\n") + "\n"
	result, err := Run(context.Background(), domain.Descriptor{Harness: "cursor-agent", Adapter: "cursor-agent-jsonl"}, fstest.MapFS{"source/records.jsonl": {Data: []byte(data)}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 || len(result.Events) != 8 {
		t.Fatalf("warnings = %#v, events = %d, want none and 8", result.Warnings, len(result.Events))
	}
	stopped := result.Events[7]
	if stopped.Role != "assistant" || string(stopped.Metadata) != `{"state":{"errorMessage":"User aborted/interrupted manually.","stopReason":"aborted"}}` {
		t.Fatalf("stopped turn = %#v metadata %s", stopped, stopped.Metadata)
	}
	for index := 1; index < 5; index++ {
		if result.Events[index].ParentKey != result.Events[index-1].Key {
			t.Fatalf("event %d parent = %q, want the event before it %q", index, result.Events[index].ParentKey, result.Events[index-1].Key)
		}
	}
	// The repeated prompt is stored with its first copy, so it must not start a branch:
	// the reply follows the injected row, and both copies are identical.
	prompt, injected, repeated, reply := result.Events[3], result.Events[4], result.Events[5], result.Events[6]
	if repeated.Key != prompt.Key || repeated.ParentKey != prompt.ParentKey {
		t.Fatalf("repeated prompt = %q under %q, want a copy of %q under %q", repeated.Key, repeated.ParentKey, prompt.Key, prompt.ParentKey)
	}
	if reply.ParentKey != injected.Key || stopped.ParentKey != reply.Key {
		t.Fatalf("reply parent = %q, stopped parent = %q; want %q and %q", reply.ParentKey, stopped.ParentKey, injected.Key, reply.Key)
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

func fixtureSource(name string) (fs.FS, error) {
	switch name {
	case "pi":
		return recordsFS(piV3Records), nil
	case "claude":
		return recordsFS(claudeCodeRecords), nil
	}
	root, err := findFixtureRoot()
	if err != nil {
		return nil, err
	}
	return os.DirFS(root + "/" + fixtureDirectory(name)), nil
}

func fixtureDirectory(name string) string {
	switch name {
	case "opencode":
		return "opencode-v2"
	case "codex":
		return "codex-app-server"
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
