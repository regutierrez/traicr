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
		`INSERT INTO ItemTable VALUES ('composerData:composer-1', '{"id":"composer-1","fullConversationHeadersOnly":[{"bubbleId":"bubble-1"}]}')`,
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
