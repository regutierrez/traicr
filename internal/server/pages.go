package server

import (
	"context"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/regutierrez/traicr/internal/store"
)

const searchTimeout = 30 * time.Second

// searchQuery reads the filters shared by the search and cards endpoints.
func searchQuery(values url.Values, limit int) store.SearchQuery {
	return store.SearchQuery{
		Query: values.Get("q"), Mode: values.Get("mode"), Harness: values.Get("harness"),
		Model: values.Get("model"), Machine: values.Get("machine"), Repository: values.Get("repository"),
		After: values.Get("after"), Before: values.Get("before"), Role: values.Get("role"),
		Kind: values.Get("kind"), Tool: values.Get("tool"), Cursor: values.Get("cursor"), Limit: limit,
	}
}

func (app *application) search(w http.ResponseWriter, r *http.Request) {
	limit, ok := pageLimit(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), searchTimeout)
	defer cancel()
	page, err := app.store.Search(ctx, searchQuery(r.URL.Query(), limit))
	if err != nil {
		app.failure(w, err)
		return
	}
	writeJSON(w, page)
}

// transcriptCard is the session summary the home page renders.
type transcriptCard struct {
	ID            int64  `json:"id"`
	Title         string `json:"title,omitempty"`
	Harness       string `json:"harness"`
	NativeTraceID string `json:"native_trace_id"`
	Repository    string `json:"repository,omitempty"`
	UpdatedAt     string `json:"updated_at"`
	Snippet       string `json:"snippet,omitempty"`
	EventCount    int    `json:"event_count"`
	RevisionCount int    `json:"revision_count"`
}

type transcriptCardPage struct {
	Cards      []transcriptCard `json:"cards"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

func newTranscriptCard(item store.TranscriptCard) transcriptCard {
	snippet := item.MatchSnippet
	if snippet == "" {
		snippet = item.Preview
	}
	return transcriptCard{
		ID: item.ID, Title: item.Title, Harness: item.Harness, NativeTraceID: item.NativeTraceID,
		Repository: item.Repository, UpdatedAt: item.UpdatedAt, Snippet: snippet,
		EventCount: item.EventCount, RevisionCount: item.RevisionCount,
	}
}

func (app *application) cards(w http.ResponseWriter, r *http.Request) {
	limit, ok := pageLimit(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), searchTimeout)
	defer cancel()
	page, err := app.store.TranscriptCards(ctx, searchQuery(r.URL.Query(), limit))
	if err != nil {
		app.failure(w, err)
		return
	}
	cards := make([]transcriptCard, 0, len(page.Cards))
	for _, item := range page.Cards {
		cards = append(cards, newTranscriptCard(item))
	}
	writeJSON(w, transcriptCardPage{Cards: cards, NextCursor: page.NextCursor})
}

func (app *application) trace(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	trace, err := app.store.Trace(r.Context(), id)
	if err != nil {
		app.failure(w, err)
		return
	}
	writeJSON(w, trace)
}

func (app *application) events(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	limit, ok := pageLimit(w, r)
	if !ok {
		return
	}
	var page store.EventPage
	var err error
	if key := r.URL.Query().Get("key"); key != "" {
		page, err = app.store.EventsFromKey(r.Context(), id, key, limit)
	} else {
		page, err = app.store.Events(r.Context(), id, r.URL.Query().Get("cursor"), limit)
	}
	if err != nil {
		app.failure(w, err)
		return
	}
	writeJSON(w, page)
}

func (app *application) eventSources(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	sources, err := app.store.Sources(r.Context(), id)
	if err != nil {
		app.failure(w, err)
		return
	}
	writeJSON(w, sources)
}

func (app *application) revisionSources(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	sources, err := app.store.RevisionSources(r.Context(), id)
	if err != nil {
		app.failure(w, err)
		return
	}
	writeJSON(w, sources)
}

// sourceFile is the API route: JSON preview on request, otherwise the original bytes.
func (app *application) sourceFile(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("preview") == "1" {
		app.previewSource(w, r)
		return
	}
	app.downloadSource(w, r)
}

// revisionFile is the browser route: the original bytes on request, otherwise the preview page.
func (app *application) revisionFile(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("download") == "1" {
		app.downloadSource(w, r)
		return
	}
	app.spa(w, r)
}

func (app *application) previewSource(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	name := r.URL.Query().Get("path")
	file, err := app.store.OpenSource(r.Context(), id, name)
	if err != nil {
		app.failure(w, err)
		return
	}
	defer file.Close()
	const previewBytes = 1 << 20
	data, err := io.ReadAll(io.LimitReader(file, previewBytes+1))
	if err != nil {
		app.failure(w, err)
		return
	}
	content := string(data[:min(len(data), previewBytes)])
	if !utf8.ValidString(content) || strings.ContainsRune(content, '\x00') {
		content = "Binary source file. Download it to inspect the original bytes."
	}
	writeJSON(w, map[string]any{"path": name, "content": content, "truncated": len(data) > previewBytes})
}

func (app *application) downloadSource(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	name := r.URL.Query().Get("path")
	file, err := app.store.OpenSource(r.Context(), id, name)
	if err != nil {
		app.failure(w, err)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(name)}))
	_, _ = io.Copy(w, file)
}

func (app *application) imports(w http.ResponseWriter, r *http.Request) {
	limit, ok := pageLimit(w, r)
	if !ok {
		return
	}
	page, err := app.store.Imports(r.Context(), r.URL.Query().Get("cursor"), limit)
	if err != nil {
		app.failure(w, err)
		return
	}
	writeJSON(w, page)
}

func (app *application) importReport(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	report, err := app.store.ImportReport(r.Context(), id)
	if err != nil {
		app.failure(w, err)
		return
	}
	writeJSON(w, report)
}

func (app *application) machines(w http.ResponseWriter, r *http.Request) {
	machines, err := app.store.Machines(r.Context())
	if err != nil {
		app.failure(w, err)
		return
	}
	writeJSON(w, machines)
}

func (app *application) deleteAPI(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	if err := app.store.DeleteTrace(r.Context(), id); err != nil {
		app.failure(w, err)
		return
	}
	app.logger.Info("trace deleted", "trace_id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (app *application) deletePage(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	trace, err := app.store.Trace(r.Context(), id)
	if err != nil {
		app.failure(w, err)
		return
	}
	if r.PostForm.Get("confirm") != trace.NativeTraceID {
		writeError(w, http.StatusBadRequest, "confirmation_required", "enter the complete native trace ID to confirm permanent deletion")
		return
	}
	if err := app.store.DeleteTrace(r.Context(), id); err != nil {
		app.failure(w, err)
		return
	}
	app.logger.Info("trace deleted", "trace_id", id)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
