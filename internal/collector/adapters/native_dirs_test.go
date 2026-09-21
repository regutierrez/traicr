package adapters

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGrokCollectHonorsSkipWithoutOpeningSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "session")
	writeTestFile(t, filepath.Join(source, "summary.json"), `{"sessionId":"snapshot-session"}`)
	writeTestFile(t, filepath.Join(source, "updates.jsonl"), "{\"id\":\"update\"}\n")
	if err := os.Chmod(filepath.Join(source, "summary.json"), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(filepath.Join(source, "summary.json"), 0o600) })
	result, err := (grokAdapter{}).Collect(context.Background(), []string{root}, nil, func(path, stamp string) bool {
		if path == "" || stamp == "" {
			t.Fatalf("skip missing source identity: %q %q", path, stamp)
		}
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Cleanup != nil {
		defer result.Cleanup()
	}
	if len(result.Inputs) != 0 || result.Skipped != 1 || len(result.Warnings) != 0 {
		t.Fatalf("skip re-opened the source: %+v", result)
	}
}

func TestGrokDescriptorUsesSnapshotSummary(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "native-directory")
	writeTestFile(t, filepath.Join(source, "summary.json"), `{"sessionId":"snapshot-session"}`)
	writeTestFile(t, filepath.Join(source, "updates.jsonl"), "{\"id\":\"update\"}\n")
	for index := range 16 {
		writeTestFile(t, filepath.Join(source, fmt.Sprintf("z-%02d.bin", index)), strings.Repeat("x", 1<<20))
	}
	t.Setenv("TMPDIR", t.TempDir())
	watchContext, stopWatching := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopWatching()
	changed := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			matches, _ := filepath.Glob(filepath.Join(os.Getenv("TMPDIR"), "traicr-grok-*", "000001", "source", "summary.json"))
			if len(matches) > 0 {
				snapshot, _ := os.ReadFile(matches[0])
				if string(snapshot) == `{"sessionId":"snapshot-session"}` {
					changed <- os.WriteFile(filepath.Join(source, "summary.json"), []byte(`{"sessionId":"live-session"}`), 0o600)
					return
				}
			}
			select {
			case <-watchContext.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	result, err := (grokAdapter{}).Collect(context.Background(), []string{root}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Cleanup()
	select {
	case err := <-changed:
		if err != nil {
			t.Fatal(err)
		}
	case <-watchContext.Done():
		t.Fatal(watchContext.Err())
	}
	if len(result.Inputs) != 1 || result.Inputs[0].Descriptor.NativeTraceID != "snapshot-session" {
		t.Fatalf("unexpected snapshot descriptor: %+v; warnings: %+v", result.Inputs, result.Warnings)
	}
}
