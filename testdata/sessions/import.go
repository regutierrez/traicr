//go:build ignore

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/regutierrez/traicr/internal/archive"
	"github.com/regutierrez/traicr/internal/domain"
)

func main() {
	root := "testdata/sessions"
	if _, err := os.Stat(root); err != nil {
		panic(err)
	}
	inputs := []archive.Input{
		{
			Directory: filepath.Join(root, "pi-herdr"),
			Descriptor: domain.Descriptor{
				Harness: "pi", Adapter: "pi-jsonl",
				NativeTraceID:    "01a07d3e-5368-756c-8fb3-6850365a9fcc",
				NativeUpdatedAt:  "2026-09-09T11:19:45.115Z",
				Title:            "Check and plan vendoring sesh for herdr",
				WorkingDirectory: "/home/user/hseh",
				Repository:       domain.Repository{Remote: "github.com/regutierrez/hseh", Root: "/home/user/hseh"},
			},
		},
		{
			Directory: filepath.Join(root, "amp-transcript-support"),
			Descriptor: domain.Descriptor{
				Harness: "amp", Adapter: "amp-thread-export",
				NativeTraceID:    "T-01a0aac5-fa9b-77cc-b272-93003af2f083",
				NativeUpdatedAt:  "2026-09-17T01:35:30.614Z",
				Title:            "Amp transcript support",
				WorkingDirectory: "file:///home/user/workspace/repo",
			},
		},
		{
			Directory: filepath.Join(root, "claude-cleanup"),
			Descriptor: domain.Descriptor{
				Harness: "claude-code", Adapter: "claude-code-jsonl",
				NativeTraceID:    "a7b136ba-2631-444f-ab0a-464778e15e3e",
				NativeUpdatedAt:  "2026-09-11T03:16:25.209Z",
				Title:            "Codebase cleanup with implementing-pragmatic-code",
				WorkingDirectory: "/home/user/hseh",
			},
		},
	}
	out := os.TempDir()
	paths, err := archive.Write(context.Background(), out, domain.Manifest{SourceMachine: domain.Machine{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Hostname: "fixture-host"}}, inputs, 0)
	if err != nil {
		panic(err)
	}
	base := os.Getenv("TRAICR_URL")
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	token := os.Getenv("TRAICR_ADMIN_TOKEN")
	if token == "" {
		token = "traicr-demo"
	}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		req, err := http.NewRequest(http.MethodPost, base+"/api/v1/imports", bytes.NewReader(body))
		if err != nil {
			panic(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/zip")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			panic(err)
		}
		progress, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("import %s status=%d\n%s\n", filepath.Base(path), resp.StatusCode, progress)
	}
}
