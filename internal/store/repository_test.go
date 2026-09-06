package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/regutierrez/traicr/internal/domain"
)

func TestLocalRepositoryIdentityIncludesMachine(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	content := []byte("source")
	digest := sha256.Sum256(content)
	for _, machine := range []string{"machine-a", "machine-b"} {
		descriptor := domain.Descriptor{Path: "trace", Harness: "synthetic", Adapter: "test", NativeTraceID: machine, RevisionDigest: machine, Repository: domain.Repository{Root: "/work/repo"}, Files: []domain.File{{Path: "source/file", Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:])}}}
		manifest := domain.Manifest{CreatedAt: "2026-08-22T10:00:00Z", SourceMachine: domain.Machine{ID: machine, Hostname: machine, OS: "linux", Arch: "amd64"}, Traces: []domain.Descriptor{descriptor}}
		sources := fstest.MapFS{"trace/source/file": &fstest.MapFile{Data: content}}
		_, err = s.Import(context.Background(), manifest, sources, func(context.Context, domain.Descriptor, fs.FS) (domain.Normalization, error) {
			return domain.Normalization{Version: 1, Status: "normalized"}, nil
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	var repositories int
	if err = s.db.QueryRow("SELECT COUNT(*) FROM repositories").Scan(&repositories); err != nil {
		t.Fatal(err)
	}
	if repositories != 2 {
		t.Fatalf("local repositories merged across machines: %d", repositories)
	}
}
