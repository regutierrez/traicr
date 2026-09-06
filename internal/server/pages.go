package server

import (
	"context"
	"errors"
	"html/template"
	"io"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/regutierrez/traicr/internal/store"
)

func markMatches(text string, query store.SearchQuery) template.HTML {
	pattern := regexp.QuoteMeta(query.Query)
	switch query.Mode {
	case "regex":
		pattern = query.Query
	case "", "fulltext":
		terms := regexp.MustCompile(`[\pL\pN_]+`).FindAllString(query.Query, -1)
		pattern = "(?i)" + strings.Join(terms, "|")
	}
	expression, err := regexp.Compile(pattern)
	if query.Query == "" || err != nil {
		return template.HTML(template.HTMLEscapeString(text))
	}
	var output strings.Builder
	last := 0
	for _, match := range expression.FindAllStringIndex(text, -1) {
		if match[0] == match[1] {
			continue
		}
		// Only these fixed mark tags are HTML; every byte from a trace is escaped.
		output.WriteString(template.HTMLEscapeString(text[last:match[0]]))
		output.WriteString("<mark>")
		output.WriteString(template.HTMLEscapeString(text[match[0]:match[1]]))
		output.WriteString("</mark>")
		last = match[1]
	}
	output.WriteString(template.HTMLEscapeString(text[last:]))
	return template.HTML(output.String())
}

func (app *application) searchPage(w http.ResponseWriter, r *http.Request) {
	limit, ok := pageLimit(w, r)
	if !ok {
		return
	}
	values := r.URL.Query()
	query := store.SearchQuery{
		Query: values.Get("q"), Mode: values.Get("mode"), Harness: values.Get("harness"),
		Model: values.Get("model"), Machine: values.Get("machine"), Repository: values.Get("repository"),
		After: values.Get("after"), Before: values.Get("before"), Role: values.Get("role"),
		Kind: values.Get("kind"), Tool: values.Get("tool"), Cursor: values.Get("cursor"), Limit: limit,
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	started := time.Now()
	page, err := app.store.Search(ctx, query)
	if strings.HasPrefix(r.URL.Path, "/api/") {
		if err != nil {
			app.failure(w, err)
			return
		}
		writeJSON(w, page)
		return
	}
	data := map[string]any{"Title": "Find the work behind the code.", "Query": query, "Results": page.Results, "Elapsed": time.Since(started).Milliseconds(), "Harnesses": []string{"amp", "claude-code", "codex", "cursor", "cursor-agent", "grok-build", "opencode", "pi"}}
	if err != nil {
		if !errors.Is(err, store.ErrInvalidQuery) {
			app.failure(w, err)
			return
		}
		data["Error"] = err.Error()
	}
	if page.NextCursor != "" {
		values.Set("cursor", page.NextCursor)
		data["Next"] = "/?" + values.Encode()
	}
	app.render(w, r, "search", data)
}

func (app *application) tracePage(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	trace, err := app.store.Trace(r.Context(), id)
	if err != nil {
		app.failure(w, err)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeJSON(w, trace)
		return
	}
	limit, ok := pageLimit(w, r)
	if !ok {
		return
	}
	var events store.EventPage
	switch {
	case r.URL.Query().Has("event"):
		eventID, parseErr := strconv.ParseInt(r.URL.Query().Get("event"), 10, 64)
		if parseErr != nil || eventID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_event", "event must be a positive integer")
			return
		}
		events, err = app.store.EventsFrom(r.Context(), id, eventID, limit)
	case r.URL.Query().Has("key"):
		events, err = app.store.EventsFromKey(r.Context(), id, r.URL.Query().Get("key"), limit)
	default:
		events, err = app.store.Events(r.Context(), id, r.URL.Query().Get("cursor"), limit)
	}
	if err != nil {
		app.failure(w, err)
		return
	}
	title := trace.Title
	if title == "" {
		title = trace.NativeTraceID
	}
	app.render(w, r, "trace", map[string]any{"Title": title, "Trace": trace, "Events": events.Events, "NextCursor": events.NextCursor})
}

func (app *application) eventsAPI(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	limit, ok := pageLimit(w, r)
	if !ok {
		return
	}
	page, err := app.store.Events(r.Context(), id, r.URL.Query().Get("cursor"), limit)
	if err != nil {
		app.failure(w, err)
		return
	}
	writeJSON(w, page)
}

func (app *application) sourcesPage(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	sources, err := app.store.Sources(r.Context(), id)
	if err != nil {
		app.failure(w, err)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeJSON(w, sources)
		return
	}
	app.render(w, r, "sources", map[string]any{"Title": "Source Records", "Sources": sources, "EventID": id})
}

func (app *application) revisionPage(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	sources, err := app.store.RevisionSources(r.Context(), id)
	if err != nil {
		app.failure(w, err)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeJSON(w, sources)
		return
	}
	app.render(w, r, "revision", map[string]any{"Title": "Native source files", "Sources": sources, "RevisionID": id})
}

func (app *application) sourceFile(w http.ResponseWriter, r *http.Request) {
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
	if r.URL.Query().Get("download") == "1" || strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(name)}))
		_, _ = io.Copy(w, file)
		return
	}
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
	app.render(w, r, "file", map[string]any{"Title": name, "RevisionID": id, "Path": name, "Content": content, "Truncated": len(data) > previewBytes})
}

func (app *application) importsPage(w http.ResponseWriter, r *http.Request) {
	limit, ok := pageLimit(w, r)
	if !ok {
		return
	}
	page, err := app.store.Imports(r.Context(), r.URL.Query().Get("cursor"), limit)
	if err != nil {
		app.failure(w, err)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeJSON(w, page)
		return
	}
	app.render(w, r, "imports", map[string]any{"Title": "Imports", "Imports": page.Imports, "NextCursor": page.NextCursor})
}

func (app *application) reportPage(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	report, err := app.store.ImportReport(r.Context(), id)
	if err != nil {
		app.failure(w, err)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeJSON(w, report)
		return
	}
	app.render(w, r, "report", map[string]any{"Title": "Import report", "Report": report})
}

func (app *application) machinesPage(w http.ResponseWriter, r *http.Request) {
	machines, err := app.store.Machines(r.Context())
	if err != nil {
		app.failure(w, err)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeJSON(w, machines)
		return
	}
	app.render(w, r, "machines", map[string]any{"Title": "Source machines", "Machines": machines})
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
