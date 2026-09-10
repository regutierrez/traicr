package collector_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/regutierrez/traicr/internal/archive"
	"github.com/regutierrez/traicr/internal/collector"
	"github.com/regutierrez/traicr/internal/config"
	"github.com/regutierrez/traicr/internal/domain"
)

func TestCollectCreatesValidArchiveAndDoesNotAdvanceState(t *testing.T) {
	output := t.TempDir()
	cfg := config.Collector{MachineID: "machine-one", State: map[string]config.CollectionRevision{}}
	result, err := collector.Collect(context.Background(), cfg, collector.CollectOptions{
		Harnesses: []string{"pi"},
		Sources:   map[string][]string{"pi": {filepath.Join("..", "..", "testdata", "collector", "pi")}},
		OutputDir: output,
		Version:   "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Archives) != 1 {
		t.Fatalf("got %d archives, want 1", len(result.Archives))
	}
	validated := validateArchive(t, result.Archives[0])
	if len(validated.Manifest.Traces) != 5 {
		t.Fatalf("got %d traces, want 5", len(validated.Manifest.Traces))
	}
	if len(cfg.State) != 0 {
		t.Fatal("collection advanced acknowledgement state")
	}

	acknowledged := validated.Manifest.Traces[0]
	cfg.State[collector.StateKey(acknowledged.Harness, acknowledged.NativeTraceID)] = config.CollectionRevision{Digest: acknowledged.RevisionDigest, MachineID: cfg.MachineID}
	second, err := collector.Collect(context.Background(), cfg, collector.CollectOptions{
		Harnesses: []string{"pi"},
		Sources:   map[string][]string{"pi": {filepath.Join("..", "..", "testdata", "collector", "pi")}},
		OutputDir: t.TempDir(),
		Version:   "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Archives) != 1 || len(validateArchive(t, second.Archives[0]).Manifest.Traces) != 4 {
		t.Fatal("acknowledged revision was not excluded")
	}
}

func TestCollectReportsProgressPerHarnessTraceAndArchive(t *testing.T) {
	var events []string
	cfg := config.Collector{MachineID: "machine-one", State: map[string]config.CollectionRevision{}}
	result, err := collector.Collect(context.Background(), cfg, collector.CollectOptions{
		Harnesses: []string{"pi"},
		Sources:   map[string][]string{"pi": {filepath.Join("..", "..", "testdata", "collector", "pi")}},
		OutputDir: t.TempDir(),
		Progress: func(phase, harness string, completed, total int) {
			events = append(events, fmt.Sprintf("%s %s %d/%d", phase, harness, completed, total))
		},
	})
	if err != nil || len(result.Archives) != 1 {
		t.Fatalf("collect: %+v, %v", result, err)
	}
	want := []string{"collecting pi 0/0", "describing pi 1/5", "describing pi 2/5", "describing pi 3/5", "describing pi 4/5", "describing pi 5/5", "archiving  0/5"}
	if strings.Join(events, "\n") != strings.Join(want, "\n") {
		t.Fatalf("progress events:\n%s\nwant:\n%s", strings.Join(events, "\n"), strings.Join(want, "\n"))
	}
}

func TestCollectDropsRepeatedTraceIdentitiesSoArchivesValidate(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	for _, dir := range []string{first, second} {
		if err := os.WriteFile(filepath.Join(dir, "session.jsonl"), []byte("{\"type\":\"session\",\"version\":3,\"id\":\"shared-id\"}\n{\"type\":\"message\",\"text\":\""+filepath.Base(dir)+"\"}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Collector{MachineID: "machine-one", State: map[string]config.CollectionRevision{}}
	result, err := collector.Collect(context.Background(), cfg, collector.CollectOptions{
		Harnesses: []string{"pi"},
		Sources:   map[string][]string{"pi": {first, second}},
		OutputDir: t.TempDir(),
	})
	if err != nil || len(result.Archives) != 1 {
		t.Fatalf("collect: %+v, %v", result, err)
	}
	if len(result.Warnings) != 1 || result.Warnings[0].Code != "duplicate_trace" {
		t.Fatalf("warnings: %+v", result.Warnings)
	}
	if traces := validateArchive(t, result.Archives[0]).Manifest.Traces; len(traces) != 1 || traces[0].NativeTraceID != "shared-id" {
		t.Fatalf("archive traces: %+v", traces)
	}
}

func TestCollectAllBackfillsPiTitleWithoutChangingRevision(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "session.jsonl")
	content := "{\"type\":\"session\",\"version\":3,\"id\":\"pi-named\"}\n{\"type\":\"session_info\",\"id\":\"name-entry\",\"parentId\":null,\"name\":\"Fix login\"}\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Collector{MachineID: "machine-one", State: map[string]config.CollectionRevision{}}
	options := collector.CollectOptions{Harnesses: []string{"pi"}, Sources: map[string][]string{"pi": {path}}, OutputDir: t.TempDir()}
	first, err := collector.Collect(ctx, cfg, options)
	if err != nil || len(first.Archives) != 1 {
		t.Fatalf("initial collection: %+v, %v", first, err)
	}
	descriptor := validateArchive(t, first.Archives[0]).Manifest.Traces[0]
	cfg.State[collector.StateKey("pi", "pi-named")] = config.CollectionRevision{Digest: descriptor.RevisionDigest, MachineID: cfg.MachineID}
	options.OutputDir = t.TempDir()
	skipped, err := collector.Collect(ctx, cfg, options)
	if err != nil || len(skipped.Archives) != 0 {
		t.Fatalf("acknowledged collection: %+v, %v", skipped, err)
	}
	options.All = true
	backfill, err := collector.Collect(ctx, cfg, options)
	if err != nil || len(backfill.Archives) != 1 {
		t.Fatalf("backfill collection: %+v, %v", backfill, err)
	}
	actual := validateArchive(t, backfill.Archives[0]).Manifest.Traces[0]
	if actual.Title != "Fix login" || actual.NativeTraceID != descriptor.NativeTraceID || actual.RevisionDigest != descriptor.RevisionDigest {
		t.Fatalf("backfill changed identity or lost title: %+v", actual)
	}
}

func TestUploadAdvancesOnlyAcknowledgedOutcomes(t *testing.T) {
	cfg := config.Collector{MachineID: "machine-one", State: map[string]config.CollectionRevision{}}
	collected, err := collector.Collect(context.Background(), cfg, collector.CollectOptions{
		Harnesses: []string{"pi"},
		Sources:   map[string][]string{"pi": {filepath.Join("..", "..", "testdata", "collector", "pi")}},
		OutputDir: t.TempDir(),
		All:       true,
		Version:   "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest := validateArchive(t, collected.Archives[0]).Manifest
	statuses := []string{"imported", "updated", "unchanged", "partially_parsed", "failed"}
	outcomes := make([]domain.TraceOutcome, len(manifest.Traces))
	for index, descriptor := range manifest.Traces {
		outcomes[index] = domain.TraceOutcome{Harness: descriptor.Harness, NativeTraceID: descriptor.NativeTraceID, RevisionDigest: descriptor.RevisionDigest, Status: statuses[index]}
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Error("missing bearer token")
		}
		if request.URL.Path != "/api/v1/imports" {
			t.Errorf("unexpected path %s", request.URL.Path)
		}
		io.Copy(io.Discard, request.Body)
		report := domain.ImportReport{Machine: manifest.SourceMachine, Imported: 1, Updated: 1, Unchanged: 1, Partial: 1, Failed: 1, Traces: outcomes}
		json.NewEncoder(response).Encode(domain.Progress{Phase: "validating"})
		json.NewEncoder(response).Encode(domain.Progress{Phase: "complete", Report: &report})
	}))
	defer server.Close()
	cfg.ServerURL = server.URL
	cfg.Token = "secret"
	configPath := filepath.Join(t.TempDir(), "collector.json")
	if _, err := collector.Upload(context.Background(), server.Client(), &cfg, configPath, collected.Archives, nil); err != nil {
		t.Fatal(err)
	}
	if len(cfg.State) != 3 {
		t.Fatalf("got %d acknowledged revisions, want 3", len(cfg.State))
	}
	if cfg.State[collector.StateKey(outcomes[0].Harness, outcomes[0].NativeTraceID)].Digest != outcomes[0].RevisionDigest {
		t.Fatal("imported revision was not acknowledged")
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), outcomes[0].RevisionDigest) || !strings.Contains(string(data), outcomes[1].RevisionDigest) || !strings.Contains(string(data), outcomes[2].RevisionDigest) || strings.Contains(string(data), outcomes[3].RevisionDigest) {
		t.Fatal("saved state does not match successful outcomes")
	}
}

func TestUploadRejectsFailedOrIncompleteResponseWithoutChangingState(t *testing.T) {
	cfg := config.Collector{MachineID: "machine-one", State: map[string]config.CollectionRevision{}}
	collected, err := collector.Collect(context.Background(), cfg, collector.CollectOptions{
		Harnesses: []string{"pi"}, Sources: map[string][]string{"pi": {filepath.Join("..", "..", "testdata", "collector", "pi")}}, OutputDir: t.TempDir(), All: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		body    string
		message string
	}{
		{name: "incomplete", body: "{\"phase\":\"validating\"}\n", message: "without a complete report"},
		{name: "failed", body: "{\"phase\":\"failed\",\"error\":\"archive rejected\"}\n", message: "archive rejected"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				io.Copy(io.Discard, request.Body)
				io.WriteString(response, test.body)
			}))
			defer server.Close()
			current := config.Collector{MachineID: cfg.MachineID, ServerURL: server.URL, Token: "secret", State: map[string]config.CollectionRevision{}}
			configPath := filepath.Join(t.TempDir(), "collector.json")
			_, err := collector.Upload(context.Background(), server.Client(), &current, configPath, collected.Archives, nil)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("got error %v", err)
			}
			if len(current.State) != 0 {
				t.Fatal("rejected response changed state")
			}
			if _, err := os.Stat(configPath); !os.IsNotExist(err) {
				t.Fatal("rejected response saved acknowledgement state")
			}
		})
	}
}

func TestUploadAcceptsForeignMachineArchiveWithoutChangingLocalState(t *testing.T) {
	foreign := config.Collector{MachineID: "foreign-machine", State: map[string]config.CollectionRevision{}}
	collected, err := collector.Collect(context.Background(), foreign, collector.CollectOptions{
		Harnesses: []string{"pi"}, Sources: map[string][]string{"pi": {filepath.Join("..", "..", "testdata", "collector", "pi")}}, OutputDir: t.TempDir(), All: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest := validateArchive(t, collected.Archives[0]).Manifest
	outcomes := make([]domain.TraceOutcome, len(manifest.Traces))
	for index, descriptor := range manifest.Traces {
		outcomes[index] = domain.TraceOutcome{Harness: descriptor.Harness, NativeTraceID: descriptor.NativeTraceID, RevisionDigest: descriptor.RevisionDigest, Status: "unchanged"}
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		io.Copy(io.Discard, request.Body)
		report := domain.ImportReport{Machine: manifest.SourceMachine, Unchanged: len(outcomes), Traces: outcomes}
		json.NewEncoder(response).Encode(domain.Progress{Phase: "complete", Report: &report})
	}))
	defer server.Close()
	local := config.Collector{MachineID: "local-machine", ServerURL: server.URL, Token: "secret", State: map[string]config.CollectionRevision{}}
	configPath := filepath.Join(t.TempDir(), "collector.json")
	reports, err := collector.Upload(context.Background(), server.Client(), &local, configPath, collected.Archives, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || len(local.State) != 0 {
		t.Fatalf("reports %d, local state %+v", len(reports), local.State)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatal("foreign archive wrote local acknowledgement state")
	}
}

func validateArchive(t *testing.T, path string) *archive.Validated {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	validated, err := archive.Validate(context.Background(), file, info.Size(), archive.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	return validated
}
