package adapters

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestAmpCollectArchivesImagesWithoutChangingExport(t *testing.T) {
	const imageURL = "https://ampcode.com/user-content/attachments/fixture.png"
	const export = `{"v":1,"id":"T-images","updatedAt":"2026-09-16T00:00:00Z","messages":[{"role":"user","content":[{"type":"image","source":{"type":"url","url":"` + imageURL + `#amp-media-width=320"}},{"type":"tool_result","run":{"progress":{"displayImages":[{"type":"image","url":"` + imageURL + `#amp-media-width=640"},{"type":"image","url":"https://ampcode.com/user-content/attachments/missing.png"}]}}},{"type":"image","sourcePath":"https://ampcode.com/user-content/attachments/path.png"},{"type":"image","url":"https://example.com/private.png"},{"type":"image","url":"https://ampcode.com/user-content/attachments/../../secret"},{"type":"image","url":"file:///private.png"}]}]}`
	if os.Getenv("TRAICR_TEST_AMP_IMAGES") == "1" {
		args := os.Args[slices.Index(os.Args, "--")+1:]
		switch args[0] + " " + args[1] {
		case "threads list":
			fmt.Print(`[{"id":"T-images","updated":"2026-09-16T00:00:00Z"}]`)
		case "threads export":
			fmt.Print(export)
		case "files get":
			if len(args) != 5 || args[3] != "-o" {
				os.Exit(2)
			}
			log, _ := os.OpenFile(os.Getenv("TRAICR_IMAGE_REQUESTS"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			fmt.Fprintln(log, args[2])
			log.Close()
			os.WriteFile(args[4], []byte("original image bytes"), 0600)
			if args[2] == "https://ampcode.com/user-content/attachments/missing.png" {
				os.Exit(1)
			}
		default:
			os.Exit(2)
		}
		os.Exit(0)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	command := filepath.Join(dir, "amp")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nexec \"$TRAICR_TEST_BINARY\" -test.run=^TestAmpCollectArchivesImagesWithoutChangingExport$ -- \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRAICR_TEST_AMP_IMAGES", "1")
	t.Setenv("TRAICR_TEST_BINARY", binary)
	t.Setenv("TRAICR_IMAGE_REQUESTS", filepath.Join(dir, "requests"))
	result, err := (commandAdapter{name: "amp", format: "amp-thread-export", executable: command}).Collect(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Cleanup()
	if len(result.Inputs) != 1 {
		t.Fatalf("lost export: %+v", result)
	}
	root := result.Inputs[0].Directory
	data, err := os.ReadFile(filepath.Join(root, "source/export.json"))
	if err != nil || string(data) != export {
		t.Fatalf("export changed: %v", err)
	}
	for _, url := range []string{imageURL, "https://ampcode.com/user-content/attachments/path.png"} {
		path := fmt.Sprintf("source/attachments/%x", sha256.Sum256([]byte(url)))
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || string(data) != "original image bytes" {
			t.Fatalf("image not archived: %q %v", data, err)
		}
	}
	files, err := os.ReadDir(filepath.Join(root, "source/attachments"))
	if err != nil || len(files) != 2 {
		t.Fatalf("partial download retained: %v %v", files, err)
	}
	requests, err := os.ReadFile(filepath.Join(dir, "requests"))
	if err != nil {
		t.Fatal(err)
	}
	urls := strings.Fields(string(requests))
	slices.Sort(urls)
	if !slices.Equal(urls, []string{imageURL, "https://ampcode.com/user-content/attachments/missing.png", "https://ampcode.com/user-content/attachments/path.png"}) {
		t.Fatalf("duplicates or unsafe requests: %q %v", requests, err)
	}
	if len(result.Warnings) != 1 || result.Warnings[0].Code != "attachment_download_failed" || len(result.Inputs[0].Descriptor.Warnings) != 1 {
		t.Fatalf("missing download warning: %+v", result)
	}
}

func TestAmpImageDownloadBudgetAndCancellation(t *testing.T) {
	dir := t.TempDir()
	command := filepath.Join(dir, "amp")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nexec head -c \"$TRAICR_IMAGE_BYTES\" /dev/zero > \"$5\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{1023, 1024, 1025} {
		t.Setenv("TRAICR_IMAGE_BYTES", strconv.Itoa(size))
		path := filepath.Join(dir, strconv.Itoa(size))
		written, err := downloadAmpImage(context.Background(), command, "https://ampcode.com/user-content/attachments/fixture.png", path, 1024)
		if size <= 1024 {
			if err != nil || written != int64(size) {
				t.Fatalf("image within budget rejected: %d %v", written, err)
			}
		} else if _, statErr := os.Stat(path); err == nil || !os.IsNotExist(statErr) {
			t.Fatalf("oversized download retained: %v %v", err, statErr)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path := filepath.Join(dir, "cancelled")
	if _, err := downloadAmpImage(ctx, command, "https://ampcode.com/user-content/attachments/fixture.png", path, 1024); err == nil {
		t.Fatal("cancelled download succeeded")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("cancelled file retained: %v", err)
	}
}

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
			fmt.Print(`{"v":1,"id":"T-1","title":"Thread","updatedAt":"2026-09-10T23:07:25.917Z","messages":[],"env":{"initial":{"workingDirectory":"/unavailable/repo","trees":[{"uri":"file:///exported/repo","repository":{"url":"https://github.com/example/project","ref":"main","sha":"fixture-commit","type":"git"}}]}}}`)
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
	if repository := result.Inputs[0].Descriptor.Repository; repository.Root != "/exported/repo" || repository.Remote != "https://github.com/example/project" {
		t.Fatalf("Amp exported repository: %+v", repository)
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
