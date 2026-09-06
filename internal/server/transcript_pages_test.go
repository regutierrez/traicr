package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/regutierrez/traicr/internal/archive"
	"github.com/regutierrez/traicr/internal/domain"
	"github.com/regutierrez/traicr/internal/store"
)

func TestTranscriptHomeAndBrowserEventPagination(t *testing.T) {
	handler := testHandler(t, false)
	var inputs []archive.Input
	for i := 0; i < 2; i++ {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "source"), 0700); err != nil {
			t.Fatal(err)
		}
		var source strings.Builder
		source.WriteString(`{"type":"session","version":3,"id":"session"}` + "\n")
		for j := 0; j < 205; j++ {
			fmt.Fprintf(&source, "{\"type\":\"message\",\"id\":\"m%d\",\"timestamp\":\"2026-09-06T10:00:00Z\",\"message\":{\"role\":\"user\",\"content\":\"matching transcript content\"}}\n", j)
		}
		if err := os.WriteFile(filepath.Join(dir, "source/records.jsonl"), []byte(source.String()), 0600); err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, archive.Input{Directory: dir, Descriptor: domain.Descriptor{Harness: "pi", Adapter: "pi-jsonl", NativeTraceID: fmt.Sprint(i), Title: fmt.Sprintf("Session %d", i)}})
	}
	archives, err := archive.Write(context.Background(), t.TempDir(), domain.Manifest{SourceMachine: domain.Machine{ID: "test", Hostname: "test"}}, inputs, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(archives[0])
	if err != nil {
		t.Fatal(err)
	}
	imported := request(handler, "POST", "/api/v1/imports", bytes.NewReader(b), adminToken)
	if imported.Code != 200 || !strings.Contains(imported.Body.String(), `"imported":2`) {
		t.Fatalf("import: %s", imported.Body)
	}
	cookie, _ := login(t, handler, false)
	browserGet := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		req.AddCookie(cookie)
		r := httptest.NewRecorder()
		handler.ServeHTTP(r, req)
		return r
	}
	for _, path := range []string{"/", "/?q=matching"} {
		response := browserGet(path)
		if response.Code != 200 || strings.Count(response.Body.String(), `class="transcript-card"`) != 2 || strings.Contains(response.Body.String(), "Recent events") {
			t.Fatalf("not session cards: %s %d %s", path, response.Code, response.Body)
		}
	}
	viewer := browserGet("/traces/1")
	for _, marker := range []string{"tree-search", "pi-transcript.js", "transcript-status", "Session details", `data-harness="pi"`, "/static/traicr-theme.css"} {
		if !strings.Contains(viewer.Body.String(), marker) {
			t.Fatalf("missing Pi viewer element: %s", marker)
		}
	}
	if strings.Contains(viewer.Header().Get("Content-Security-Policy"), "unsafe-inline") {
		t.Fatal("viewer weakened CSP")
	}
	if response := request(handler, "GET", "/traces/1/events", nil, ""); response.Code != http.StatusSeeOther {
		t.Fatalf("unauthenticated event access: %d", response.Code)
	}
	first := browserGet("/traces/1/events?limit=200")
	var page store.EventPage
	if err := json.Unmarshal(first.Body.Bytes(), &page); err != nil || len(page.Events) != 200 || page.NextCursor == "" {
		t.Fatalf("first event page: %d %v", len(page.Events), err)
	}
	second := browserGet("/traces/1/events?limit=200&cursor=" + page.NextCursor)
	page = store.EventPage{}
	if err := json.Unmarshal(second.Body.Bytes(), &page); err != nil || len(page.Events) != 5 || page.NextCursor != "" {
		t.Fatalf("second event page: %d %v", len(page.Events), err)
	}
	if response := browserGet("/traces/1?view=records"); response.Code != 200 || !strings.Contains(response.Body.String(), "Retained revisions") {
		t.Fatalf("source details unavailable: %d", response.Code)
	}
}
