package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	config  config.ServerConfig
	store   *store.Store
	logger  *slog.Logger
	uploads chan struct{}
	ui      spaDocument
}

func NewHTTPHandler(configuration config.ServerConfig, database *store.Store, logger *slog.Logger) (http.Handler, error) {
	if configuration.AdminToken == "" || database == nil {
		return nil, errors.New("HTTP server requires an admin token and database")
	}
	if configuration.ArchiveLimits.ArchiveBytes == 0 {
		configuration.ArchiveLimits = archive.DefaultLimits()
	}
	app := &application{config: configuration, store: database, logger: logger, uploads: make(chan struct{}, 1), ui: loadSPA()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", serveProcessHealth)
	mux.Handle("GET /static/", http.FileServerFS(assets.Files))
	if ui, err := fs.Sub(assets.Files, "static/ui"); err == nil {
		mux.Handle("GET /_app/", immutableAssets(http.FileServer(http.FS(ui))))
	}
	mux.HandleFunc("GET /login", app.loginPage)
	mux.HandleFunc("POST /login", app.login)
	mux.Handle("POST /logout", app.browser(app.logout))
	// JSON the transcript page fetches. The page itself is the Svelte app.
	mux.Handle("GET /traces/{id}/events", app.browser(app.events))
	mux.Handle("GET /traces/{id}/transcript", app.browser(app.transcriptAPI))
	mux.Handle("GET /traces/resolve", app.browser(app.resolveTrace))
	mux.Handle("POST /traces/{id}/delete", app.browser(app.deletePage))
	mux.Handle("GET /revisions/{id}/block", app.browser(app.sourceBlock))
	mux.Handle("GET /revisions/{id}/attachment", app.browser(app.sourceAttachment))
	mux.Handle("GET /revisions/{id}/file", app.browser(app.revisionFile))
	mux.Handle("POST /api/v1/imports", app.api(app.importZIP))
	mux.Handle("GET /api/v1/search", app.api(app.search))
	mux.Handle("GET /api/v1/cards", app.api(app.cards))
	mux.HandleFunc("GET /api/v1/csrf", app.csrfAPI)
	mux.Handle("GET /api/v1/imports", app.api(app.imports))
	mux.Handle("GET /api/v1/imports/{id}", app.api(app.importReport))
	mux.Handle("GET /api/v1/traces/{id}", app.api(app.trace))
	mux.Handle("GET /api/v1/traces/{id}/events", app.api(app.events))
	mux.Handle("GET /api/v1/events/{id}/sources", app.api(app.eventSources))
	mux.Handle("GET /api/v1/revisions/{id}/sources", app.api(app.revisionSources))
	mux.Handle("GET /api/v1/revisions/{id}/file", app.api(app.sourceFile))
	mux.Handle("GET /api/v1/machines", app.api(app.machines))
	mux.Handle("DELETE /api/v1/traces/{id}", app.api(app.deleteAPI))
	// Unknown API paths must not fall through to the browser shell below.
	mux.HandleFunc("GET /api/", apiNotFound)
	mux.Handle("GET /", app.browser(app.spa))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		// Chrome needs same-origin referrers to send a non-null Origin on form POSTs.
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
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
