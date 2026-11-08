package normalize

import (
	"encoding/json"
	"testing"

	"github.com/regutierrez/traicr/internal/domain"
)

func toolCall(id int64, tool, callID, args string) domain.Event {
	return domain.Event{ID: id, Kind: "tool_call", Tool: tool, CallID: callID, Text: args}
}

func toolResult(id int64, callID, text, metadata string) domain.Event {
	event := domain.Event{ID: id, Kind: "tool_result", CallID: callID, Text: text}
	if metadata != "" {
		event.Metadata = json.RawMessage(metadata)
	}
	return event
}

func TestPatchesPreferTheDiffTheHarnessRecorded(t *testing.T) {
	// Amp's edit_file result carries a fenced diff with file headers and real line numbers.
	ampDiff := "```diff\n--- /repo/a.py\toriginal\n+++ /repo/a.py\tmodified\n@@ -10,2 +10,3 @@\n keep\n-old\n+new\n+added\n```"
	files := `{"run":{"status":"done","result":{"files":[` +
		`{"path":"/repo/b.md","additions":1,"deletions":0,"diff":"Index: /repo/b.md\n===\n--- /repo/b.md\n+++ /repo/b.md\n@@ -1,1 +1,2 @@\n line\n+more"},` +
		`{"uri":"file:///repo/c.md","type":"add","diff":"@@ -0,0 +1 @@\n+y"}]}}}`
	events := []domain.Event{
		toolCall(1, "edit_file", "e1", `{"path":"/repo/a.py","old_str":"old","new_str":"new\nadded"}`),
		toolCall(2, "shell_command", "s1", `{"command":"ls"}`),
		toolResult(3, "e1", "", `{"run":{"status":"done","result":{"diff":`+quote(ampDiff)+`}}}`),
		toolCall(4, "apply_patch", "p1", `{"patchText":"*** Begin Patch\n*** Update File: /repo/ignored\n@@\n-a\n+b\n*** End Patch"}`),
		toolResult(5, "p1", "", files),
	}
	patches := Patches(events)
	if len(patches) != 3 {
		t.Fatalf("patches = %#v", patches)
	}
	edit := patches[0]
	if edit.Path != "/repo/a.py" || edit.EventID != 1 || edit.Operation != "edit" || edit.Diff != "@@ -10,2 +10,3 @@\n keep\n-old\n+new\n+added" || edit.Additions != 2 || edit.Deletions != 1 {
		t.Fatalf("edit_file patch = %#v", edit)
	}
	if patches[1].Path != "/repo/b.md" || patches[1].Diff != "@@ -1,1 +1,2 @@\n line\n+more" || patches[1].EventID != 4 {
		t.Fatalf("first apply_patch file = %#v", patches[1])
	}
	if patches[2].Path != "/repo/c.md" || patches[2].Operation != "write" || patches[2].Additions != 1 || patches[2].Deletions != 0 {
		t.Fatalf("second apply_patch file = %#v", patches[2])
	}
}

func TestPatchesRebuildDiffsFromCallArguments(t *testing.T) {
	events := []domain.Event{
		// Claude Code: a replacement that keeps its first and last lines.
		toolCall(1, "Edit", "c1", `{"file_path":"/repo/a.go","old_string":"a\nb\nc","new_string":"a\nB\nc\nd"}`),
		toolResult(2, "c1", "The file /repo/a.go has been updated successfully.", ""),
		// Pi: several replacements in one call, each its own hunk.
		toolCall(3, "edit", "p1", `{"path":"/repo/keys","edits":[{"oldText":"one","newText":"uno"},{"oldText":"two\n","newText":""}]}`),
		// The OpenAI patch format, with a rename, an added file, and a deleted file.
		toolCall(4, "apply_patch", "o1", `{"input":"*** Begin Patch\n*** Update File: /repo/old.go\n*** Move to: /repo/new.go\n@@ func main\n-x\n+y\n*** Add File: /repo/new.txt\n+hello\n+world\n*** Delete File: /repo/gone.txt\n*** End Patch"}`),
		// Codex lists changes; an added file's diff is its content.
		toolCall(5, "apply_patch", "x1", `{"changes":[{"path":"/repo/add.json","kind":{"type":"add"},"diff":"{\n}\n"},{"path":"/repo/up.go","kind":{"type":"update","move_path":null},"diff":"@@ -1 +1 @@\n-a\n+b\n"}]}`),
		// Cursor Agent writes and deletes whole files.
		toolCall(6, "Write", "w1", `{"path":"/repo/w.css","contents":"body {}\n"}`),
		toolCall(7, "Delete", "d1", `{"path":"/repo/w.old"}`),
	}
	got := map[string]Patch{}
	for _, patch := range Patches(events) {
		got[patch.Path] = patch
	}
	want := map[string]Patch{
		"/repo/a.go":     {Operation: "edit", Diff: "@@\n a\n-b\n+B\n c\n+d", Additions: 2, Deletions: 1},
		"/repo/keys":     {Operation: "edit", Diff: "@@\n-one\n+uno\n@@\n-two", Additions: 1, Deletions: 2},
		"/repo/new.go":   {Operation: "edit", Diff: "@@ func main\n-x\n+y", Additions: 1, Deletions: 1},
		"/repo/new.txt":  {Operation: "write", Diff: "+hello\n+world", Additions: 2},
		"/repo/gone.txt": {Operation: "delete"},
		"/repo/add.json": {Operation: "write", Diff: "+{\n+}", Additions: 2},
		"/repo/up.go":    {Operation: "edit", Diff: "@@ -1 +1 @@\n-a\n+b", Additions: 1, Deletions: 1},
		"/repo/w.css":    {Operation: "write", Diff: "+body {}", Additions: 1},
		"/repo/w.old":    {Operation: "delete"},
	}
	if len(got) != len(want) {
		t.Fatalf("patches = %#v", got)
	}
	for path, expected := range want {
		patch := got[path]
		if patch.Operation != expected.Operation || patch.Diff != expected.Diff || patch.Additions != expected.Additions || patch.Deletions != expected.Deletions {
			t.Errorf("%s = %+v, want %+v", path, patch, expected)
		}
	}
}

func TestPatchesKeepCallsWithoutADiffAndMarkFailures(t *testing.T) {
	events := []domain.Event{
		// Cursor's IDE edit records only the new text and content IDs.
		toolCall(1, "edit_file_v2", "c1", `{"relativeWorkspacePath":"/repo/a.go","streamingContent":"type Input struct{}"}`),
		toolResult(2, "c1", `{"afterContentId":"x"}`, ""),
		toolCall(3, "edit_file", "a1", `{"path":"/repo/b.go","old_str":"x","new_str":"y"}`),
		// A text result must not hide the status next to it.
		toolResult(4, "a1", "old_str not found", `{"run":{"result":"old_str not found","status":"error"}}`),
		toolCall(5, "Edit", "c2", `{"file_path":"/repo/c.go","old_string":"x","new_string":"y"}`),
		toolResult(6, "c2", "<tool_use_error>String to replace not found in file.</tool_use_error>", ""),
		// A call that names no file has nothing to list.
		toolCall(7, "edit", "n1", `{"note":"no path"}`),
	}
	patches := Patches(events)
	if len(patches) != 3 {
		t.Fatalf("patches = %#v", patches)
	}
	cursor := patches[0]
	if cursor.Path != "/repo/a.go" || cursor.Diff != "" || cursor.Arguments != events[0].Text || cursor.Result != `{"afterContentId":"x"}` || cursor.Failed {
		t.Fatalf("cursor patch = %#v", cursor)
	}
	if !patches[1].Failed || !patches[2].Failed || patches[1].Diff == "" {
		t.Fatalf("failed edits = %#v %#v", patches[1], patches[2])
	}
}

func quote(text string) string {
	data, _ := json.Marshal(text)
	return string(data)
}
