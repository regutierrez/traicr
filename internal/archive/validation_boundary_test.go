package archive_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/regutierrez/traicr/internal/archive"
	"github.com/regutierrez/traicr/internal/domain"
)

func TestValidationAcceptsExactSafetyLimits(t *testing.T) {
	data := archiveBytes(t)
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	limits := archive.DefaultLimits()
	limits.ArchiveBytes = int64(len(data))
	limits.Files = len(reader.File)
	limits.ExpandedBytes = 0
	limits.FileBytes = 0
	limits.MetadataBytes = 0
	limits.Depth = 0
	for _, file := range reader.File {
		limits.ExpandedBytes += int64(file.UncompressedSize64)
		limits.FileBytes = max(limits.FileBytes, int64(file.UncompressedSize64))
		if file.Name == "manifest.json" || strings.HasSuffix(file.Name, "/descriptor.json") {
			limits.MetadataBytes = max(limits.MetadataBytes, int64(file.UncompressedSize64))
		}
		limits.Depth = max(limits.Depth, strings.Count(file.Name, "/")+1)
	}
	if _, err := archive.Validate(context.Background(), bytes.NewReader(data), int64(len(data)), limits); err != nil {
		t.Fatalf("exact limits rejected: %v", err)
	}

	for name, reduce := range map[string]func(*archive.Limits){
		"archive bytes":  func(value *archive.Limits) { value.ArchiveBytes-- },
		"expanded bytes": func(value *archive.Limits) { value.ExpandedBytes-- },
		"file bytes":     func(value *archive.Limits) { value.FileBytes-- },
		"file count":     func(value *archive.Limits) { value.Files-- },
		"metadata bytes": func(value *archive.Limits) { value.MetadataBytes-- },
		"path depth":     func(value *archive.Limits) { value.Depth-- },
	} {
		t.Run(name, func(t *testing.T) {
			tooSmall := limits
			reduce(&tooSmall)
			if _, err := archive.Validate(context.Background(), bytes.NewReader(data), int64(len(data)), tooSmall); err == nil {
				t.Fatal("limit below the archive requirement was accepted")
			}
		})
	}
}

func TestValidationRequiresEveryLimitToBePositive(t *testing.T) {
	data := archiveBytes(t)
	for name, clear := range map[string]func(*archive.Limits){
		"archive bytes":  func(value *archive.Limits) { value.ArchiveBytes = 0 },
		"expanded bytes": func(value *archive.Limits) { value.ExpandedBytes = 0 },
		"file bytes":     func(value *archive.Limits) { value.FileBytes = 0 },
		"file count":     func(value *archive.Limits) { value.Files = 0 },
		"metadata bytes": func(value *archive.Limits) { value.MetadataBytes = 0 },
		"path depth":     func(value *archive.Limits) { value.Depth = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			limits := archive.DefaultLimits()
			clear(&limits)
			if _, err := archive.Validate(context.Background(), bytes.NewReader(data), int64(len(data)), limits); err == nil || err.Error() != "archive limits must be positive" {
				t.Fatalf("zero limit returned %v", err)
			}
		})
	}
}

func TestValidationAcceptsEmptySourceAndSourceAtFileLimit(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "source"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "source", "empty"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	const fileLimit = 4 << 10
	if err := os.WriteFile(filepath.Join(root, "source", "exact"), bytes.Repeat([]byte("x"), fileLimit), 0o600); err != nil {
		t.Fatal(err)
	}
	paths, err := archive.Write(context.Background(), t.TempDir(), domain.Manifest{SourceMachine: domain.Machine{ID: "machine", Hostname: "host"}}, []archive.Input{{Directory: root, Descriptor: domain.Descriptor{Harness: "boundary", NativeTraceID: "empty-and-exact"}}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	limits := archive.DefaultLimits()
	limits.FileBytes = fileLimit
	if _, err := archive.Validate(context.Background(), bytes.NewReader(data), int64(len(data)), limits); err != nil {
		t.Fatalf("empty or exact-limit source rejected: %v", err)
	}
}

func TestValidationHashesBytesAgainstAConsistentDescriptor(t *testing.T) {
	data := archiveBytes(t)
	corrupt := rewriteZIP(t, data, func(entries []entry) []entry {
		var manifest domain.Manifest
		for i := range entries {
			if entries[i].name == "manifest.json" {
				if err := json.Unmarshal(entries[i].data, &manifest); err != nil {
					t.Fatal(err)
				}
			}
		}
		manifest.Traces[0].Files[0].SHA256 = strings.Repeat("0", 64)
		manifest.Traces[0].RevisionDigest = archive.RevisionDigest(manifest.Traces[0].Files)
		for i := range entries {
			switch entries[i].name {
			case "manifest.json":
				entries[i].data, _ = json.Marshal(manifest)
			case manifest.Traces[0].Path + "/descriptor.json":
				entries[i].data, _ = json.Marshal(manifest.Traces[0])
			}
		}
		return entries
	})
	if _, err := archive.Validate(context.Background(), bytes.NewReader(corrupt), int64(len(corrupt)), archive.DefaultLimits()); err == nil || err.Error() != "source digest mismatch" {
		t.Fatalf("consistent false digest returned %v", err)
	}
}

func TestValidationRejectsMalformedDirectoryStructures(t *testing.T) {
	data := archiveBytes(t)
	central := bytes.Index(data, []byte{'P', 'K', 1, 2})
	if central < 0 {
		t.Fatal("fixture has no central directory")
	}
	badSignature := bytes.Clone(data)
	badSignature[central] = 0
	if _, err := archive.Validate(context.Background(), bytes.NewReader(badSignature), int64(len(badSignature)), archive.DefaultLimits()); err == nil || err.Error() != "invalid ZIP directory entry" {
		t.Fatalf("bad directory signature returned %v", err)
	}

	truncatedEntry := bytes.Clone(data)
	binary.LittleEndian.PutUint16(truncatedEntry[central+28:], ^uint16(0))
	if _, err := archive.Validate(context.Background(), bytes.NewReader(truncatedEntry), int64(len(truncatedEntry)), archive.DefaultLimits()); err == nil || err.Error() != "ZIP directory count or size mismatch" {
		t.Fatalf("oversized directory name returned %v", err)
	}
}

func TestValidationAcceptsZIPCommentsAndExtraFields(t *testing.T) {
	data := archiveBytes(t)
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	if err := writer.SetComment("archive comment"); err != nil {
		t.Fatal(err)
	}
	for _, source := range reader.File {
		content, err := source.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(content)
		content.Close()
		if err != nil {
			t.Fatal(err)
		}
		header := &zip.FileHeader{Name: source.Name, Method: zip.Store, Comment: "entry comment", Extra: []byte{0xfe, 0xca, 0, 0}}
		header.SetMode(0o600)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Validate(context.Background(), bytes.NewReader(output.Bytes()), int64(output.Len()), archive.DefaultLimits()); err != nil {
		t.Fatalf("valid ZIP metadata rejected: %v", err)
	}
}

func TestValidationChecksZIP64LocatorOffsetAndHeader(t *testing.T) {
	data := archiveBytes(t)
	end := bytes.LastIndex(data, []byte{'P', 'K', 5, 6})
	if end < 0 {
		t.Fatal("fixture has no end directory")
	}

	missingLocator := bytes.Clone(data)
	binary.LittleEndian.PutUint16(missingLocator[end+10:], ^uint16(0))
	if _, err := archive.Validate(context.Background(), bytes.NewReader(missingLocator), int64(len(missingLocator)), archive.DefaultLimits()); err == nil || err.Error() != "invalid ZIP64 directory" {
		t.Fatalf("missing ZIP64 locator returned %v", err)
	}

	zip64 := promoteZIP64(t, data)
	if _, err := archive.Validate(context.Background(), bytes.NewReader(zip64), int64(len(zip64)), archive.DefaultLimits()); err != nil {
		t.Fatalf("valid ZIP64 directory rejected: %v", err)
	}
	zip64End := bytes.LastIndex(zip64, []byte{'P', 'K', 5, 6})
	locator := zip64End - 20
	header := int(binary.LittleEndian.Uint64(zip64[locator+8:]))

	badOffset := bytes.Clone(zip64)
	binary.LittleEndian.PutUint64(badOffset[locator+8:], uint64(len(badOffset)))
	if _, err := archive.Validate(context.Background(), bytes.NewReader(badOffset), int64(len(badOffset)), archive.DefaultLimits()); err == nil || err.Error() != "invalid ZIP64 offset" {
		t.Fatalf("bad ZIP64 offset returned %v", err)
	}

	badHeader := bytes.Clone(zip64)
	badHeader[header] = 0
	if _, err := archive.Validate(context.Background(), bytes.NewReader(badHeader), int64(len(badHeader)), archive.DefaultLimits()); err == nil || err.Error() != "invalid ZIP64 header" {
		t.Fatalf("bad ZIP64 header returned %v", err)
	}
}

func TestValidationStopsWhenContextIsCancelled(t *testing.T) {
	data := archiveBytes(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := archive.Validate(ctx, bytes.NewReader(data), int64(len(data)), archive.DefaultLimits()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled validation returned %v", err)
	}
}

func TestWriteDefaultAndExactSplitBoundaries(t *testing.T) {
	inputs := make([]archive.Input, 2)
	var total int64
	for i := range inputs {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, "source"), 0o700); err != nil {
			t.Fatal(err)
		}
		content := []byte(strings.Repeat(string(rune('a'+i)), 32))
		if err := os.WriteFile(filepath.Join(root, "source", "records.jsonl"), content, 0o600); err != nil {
			t.Fatal(err)
		}
		inputs[i] = archive.Input{Directory: root, Descriptor: domain.Descriptor{Harness: "boundary", NativeTraceID: string(rune('a' + i))}}
		total += int64(len(content))
	}
	manifest := domain.Manifest{SourceMachine: domain.Machine{ID: "machine", Hostname: "host"}}
	for name, split := range map[string]int64{"default": 0, "exact": total} {
		t.Run(name, func(t *testing.T) {
			paths, err := archive.Write(context.Background(), t.TempDir(), manifest, inputs, split)
			if err != nil {
				t.Fatal(err)
			}
			if len(paths) != 1 {
				t.Fatalf("split %d produced %d archives; want 1", split, len(paths))
			}
			data, err := os.ReadFile(paths[0])
			if err != nil {
				t.Fatal(err)
			}
			validated, err := archive.Validate(context.Background(), bytes.NewReader(data), int64(len(data)), archive.DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			if validated.Manifest.CreatedAt == "" {
				t.Fatal("writer did not generate collection time")
			}
		})
	}
	paths, err := archive.Write(context.Background(), t.TempDir(), manifest, inputs, total-1)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || !strings.HasSuffix(paths[0], "-001.zip") || !strings.HasSuffix(paths[1], "-002.zip") {
		t.Fatalf("below-boundary split produced %v", paths)
	}
}

func archiveBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(fixtureZIP(t, 1, 0)[0])
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func promoteZIP64(t *testing.T, data []byte) []byte {
	t.Helper()
	end := bytes.LastIndex(data, []byte{'P', 'K', 5, 6})
	if end < 0 {
		t.Fatal("fixture has no end directory")
	}
	eocd := bytes.Clone(data[end:])
	count := binary.LittleEndian.Uint16(eocd[10:])
	directorySize := binary.LittleEndian.Uint32(eocd[12:])
	directoryOffset := binary.LittleEndian.Uint32(eocd[16:])
	headerOffset := uint64(end)
	header := make([]byte, 56)
	copy(header, []byte{'P', 'K', 6, 6})
	binary.LittleEndian.PutUint64(header[4:], 44)
	binary.LittleEndian.PutUint16(header[12:], 45)
	binary.LittleEndian.PutUint16(header[14:], 45)
	binary.LittleEndian.PutUint64(header[24:], uint64(count))
	binary.LittleEndian.PutUint64(header[32:], uint64(count))
	binary.LittleEndian.PutUint64(header[40:], uint64(directorySize))
	binary.LittleEndian.PutUint64(header[48:], uint64(directoryOffset))
	locator := make([]byte, 20)
	copy(locator, []byte{'P', 'K', 6, 7})
	binary.LittleEndian.PutUint64(locator[8:], headerOffset)
	binary.LittleEndian.PutUint32(locator[16:], 1)
	binary.LittleEndian.PutUint16(eocd[8:], ^uint16(0))
	binary.LittleEndian.PutUint16(eocd[10:], ^uint16(0))
	binary.LittleEndian.PutUint32(eocd[12:], ^uint32(0))
	binary.LittleEndian.PutUint32(eocd[16:], ^uint32(0))
	return bytes.Join([][]byte{data[:end], header, locator, eocd}, nil)
}
