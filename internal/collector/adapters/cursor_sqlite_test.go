package adapters_test

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/regutierrez/traicr/internal/collector/adapters"
	_ "modernc.org/sqlite"
)

func TestCursorCollectsComposerAndReferencedBubblesFromLiveWALReadOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.vscdb")
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	for _, statement := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA wal_autocheckpoint=0",
		"CREATE TABLE ItemTable (key TEXT PRIMARY KEY, value TEXT)",
		`INSERT INTO ItemTable VALUES ('composerData:composer-1', '{"id":"composer-1","name":" Fix login redirect ","fullConversationHeadersOnly":[{"bubbleId":"bubble-1"}]}')`,
		`INSERT INTO ItemTable VALUES ('bubbleId:bubble-1', '{"role":"user","text":"hello"}')`,
		`INSERT INTO ItemTable VALUES ('bubbleId:unreferenced', '{"role":"assistant","text":"not part of trace"}')`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	var cursor adapters.Adapter
	for _, adapter := range adapters.All() {
		if adapter.Name() == "cursor" {
			cursor = adapter
		}
	}
	result, err := cursor.Collect(context.Background(), []string{dir}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Cleanup()
	if len(result.Inputs) != 1 || result.Inputs[0].Descriptor.NativeTraceID != "composer-1" {
		t.Fatalf("unexpected inputs: %+v", result.Inputs)
	}
	if title := result.Inputs[0].Descriptor.Title; title != "Fix login redirect" {
		t.Fatalf("title = %q, want the Cursor composer name", title)
	}
	file, err := os.Open(filepath.Join(result.Inputs[0].Directory, "source", "rows.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var keys []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var row struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, row.Key)
	}
	if len(keys) != 2 || keys[0] != "composerData:composer-1" || keys[1] != "bubbleId:bubble-1" {
		t.Fatalf("got rows %v", keys)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("collector modified Cursor database")
	}
}

func TestCursorAgentTranscriptTakesNameAndFolderFromCursorState(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)
	state := filepath.Join(config, "Cursor", "User", "globalStorage")
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", filepath.Join(state, "state.vscdb"))
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"CREATE TABLE cursorDiskKV (key TEXT PRIMARY KEY, value TEXT)",
		"CREATE TABLE composerHeaders (composerId TEXT PRIMARY KEY, workspaceId TEXT, value TEXT)",
		`INSERT INTO cursorDiskKV VALUES ('composerData:chat-1', '{"composerId":"chat-1","name":" Fix the importer "}')`,
		`INSERT INTO cursorDiskKV VALUES ('composerData:chat-2', '{"composerId":"chat-2","name":"Draft without folder"}')`,
		`INSERT INTO composerHeaders VALUES ('chat-1', 'w1', '{"workspaceIdentifier":{"id":"w1","uri":{"fsPath":"/work/importer"}}}')`,
		`INSERT INTO composerHeaders VALUES ('chat-2', 'empty-window', '{"workspaceIdentifier":{"id":"empty-window"}}')`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	database.Close()

	projects := filepath.Join(config, "projects")
	for _, id := range []string{"chat-1", "chat-2", "chat-3"} {
		dir := filepath.Join(projects, "work-importer", "agent-transcripts", id)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, id+".jsonl"), []byte(`{"role":"user","message":{"content":[{"type":"text","text":"hi"}]}}`+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var agent adapters.Adapter
	for _, adapter := range adapters.All() {
		if adapter.Name() == "cursor-agent" {
			agent = adapter
		}
	}
	result, err := agent.Collect(context.Background(), []string{projects}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Cleanup()
	got := map[string][2]string{}
	for _, input := range result.Inputs {
		got[input.Descriptor.NativeTraceID] = [2]string{input.Descriptor.Title, input.Descriptor.WorkingDirectory}
	}
	want := map[string][2]string{"chat-1": {"Fix the importer", "/work/importer"}, "chat-2": {"Draft without folder", ""}, "chat-3": {"", ""}}
	if len(got) != len(want) || got["chat-1"] != want["chat-1"] || got["chat-2"] != want["chat-2"] || got["chat-3"] != want["chat-3"] {
		t.Fatalf("title and folder = %v, want %v", got, want)
	}
}
