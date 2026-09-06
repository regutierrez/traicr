package archive_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/regutierrez/traicr/internal/archive"
	"github.com/regutierrez/traicr/internal/domain"
)

func fixtureZIP(t *testing.T, count int, split int64) []string {
	t.Helper()
	var inputs []archive.Input
	for i := range count {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, "source"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "source", "records.jsonl"), []byte("{\"type\":\"unknown\",\"text\":\"preserve me\"}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "source", "image.bin"), []byte{0, 255, 1, 2, 0}, 0o600); err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, archive.Input{Directory: root, Descriptor: domain.Descriptor{Harness: "future", Adapter: "native", NativeTraceID: string(rune('a' + i))}})
	}
	paths, err := archive.Write(context.Background(), t.TempDir(), domain.Manifest{SourceMachine: domain.Machine{ID: "machine-a", Hostname: "laptop"}}, inputs, split)
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func TestIndependentSplitArchivesPreserveSourceBytes(t *testing.T) {
	paths := fixtureZIP(t, 3, 1)
	if len(paths) != 3 {
		t.Fatalf("got %d archives, want one complete trace per archive", len(paths))
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		validated, err := archive.Validate(context.Background(), bytes.NewReader(data), int64(len(data)), archive.DefaultLimits())
		if err != nil {
			t.Fatal(err)
		}
		if len(validated.Manifest.Traces) != 1 {
			t.Fatal("trace was split")
		}
		content, err := fs.ReadFile(validated.ZIP, validated.Manifest.Traces[0].Path+"/source/image.bin")
		if err != nil || !bytes.Equal(content, []byte{0, 255, 1, 2, 0}) {
			t.Fatalf("attachment changed: %x, %v", content, err)
		}
	}
}

type entry struct {
	name string
	data []byte
	mode fs.FileMode
}

func rewriteZIP(t *testing.T, source []byte, mutate func([]entry) []entry) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(source), int64(len(source)))
	if err != nil {
		t.Fatal(err)
	}
	var entries []entry
	for _, file := range reader.File {
		content, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(content)
		content.Close()
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry{file.Name, data, 0o600})
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, item := range mutate(entries) {
		header := &zip.FileHeader{Name: item.name, Method: zip.Store, Modified: time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)}
		header.SetMode(item.mode)
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(item.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestRevisionIgnoresZIPMetadataAndFileOrder(t *testing.T) {
	data, err := os.ReadFile(fixtureZIP(t, 1, 0)[0])
	if err != nil {
		t.Fatal(err)
	}
	repacked := rewriteZIP(t, data, func(entries []entry) []entry {
		slices.Reverse(entries)
		return entries
	})
	original, err := archive.Validate(context.Background(), bytes.NewReader(data), int64(len(data)), archive.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	other, err := archive.Validate(context.Background(), bytes.NewReader(repacked), int64(len(repacked)), archive.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if original.Manifest.Traces[0].RevisionDigest != other.Manifest.Traces[0].RevisionDigest {
		t.Fatal("container metadata changed revision identity")
	}
}

func TestUnsafeArchivesAreRejectedBeforeImport(t *testing.T) {
	data, err := os.ReadFile(fixtureZIP(t, 1, 0)[0])
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func([]entry) []entry{
		"traversal": func(e []entry) []entry { return append(e, entry{"../escape", []byte("bad"), 0o600}) },
		"absolute":  func(e []entry) []entry { return append(e, entry{"/escape", nil, 0o600}) },
		"windows":   func(e []entry) []entry { return append(e, entry{"C:\\escape", nil, 0o600}) },
		"duplicate": func(e []entry) []entry { return append(e, e[0]) },
		"link":      func(e []entry) []entry { e[0].mode = fs.ModeSymlink | 0o777; return e },
		"missing":   func(e []entry) []entry { return e[1:] },
		"unlisted":  func(e []entry) []entry { return append(e, entry{"unlisted", nil, 0o600}) },
		"digest": func(e []entry) []entry {
			for i := range e {
				if e[i].name == "traces/000001/source/image.bin" {
					e[i].data[0] = 3
				}
			}
			return e
		},
		"unsupported version": func(e []entry) []entry {
			for i := range e {
				if e[i].name == "manifest.json" {
					var m domain.Manifest
					if err := json.Unmarshal(e[i].data, &m); err != nil {
						t.Fatal(err)
					}
					m.FormatVersion = 99
					e[i].data, _ = json.Marshal(m)
				}
			}
			return e
		},
	} {
		t.Run(name, func(t *testing.T) {
			unsafe := rewriteZIP(t, data, mutate)
			if _, err := archive.Validate(context.Background(), bytes.NewReader(unsafe), int64(len(unsafe)), archive.DefaultLimits()); err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
	for name, alter := range map[string]func(*archive.Limits){
		"upload":     func(l *archive.Limits) { l.ArchiveBytes = 1 },
		"expansion":  func(l *archive.Limits) { l.ExpandedBytes = 1 },
		"file":       func(l *archive.Limits) { l.FileBytes = 1 },
		"file count": func(l *archive.Limits) { l.Files = 1 },
		"metadata":   func(l *archive.Limits) { l.MetadataBytes = 1 },
		"depth":      func(l *archive.Limits) { l.Depth = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			limits := archive.DefaultLimits()
			alter(&limits)
			if _, err := archive.Validate(context.Background(), bytes.NewReader(data), int64(len(data)), limits); err == nil {
				t.Fatal("safety limit ignored")
			}
		})
	}
	for _, invalid := range [][]byte{nil, []byte("not a zip"), data[:len(data)-10]} {
		if _, err := archive.Validate(context.Background(), bytes.NewReader(invalid), int64(len(invalid)), archive.DefaultLimits()); err == nil {
			t.Fatal("corrupt archive accepted")
		}
	}
}

func TestDirectoryCountCannotBypassResourceLimit(t *testing.T) {
	data, err := os.ReadFile(fixtureZIP(t, 1, 0)[0])
	if err != nil {
		t.Fatal(err)
	}
	end := bytes.LastIndex(data, []byte{'P', 'K', 5, 6})
	if end < 0 {
		t.Fatal("test ZIP has no end directory")
	}
	binary.LittleEndian.PutUint16(data[end+8:], 1)
	binary.LittleEndian.PutUint16(data[end+10:], 1)
	limits := archive.DefaultLimits()
	limits.Files = 2
	if _, err := archive.Validate(context.Background(), bytes.NewReader(data), int64(len(data)), limits); err == nil {
		t.Fatal("lying directory count bypassed file limit")
	}
}

func TestCancelledArchiveWorkDoesNotPublishOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "source"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "source", "text"), []byte("retained"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	_, err := archive.Write(ctx, out, domain.Manifest{}, []archive.Input{{Directory: dir}}, 0)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled write: %v", err)
	}
	entries, err := os.ReadDir(out)
	if err != nil || len(entries) != 0 {
		t.Fatalf("cancelled archive left outputs: %v %v", entries, err)
	}
}
