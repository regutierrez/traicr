package adapters

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommandListSchemasMatchAmpAndOpenCode(t *testing.T) {
	amp, err := parseAmpList([]byte(`[{"id":"T-1","title":"Thread","updated":"2026-09-06T00:00:00Z","tree":"","messageCount":3}]`))
	if err != nil || len(amp) != 1 || amp[0].ID != "T-1" {
		t.Fatalf("Amp list: %+v, %v", amp, err)
	}
	if _, err := parseAmpList([]byte(`{"threads":[{"id":"T-1"}]}`)); err == nil {
		t.Fatal("accepted invented Amp wrapper schema")
	}
	opencode, err := parseOpenCodeList([]byte(`[{"id":"ses_1","title":"Session","updated":1770000000000,"created":1760000000000,"projectId":"p1","directory":"/repo"}]`))
	if err != nil || len(opencode) != 1 || opencode[0].ID != "ses_1" || opencode[0].CWD != "/repo" {
		t.Fatalf("OpenCode list: %+v, %v", opencode, err)
	}
	if _, err := parseOpenCodeList([]byte(`[{"sessionId":"ses_1","updatedAt":"now"}]`)); err == nil {
		t.Fatal("accepted guessed OpenCode aliases")
	}
}

func TestCommandExportSchemasProvideKnownMetadata(t *testing.T) {
	dir := t.TempDir()
	ampPath := filepath.Join(dir, "amp.json")
	if err := os.WriteFile(ampPath, []byte(`{"v":1,"id":"T-1","title":"Thread","updatedAt":"2026-09-06T00:00:00Z","messages":[],"env":{"initial":{"workingDirectory":"/amp-repo"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	amp, err := inspectCommandExport("amp", ampPath, "T-1")
	if err != nil || amp.CWD != "/amp-repo" {
		t.Fatalf("Amp export: %+v, %v", amp, err)
	}
	opencodePath := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(opencodePath, []byte(`{"info":{"id":"ses_1","title":"Session","parentID":"ses_parent","directory":"/repo","time":{"updated":1770000000000}},"messages":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	opencode, err := inspectCommandExport("opencode", opencodePath, "ses_1")
	if err != nil || opencode.ParentID != "ses_parent" || opencode.CWD != "/repo" {
		t.Fatalf("OpenCode export: %+v, %v", opencode, err)
	}
}
