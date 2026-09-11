package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestAmpListPaginatesWithinCLILimitAndDropsRepeatedThreads(t *testing.T) {
	if os.Getenv("TRAICR_TEST_AMP_LIST") == "1" {
		args := os.Args
		limit, _ := strconv.Atoi(args[len(args)-3])
		offset, _ := strconv.Atoi(args[len(args)-1])
		if limit <= 0 || limit > 500 {
			fmt.Fprintln(os.Stderr, "--limit must be an integer <= 500")
			os.Exit(1)
		}
		entries := make([]map[string]string, 0)
		for i := offset; i < offset+limit && i < 501; i++ {
			entries = append(entries, map[string]string{"id": fmt.Sprintf("T-%d", i), "updated": "2026-09-06T00:00:00Z"})
		}
		if offset > 0 {
			// Amp reorders pages between requests, so a later page can repeat earlier threads.
			entries = append(entries, map[string]string{"id": "T-7", "updated": "2026-09-06T00:00:00Z"}, map[string]string{"id": "T-9", "updated": "2026-09-06T00:00:00Z"})
		}
		json.NewEncoder(os.Stdout).Encode(entries)
		os.Exit(0)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	command := filepath.Join(dir, "amp")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nexec \"$TRAICR_TEST_BINARY\" -test.run=^TestAmpListPaginatesWithinCLILimitAndDropsRepeatedThreads$ -- \"$@\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRAICR_TEST_AMP_LIST", "1")
	t.Setenv("TRAICR_TEST_BINARY", binary)
	traces, warnings := (commandAdapter{name: "amp", executable: command}).listAmp(context.Background())
	if len(warnings) != 1 || warnings[0].Code != "unstable_listing" || len(traces) != 501 {
		t.Fatalf("Amp pagination: %d traces, warnings: %+v", len(traces), warnings)
	}
	for i, trace := range traces {
		if trace.ID != fmt.Sprintf("T-%d", i) {
			t.Fatalf("Amp trace %d: %q", i, trace.ID)
		}
	}
}

func TestAmpCollectUsesExportTimestampWithoutLiveChangeWarning(t *testing.T) {
	if os.Getenv("TRAICR_TEST_AMP_COLLECT") == "1" {
		args := os.Args
		if args[len(args)-2] == "export" {
			fmt.Print(`{"v":1,"id":"T-1","title":"Thread","updatedAt":"2026-09-10T23:07:25.917Z","messages":[],"env":{"initial":{"workingDirectory":"/"}}}`)
		} else {
			fmt.Print(`[{"id":"T-1","title":"Thread","updated":"2026-09-10T15:47:53.910Z","tree":"","messageCount":1}]`)
		}
		os.Exit(0)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := filepath.Join(t.TempDir(), "amp")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nexec \"$TRAICR_TEST_BINARY\" -test.run=^TestAmpCollectUsesExportTimestampWithoutLiveChangeWarning$ -- \"$@\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRAICR_TEST_AMP_COLLECT", "1")
	t.Setenv("TRAICR_TEST_BINARY", binary)
	result, err := (commandAdapter{name: "amp", format: "amp-thread-export", executable: command}).Collect(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Cleanup()
	if len(result.Warnings) != 0 || len(result.Inputs) != 1 || result.Inputs[0].Descriptor.NativeUpdatedAt != "2026-09-10T23:07:25.917Z" {
		t.Fatalf("Amp collect: %+v, warnings: %+v", result.Inputs, result.Warnings)
	}
}

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
