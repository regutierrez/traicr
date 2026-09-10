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

func TestJSONLMetadataUsesHarnessIdentityAndExplicitParents(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, "claude-fallback.jsonl")
	writeTestFile(t, claude, "{\"uuid\":\"message-id\",\"id\":\"also-message\"}\n{\"sessionId\":\"claude-session\",\"cwd\":\"/repo\",\"uuid\":\"message-two\"}\n")
	metadata, warnings := jsonlMetadata("claude-code", claude, claude)
	if metadata.ID != "claude-session" || metadata.CWD != "/repo" || len(warnings) != 0 {
		t.Fatalf("unexpected Claude metadata: %+v, warnings: %+v", metadata, warnings)
	}
	claudeSubagent := filepath.Join(dir, "claude-session", "subagents", "agent-child-agent.jsonl")
	writeTestFile(t, claudeSubagent, "{\"sessionId\":\"claude-session\",\"agentId\":\"child-agent\",\"uuid\":\"message-id\"}\n")
	metadata, warnings = jsonlMetadata("claude-code", claudeSubagent, claudeSubagent)
	if metadata.ID != "child-agent" || metadata.ParentID != "claude-session" || len(warnings) != 0 {
		t.Fatalf("unexpected Claude subagent metadata: %+v, warnings: %+v", metadata, warnings)
	}

	parent := filepath.Join(dir, "parent.jsonl")
	child := filepath.Join(dir, "child.jsonl")
	writeTestFile(t, parent, "{\"type\":\"session\",\"id\":\"pi-parent\",\"cwd\":\"/repo\"}\n")
	writeTestFile(t, child, fmt.Sprintf("{\"type\":\"session\",\"id\":\"pi-child\",\"parentSession\":%q}\n", parent))
	metadata, warnings = jsonlMetadata("pi", child, child)
	if metadata.ID != "pi-child" || metadata.ParentID != "pi-parent" || len(warnings) != 0 {
		t.Fatalf("unexpected Pi metadata: %+v, warnings: %+v", metadata, warnings)
	}

	subagent := filepath.Join(dir, "cursor-parent", "subagents", "cursor-child.jsonl")
	writeTestFile(t, subagent, "{\"role\":\"user\",\"id\":\"event-id\"}\n")
	metadata, warnings = jsonlMetadata("cursor-agent", subagent, subagent)
	if metadata.ID != "cursor-child" || metadata.ParentID != "cursor-parent" || len(warnings) != 0 {
		t.Fatalf("unexpected Cursor metadata: %+v, warnings: %+v", metadata, warnings)
	}

	codex := filepath.Join(dir, "rollout.jsonl")
	writeTestFile(t, codex, "{\"type\":\"session_meta\",\"payload\":{\"id\":\"codex-thread\",\"session_id\":\"root-session\",\"cwd\":\"/repo\",\"parent_thread_id\":\"codex-parent\"}}\n")
	metadata, warnings = jsonlMetadata("codex", codex, codex)
	if metadata.ID != "codex-thread" || metadata.ParentID != "codex-parent" || metadata.CWD != "/repo" || len(warnings) != 0 {
		t.Fatalf("unexpected Codex metadata: %+v, warnings: %+v", metadata, warnings)
	}
}

func TestPiCollectUsesLatestSessionName(t *testing.T) {
	for _, test := range []struct {
		name, records, title string
	}{
		{"unnamed", "", ""},
		{"named", "{\"type\":\"session_info\",\"name\":\"Fix login\"}\n", "Fix login"},
		{"renamed", "{\"type\":\"session_info\",\"name\":\"Old\"}\n{\"type\":\"message\",\"name\":\"Ignore\"}\n{\"type\":\"session_info\",\"name\":\" New title \"}\n", "New title"},
		{"cleared", "{\"type\":\"session_info\",\"name\":\"Old\"}\n{\"type\":\"session_info\",\"name\":\"\"}\n", ""},
		{"missing name clears", "{\"type\":\"session_info\",\"name\":\"Old\"}\n{\"type\":\"session_info\"}\n", ""},
		{"partial tail", "{\"type\":\"session_info\",\"name\":\"Keep\"}\n{\"type\":\"session_info\",\"name\":\"Partial\"}", "Keep"},
		{"invalid record", "not json\n{\"type\":\"session_info\",\"name\":\"Keep\"}\n", "Keep"},
		{"large message", "{\"type\":\"message\",\"text\":\"" + strings.Repeat("x", 17<<20) + "\"}\n{\"type\":\"session_info\",\"name\":\"After image\"}\n", "After image"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "session.jsonl")
			content := "{\"type\":\"session\",\"version\":3,\"id\":\"stable-id\"}\n" + test.records
			writeTestFile(t, path, content)
			adapter := jsonlAdapter{name: "pi", format: "pi-jsonl", defaultRoots: func() []string { return nil }}
			result, err := adapter.Collect(context.Background(), []string{path})
			if err != nil {
				t.Fatal(err)
			}
			defer result.Cleanup()
			if len(result.Inputs) != 1 || result.Inputs[0].Descriptor.Title != test.title || result.Inputs[0].Descriptor.NativeTraceID != "stable-id" {
				t.Fatalf("collected descriptors: %+v; want title %q", result.Inputs, test.title)
			}
			original, err := os.ReadFile(path)
			if err != nil || string(original) != content {
				t.Fatalf("source changed: %v", err)
			}
		})
	}
}

func TestJSONLMetadataFallsBackToFilenameNotMessageID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stable-session.jsonl")
	writeTestFile(t, path, "{\"uuid\":\"message-id\",\"id\":\"other-message-id\"}\n")
	metadata, warnings := jsonlMetadata("claude-code", path, path)
	if metadata.ID != "stable-session" || len(warnings) != 1 {
		t.Fatalf("unexpected fallback: %+v, warnings: %+v", metadata, warnings)
	}
}

func TestSnapshotUsesInitialLengthAndHonorsCancellation(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.jsonl")
	destination := filepath.Join(dir, "snapshot.jsonl")
	line := strings.Repeat("x", 1024) + "\n"
	file, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	for range 32 * 1024 {
		if _, err := file.WriteString(line); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	initial, _ := os.Stat(source)
	watchContext, stopWatching := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopWatching()
	appended := appendWhenCreated(watchContext, destination, source, "{\"late\":true}\n")
	_, changed, _, _, err := snapshotCompleteLines(context.Background(), source, destination)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-appended:
		if err != nil {
			t.Fatal(err)
		}
	case <-watchContext.Done():
		t.Fatal(watchContext.Err())
	}
	snapshot, _ := os.Stat(destination)
	if snapshot.Size() != initial.Size() || !changed {
		t.Fatalf("snapshot size %d, initial size %d, changed %v", snapshot.Size(), initial.Size(), changed)
	}
	partialSource := filepath.Join(dir, "partial-source.jsonl")
	partialSnapshot := filepath.Join(dir, "partial-snapshot.jsonl")
	writeTestFile(t, partialSource, "{\"complete\":true}\n{\"partial\":")
	complete, _, _, _, err := snapshotCompleteLines(context.Background(), partialSource, partialSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	partialData, _ := os.ReadFile(partialSnapshot)
	if complete || string(partialData) != "{\"complete\":true}\n" {
		t.Fatalf("complete %v, snapshot %q", complete, partialData)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, _, _, err = snapshotCompleteLines(canceled, source, filepath.Join(dir, "canceled.jsonl"))
	if err != context.Canceled {
		t.Fatalf("got cancellation error %v", err)
	}
}

func TestCopyFileUsesInitialLengthAndHonorsCancellation(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.bin")
	destination := filepath.Join(dir, "snapshot.bin")
	initial := []byte(strings.Repeat("x", 8<<20))
	writeTestFile(t, source, string(initial))
	watchContext, stopWatching := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopWatching()
	appended := appendWhenCreated(watchContext, destination, source, "late bytes")
	if err := copyFile(context.Background(), source, destination); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-appended:
		if err != nil {
			t.Fatal(err)
		}
	case <-watchContext.Done():
		t.Fatal(watchContext.Err())
	}
	snapshot, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot) != len(initial) {
		t.Fatalf("copied %d bytes, wanted initial %d", len(snapshot), len(initial))
	}

	cancelSource := filepath.Join(dir, "cancel-source.bin")
	file, err := os.Create(cancelSource)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(128 << 20); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	defer cancel()
	canceledDestination := filepath.Join(dir, "canceled.bin")
	copyResult := make(chan error, 1)
	go func() { copyResult <- copyFile(canceled, cancelSource, canceledDestination) }()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		if info, err := os.Stat(canceledDestination); err == nil && info.Size() > 0 {
			cancel()
			break
		}
		select {
		case err := <-copyResult:
			t.Fatalf("copy ended before cancellation: %v", err)
		case <-deadline.C:
			t.Fatal("copy did not start")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	select {
	case err := <-copyResult:
		if err != context.Canceled {
			t.Fatalf("got cancellation error %v", err)
		}
	case <-deadline.C:
		t.Fatal("copy did not stop after cancellation")
	}
}

func TestSanitizeRemoteCanonicalizesTransportAndRemovesSecrets(t *testing.T) {
	remotes := []string{
		"https://user:secret@GitHub.com/owner/repo.git?token=secret#fragment",
		"ssh://git@github.com:22/owner/repo.git",
		"git@github.com:owner/repo.git",
	}
	for _, remote := range remotes {
		if got := sanitizeRemote(remote); got != "github.com/owner/repo" {
			t.Errorf("sanitizeRemote(%q) = %q", remote, got)
		}
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func appendWhenCreated(ctx context.Context, destination, source, contents string) <-chan error {
	result := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			if _, err := os.Stat(destination); err == nil {
				file, err := os.OpenFile(source, os.O_APPEND|os.O_WRONLY, 0)
				if err == nil {
					_, err = file.WriteString(contents)
					if closeErr := file.Close(); err == nil {
						err = closeErr
					}
				}
				result <- err
				return
			} else if !os.IsNotExist(err) {
				result <- err
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return result
}
