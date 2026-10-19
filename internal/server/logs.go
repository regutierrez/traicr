package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/regutierrez/traicr/internal/config"
)

// logCapacity is how many records GET /api/v1/logs can return. Older records
// remain only in the process's stdout.
const logCapacity = 1000

// logBuffer keeps the newest redacted log records in memory. It is shared by
// every handler derived from one recording logger.
type logBuffer struct {
	// instance makes cursors from an earlier process invalid instead of wrong.
	instance string
	mu       sync.Mutex
	last     int64
	records  []logRecord
}

type logRecord struct {
	Seq     int64          `json:"seq"`
	Time    string         `json:"time"`
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	level   slog.Level
	at      time.Time
}

func (b *logBuffer) add(record logRecord) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.last++
	record.Seq = b.last
	if len(b.records) == logCapacity {
		b.records = slices.Delete(b.records, 0, 1)
	}
	b.records = append(b.records, record)
}

type logPage struct {
	Records    []logRecord `json:"records"`
	NextCursor string      `json:"next_cursor,omitempty"`
}

type logCursor struct {
	Instance string `json:"instance"`
	Before   int64  `json:"before"`
}

// page returns matching records newest first, strictly below the before sequence when it is set.
func (b *logBuffer) page(minimum slog.Level, since time.Time, before int64, limit int) logPage {
	b.mu.Lock()
	defer b.mu.Unlock()
	page := logPage{Records: []logRecord{}}
	for i := len(b.records) - 1; i >= 0; i-- {
		record := b.records[i]
		if (before > 0 && record.Seq >= before) || record.level < minimum || record.at.Before(since) {
			continue
		}
		if len(page.Records) == limit {
			encoded, _ := json.Marshal(logCursor{Instance: b.instance, Before: page.Records[limit-1].Seq})
			page.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
			break
		}
		page.Records = append(page.Records, record)
	}
	return page
}

// logHandler redacts secrets from every record, writes it to the wrapped
// handler, and keeps a copy for GET /api/v1/logs.
type logHandler struct {
	inner  slog.Handler
	buffer *logBuffer
	// redact replaces configured secrets; nil when none are set.
	redact *strings.Replacer
	// bound and groups mirror WithAttrs and WithGroup for the buffered copy.
	bound  []boundAttr
	groups []string
}

type boundAttr struct {
	groups []string
	attr   slog.Attr
}

// recordLogs wraps logger so its records are redacted and kept for GET
// /api/v1/logs. A logger that already records is returned unchanged.
func recordLogs(logger *slog.Logger, configuration config.ServerConfig) (*slog.Logger, *logBuffer) {
	if handler, ok := logger.Handler().(*logHandler); ok {
		return logger, handler.buffer
	}
	handler := &logHandler{inner: logger.Handler(), buffer: &logBuffer{instance: rand.Text()}}
	secrets := []string{configuration.AdminToken, configuration.Titles.APIKey}
	// Longer secrets first, so one that contains the other is replaced whole.
	slices.SortFunc(secrets, func(a, b string) int { return len(b) - len(a) })
	var pairs []string
	for _, secret := range secrets {
		if secret != "" {
			pairs = append(pairs, secret, "[redacted]")
		}
	}
	if len(pairs) > 0 {
		handler.redact = strings.NewReplacer(pairs...)
	}
	return slog.New(handler), handler.buffer
}

func (h *logHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *logHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, len(attrs))
	bound := slices.Clip(h.bound)
	for i, attr := range attrs {
		clean[i] = h.cleanAttr(attr)
		bound = append(bound, boundAttr{groups: h.groups, attr: clean[i]})
	}
	return &logHandler{inner: h.inner.WithAttrs(clean), buffer: h.buffer, redact: h.redact, bound: bound, groups: h.groups}
}

func (h *logHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &logHandler{inner: h.inner.WithGroup(name), buffer: h.buffer, redact: h.redact, bound: h.bound, groups: append(slices.Clip(h.groups), name)}
}

func (h *logHandler) Handle(ctx context.Context, record slog.Record) error {
	clean := slog.NewRecord(record.Time, record.Level, h.text(record.Message), record.PC)
	var attrs []slog.Attr
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, h.cleanAttr(attr))
		return true
	})
	clean.AddAttrs(attrs...)
	fields := map[string]any{}
	for _, bound := range h.bound {
		putLogAttr(fields, bound.groups, bound.attr)
	}
	for _, attr := range attrs {
		putLogAttr(fields, h.groups, attr)
	}
	h.buffer.add(logRecord{
		Time: record.Time.UTC().Format(time.RFC3339Nano), Level: strings.ToLower(record.Level.String()),
		Message: clean.Message, Attrs: fields, level: record.Level, at: record.Time,
	})
	return h.inner.Handle(ctx, clean)
}

func (h *logHandler) text(value string) string {
	if h.redact == nil {
		return value
	}
	return h.redact.Replace(value)
}

// cleanAttr resolves a value and redacts its text. Errors, Stringers, and other
// arbitrary values become strings so no secret can hide inside them.
func (h *logHandler) cleanAttr(attr slog.Attr) slog.Attr {
	value := attr.Value.Resolve()
	switch value.Kind() {
	case slog.KindString:
		return slog.String(attr.Key, h.text(value.String()))
	case slog.KindGroup:
		group := value.Group()
		clean := make([]slog.Attr, len(group))
		for i, member := range group {
			clean[i] = h.cleanAttr(member)
		}
		return slog.Attr{Key: attr.Key, Value: slog.GroupValue(clean...)}
	case slog.KindAny:
		if err, ok := value.Any().(error); ok {
			return slog.String(attr.Key, h.text(err.Error()))
		}
		return slog.String(attr.Key, h.text(fmt.Sprint(value.Any())))
	}
	return slog.Attr{Key: attr.Key, Value: value}
}

// putLogAttr places a cleaned attribute the way slog's JSON handler would:
// empty attributes and groups are dropped and an unnamed group is inlined.
func putLogAttr(fields map[string]any, groups []string, attr slog.Attr) {
	if attr.Equal(slog.Attr{}) {
		return
	}
	if attr.Value.Kind() == slog.KindGroup {
		if attr.Key != "" {
			groups = append(slices.Clip(groups), attr.Key)
		}
		for _, member := range attr.Value.Group() {
			putLogAttr(fields, groups, member)
		}
		return
	}
	for _, group := range groups {
		child, ok := fields[group].(map[string]any)
		if !ok {
			child = map[string]any{}
			fields[group] = child
		}
		fields = child
	}
	fields[attr.Key] = logValue(attr.Value)
}

func logValue(value slog.Value) any {
	switch value.Kind() {
	case slog.KindString:
		return value.String()
	case slog.KindInt64:
		return value.Int64()
	case slog.KindUint64:
		return value.Uint64()
	case slog.KindFloat64:
		if f := value.Float64(); !math.IsNaN(f) && !math.IsInf(f, 0) {
			return f
		}
	case slog.KindBool:
		return value.Bool()
	case slog.KindTime:
		return value.Time().UTC().Format(time.RFC3339Nano)
	}
	return value.String()
}

func (app *application) logs(w http.ResponseWriter, r *http.Request) {
	limit, ok := pageLimit(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	minimum := slog.LevelInfo
	switch query.Get("level") {
	case "", "info":
	case "warn":
		minimum = slog.LevelWarn
	case "error":
		minimum = slog.LevelError
	default:
		writeError(w, http.StatusBadRequest, "invalid_query", "level must be info, warn, or error")
		return
	}
	var since time.Time
	if value := query.Get("since"); value != "" {
		var err error
		if since, err = time.Parse(time.RFC3339Nano, value); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_query", "since must be an RFC 3339 timestamp")
			return
		}
	}
	var position logCursor
	if value := query.Get("cursor"); value != "" {
		encoded, err := base64.RawURLEncoding.DecodeString(value)
		if err == nil {
			err = json.Unmarshal(encoded, &position)
		}
		if err != nil || position.Instance != app.logBuffer.instance || position.Before <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_query", "invalid or expired cursor; logs start over when the server restarts")
			return
		}
	}
	writeJSON(w, app.logBuffer.page(minimum, since, position.Before, limit))
}
