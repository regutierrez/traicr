package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/regutierrez/traicr/internal/archive"
	"github.com/regutierrez/traicr/internal/config"
	"github.com/regutierrez/traicr/internal/domain"
	"github.com/regutierrez/traicr/internal/normalize"
	"github.com/regutierrez/traicr/internal/store"
	assets "github.com/regutierrez/traicr/web"
)

type application struct {
	config    config.ServerConfig
	store     *store.Store
	templates *template.Template
	logger    *slog.Logger
	uploads   chan struct{}
}

func NewHTTPHandler(configuration config.ServerConfig, database *store.Store, logger *slog.Logger) (http.Handler, error) {
	if configuration.AdminToken == "" || database == nil {
		return nil, errors.New("HTTP server requires an admin token and database")
	}
	if configuration.ArchiveLimits.ArchiveBytes == 0 {
		configuration.ArchiveLimits = archive.DefaultLimits()
	}
	templates, err := template.New("").Funcs(template.FuncMap{
		"json": func(value any) string { data, _ := json.MarshalIndent(value, "", "  "); return string(data) },
		"mark": markMatches,
		"dict": templateDict,
	}).ParseFS(assets.Files, "templates/*.html")
	if err != nil {
		return nil, err
	}
	app := &application{config: configuration, store: database, templates: templates, logger: logger, uploads: make(chan struct{}, 1)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", serveProcessHealth)
	mux.Handle("GET /static/", http.FileServerFS(assets.Files))
	mux.HandleFunc("GET /login", app.loginPage)
	mux.HandleFunc("POST /login", app.login)
	mux.Handle("POST /logout", app.browser(app.logout))
	mux.Handle("GET /{$}", app.browser(app.searchPage))
	mux.Handle("GET /traces/{id}", app.browser(app.tracePage))
	mux.Handle("GET /traces/{id}/events", app.browser(app.eventsAPI))
	mux.Handle("GET /traces/{id}/transcript", app.browser(app.transcriptAPI))
	mux.Handle("GET /traces/resolve", app.browser(app.resolveTrace))
	mux.Handle("POST /traces/{id}/delete", app.browser(app.deletePage))
	mux.Handle("GET /events/{id}/sources", app.browser(app.sourcesPage))
	mux.Handle("GET /revisions/{id}/sources", app.browser(app.revisionPage))
	mux.Handle("GET /revisions/{id}/file", app.browser(app.sourceFile))
	mux.Handle("GET /revisions/{id}/block", app.browser(app.sourceBlock))
	mux.Handle("GET /revisions/{id}/attachment", app.browser(app.sourceAttachment))
	mux.Handle("GET /imports", app.browser(app.importsPage))
	mux.Handle("GET /imports/{id}", app.browser(app.reportPage))
	mux.Handle("GET /machines", app.browser(app.machinesPage))
	mux.Handle("POST /api/v1/imports", app.api(app.importZIP))
	mux.Handle("GET /api/v1/search", app.api(app.searchPage))
	mux.Handle("GET /api/v1/imports", app.api(app.importsPage))
	mux.Handle("GET /api/v1/imports/{id}", app.api(app.reportPage))
	mux.Handle("GET /api/v1/traces/{id}", app.api(app.tracePage))
	mux.Handle("GET /api/v1/traces/{id}/events", app.api(app.eventsAPI))
	mux.Handle("GET /api/v1/events/{id}/sources", app.api(app.sourcesPage))
	mux.Handle("GET /api/v1/revisions/{id}/sources", app.api(app.revisionPage))
	mux.Handle("GET /api/v1/revisions/{id}/file", app.api(app.sourceFile))
	mux.Handle("GET /api/v1/machines", app.api(app.machinesPage))
	mux.Handle("DELETE /api/v1/traces/{id}", app.api(app.deleteAPI))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		// Chrome needs same-origin referrers to send a non-null Origin on form POSTs.
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; font-src 'self'; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
		mux.ServeHTTP(w, r)
	}), nil
}

func (app *application) importZIP(w http.ResponseWriter, r *http.Request) {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/zip" {
		writeError(w, http.StatusUnsupportedMediaType, "content_type", "send a Trace ZIP with Content-Type: application/zip")
		return
	}
	// Do not let waiting uploads fill the data volume while one writer is busy.
	select {
	case app.uploads <- struct{}{}:
		defer func() { <-app.uploads }()
	default:
		w.Header().Set("Retry-After", "5")
		writeError(w, http.StatusServiceUnavailable, "import_busy", "another import is running; retry this ZIP")
		return
	}
	file, err := os.CreateTemp(filepath.Join(app.config.DataDir, "tmp"), "upload-*.zip")
	if err != nil {
		app.failure(w, err)
		return
	}
	defer os.Remove(file.Name())
	defer file.Close()
	r.Body = http.MaxBytesReader(w, r.Body, app.config.ArchiveLimits.ArchiveBytes)
	size, err := io.Copy(file, r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "upload_failed", "upload is incomplete or exceeds the configured limit")
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	encoder := json.NewEncoder(w)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	progress := func(value domain.Progress) {
		if err := encoder.Encode(value); err != nil {
			cancel()
			return
		}
		if err := http.NewResponseController(w).Flush(); err != nil {
			cancel()
		}
	}
	progress(domain.Progress{Phase: "validating"})
	validated, err := archive.Validate(ctx, file, size, app.config.ArchiveLimits)
	if err != nil {
		progress(domain.Progress{Phase: "failed", Error: err.Error()})
		return
	}
	started := time.Now()
	report, err := app.store.Import(ctx, validated.Manifest, validated.ZIP, normalize.Run, progress)
	if err != nil {
		app.logger.Error("import failed", "error_type", fmt.Sprintf("%T", err))
		progress(domain.Progress{Phase: "failed", Error: "import interrupted; retry the ZIP"})
		return
	}
	app.logger.Info("import finished", "import_id", report.ID, "traces", len(report.Traces), "failed", report.Failed, "duration", time.Since(started))
}

// templateDict builds a map from alternating keys and values for shared template fragments.
func templateDict(values ...any) map[string]any {
	result := make(map[string]any, len(values)/2)
	for i := 0; i+1 < len(values); i += 2 {
		key, _ := values[i].(string)
		result[key] = values[i+1]
	}
	return result
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"api_version": 1, "error": map[string]string{"code": code, "message": message}})
}

func (app *application) failure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		writeError(w, http.StatusNotFound, "not_found", "record not found")
	case errors.Is(err, store.ErrTranscriptChanged):
		writeError(w, http.StatusConflict, "transcript_changed", err.Error())
	case errors.Is(err, store.ErrInvalidQuery):
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusRequestTimeout, "cancelled", "request cancelled or timed out")
	default:
		app.logger.Error("request failed", "error_type", fmt.Sprintf("%T", err))
		writeError(w, http.StatusInternalServerError, "internal_error", "request failed; check server diagnostics")
	}
}

func requestID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a positive integer")
		return 0, false
	}
	return id, true
}

func pageLimit(w http.ResponseWriter, r *http.Request) (int, bool) {
	if r.URL.Query().Get("limit") == "" {
		return 50, true
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 200 {
		writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 200")
		return 0, false
	}
	return limit, true
}

func (app *application) render(w http.ResponseWriter, r *http.Request, name string, data map[string]any) {
	data["CSRF"] = app.csrf(r)
	data["Page"] = name
	var body bytes.Buffer
	if err := app.templates.ExecuteTemplate(&body, name, data); err != nil {
		app.failure(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = body.WriteTo(w)
}
