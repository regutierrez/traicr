//go:build scale

package scale_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/regutierrez/traicr/internal/archive"
	"github.com/regutierrez/traicr/internal/domain"
	"github.com/regutierrez/traicr/internal/normalize"
	"github.com/regutierrez/traicr/internal/store"
)

const eventsPerTrace = 1_000

type scaleConfig struct {
	events          int
	attachmentBytes int64
	splitBytes      int64
}

func TestArchiveImportSearchAndDeleteAtScale(t *testing.T) {
	config := loadConfig(t)
	started := time.Now()
	root := t.TempDir()
	stopHeap := sampleHeap()

	inputs, manifest := generateCorpus(t, root, config)
	writeStarted := time.Now()
	archives, err := archive.Write(context.Background(), filepath.Join(root, "archives"), manifest, inputs, config.splitBytes)
	if err != nil {
		t.Fatal(err)
	}
	writeDuration := time.Since(writeStarted)
	if len(archives) < 2 && config.attachmentBytes > config.splitBytes {
		t.Fatalf("archive split produced %d archive; want at least 2", len(archives))
	}

	database, err := store.Open(filepath.Join(root, "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	seenTraces := make(map[string]bool, len(inputs))
	traceIDs := make(map[string]int64, len(inputs))
	var importedEvents int
	importStarted := time.Now()
	for _, path := range archives {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := file.Stat()
		if err != nil {
			file.Close()
			t.Fatal(err)
		}
		validated, err := archive.Validate(context.Background(), file, info.Size(), archive.DefaultLimits())
		if err != nil {
			file.Close()
			t.Fatal(err)
		}
		for _, descriptor := range validated.Manifest.Traces {
			if seenTraces[descriptor.NativeTraceID] {
				file.Close()
				t.Fatalf("trace %q was split across archives", descriptor.NativeTraceID)
			}
			seenTraces[descriptor.NativeTraceID] = true
		}
		report, err := database.Import(context.Background(), validated.Manifest, validated.ZIP, normalize.Run, nil)
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if report.Failed != 0 || report.Partial != 0 || report.Unsupported != 0 {
			t.Fatalf("unexpected import report: %+v", report)
		}
		for _, outcome := range report.Traces {
			traceIDs[outcome.NativeTraceID] = outcome.TraceID
		}
	}
	importDuration := time.Since(importStarted)
	if len(seenTraces) != len(inputs) {
		t.Fatalf("validated %d traces; want %d", len(seenTraces), len(inputs))
	}

	for _, traceID := range traceIDs {
		cursor := ""
		for {
			page, err := database.Events(context.Background(), traceID, cursor, 200)
			if err != nil {
				t.Fatal(err)
			}
			importedEvents += len(page.Events)
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
	}
	if importedEvents != config.events {
		t.Fatalf("stored %d events; want %d", importedEvents, config.events)
	}

	attachmentID := traceIDs["scale-000000"]
	trace, err := database.Trace(context.Background(), attachmentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(trace.Revisions) != 1 {
		t.Fatalf("attachment trace has %d revisions; want 1", len(trace.Revisions))
	}
	sources, err := database.RevisionSources(context.Background(), trace.Revisions[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	assertRetainedSources(t, sources, inputs[0].Descriptor)

	latencies := exerciseSearchModes(t, database)
	assertSearchPagination(t, database)
	assertCancellation(t, archives[0], database)

	dataDir := filepath.Join(root, "data")
	diskBeforeDelete := treeBytes(t, dataDir)
	objectsBeforeDelete := treeBytes(t, filepath.Join(dataDir, "objects"))
	retainedID := traceIDs["scale-000001"]
	exerciseConcurrentReadsAndDelete(t, database, retainedID, attachmentID)
	if _, err := database.Trace(context.Background(), attachmentID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted trace lookup returned %v; want sql.ErrNoRows", err)
	}
	objectsAfterDelete := treeBytes(t, filepath.Join(dataDir, "objects"))
	if objectsAfterDelete > objectsBeforeDelete-config.attachmentBytes {
		t.Fatalf("delete retained attachment bytes: objects before=%d after=%d attachment=%d", objectsBeforeDelete, objectsAfterDelete, config.attachmentBytes)
	}

	peakHeap := stopHeap()
	diskAfterDelete := treeBytes(t, dataDir)
	archiveBytes := treeBytes(t, filepath.Join(root, "archives"))
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	if config.attachmentBytes >= 1<<30 && peakHeap >= uint64(config.attachmentBytes) {
		t.Fatalf("peak Go heap %d bytes grew to attachment size %d", peakHeap, config.attachmentBytes)
	}
	t.Logf("scale events=%d attachment_bytes=%d archives=%d archive_bytes=%d split_bytes=%d", config.events, config.attachmentBytes, len(archives), archiveBytes, config.splitBytes)
	t.Logf("runtime total=%s archive_write=%s import=%s throughput=%.0f_events/s", time.Since(started), writeDuration, importDuration, float64(config.events)/importDuration.Seconds())
	t.Logf("disk_before_delete=%d disk_after_delete=%d objects_before_delete=%d objects_after_delete=%d database=%d wal=%d", diskBeforeDelete, diskAfterDelete, objectsBeforeDelete, objectsAfterDelete, fileBytes(t, filepath.Join(dataDir, "traicr.db")), fileBytes(t, filepath.Join(dataDir, "traicr.db-wal")))
	t.Logf("go_heap_alloc=%d go_heap_sys=%d peak_go_heap_alloc=%d", memory.HeapAlloc, memory.HeapSys, peakHeap)
	t.Logf("search_latency fulltext=%s exact=%s regex=%s", latencies["fulltext"], latencies["exact"], latencies["regex"])
}

func loadConfig(t *testing.T) scaleConfig {
	t.Helper()
	config := scaleConfig{
		events:          envInt(t, "TRAICR_SCALE_EVENTS", 1_000_000),
		attachmentBytes: envInt64(t, "TRAICR_SCALE_ATTACHMENT_BYTES", 3<<30),
		splitBytes:      envInt64(t, "TRAICR_SCALE_SPLIT_BYTES", 2_000_000_000),
	}
	if config.events < 2 || config.attachmentBytes < 1 || config.splitBytes < 1 {
		t.Fatal("scale settings must request at least 2 events and positive byte sizes")
	}
	return config
}

func envInt(t *testing.T, name string, fallback int) int {
	t.Helper()
	value, err := strconv.Atoi(envString(name, strconv.Itoa(fallback)))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return value
}

func envInt64(t *testing.T, name string, fallback int64) int64 {
	t.Helper()
	value, err := strconv.ParseInt(envString(name, strconv.FormatInt(fallback, 10)), 10, 64)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return value
}

func envString(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func generateCorpus(t *testing.T, root string, config scaleConfig) ([]archive.Input, domain.Manifest) {
	t.Helper()
	traceCount := (config.events + eventsPerTrace - 1) / eventsPerTrace
	if traceCount < 2 {
		traceCount = 2
	}
	inputs := make([]archive.Input, 0, traceCount)
	remaining := config.events
	for traceNumber := 0; traceNumber < traceCount; traceNumber++ {
		directory := filepath.Join(root, "source", fmt.Sprintf("trace-%06d", traceNumber))
		if err := os.MkdirAll(filepath.Join(directory, "source"), 0o700); err != nil {
			t.Fatal(err)
		}
		count := min(remaining, eventsPerTrace)
		writePiTrace(t, filepath.Join(directory, "source", "records.jsonl"), traceNumber, count)
		remaining -= count
		input := archive.Input{Directory: directory, Descriptor: domain.Descriptor{
			Harness: "pi", Adapter: "pi-v3", NativeTraceID: fmt.Sprintf("scale-%06d", traceNumber),
			NativeUpdatedAt: "2026-01-01T00:00:00Z", Title: fmt.Sprintf("Scale trace %06d", traceNumber),
			Repository: domain.Repository{Remote: "https://example.invalid/scale.git", Root: "/work/scale"},
		}}
		inputs = append(inputs, input)
	}
	attachment := filepath.Join(inputs[0].Directory, "source", "attachments", "large.bin")
	writeAttachment(t, attachment, config.attachmentBytes)
	for i := range inputs {
		descriptor, err := archive.Describe(inputs[i])
		if err != nil {
			t.Fatal(err)
		}
		inputs[i].Descriptor = descriptor
	}
	manifest := domain.Manifest{
		CollectorVersion: "scale-test", CreatedAt: "2026-01-01T00:00:00Z",
		SourceMachine: domain.Machine{ID: "scale-machine", Hostname: "scale-host", OS: "linux", Arch: "amd64"},
	}
	return inputs, manifest
}

func writePiTrace(t *testing.T, path string, traceNumber, count int) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(map[string]any{"type": "session", "version": 3, "id": fmt.Sprintf("scale-%06d", traceNumber)}); err != nil {
		file.Close()
		t.Fatal(err)
	}
	parent := ""
	for eventNumber := 0; eventNumber < count; eventNumber++ {
		id := fmt.Sprintf("event-%06d", eventNumber)
		text := fmt.Sprintf("needlecommon exact/path/trace-%06d/event-%06d regex-token-%06d realistic source text for scale verification", traceNumber, eventNumber, eventNumber)
		record := map[string]any{
			"type": "message", "id": id, "parentId": parent, "timestamp": "2026-01-01T00:00:00Z",
			"message": map[string]any{"role": "user", "model": "scale-model", "content": []any{map[string]any{"type": "text", "text": text}}},
		}
		if err := encoder.Encode(record); err != nil {
			file.Close()
			t.Fatal(err)
		}
		parent = id
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeAttachment(t *testing.T, path string, size int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	block := make([]byte, 1<<20)
	var state uint64 = 0x9e3779b97f4a7c15
	for i := range block {
		state ^= state << 7
		state ^= state >> 9
		state ^= state << 8
		block[i] = byte(state)
	}
	for written := int64(0); written < size; {
		chunk := min(int64(len(block)), size-written)
		if _, err := file.Write(block[:chunk]); err != nil {
			file.Close()
			t.Fatal(err)
		}
		written += chunk
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func assertRetainedSources(t *testing.T, sources []store.SourceObject, descriptor domain.Descriptor) {
	t.Helper()
	want := make(map[string]domain.File, len(descriptor.Files))
	for _, file := range descriptor.Files {
		want[file.Path] = file
	}
	if len(sources) != len(want) {
		t.Fatalf("retained %d source files; want %d", len(sources), len(want))
	}
	for _, source := range sources {
		file, ok := want[source.Path]
		if !ok || source.Digest != file.SHA256 || source.Size != file.Size {
			t.Fatalf("retained source mismatch: %+v", source)
		}
	}
}

func exerciseSearchModes(t *testing.T, database *store.Store) map[string]time.Duration {
	t.Helper()
	queries := map[string]string{
		"fulltext": "needlecommon",
		"exact":    "exact/path/trace-000000/event-000000",
		"regex":    `regex-token-[0-9]{6}`,
	}
	latencies := make(map[string]time.Duration, len(queries))
	for mode, query := range queries {
		started := time.Now()
		page, err := database.Search(context.Background(), store.SearchQuery{Query: query, Mode: mode, Limit: 25})
		latencies[mode] = time.Since(started)
		if err != nil {
			t.Fatalf("%s search: %v", mode, err)
		}
		if len(page.Results) == 0 {
			t.Fatalf("%s search returned no results", mode)
		}
	}
	return latencies
}

func assertSearchPagination(t *testing.T, database *store.Store) {
	t.Helper()
	first, err := database.Search(context.Background(), store.SearchQuery{Query: "needlecommon", Mode: "fulltext", Limit: 31})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Results) != 31 || first.NextCursor == "" {
		t.Fatalf("first search page has %d results and cursor %q", len(first.Results), first.NextCursor)
	}
	second, err := database.Search(context.Background(), store.SearchQuery{Query: "needlecommon", Mode: "fulltext", Cursor: first.NextCursor, Limit: 31})
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[int64]bool, len(first.Results))
	for _, result := range first.Results {
		seen[result.EventID] = true
	}
	for _, result := range second.Results {
		if seen[result.EventID] {
			t.Fatalf("event %d repeated across search pages", result.EventID)
		}
	}
}

func assertCancellation(t *testing.T, archivePath string, database *store.Store) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	file, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		t.Fatal(err)
	}
	_, err = archive.Validate(ctx, file, info.Size(), archive.DefaultLimits())
	file.Close()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled validation returned %v", err)
	}
	_, err = database.Search(ctx, store.SearchQuery{Query: `.*`, Mode: "regex"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled search returned %v", err)
	}
}

func exerciseConcurrentReadsAndDelete(t *testing.T, database *store.Store, retainedID, deletedID int64) {
	t.Helper()
	const readers = 4
	started := make(chan struct{}, readers)
	continueReads := make(chan struct{})
	errorsFound := make(chan error, readers)
	var group sync.WaitGroup
	for range readers {
		group.Add(1)
		go func() {
			defer group.Done()
			for iteration := 0; iteration < 8; iteration++ {
				if _, err := database.Events(context.Background(), retainedID, "", 25); err != nil {
					if iteration == 0 {
						started <- struct{}{}
					}
					errorsFound <- err
					return
				}
				if _, err := database.Search(context.Background(), store.SearchQuery{Query: "needlecommon", Mode: "fulltext", Limit: 25}); err != nil {
					if iteration == 0 {
						started <- struct{}{}
					}
					errorsFound <- err
					return
				}
				if iteration == 0 {
					started <- struct{}{}
					<-continueReads
				}
			}
		}()
	}
	for range readers {
		<-started
	}
	close(continueReads)
	if err := database.DeleteTrace(context.Background(), deletedID); err != nil {
		t.Fatal(err)
	}
	group.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Fatalf("concurrent read failed: %v", err)
	}
}

func sampleHeap() func() uint64 {
	done := make(chan struct{})
	var peak atomic.Uint64
	var group sync.WaitGroup
	group.Add(1)
	go func() {
		defer group.Done()
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			var memory runtime.MemStats
			runtime.ReadMemStats(&memory)
			for current := peak.Load(); memory.HeapAlloc > current && !peak.CompareAndSwap(current, memory.HeapAlloc); current = peak.Load() {
			}
			select {
			case <-done:
				return
			case <-ticker.C:
			}
		}
	}()
	return func() uint64 {
		close(done)
		group.Wait()
		return peak.Load()
	}
}

func treeBytes(t *testing.T, root string) int64 {
	t.Helper()
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return total
}

func fileBytes(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}
