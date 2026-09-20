package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/regutierrez/traicr/internal/archive"
	"github.com/regutierrez/traicr/internal/config"
	"github.com/regutierrez/traicr/internal/domain"
	"github.com/regutierrez/traicr/internal/server"
	"github.com/regutierrez/traicr/internal/store"
	assets "github.com/regutierrez/traicr/web"
)

const adminToken = "synthetic-test-token"

// TestBuiltUIAssetsAreServed only runs after `npm run build`: every script the shell references
// must be embedded and cacheable. A directory embed pattern silently drops SvelteKit's _app folder.
func TestBuiltUIAssetsAreServed(t *testing.T) {
	shell, err := assets.Files.ReadFile("static/ui/index.html")
	if err != nil {
		t.Skip("web/app is not built")
	}
	handler := testHandler(t, false)
	scripts := regexp.MustCompile(`/_app/immutable/[^"]+\.js`).FindAllString(string(shell), -1)
	if len(scripts) == 0 {
		t.Fatalf("shell references no scripts: %s", shell)
	}
	for _, path := range scripts {
		response := request(handler, "GET", path, nil, "")
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
			t.Fatalf("%s: %d %q", path, response.Code, response.Header().Get("Cache-Control"))
		}
	}
	login := request(handler, "GET", "/login", nil, "")
	html := login.Body.String()
	if login.Code != http.StatusOK || !strings.HasPrefix(login.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("login document: %d %s", login.Code, html)
	}
	if !strings.Contains(html, "data-sveltekit") && !strings.Contains(html, "/_app/immutable/") {
		t.Fatalf("login is not the Svelte document: %s", html)
	}
}

func testHandler(t *testing.T, secure bool) http.Handler {
	t.Helper()
	dir := t.TempDir()
	database, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	handler, err := server.NewHTTPHandler(config.ServerConfig{DataDir: dir, AdminToken: adminToken, SecureCookies: secure}, database, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func request(handler http.Handler, method, target string, body io.Reader, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/zip")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func TestHTTPAuthenticationAndValidation(t *testing.T) {
	handler := testHandler(t, false)
	for _, path := range []string{"/api/v1/search", "/api/v1/imports", "/api/v1/imports/1", "/api/v1/traces/1", "/api/v1/traces/1/events", "/api/v1/events/1/sources", "/api/v1/revisions/1/sources", "/api/v1/revisions/1/file", "/api/v1/machines"} {
		for _, token := range []string{"", "wrong"} {
			response := request(handler, "GET", path, nil, token)
			if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), `"api_version":1`) {
				t.Fatalf("unauthenticated %s: %d %s", path, response.Code, response.Body)
			}
		}
	}
	for _, method := range []string{"POST", "DELETE"} {
		path := "/api/v1/imports"
		if method == "DELETE" {
			path = "/api/v1/traces/1"
		}
		if got := request(handler, method, path, nil, ""); got.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated mutation: %d", got.Code)
		}
	}
	for path, status := range map[string]int{
		"/healthz": http.StatusOK, "/": http.StatusSeeOther,
		"/api/v1/search?mode=invalid&q=test": http.StatusBadRequest,
		"/api/v1/search?mode=regex&q=%5B":    http.StatusBadRequest,
		"/api/v1/search?q=test&limit=201":    http.StatusBadRequest,
		"/api/v1/search?q=test&cursor=bad":   http.StatusBadRequest,
		"/api/v1/traces/not-a-number":        http.StatusBadRequest,
		"/api/v1/traces/999":                 http.StatusNotFound,
		"/api/v1/unknown":                    http.StatusNotFound,
	} {
		got := request(handler, "GET", path, nil, adminToken)
		if got.Code != status {
			t.Errorf("%s = %d, want %d: %s", path, got.Code, status, got.Body)
		}
		if got.Header().Get("Content-Security-Policy") == "" || got.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Error("missing browser security headers")
		}
		if got.Header().Get("Referrer-Policy") != "same-origin" {
			t.Error("browser forms need a same-origin referrer policy for CSRF validation")
		}
	}
	if got := request(handler, "POST", "/healthz", nil, ""); got.Code != http.StatusMethodNotAllowed {
		t.Fatalf("health mutation = %d", got.Code)
	}
}

// csrfToken fetches the form token the way the browser app does: from the JSON endpoint, using the cookie.
func csrfToken(t *testing.T, handler http.Handler, cookie *http.Cookie) string {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/v1/csrf", nil)
	req.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	var body struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != http.StatusOK || body.CSRF == "" {
		t.Fatalf("csrf token: %d %s %v", response.Code, response.Body, err)
	}
	return body.CSRF
}

func login(t *testing.T, handler http.Handler, secure bool) (*http.Cookie, string) {
	t.Helper()
	page := request(handler, "GET", "/login", nil, "")
	if page.Code != http.StatusOK || len(page.Result().Cookies()) != 1 {
		t.Fatalf("login page: %d %s", page.Code, page.Body)
	}
	cookie := page.Result().Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Secure != secure {
		t.Fatalf("unsafe session cookie: %#v", cookie)
	}
	origin := "http://example.com"
	if secure {
		origin = "https://example.com"
	}
	form := url.Values{"csrf": {csrfToken(t, handler, cookie)}, "token": {adminToken}}
	for _, badOrigin := range []string{"https://attacker.invalid", ""} {
		response := submit(handler, "/login", form, cookie, badOrigin)
		if response.Code != http.StatusForbidden {
			t.Fatalf("login accepted origin %q: %d", badOrigin, response.Code)
		}
	}
	response := submit(handler, "/login", form, cookie, origin)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("login: %d %s", response.Code, response.Body)
	}
	cookie = response.Result().Cookies()[0]
	req := httptest.NewRequest("GET", "/machines", nil)
	req.AddCookie(cookie)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("authenticated page: %d %s", response.Code, response.Body)
	}
	return cookie, csrfToken(t, handler, cookie)
}

func submit(handler http.Handler, path string, form url.Values, cookie *http.Cookie, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", origin)
	req.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func TestBrowserSessionAndCSRF(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(fmt.Sprint("secure=", secure), func(t *testing.T) {
			handler := testHandler(t, secure)
			cookie, csrf := login(t, handler, secure)
			origin := "http://example.com"
			if secure {
				origin = "https://example.com"
			}
			for _, form := range []url.Values{{}, {"csrf": {"forged"}}} {
				if got := submit(handler, "/logout", form, cookie, origin); got.Code != http.StatusForbidden {
					t.Fatalf("accepted invalid CSRF: %d", got.Code)
				}
			}
			got := submit(handler, "/logout", url.Values{"csrf": {csrf}}, cookie, origin)
			if got.Code != http.StatusSeeOther || got.Result().Cookies()[0].MaxAge != -1 {
				t.Fatalf("logout: %d", got.Code)
			}
			cookie.Value += "tampered"
			req := httptest.NewRequest("GET", "/machines", nil)
			req.AddCookie(cookie)
			got = httptest.NewRecorder()
			handler.ServeHTTP(got, req)
			if got.Code != http.StatusSeeOther {
				t.Fatalf("tampered session accepted: %d", got.Code)
			}
		})
	}
}

func TestImportSearchInspectRetryAndDelete(t *testing.T) {
	handler := testHandler(t, false)
	source := "{\"type\":\"session\",\"version\":3,\"id\":\"http-session\"}\n" +
		"{\"type\":\"message\",\"id\":\"m1\",\"timestamp\":\"2026-08-22T10:00:00Z\",\"message\":{\"role\":\"user\",\"content\":[{\"type\":\"text\",\"text\":\"Find needle/path.go <script>alert(1)</script>\"}]}}\n"
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "source"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "source", "records.jsonl"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	paths, err := archive.Write(context.Background(), t.TempDir(), domain.Manifest{SourceMachine: domain.Machine{ID: "http-machine", Hostname: "fixture"}}, []archive.Input{{Directory: dir, Descriptor: domain.Descriptor{Harness: "pi", Adapter: "pi-jsonl", NativeTraceID: "http-session", Title: "<script>title</script>"}}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	zipBytes, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	var report domain.ImportReport
	for i := range 2 {
		response := request(handler, "POST", "/api/v1/imports", bytes.NewReader(zipBytes), adminToken)
		decoder := json.NewDecoder(response.Body)
		complete := 0
		for {
			var progress domain.Progress
			if err := decoder.Decode(&progress); err == io.EOF {
				break
			} else if err != nil {
				t.Fatal(err)
			}
			if progress.Phase == "complete" {
				complete++
				report = *progress.Report
			}
			if progress.Phase == "failed" {
				t.Fatalf("upload: %s", progress.Error)
			}
		}
		if response.Code != http.StatusOK || complete != 1 || report.Failed != 0 || i == 0 && report.Imported != 1 || i == 1 && report.Unchanged != 1 {
			t.Fatalf("upload %d: status=%d complete=%d report=%+v", i, response.Code, complete, report)
		}
	}
	var result store.SearchResult
	for _, mode := range []string{"fulltext", "exact", "regex"} {
		response := request(handler, "GET", "/api/v1/search?q=needle&mode="+mode, nil, adminToken)
		var page store.SearchPage
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || response.Code != 200 || len(page.Results) != 1 {
			t.Fatalf("%s search: %d %s %v", mode, response.Code, response.Body, err)
		}
		result = page.Results[0]
	}
	cookie, csrf := login(t, handler, false)
	tracePath := fmt.Sprintf("/traces/%d", result.TraceID)
	for _, path := range []string{"/?q=needle", "/?q=%3Cscript%3E&mode=exact", tracePath + "?event=" + fmt.Sprint(result.EventID), "/events/" + fmt.Sprint(result.EventID) + "/sources", "/imports", "/imports/" + fmt.Sprint(report.ID), "/machines"} {
		req := httptest.NewRequest("GET", path, nil)
		req.AddCookie(cookie)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "<script>alert") || strings.Contains(response.Body.String(), "<script>title") {
			t.Fatalf("unsafe or failed page %s: %d %s", path, response.Code, response.Body)
		}
	}
	cards := request(handler, "GET", "/api/v1/cards?q=needle", nil, adminToken)
	if cards.Code != 200 || !strings.Contains(cards.Body.String(), "needle") || strings.Contains(cards.Body.String(), "<script>") {
		t.Fatalf("cards: %d %s", cards.Code, cards.Body)
	}
	scripted := request(handler, "GET", "/api/v1/cards?q=%3Cscript%3E&mode=exact", nil, adminToken)
	if scripted.Code != 200 || strings.Contains(scripted.Body.String(), "<script>") {
		t.Fatalf("cards reflected markup: %d %s", scripted.Code, scripted.Body)
	}
	response := request(handler, "GET", fmt.Sprintf("/api/v1/events/%d/sources", result.EventID), nil, adminToken)
	var sources []store.Source
	if err := json.Unmarshal(response.Body.Bytes(), &sources); err != nil || len(sources) == 0 {
		t.Fatalf("sources: %s %v", response.Body, err)
	}
	response = request(handler, "GET", fmt.Sprintf("/api/v1/revisions/%d/file?path=source/records.jsonl", sources[0].RevisionID), nil, adminToken)
	if response.Code != 200 || response.Body.String() != source || !strings.HasPrefix(response.Header().Get("Content-Disposition"), "attachment;") {
		t.Fatalf("original source changed: %d %s", response.Code, response.Body)
	}
	response = submit(handler, tracePath+"/delete", url.Values{"csrf": {csrf}, "confirm": {"wrong"}}, cookie, "http://example.com")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("deletion without confirmation: %d", response.Code)
	}
	response = submit(handler, tracePath+"/delete", url.Values{"csrf": {csrf}, "confirm": {"http-session"}}, cookie, "http://example.com")
	if response.Code != http.StatusSeeOther {
		t.Fatalf("delete: %d %s", response.Code, response.Body)
	}
	response = request(handler, "GET", "/api/v1/search?q=needle", nil, adminToken)
	var empty store.SearchPage
	if err := json.Unmarshal(response.Body.Bytes(), &empty); err != nil || len(empty.Results) != 0 {
		t.Fatalf("deleted event remains: %s %v", response.Body, err)
	}
	response = request(handler, "GET", fmt.Sprintf("/api/v1/revisions/%d/file?path=source/records.jsonl", sources[0].RevisionID), nil, adminToken)
	if response.Code != http.StatusNotFound {
		t.Fatalf("deleted source remains: %d", response.Code)
	}
	response = request(handler, "POST", "/api/v1/imports", strings.NewReader("corrupt zip"), adminToken)
	if !strings.Contains(response.Body.String(), `"phase":"failed"`) || strings.Contains(response.Body.String(), `"phase":"complete"`) {
		t.Fatalf("corrupt ZIP accepted: %s", response.Body)
	}
}
