package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/regutierrez/traicr/internal/collector"
)

func TestLoginReadsTokenFromInputWithoutPrintingIt(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TRAICR_CONFIG_DIR", dir)
	t.Setenv("TRAICR_ADMIN_TOKEN", "")
	var stdout, stderr bytes.Buffer
	if err := run(context.Background(), []string{"login", "https://traicr.example"}, strings.NewReader("very-secret\n"), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String()+stderr.String(), "very-secret") {
		t.Fatal("login printed the token")
	}
	info, err := os.Stat(filepath.Join(dir, "collector.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("configuration mode is %o, want 600", info.Mode().Perm())
	}
}

func TestCollectStatusLeavesOneLinePerHarness(t *testing.T) {
	var stderr bytes.Buffer
	clock := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	status := &collectStatus{statusLine: statusLine{out: &stderr}, now: func() time.Time { return clock }}
	status.report(collector.Progress{Phase: "collecting", Harness: "amp"})
	status.report(collector.Progress{Phase: "collecting", Harness: "amp", Completed: 1, Total: 485})
	clock = clock.Add(133 * time.Second)
	status.report(collector.Progress{Phase: "describing", Harness: "amp", Completed: 485, Total: 485})
	status.report(collector.Progress{Phase: "collected", Harness: "amp", Completed: 3, Total: 485})
	status.report(collector.Progress{Phase: "collecting", Harness: "opencode"})
	status.report(collector.Progress{Phase: "collected", Harness: "opencode"})
	status.report(collector.Progress{Phase: "archiving", Total: 3})
	status.report(collector.Progress{Phase: "archived", Completed: 3, Total: 1})
	status.finish()
	var lines []string
	for _, line := range strings.Split(stderr.String(), "\n") {
		frames := strings.Split(line, "\r")
		lines = append(lines, strings.TrimSpace(frames[len(frames)-1]))
	}
	want := []string{"amp: 485 traces found, 3 new or changed (2m13s)", "opencode: no traces found (0s)", "wrote 3 traces to 1 archive(s)", ""}
	if strings.Join(lines, "|") != strings.Join(want, "|") {
		t.Fatalf("status lines %q, want %q", lines, want)
	}
	if !strings.Contains(stderr.String(), "\ramp: collecting 1/485") || !strings.Contains(stderr.String(), "\ramp: describing 485/485") {
		t.Fatalf("missing in-progress frames: %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "\n\n") {
		t.Fatalf("finish added a blank line after a persisted line: %q", stderr.String())
	}
}

func TestUploadStatusShowsServerImportProgress(t *testing.T) {
	var stderr bytes.Buffer
	status := &uploadStatus{statusLine: statusLine{out: &stderr}}
	status.report(collector.UploadStatus{File: "/tmp/a.zip", Phase: "sending", Sent: 50, Size: 100})
	status.report(collector.UploadStatus{File: "/tmp/a.zip", Phase: "sending", Sent: 100, Size: 100})
	status.report(collector.UploadStatus{File: "/tmp/a.zip", Phase: "validating"})
	status.report(collector.UploadStatus{File: "/tmp/a.zip", Phase: "normalizing", Completed: 812, Total: 2326})
	status.report(collector.UploadStatus{File: "/tmp/a.zip", Phase: "complete", Completed: 2326, Total: 2326})
	out := stderr.String()
	for _, frame := range []string{
		"\ra.zip: uploading 50/100 bytes (50.0%)",
		"\ra.zip: uploading 100/100 bytes (100.0%)",
		"\ra.zip: server validating archive...",
		"\ra.zip: server importing 812/2326 traces",
	} {
		if !strings.Contains(out, frame) {
			t.Fatalf("missing frame %q in %q", frame, out)
		}
	}
	if strings.Contains(out, "indexing") {
		t.Fatalf("progress listed dead indexing phase: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("expected one line ending after completion: %q", out)
	}
}

func TestRootHelpPrintsUsage(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}} {
		var stdout, stderr bytes.Buffer
		if err := run(t.Context(), args, strings.NewReader(""), &stdout, &stderr); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !strings.Contains(stderr.String(), usage) {
			t.Fatalf("%v stderr %q", args, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Fatalf("%v wrote stdout %q", args, stdout.String())
		}
	}
}

func TestEmptyArgsAndUnknownCommandShareUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"nope"}} {
		err := run(t.Context(), args, strings.NewReader(""), io.Discard, io.Discard)
		if err == nil || err.Error() != usage {
			t.Fatalf("%v: got %v", args, err)
		}
	}
}

func TestCollectHelpPrintsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run(t.Context(), []string{"collect", "-h"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "usage: traicr collect --output DIR") {
		t.Fatalf("stderr %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout %q", stdout.String())
	}
}

func TestCollectUnknownFlagReturnsFlagError(t *testing.T) {
	var stderr bytes.Buffer
	err := run(t.Context(), []string{"collect", "-bogus"}, strings.NewReader(""), io.Discard, &stderr)
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined: -bogus") {
		t.Fatalf("got %v", err)
	}
}
