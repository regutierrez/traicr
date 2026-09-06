package store_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/regutierrez/traicr/internal/domain"
	"github.com/regutierrez/traicr/internal/store"
)

func TestDeleteTraceRemovesMoreThanOneBatchOfObjects(t *testing.T) {
	database, dataDir := openMaintenanceStore(t)
	const objectCount = 101
	descriptor := domain.Descriptor{
		Path: "traces/000001", Harness: "synthetic", Adapter: "boundary", NativeTraceID: "many-objects", RevisionDigest: "many-objects",
		Files: make([]domain.File, 0, objectCount),
	}
	sources := fstest.MapFS{}
	for i := range objectCount {
		path := fmt.Sprintf("source/object-%03d", i)
		content := []byte(fmt.Sprintf("unique object %03d", i))
		digest := sha256.Sum256(content)
		descriptor.Files = append(descriptor.Files, domain.File{Path: path, Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:])})
		sources[descriptor.Path+"/"+path] = &fstest.MapFile{Data: content}
	}
	manifest := domain.Manifest{
		CreatedAt: "2026-08-22T10:00:00Z", SourceMachine: domain.Machine{ID: "machine", Hostname: "host"},
		Traces: []domain.Descriptor{descriptor},
	}
	report, err := database.Import(context.Background(), manifest, sources, normalizer(nil, "unsupported", 1), nil)
	if err != nil || report.Unsupported != 1 {
		t.Fatalf("import: report=%+v err=%v", report, err)
	}
	if files := objectFiles(t, dataDir); len(files) != objectCount {
		t.Fatalf("stored %d objects; want %d", len(files), objectCount)
	}
	if err := database.DeleteTrace(context.Background(), report.Traces[0].TraceID); err != nil {
		t.Fatal(err)
	}
	if files := objectFiles(t, dataDir); len(files) != 0 {
		t.Fatalf("objects remain after deletion: %v", files)
	}
}

func TestDeleteCleanupErrorIsReturnedAndGCRecovers(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory write permissions")
	}
	database, dataDir := openMaintenanceStore(t)
	input := traceInput("machine", "cleanup-error", "2026-08-22T10:00:00Z", "Cleanup", nil)
	report, err := database.Import(context.Background(), input.manifest, input.files, normalizer(nil, "unsupported", 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	traceID := report.Traces[0].TraceID
	trace, err := database.Trace(context.Background(), traceID)
	if err != nil {
		t.Fatal(err)
	}
	sources, err := database.RevisionSources(context.Background(), trace.Revisions[0].ID)
	if err != nil || len(sources) != 1 {
		t.Fatalf("sources=%+v err=%v", sources, err)
	}
	objectPath := filepath.Join(dataDir, "objects", "sha256", sources[0].Digest[:2], sources[0].Digest[2:])
	objectDir := filepath.Dir(objectPath)
	if err := os.Chmod(objectDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(objectDir, 0o700) })
	if err := database.DeleteTrace(context.Background(), traceID); err == nil {
		t.Fatal("filesystem cleanup error was suppressed")
	}
	if _, err := database.Trace(context.Background(), traceID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("trace deletion was not committed: %v", err)
	}
	if _, err := os.Stat(objectPath); err != nil {
		t.Fatalf("object should remain for recovery: %v", err)
	}
	if err := os.Chmod(objectDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := database.GC(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(objectPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("recovery left object behind: %v", err)
	}
}

func TestGCRemovesFilesystemObjectMissingFromDatabase(t *testing.T) {
	database, dataDir := openMaintenanceStore(t)
	objectPath := filepath.Join(dataDir, "objects", "sha256", "aa", "untracked")
	if err := os.MkdirAll(filepath.Dir(objectPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(objectPath, []byte("left by interrupted import"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := database.GC(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(objectPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("untracked object remains: %v", err)
	}
}

func TestRenormalizeSkipsCurrentVersion(t *testing.T) {
	database, _ := openMaintenanceStore(t)
	input := traceInput("machine", "current-version", "2026-08-22T10:00:00Z", "Current", []domain.Event{{Key: "event", Kind: "message", Text: "current"}})
	report, err := database.Import(context.Background(), input.manifest, input.files, normalizer(input.events, "normalized", 2), nil)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	err = database.Renormalize(context.Background(), func(string) int { return 2 }, func(context.Context, domain.Descriptor, fs.FS) (domain.Normalization, error) {
		calls++
		return domain.Normalization{Version: 2, Status: "normalized"}, nil
	})
	if err != nil || calls != 0 {
		t.Fatalf("equal version rebuilt: calls=%d err=%v", calls, err)
	}
	trace, err := database.Trace(context.Background(), report.Traces[0].TraceID)
	if err != nil || trace.Revisions[0].NormalizerVersion != 2 {
		t.Fatalf("revision=%+v err=%v", trace.Revisions, err)
	}
}

func TestRenormalizeProcessesEveryQueuedRevisionAcrossBatches(t *testing.T) {
	database, _ := openMaintenanceStore(t)
	const revisionCount = 101
	traceIDs := make([]int64, 0, revisionCount)
	for i := range revisionCount {
		input := traceInput("machine", fmt.Sprintf("queued-%03d", i), "2026-08-22T10:00:00Z", fmt.Sprintf("Queued %03d", i), []domain.Event{{Key: "event", Kind: "message", Text: "old"}})
		input.manifest.Traces[0].NativeTraceID = fmt.Sprintf("trace-%03d", i)
		report, err := database.Import(context.Background(), input.manifest, input.files, normalizer(input.events, "normalized", 1), nil)
		if err != nil {
			t.Fatalf("import %d: %v", i, err)
		}
		traceIDs = append(traceIDs, report.Traces[0].TraceID)
	}
	seen := map[string]bool{}
	err := database.Renormalize(context.Background(), func(string) int { return 2 }, func(_ context.Context, descriptor domain.Descriptor, _ fs.FS) (domain.Normalization, error) {
		seen[descriptor.NativeTraceID] = true
		return domain.Normalization{Version: 2, Status: "normalized", Events: []domain.Event{{Key: "event", Kind: "message", Text: "new"}}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != revisionCount {
		t.Fatalf("rebuilt %d revisions; want %d", len(seen), revisionCount)
	}
	for _, index := range []int{0, revisionCount - 1} {
		trace, err := database.Trace(context.Background(), traceIDs[index])
		if err != nil || trace.Revisions[0].NormalizerVersion != 2 {
			t.Fatalf("revision %d=%+v err=%v", index, trace.Revisions, err)
		}
	}
}

func openMaintenanceStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	dataDir := t.TempDir()
	database, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database, dataDir
}

func objectFiles(t *testing.T, dataDir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(filepath.Join(dataDir, "objects", "sha256"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
