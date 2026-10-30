package server_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/regutierrez/traicr/internal/config"
	"github.com/regutierrez/traicr/internal/domain"
	"github.com/regutierrez/traicr/internal/server"
	"github.com/regutierrez/traicr/internal/store"
)

func archiveHandler(t *testing.T, cfg config.ServerConfig, logger *slog.Logger) (http.Handler, *store.Store, string) {
	t.Helper()
	cfg.DataDir = t.TempDir()
	cfg.AdminToken = adminToken
	database, err := store.Open(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	handler, err := server.NewHTTPHandler(cfg, database, logger)
	if err != nil {
		t.Fatal(err)
	}
	return handler, database, cfg.DataDir
}

// importOne imports one synthetic trace from machine. A missing source file
// makes that trace fail; normalize may block or cancel to model a running or
// interrupted import.
func importOne(ctx context.Context, database *store.Store, machine, nativeID string, missingFile bool, normalize func()) (domain.ImportReport, error) {
	content := []byte(nativeID)
	sum := sha256.Sum256(content)
	descriptor := domain.Descriptor{Path: "traces/000001", Harness: "synthetic", Adapter: "test", NativeTraceID: nativeID, NativeUpdatedAt: "2026-10-01T10:00:00Z", RevisionDigest: nativeID,
		Files: []domain.File{{Path: "source/records.jsonl", Size: int64(len(content)), SHA256: hex.EncodeToString(sum[:])}}}
	manifest := domain.Manifest{FormatVersion: 1, CreatedAt: "2026-10-01T10:00:00Z", SourceMachine: domain.Machine{ID: machine, Hostname: machine + "-host", OS: "linux", Arch: "amd64"}, Traces: []domain.Descriptor{descriptor}}
	files := fstest.MapFS{"traces/000001/source/records.jsonl": &fstest.MapFile{Data: content}}
	if missingFile {
		files = fstest.MapFS{}
	}
	return database.Import(ctx, manifest, files, func(context.Context, domain.Descriptor, fs.FS) (domain.Normalization, error) {
		if normalize != nil {
			normalize()
		}
		return domain.Normalization{Version: 1, Status: "normalized", Events: []domain.Event{{Key: nativeID, Kind: "message", Role: "user", Text: "hello " + nativeID}}}, nil
	}, nil)
}

func decode[T any](t *testing.T, response interface{ Result() *http.Response }, into *T) {
	t.Helper()
	result := response.Result()
	if result.StatusCode != http.StatusOK {
		t.Fatalf("status %d", result.StatusCode)
	}
	if err := json.NewDecoder(result.Body).Decode(into); err != nil {
		t.Fatal(err)
	}
}

func TestStatusReportsLatestImportPerMachineAndExcludesTheRunningImport(t *testing.T) {
	handler, database, _ := archiveHandler(t, config.ServerConfig{}, nil)
	ctx := context.Background()
	mustImport := func(machine, nativeID string, missing bool) domain.ImportReport {
		t.Helper()
		report, err := importOne(ctx, database, machine, nativeID, missing, nil)
		if err != nil {
			t.Fatal(err)
		}
		return report
	}
	mustImport("recovered", "old-failure", true)
	clean := mustImport("recovered", "clean", false)
	mustImport("failing", "new-failure", true)
	mustImport("failing", "untitled", false)
	mustImport("failing", "another-failure", true)
	// A manual title takes a trace out of the title backlog.
	renamed := request(handler, http.MethodPatch, "/api/v1/traces/"+itoa(clean.Traces[0].TraceID), strings.NewReader(`{"title_override":"Named by hand"}`), adminToken)
	if renamed.Code != http.StatusOK {
		t.Fatalf("rename: %d %s", renamed.Code, renamed.Body)
	}

	interruptedCtx, interrupt := context.WithCancel(ctx)
	if _, err := importOne(interruptedCtx, database, "interrupted", "cut-off", false, interrupt); !errors.Is(err, context.Canceled) {
		t.Fatalf("interrupted import err = %v", err)
	}

	started, release := make(chan struct{}), make(chan struct{})
	running := make(chan domain.ImportReport)
	go func() {
		report, _ := importOne(ctx, database, "busy", "in-progress", false, func() { close(started); <-release })
		running <- report
	}()
	<-started

	var status struct {
		Version         string `json:"version"`
		RunningImportID int64  `json:"running_import_id"`
		Machines        []struct {
			ID                     string `json:"id"`
			LastImportComplete     bool   `json:"last_import_complete"`
			LastImportFailedTraces int    `json:"last_import_failed_traces"`
		} `json:"machines"`
		Imports struct {
			Total            int64 `json:"total"`
			WithFailedTraces int64 `json:"with_failed_traces"`
			Incomplete       int64 `json:"incomplete"`
		} `json:"imports"`
		Traces int64 `json:"traces"`
		Titles struct {
			Enabled bool  `json:"enabled"`
			Pending int64 `json:"pending"`
		} `json:"titles"`
		Storage struct {
			DatabaseBytes int64 `json:"database_bytes"`
		} `json:"storage"`
	}
	decode(t, request(handler, http.MethodGet, "/api/v1/status", nil, adminToken), &status)
	close(release)
	finished := <-running

	if status.RunningImportID == 0 || status.RunningImportID != finished.ID {
		t.Fatalf("running_import_id = %d, want %d", status.RunningImportID, finished.ID)
	}
	machines := map[string][2]any{}
	for _, m := range status.Machines {
		machines[m.ID] = [2]any{m.LastImportComplete, m.LastImportFailedTraces}
	}
	want := map[string][2]any{
		"recovered":   {true, 0}, // its older failure must not leak into its latest import
		"failing":     {true, 1}, // its latest import, not the earlier clean one
		"interrupted": {false, 0},
		"busy":        {false, 0},
	}
	for id, w := range want {
		if machines[id] != w {
			t.Errorf("machine %s = %v, want %v", id, machines[id], w)
		}
	}
	// Seven imports; three listed a failed trace; only the cancelled one is
	// interrupted, because the running one is excluded.
	if status.Imports.Total != 7 || status.Imports.WithFailedTraces != 3 || status.Imports.Incomplete != 1 {
		t.Fatalf("imports = %+v", status.Imports)
	}
	// Every stored trace but the renamed one still needs a generated title.
	if status.Version == "" || status.Traces < 2 || status.Titles.Enabled || status.Titles.Pending != status.Traces-1 || status.Storage.DatabaseBytes == 0 {
		t.Fatalf("status = %+v", status)
	}

	status.RunningImportID = 0 // omitted from the response when nothing runs
	decode(t, request(handler, http.MethodGet, "/api/v1/status", nil, adminToken), &status)
	if status.RunningImportID != 0 || status.Imports.Incomplete != 1 {
		t.Fatalf("after the import finished: running=%d incomplete=%d", status.RunningImportID, status.Imports.Incomplete)
	}
}

func TestImportReportsDeriveCompleteFromTheStoredReport(t *testing.T) {
	handler, database, dir := archiveHandler(t, config.ServerConfig{}, nil)
	ctx := context.Background()
	finished, err := importOne(ctx, database, "laptop", "done", false, nil)
	if err != nil || !finished.Complete {
		t.Fatalf("finished import: %+v %v", finished, err)
	}
	legacy, err := importOne(ctx, database, "laptop", "legacy", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	interruptedCtx, interrupt := context.WithCancel(ctx)
	interrupted, _ := importOne(interruptedCtx, database, "laptop", "cut-off", false, interrupt)

	// Reports stored before the field existed have no "complete" key.
	raw, err := sql.Open("sqlite", "file:"+filepath.Join(dir, "traicr.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if _, err := raw.Exec(`UPDATE imports SET report_json=CAST(json_remove(CAST(report_json AS TEXT),'$.complete') AS BLOB) WHERE id=?`, legacy.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`UPDATE import_summaries SET summary_json=CAST(json_remove(CAST(summary_json AS TEXT),'$.complete') AS BLOB) WHERE import_id=?`, legacy.ID); err != nil {
		t.Fatal(err)
	}

	var page struct {
		Imports []domain.ImportReport `json:"imports"`
	}
	decode(t, request(handler, http.MethodGet, "/api/v1/imports", nil, adminToken), &page)
	complete := map[int64]bool{}
	for _, report := range page.Imports {
		complete[report.ID] = report.Complete
	}
	if !complete[finished.ID] || !complete[legacy.ID] || complete[interrupted.ID] || len(complete) != 3 {
		t.Fatalf("complete by id = %v (finished %d, legacy %d, interrupted %d)", complete, finished.ID, legacy.ID, interrupted.ID)
	}
	var one domain.ImportReport
	decode(t, request(handler, http.MethodGet, "/api/v1/imports/"+itoa(interrupted.ID), nil, adminToken), &one)
	if one.Complete {
		t.Fatal("interrupted import reads as complete")
	}
}

type logRecord struct {
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Attrs   map[string]any `json:"attrs"`
}

type logPage struct {
	Records []logRecord `json:"records"`
}

func TestBrowserLoadReportReachesLogs(t *testing.T) {
	handler, database, _ := archiveHandler(t, config.ServerConfig{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	report, err := importOne(context.Background(), database, "laptop", "loaded", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	traceID := report.Traces[0].TraceID
	cookie, csrf := login(t, handler, false)
	body := `{"revision":"","fetch_ms":1500.7,"adapter_ms":20,"pages":3,"events":450,"anomalies":[{"kind":"unpaired_tool_result","count":2,"message":"2 tool results have no call","event_ids":[7,9]}]}`
	if response := submit(handler, "/traces/"+itoa(traceID)+"/load-report", url.Values{"csrf": {csrf}, "report": {body}}, cookie, "http://example.com"); response.Code != http.StatusNoContent {
		t.Fatalf("load report: %d %s", response.Code, response.Body)
	}
	if response := submit(handler, "/traces/"+itoa(traceID)+"/load-report", url.Values{"report": {body}}, cookie, "http://example.com"); response.Code != http.StatusForbidden {
		t.Fatalf("load report without CSRF: %d", response.Code)
	}
	var page logPage
	decode(t, request(handler, http.MethodGet, "/api/v1/logs?level=warn", nil, adminToken), &page)
	found := map[string]map[string]any{}
	for _, record := range page.Records {
		if record.Attrs["trace_id"] == float64(traceID) {
			found[record.Message] = record.Attrs
		}
	}
	// 1.5 s of loading is slow, so the timing is a warning too.
	load, anomaly := found["transcript load"], found["transcript anomaly"]
	if load["fetch_ms"] != float64(1500) || load["pages"] != float64(3) || load["events"] != float64(450) {
		t.Fatalf("load record = %+v", load)
	}
	if anomaly["kind"] != "unpaired_tool_result" || anomaly["count"] != float64(2) || anomaly["event_ids"] != "[7 9]" {
		t.Fatalf("anomaly record = %+v", anomaly)
	}
}

func TestLogsRecordRejectedUploadsAndFilterByLevel(t *testing.T) {
	var stdout bytes.Buffer
	handler, database, _ := archiveHandler(t, config.ServerConfig{}, slog.New(slog.NewJSONHandler(&stdout, nil)))
	report, err := importOne(context.Background(), database, "laptop", "named", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	// An info record the warn filter must drop.
	if response := request(handler, http.MethodPatch, "/api/v1/traces/"+itoa(report.Traces[0].TraceID), strings.NewReader(`{"title_override":"x"}`), adminToken); response.Code != http.StatusOK {
		t.Fatalf("rename: %d", response.Code)
	}
	upload := request(handler, http.MethodPost, "/api/v1/imports", strings.NewReader("not a zip"), adminToken)
	if upload.Code != http.StatusOK || !strings.Contains(upload.Body.String(), `"phase":"failed"`) {
		t.Fatalf("upload: %d %s", upload.Code, upload.Body)
	}
	var page logPage
	decode(t, request(handler, http.MethodGet, "/api/v1/logs", nil, adminToken), &page)
	// Every request is logged; set those records aside to check the others' order.
	var requests []map[string]any
	page.Records = slices.DeleteFunc(page.Records, func(record logRecord) bool {
		if record.Message == "request" {
			requests = append(requests, record.Attrs)
		}
		return record.Message == "request"
	})
	if len(requests) != 2 || requests[0]["route"] != "POST /api/v1/imports" || requests[1]["route"] != "PATCH /api/v1/traces/{id}" ||
		requests[1]["status"] != float64(200) || requests[1]["trace_id"] != float64(report.Traces[0].TraceID) || requests[1]["bytes"].(float64) <= 0 {
		t.Fatalf("request records = %+v", requests)
	}
	if len(page.Records) < 2 || page.Records[0].Message != "import rejected" || page.Records[0].Level != "warn" || page.Records[0].Attrs["error"] != "ZIP end directory missing" || page.Records[1].Message != "trace title changed" {
		t.Fatalf("newest records = %+v", page.Records)
	}
	decode(t, request(handler, http.MethodGet, "/api/v1/logs?level=warn", nil, adminToken), &page)
	for _, record := range page.Records {
		if record.Level == "info" {
			t.Fatalf("level=warn returned %+v", record)
		}
	}
	if len(page.Records) == 0 || page.Records[0].Message != "import rejected" {
		t.Fatalf("warn records = %+v", page.Records)
	}
	if !strings.Contains(stdout.String(), `"msg":"import rejected"`) {
		t.Fatalf("stdout lost the record: %s", stdout.String())
	}
	if response := request(handler, http.MethodGet, "/api/v1/logs?level=debug", nil, adminToken); response.Code != http.StatusBadRequest {
		t.Fatalf("level=debug: %d", response.Code)
	}
}

func TestOpenAPIIsPublicButStatusAndLogsNeedAToken(t *testing.T) {
	handler := testHandler(t, false)
	response := request(handler, http.MethodGet, "/api/v1/openapi.json", nil, "")
	var doc struct {
		OpenAPI string `json:"openapi"`
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &doc) != nil || !strings.HasPrefix(doc.OpenAPI, "3.1") {
		t.Fatalf("openapi.json: %d %.80s", response.Code, response.Body)
	}
	for _, path := range []string{"/api/v1/status", "/api/v1/logs"} {
		if response := request(handler, http.MethodGet, path, nil, ""); response.Code != http.StatusUnauthorized {
			t.Fatalf("%s without token: %d", path, response.Code)
		}
	}
}

func itoa(id int64) string { return strconv.FormatInt(id, 10) }

// TestResponsesMatchThePublishedSchemas checks real responses against the
// required fields and types openapi.json promises, so agents and generated
// tools can rely on them.
func TestResponsesMatchThePublishedSchemas(t *testing.T) {
	handler, database, _ := archiveHandler(t, config.ServerConfig{}, nil)
	ctx := context.Background()
	report, err := importOne(ctx, database, "laptop", "readable", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	interruptedCtx, interrupt := context.WithCancel(ctx)
	interrupted, _ := importOne(interruptedCtx, database, "laptop", "cut-off", false, interrupt)
	traceID := itoa(report.Traces[0].TraceID)
	var events struct {
		Events []struct {
			ID int64 `json:"id"`
		} `json:"events"`
	}
	decode(t, request(handler, http.MethodGet, "/api/v1/traces/"+traceID+"/events", nil, adminToken), &events)
	if len(events.Events) == 0 {
		t.Fatal("no events")
	}

	var doc map[string]any
	if err := json.Unmarshal(request(handler, http.MethodGet, "/api/v1/openapi.json", nil, "").Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct{ path, method, target string }{
		{"/api/v1/cards", "get", "/api/v1/cards"},
		{"/api/v1/search", "get", "/api/v1/search?q=hello"},
		{"/api/v1/traces/{id}", "get", "/api/v1/traces/" + traceID},
		{"/api/v1/traces/{id}/events", "get", "/api/v1/traces/" + traceID + "/events"},
		{"/api/v1/traces/{id}/transcript", "get", "/api/v1/traces/" + traceID + "/transcript"},
		{"/api/v1/events/{id}/sources", "get", "/api/v1/events/" + itoa(events.Events[0].ID) + "/sources"},
		{"/api/v1/imports", "get", "/api/v1/imports"},
		{"/api/v1/imports/{id}", "get", "/api/v1/imports/" + itoa(interrupted.ID)},
		{"/api/v1/machines", "get", "/api/v1/machines"},
		{"/api/v1/status", "get", "/api/v1/status"},
		{"/api/v1/logs", "get", "/api/v1/logs"},
	} {
		response := request(handler, http.MethodGet, check.target, nil, adminToken)
		if response.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", check.target, response.Code, response.Body)
		}
		checkDocumented(t, doc, check.path, check.method, response)
	}

	// Errors must match their documented schema exactly, including the
	// top-level _tag that OpenAPI clients such as Executor match on.
	for _, check := range []struct{ path, method, target, body, token string }{
		{"/api/v1/traces/{id}", "get", "/api/v1/traces/999999", "", adminToken},
		{"/api/v1/traces/{id}", "get", "/api/v1/traces/0", "", adminToken},
		{"/api/v1/traces/{id}", "patch", "/api/v1/traces/" + traceID, `{"other":1}`, adminToken},
		{"/api/v1/cards", "get", "/api/v1/cards?after=not-a-date", "", adminToken},
		{"/api/v1/traces/{id}/transcript", "get", "/api/v1/traces/" + traceID + "/transcript?revision=-1", "", adminToken},
		{"/api/v1/logs", "get", "/api/v1/logs?cursor=stale", "", adminToken},
		{"/api/v1/status", "get", "/api/v1/status", "", ""},
	} {
		var body io.Reader
		if check.body != "" {
			body = strings.NewReader(check.body)
		}
		response := request(handler, strings.ToUpper(check.method), check.target, body, check.token)
		if response.Code < 400 {
			t.Fatalf("%s %s: %d, want an error", check.method, check.target, response.Code)
		}
		checkDocumented(t, doc, check.path, check.method, response)
	}
}

// checkDocumented checks a response against the schema openapi.json
// documents for its status code.
func checkDocumented(t *testing.T, doc map[string]any, path, method string, response *httptest.ResponseRecorder) {
	t.Helper()
	var body any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	operation := doc["paths"].(map[string]any)[path].(map[string]any)[method].(map[string]any)
	documented, ok := operation["responses"].(map[string]any)[strconv.Itoa(response.Code)].(map[string]any)
	if !ok {
		t.Fatalf("%s %s: status %d is not documented", method, path, response.Code)
	}
	if ref, ok := documented["$ref"].(string); ok {
		documented = doc["components"].(map[string]any)["responses"].(map[string]any)[strings.TrimPrefix(ref, "#/components/responses/")].(map[string]any)
	}
	schema := documented["content"].(map[string]any)["application/json"].(map[string]any)["schema"]
	if problem := conforms(doc, schema, body, "$"); problem != "" {
		t.Errorf("%s %s (%d): %s", method, path, response.Code, problem)
	}
}

// conforms checks types, required properties, and array items, following
// $ref and allOf. It is not a full JSON Schema validator.
func conforms(doc map[string]any, schema, value any, at string) string {
	s := schema.(map[string]any)
	if ref, ok := s["$ref"].(string); ok {
		node := any(doc)
		for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
			node = node.(map[string]any)[part]
		}
		return conforms(doc, node, value, at)
	}
	if variants := asList(s["anyOf"]); len(variants) > 0 {
		problems := []string{}
		for _, variant := range variants {
			problem := conforms(doc, variant, value, at)
			if problem == "" {
				return ""
			}
			problems = append(problems, problem)
		}
		return fmt.Sprintf("%s matches no anyOf variant: %s", at, strings.Join(problems, "; "))
	}
	if want, ok := s["const"]; ok && want != value {
		return fmt.Sprintf("%s is %v, schema requires %v", at, value, want)
	}
	for _, part := range asList(s["allOf"]) {
		if problem := conforms(doc, part, value, at); problem != "" {
			return problem
		}
	}
	types := asList(s["type"])
	if len(types) > 0 && !slicesContainsType(types, value) {
		return fmt.Sprintf("%s is %T, schema allows %v", at, value, types)
	}
	switch v := value.(type) {
	case map[string]any:
		for _, name := range asList(s["required"]) {
			if _, ok := v[name.(string)]; !ok {
				return fmt.Sprintf("%s is missing required %q", at, name)
			}
		}
		properties, _ := s["properties"].(map[string]any)
		for name, field := range v {
			if property, ok := properties[name]; ok {
				if problem := conforms(doc, property, field, at+"."+name); problem != "" {
					return problem
				}
			}
		}
	case []any:
		if items, ok := s["items"]; ok {
			for i, item := range v {
				if problem := conforms(doc, items, item, fmt.Sprintf("%s[%d]", at, i)); problem != "" {
					return problem
				}
			}
		}
	}
	return ""
}

func asList(value any) []any {
	switch v := value.(type) {
	case []any:
		return v
	case nil:
		return nil
	default:
		return []any{v}
	}
}

func slicesContainsType(types []any, value any) bool {
	for _, want := range types {
		switch v := value.(type) {
		case nil:
			if want == "null" {
				return true
			}
		case bool:
			if want == "boolean" {
				return true
			}
		case float64:
			if want == "number" || (want == "integer" && v == float64(int64(v))) {
				return true
			}
		case string:
			if want == "string" {
				return true
			}
		case []any:
			if want == "array" {
				return true
			}
		case map[string]any:
			if want == "object" {
				return true
			}
		}
	}
	return false
}
