package adapters

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func piAdapter(t *testing.T) Adapter {
	t.Helper()
	for _, adapter := range All() {
		if adapter.Name() == "pi" {
			return adapter
		}
	}
	t.Fatal("pi adapter is not registered")
	return nil
}

func isolatePiHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv("BB_PI_BRIDGE_SESSION_DIR", "")
	return home
}

func TestPiRootsAddBBBridgeOnlyWhenItExists(t *testing.T) {
	home := isolatePiHome(t)
	native := filepath.Join(home, ".pi", "agent", "sessions")
	if got := piRoots(); !reflect.DeepEqual(got, []string{native}) {
		t.Fatalf("roots without a bb bridge: %q", got)
	}

	bridge := filepath.Join(home, ".bb", "pi-bridge-sessions")
	if err := os.MkdirAll(bridge, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := piRoots(); !reflect.DeepEqual(got, []string{native, bridge}) {
		t.Fatalf("roots with the default bb bridge: %q", got)
	}

	agentDir := filepath.Join(home, "custom-pi")
	custom := filepath.Join(home, "custom-bridge")
	if err := os.MkdirAll(custom, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PI_CODING_AGENT_DIR", agentDir)
	t.Setenv("BB_PI_BRIDGE_SESSION_DIR", " "+custom+" ")
	if got := piRoots(); !reflect.DeepEqual(got, []string{filepath.Join(agentDir, "sessions"), custom}) {
		t.Fatalf("roots with overrides: %q", got)
	}

	t.Setenv("BB_PI_BRIDGE_SESSION_DIR", filepath.Join(home, "missing"))
	if got := piRoots(); !reflect.DeepEqual(got, []string{filepath.Join(agentDir, "sessions")}) {
		t.Fatalf("roots with a missing bridge override: %q", got)
	}
}

func TestPiCollectsBBBridgeSessionsWithDefaultConfig(t *testing.T) {
	home := isolatePiHome(t)
	workspace := filepath.Join(home, "workspaces", "thr_qamf4ey949")
	bridge := filepath.Join(home, ".bb", "pi-bridge-sessions", "pi_7f01ff8d-c831-4356-a637-ec309c593fe3.jsonl")
	writeTestFile(t, bridge, `{"type":"session","version":3,"id":"01a0eb21-7e07-7124-87a1-0d302eb5180a","timestamp":"2026-09-29T03:07:15.848Z","cwd":"`+workspace+`"}
{"type":"model_change","provider":"llm","modelId":"model"}
{"type":"message","message":{"role":"user","content":"hello"}}
`)
	native := filepath.Join(home, ".pi", "agent", "sessions", "--repo--", "2026-09-28T00-00-00-000Z_native-id.jsonl")
	writeTestFile(t, native, `{"type":"session","version":3,"id":"native-id","cwd":"/repo"}
`)

	adapter := piAdapter(t)
	if source := adapter.Discover(context.Background(), nil); source.Traces != 2 {
		t.Fatalf("discovered %+v; want the native and bridge sessions", source)
	}
	result, err := adapter.Collect(context.Background(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Cleanup()
	byID := map[string]string{}
	for _, input := range result.Inputs {
		if input.Descriptor.Adapter != "pi-jsonl" || len(input.Descriptor.Warnings) != 0 {
			t.Fatalf("unexpected descriptor: %+v", input.Descriptor)
		}
		byID[input.Descriptor.NativeTraceID] = input.Descriptor.WorkingDirectory
	}
	want := map[string]string{"01a0eb21-7e07-7124-87a1-0d302eb5180a": workspace, "native-id": "/repo"}
	if !reflect.DeepEqual(byID, want) || len(result.Warnings) != 0 {
		t.Fatalf("collected %v with warnings %+v; want %v", byID, result.Warnings, want)
	}
}

func TestPiCollectsOneTraceWhenSessionIsInNativeStoreAndBBBridge(t *testing.T) {
	home := isolatePiHome(t)
	header := `{"type":"session","version":3,"id":"shared-id","cwd":"/repo"}` + "\n"
	native := filepath.Join(home, ".pi", "agent", "sessions", "--repo--", "2026-09-28T00-00-00-000Z_shared-id.jsonl")
	bridge := filepath.Join(home, ".bb", "pi-bridge-sessions", "pi_thread.jsonl")
	writeTestFile(t, native, header+`{"type":"message","message":{"role":"user","content":"native"}}`+"\n")
	writeTestFile(t, bridge, header+`{"type":"message","message":{"role":"user","content":"bridge"}}`+"\n")

	older := time.Now().Add(-time.Hour)
	for _, test := range []struct {
		name, stale, want string
	}{
		{"bridge is newer", native, bridge},
		{"native is newer", bridge, native},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := time.Now()
			if err := os.Chtimes(test.want, now, now); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(test.stale, older, older); err != nil {
				t.Fatal(err)
			}
			adapter := piAdapter(t)
			if source := adapter.Discover(context.Background(), nil); source.Traces != 1 {
				t.Fatalf("discovered %+v; want one trace", source)
			}
			result, err := adapter.Collect(context.Background(), nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer result.Cleanup()
			if len(result.Inputs) != 1 || result.Inputs[0].Descriptor.NativeTraceID != "shared-id" || result.Inputs[0].Source != test.want {
				t.Fatalf("collected %+v; want only %s", result.Inputs, test.want)
			}
		})
	}
}

func TestUniquePiSessionsKeepsFilesWithoutHeaders(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "a", "same-name.jsonl")
	second := filepath.Join(dir, "b", "same-name.jsonl")
	unreadable := filepath.Join(dir, "c", "unreadable.jsonl")
	writeTestFile(t, first, `{"type":"message","id":"message-id"}`+"\n")
	writeTestFile(t, second, `{"type":"message","id":"message-id"}`+"\n")
	writeTestFile(t, unreadable, `{"type":"session","id":"hidden"}`+"\n")
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(unreadable, 0o600) })
	files := []string{first, second, unreadable}
	if got := uniquePiSessions(files); !reflect.DeepEqual(got, files) {
		t.Fatalf("unique dropped header-less files: %q", got)
	}
}
