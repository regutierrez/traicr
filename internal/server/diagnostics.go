package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// slowRequest is the duration above which a request is logged as a warning,
// so GET /api/v1/logs?level=warn lists slow requests.
const slowRequest = time.Second

// responseRecorder notes the status and body size a handler wrote.
type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	n, err := r.ResponseWriter.Write(data)
	r.bytes += int64(n)
	return n, err
}

// Unwrap lets http.ResponseController reach the connection, so streaming
// import progress can still flush.
func (r *responseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (app *application) logRequest(r *http.Request, response *responseRecorder, duration time.Duration) {
	// The mux sets the pattern and path values on the request it routed; the
	// pattern is empty when no route matched.
	route := r.Pattern
	// The path tells browser pages apart, which all share the "GET /" route.
	attrs := []any{"route", route, "path", r.URL.Path, "status", response.status, "duration_ms", duration.Milliseconds(), "bytes", response.bytes}
	if id, err := strconv.ParseInt(r.PathValue("id"), 10, 64); err == nil && strings.Contains(route, "/traces/{id}") {
		attrs = append(attrs, "trace_id", id)
	}
	level := slog.LevelInfo
	if duration >= slowRequest || response.status >= 500 {
		level = slog.LevelWarn
	}
	app.logger.Log(r.Context(), level, "request", attrs...)
}

// loadReport is what the transcript page measured while loading a trace.
type loadReport struct {
	Revision  string  `json:"revision"`
	FetchMs   float64 `json:"fetch_ms"`
	AdapterMs float64 `json:"adapter_ms"`
	Pages     int     `json:"pages"`
	Events    int     `json:"events"`
	Anomalies []struct {
		Kind     string  `json:"kind"`
		Count    int     `json:"count"`
		Message  string  `json:"message"`
		EventIDs []int64 `json:"event_ids"`
	} `json:"anomalies"`
}

// loadReport records the browser's transcript load timing and adapter
// anomalies in the server log, where GET /api/v1/logs returns them.
func (app *application) loadReport(w http.ResponseWriter, r *http.Request) {
	traceID, ok := requestID(w, r)
	if !ok {
		return
	}
	var report loadReport
	if err := json.Unmarshal([]byte(r.PostForm.Get("report")), &report); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "report must be a JSON load report")
		return
	}
	level := slog.LevelInfo
	if time.Duration(report.FetchMs+report.AdapterMs)*time.Millisecond >= slowRequest {
		level = slog.LevelWarn
	}
	app.logger.Log(r.Context(), level, "transcript load", "trace_id", traceID, "revision", report.Revision,
		"fetch_ms", int64(report.FetchMs), "adapter_ms", int64(report.AdapterMs), "pages", report.Pages, "events", report.Events, "anomalies", len(report.Anomalies))
	for _, anomaly := range report.Anomalies {
		// Pi and Claude Code sessions branch normally, so off-path entries are only informative.
		level := slog.LevelWarn
		if anomaly.Kind == "off_path" {
			level = slog.LevelInfo
		}
		app.logger.Log(r.Context(), level, "transcript anomaly", "trace_id", traceID, "revision", report.Revision,
			"kind", anomaly.Kind, "count", anomaly.Count, "message", anomaly.Message, "event_ids", anomaly.EventIDs)
	}
	w.WriteHeader(http.StatusNoContent)
}
