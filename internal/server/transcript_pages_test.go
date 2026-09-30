package server_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/regutierrez/traicr/internal/archive"
	"github.com/regutierrez/traicr/internal/domain"
	"github.com/regutierrez/traicr/internal/store"
)

func TestAmpAgentFixturePairsToolsAndPreservesThreadOrigin(t *testing.T) {
	handler := testHandler(t, false)
	archives, err := archive.Write(context.Background(), t.TempDir(), domain.Manifest{SourceMachine: domain.Machine{ID: "test", Hostname: "test"}}, []archive.Input{{Directory: writeExportDir(t, ampAgentsExport), Descriptor: domain.Descriptor{Harness: "amp", Adapter: "amp-thread-export", NativeTraceID: "T-amp-agents-fixture"}}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(archives[0])
	if err != nil {
		t.Fatal(err)
	}
	if response := request(handler, "POST", "/api/v1/imports", bytes.NewReader(data), adminToken); response.Code != 200 {
		t.Fatalf("import: %s", response.Body)
	}
	cookie, _ := login(t, handler, false)
	get := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	response := get("/traces/1/transcript")
	var page store.EventPage
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || len(page.Events) != 16 {
		t.Fatalf("transcript: %s %v", response.Body, err)
	}
	calls, results := map[string]store.Event{}, map[string]store.Event{}
	for _, event := range page.Events {
		if event.Kind == "tool_call" {
			calls[event.CallID] = event
		}
		if event.Kind == "tool_result" {
			results[event.CallID] = event
		}
	}
	if len(calls) != 6 || len(results) != 6 {
		t.Fatalf("lost tool calls or results: %d calls, %d results", len(calls), len(results))
	}
	for _, expected := range []struct{ id, tool, argument, answer string }{
		{"TU-skill", "skill", "tdd", "Check both the inclusive endpoint"},
		{"TU-task", "Task", "Review range boundaries", "Subagent review"},
		{"TU-red", "shell_command", "TestInclusiveEnd", "endpoint 3 should be included"},
		{"TU-patch", "apply_patch", "*** Begin Patch", "Updated range.go"},
		{"TU-green", "shell_command", "go test ./internal/range -count=1", "PASS: inclusive endpoint"},
		{"TU-thread", "create_thread", "Review boundary documentation", "Review boundary documentation"},
	} {
		call, result := calls[expected.id], results[expected.id]
		if call.Tool != expected.tool || result.Tool != expected.tool || !strings.Contains(call.Text, expected.argument) || !strings.Contains(result.Text, expected.answer) {
			t.Fatalf("tool pairing %s: call=%+v result=%+v", expected.id, call, result)
		}
		if result.RevisionID != 1 || !strings.Contains(string(result.Metadata), `"source_pointer":"/messages/`) {
			t.Fatalf("tool result provenance lost: %+v", result)
		}
	}
	if !strings.Contains(string(results["TU-red"].Metadata), `"exitCode":1`) || !strings.Contains(string(results["TU-green"].Metadata), `"exitCode":0`) {
		t.Fatal("shell exit codes lost")
	}
	if !strings.Contains(string(results["TU-thread"].Metadata), `"threadID":"T-amp-child-fixture"`) {
		t.Fatal("spawned thread link lost")
	}
	originIndex := slices.IndexFunc(page.Events, func(event store.Event) bool { return event.Key == "message:13:0" })
	if originIndex < 0 {
		t.Fatal("incoming thread message missing")
	}
	if origin := page.Events[originIndex]; !strings.Contains(string(origin.Metadata), `"fromExecutorThreadID":"T-amp-child-fixture"`) || !strings.Contains(origin.Text, "spawned thread confirmed") {
		t.Fatalf("thread origin lost: %+v", origin)
	}
	if response := get("/revisions/1/block?pointer=/messages/4/content/0"); response.Code != 200 || !strings.Contains(response.Body.String(), "No files changed") {
		t.Fatalf("subagent source block unavailable: %d %s", response.Code, response.Body)
	}
	if response := get("/traces/resolve?native_id=T-amp-child-fixture"); response.Code != 404 {
		t.Fatalf("uncollected child trace was invented: %d", response.Code)
	}
}

func TestAmpDownloadedImagesSurviveArchiveImport(t *testing.T) {
	handler := testHandler(t, false)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "source/attachments"), 0700); err != nil {
		t.Fatal(err)
	}
	const imagePath = "source/attachments/582a1f5b341842222df56057a4c2851406c193ab079e6c4f1237d3518fc46e9c"
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9ZQmcAAAAASUVORK5CYII=")
	if err := os.WriteFile(filepath.Join(dir, imagePath), png, 0600); err != nil {
		t.Fatal(err)
	}
	const export = `{"messages":[{"id":"image","role":"user","content":[{"type":"image","sourcePath":"screenshot.png","source":{"type":"url","url":"https://ampcode.com/user-content/attachments/fixture.png#amp-media-width=320"}},{"type":"image","url":"https://ampcode.com/user-content/attachments/missing.png"}]}]}`
	if err := os.WriteFile(filepath.Join(dir, "source/export.json"), []byte(export), 0600); err != nil {
		t.Fatal(err)
	}
	archives, err := archive.Write(context.Background(), t.TempDir(), domain.Manifest{SourceMachine: domain.Machine{ID: "test", Hostname: "test"}}, []archive.Input{{Directory: dir, Descriptor: domain.Descriptor{Harness: "amp", Adapter: "amp-thread-export", NativeTraceID: "T-images"}}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(archives[0])
	if err != nil {
		t.Fatal(err)
	}
	if response := request(handler, "POST", "/api/v1/imports", bytes.NewReader(data), adminToken); response.Code != 200 {
		t.Fatalf("import: %s", response.Body)
	}
	cookie, _ := login(t, handler, false)
	get := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	page := get("/traces/1/transcript?revision=1")
	if !strings.Contains(page.Body.String(), `"archived_path":"`+imagePath+`"`) || !strings.Contains(page.Body.String(), `"path":"screenshot.png"`) {
		t.Fatalf("archived image missing from transcript: %s", page.Body)
	}
	if response := request(handler, "GET", "/api/v1/traces/1", nil, adminToken); response.Code != 200 || !strings.Contains(response.Body.String(), "amp_image_unavailable") || !strings.Contains(response.Body.String(), "Recollect and upload") {
		t.Fatalf("missing image diagnostic not shown: %d %s", response.Code, response.Body)
	}
	for _, path := range []string{"/revisions/1/attachment?pointer=/messages/0/content/0", "/revisions/1/file?path=" + imagePath + "&download=1"} {
		if response := request(handler, "GET", path, nil, ""); response.Code != http.StatusSeeOther {
			t.Fatalf("unprotected image: %d", response.Code)
		}
		response := get(path)
		if response.Code != 200 || !bytes.Equal(response.Body.Bytes(), png) {
			t.Fatalf("image bytes changed: %d %s", response.Code, response.Body)
		}
	}
	if response := get("/revisions/1/attachment?pointer=/messages/0/content/1"); response.Code != 404 {
		t.Fatalf("unavailable image fetched: %d", response.Code)
	}
	if response := get("/revisions/1/file?path=source/export.json&download=1"); response.Body.String() != export {
		t.Fatal("original export changed")
	}
	var merged, selected store.EventPage
	for route, page := range map[string]*store.EventPage{"/traces/1/transcript?limit=1": &merged, "/traces/1/transcript?revision=1&limit=1": &selected} {
		response := get(route)
		if err := json.Unmarshal(response.Body.Bytes(), page); err != nil || page.NextCursor == "" {
			t.Fatalf("first page: %s %v", response.Body, err)
		}
	}
	large := append(png, make([]byte, (10<<20)+1-len(png))...)
	if err := os.WriteFile(filepath.Join(dir, imagePath), large, 0600); err != nil {
		t.Fatal(err)
	}
	archives, err = archive.Write(context.Background(), t.TempDir(), domain.Manifest{SourceMachine: domain.Machine{ID: "test", Hostname: "test"}}, []archive.Input{{Directory: dir, Descriptor: domain.Descriptor{Harness: "amp", Adapter: "amp-thread-export", NativeTraceID: "T-images"}}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(archives[0])
	if err != nil {
		t.Fatal(err)
	}
	if response := request(handler, "POST", "/api/v1/imports", bytes.NewReader(data), adminToken); response.Code != 200 {
		t.Fatalf("import large image: %s", response.Body)
	}
	if response := get("/traces/1/transcript?cursor=" + merged.NextCursor); response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "transcript_changed") {
		t.Fatalf("stale transcript cursor: %d %s", response.Code, response.Body)
	}
	response := get("/traces/1/transcript?revision=1&cursor=" + selected.NextCursor)
	var last store.EventPage
	if err := json.Unmarshal(response.Body.Bytes(), &last); err != nil || response.Code != 200 || len(last.Events) != 1 || last.Events[0].RevisionID != 1 || last.NextCursor != "" {
		t.Fatalf("unchanged selected revision invalidated: %s %v", response.Body, err)
	}
	if response := get("/revisions/2/attachment?pointer=/messages/0/content/0"); response.Code != 413 {
		t.Fatalf("oversized preview allowed: %d", response.Code)
	}
	if response := get("/revisions/2/file?path=" + imagePath + "&download=1"); response.Code != 200 || !bytes.Equal(response.Body.Bytes(), large) {
		t.Fatalf("large original not downloadable: %d", response.Code)
	}
}

func TestAmpImagesAndNumericIdentityThroughRepeatedImports(t *testing.T) {
	handler := testHandler(t, false)
	png := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9ZQmcAAAAASUVORK5CYII="
	svg := base64.StdEncoding.EncodeToString([]byte(`<svg onload="alert(1)"></svg>`))
	for index, text := range []string{"draft", "final", "draft"} {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "source"), 0700); err != nil {
			t.Fatal(err)
		}
		content := fmt.Sprintf(`{"messages":[{"messageId":0,"readAt":%d,"role":"assistant","content":[{"type":"text","text":%q},{"type":"image","source":{"type":"base64","media_type":"image/png","data":%q}},{"type":"image","source":{"type":"base64","data":%q}}]}]}`, index, text, png, svg)
		if err := os.WriteFile(filepath.Join(dir, "source/export.json"), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		updated := "2026-09-16T10:00:00Z"
		if text == "final" {
			updated = "2026-09-16T11:00:00Z"
		}
		files, err := archive.Write(context.Background(), t.TempDir(), domain.Manifest{SourceMachine: domain.Machine{ID: "test", Hostname: "test"}}, []archive.Input{{Directory: dir, Descriptor: domain.Descriptor{Harness: "amp", Adapter: "amp-thread-export", NativeTraceID: "numeric", NativeUpdatedAt: updated}}}, 0)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(files[0])
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if response := request(handler, "POST", "/api/v1/imports", bytes.NewReader(data), adminToken); response.Code != 200 {
				t.Fatalf("import: %s", response.Body)
			}
		}
	}
	cookie, _ := login(t, handler, false)
	get := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	var page store.EventPage
	response := get("/traces/1/transcript")
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || len(page.Events) != 3 || page.Events[0].Text != "final" {
		t.Fatalf("stale or duplicate numeric message: %s %v", response.Body, err)
	}
	response = get("/traces/1/transcript?revision=1")
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || page.Events[0].Text != "draft" {
		t.Fatalf("revision lost: %s %v", response.Body, err)
	}
	response = get("/revisions/1/attachment?pointer=/messages/0/content/1&download=1")
	expected, _ := base64.StdEncoding.DecodeString(png)
	if response.Code != 200 || response.Header().Get("Content-Type") != "image/png" || !bytes.Equal(response.Body.Bytes(), expected) || response.Header().Get("Content-Disposition") == "" {
		t.Fatalf("archived image: %d %s", response.Code, response.Header())
	}
	if response := get("/revisions/1/attachment?pointer=/messages/0/content/2"); response.Code != 415 {
		t.Fatalf("active image content was allowed: %d", response.Code)
	}
}

func TestAmpTranscriptRevisionAndSourceInspection(t *testing.T) {
	handler := testHandler(t, false)
	archives, err := archive.Write(context.Background(), t.TempDir(), domain.Manifest{SourceMachine: domain.Machine{ID: "test", Hostname: "test"}}, []archive.Input{{Directory: writeExportDir(t, ampRichExport), Descriptor: domain.Descriptor{Harness: "amp", Adapter: "amp-thread-export", NativeTraceID: "T-amp-rich-fixture", Title: "Amp details"}}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(archives[0])
	if err != nil {
		t.Fatal(err)
	}
	if response := request(handler, "POST", "/api/v1/imports", bytes.NewReader(data), adminToken); response.Code != 200 {
		t.Fatalf("import: %s", response.Body)
	}
	cookie, _ := login(t, handler, false)
	get := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, path := range []string{"/traces/1/transcript", "/revisions/1/block?pointer=/messages/5/content/0", "/revisions/1/attachment?pointer=/messages/0/content/1", "/traces/resolve?native_id=T-amp-rich-fixture"} {
		if response := request(handler, "GET", path, nil, ""); response.Code != http.StatusSeeOther {
			t.Fatalf("unprotected %s: %d", path, response.Code)
		}
	}
	var page store.EventPage
	response := get("/traces/1/transcript?revision=1")
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || len(page.Events) < 16 || page.Events[0].Kind != "session_info" {
		t.Fatalf("transcript: %s %v", response.Body, err)
	}
	response = get("/revisions/1/block?pointer=" + url.QueryEscape("/messages/5/content/0"))
	if response.Code != 200 || !strings.Contains(response.Body.String(), "Preserve the boundary case") || strings.Contains(response.Body.String(), "Inspect the build") {
		t.Fatalf("precise block: %s", response.Body)
	}
	// Revision selection now happens in the browser app, so every viewer URL serves the shell.
	for _, path := range []string{"/traces/1?revision=1", "/traces/1?revision=999"} {
		if response := get(path); response.Code != 200 || !strings.HasPrefix(response.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("viewer shell %s: %d %s", path, response.Code, response.Body)
		}
	}
	response = get("/revisions/1/file?path=source/export.json&download=1")
	if response.Code != 200 || !strings.Contains(response.Header().Get("Content-Disposition"), "attachment") || !strings.Contains(response.Body.String(), "PRIVATE_SIGNATURE") {
		t.Fatalf("native download is not lossless: %d", response.Code)
	}
	if response := get("/revisions/1/attachment?pointer=/messages/0/content/1"); response.Code != http.StatusNotFound {
		t.Fatalf("external attachment must not be fetched: %d", response.Code)
	}
	if response := get("/traces/resolve?native_id=T-amp-rich-fixture"); response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/traces/1" {
		t.Fatalf("related trace: %d %s", response.Code, response.Header())
	}
	if response := get("/traces/1/transcript?revision=999"); response.Code != 404 {
		t.Fatalf("unrelated revision: %d", response.Code)
	}
}

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
	for _, path := range []string{"/api/v1/cards", "/api/v1/cards?q=matching"} {
		response := request(handler, "GET", path, nil, adminToken)
		var payload struct {
			Cards []struct {
				ID int64 `json:"id"`
			} `json:"cards"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || response.Code != 200 || len(payload.Cards) != 2 {
			t.Fatalf("not session cards: %s %d %s %v", path, response.Code, response.Body, err)
		}
	}
	if response := browserGet("/"); response.Code != 200 || strings.Contains(response.Body.String(), "Recent events") {
		t.Fatalf("home: %d %s", response.Code, response.Body)
	}
	viewer := browserGet("/traces/1")
	if viewer.Code != 200 || strings.Contains(viewer.Body.String(), "Recent events") {
		t.Fatalf("viewer: %d %s", viewer.Code, viewer.Body)
	}
	csp := viewer.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "script-src 'unsafe-inline'") {
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
	if response := request(handler, "GET", "/api/v1/traces/1", nil, adminToken); response.Code != 200 || !strings.Contains(response.Body.String(), `"revisions"`) {
		t.Fatalf("source details unavailable: %d %s", response.Code, response.Body)
	}
}
