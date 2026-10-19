package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/regutierrez/traicr/internal/config"
)

func TestLogHandlerRedactsSecretsWhereverTheyAppear(t *testing.T) {
	const admin, titleKey = "admin-SECRET-token", "sk-title-KEY"
	var stdout bytes.Buffer
	logger, buffer := recordLogs(slog.New(slog.NewJSONHandler(&stdout, nil)), config.ServerConfig{AdminToken: admin, Titles: config.TitleConfig{APIKey: titleKey}})
	logger.With("bound", "pre "+admin).WithGroup("request").Error("saw "+titleKey,
		slog.Group("nested", "token", admin), "err", errors.New("failed with "+titleKey), "plain", "kept")

	page := buffer.page(slog.LevelInfo, time.Time{}, 0, 10)
	buffered, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	for name, output := range map[string]string{"stdout": stdout.String(), "buffer": string(buffered)} {
		if strings.Contains(output, admin) || strings.Contains(output, titleKey) || !strings.Contains(output, "[redacted]") || !strings.Contains(output, "kept") {
			t.Errorf("%s: %s", name, output)
		}
	}
	// The buffered copy keeps the same structure stdout has: attributes bound
	// before WithGroup stay at the top, later ones nest inside the group.
	var line map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &line); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"time", "level", "msg"} {
		delete(line, key)
	}
	want, _ := json.Marshal(line)
	got, _ := json.Marshal(page.Records[0].Attrs)
	if string(got) != string(want) || page.Records[0].Message != "saw [redacted]" {
		t.Fatalf("buffered attrs %s, message %q; stdout attrs %s", got, page.Records[0].Message, want)
	}
}

func TestLogHandlerWithoutTitleKeyLeavesTextAlone(t *testing.T) {
	logger, buffer := recordLogs(slog.New(slog.NewJSONHandler(io.Discard, nil)), config.ServerConfig{AdminToken: "token"})
	logger.Info("hello world", "path", "/data")
	record := buffer.page(slog.LevelInfo, time.Time{}, 0, 1).Records[0]
	if record.Message != "hello world" || record.Attrs["path"] != "/data" {
		t.Fatalf("record = %+v", record)
	}
}

func TestLogsPageThroughTheWholeBufferOnce(t *testing.T) {
	logger, buffer := recordLogs(slog.New(slog.NewJSONHandler(io.Discard, nil)), config.ServerConfig{AdminToken: "token"})
	const written = logCapacity + 234
	for i := 1; i <= written; i++ {
		logger.Info("record", "n", i)
	}
	app := &application{logBuffer: buffer}
	get := func(query string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		app.logs(response, httptest.NewRequest(http.MethodGet, "/api/v1/logs?"+query, nil))
		return response
	}
	var seqs []int64
	cursor := ""
	for {
		var page logPage
		if err := json.Unmarshal(get("limit=200&cursor="+cursor).Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		for _, record := range page.Records {
			seqs = append(seqs, record.Seq)
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	// Only the newest logCapacity records are kept, newest first, each once.
	if len(seqs) != logCapacity || seqs[0] != written || seqs[len(seqs)-1] != written-logCapacity+1 {
		t.Fatalf("got %d records from %d to %d", len(seqs), seqs[0], seqs[len(seqs)-1])
	}
	for i := 1; i < len(seqs); i++ {
		if seqs[i] != seqs[i-1]-1 {
			t.Fatalf("records %d then %d", seqs[i-1], seqs[i])
		}
	}

	_, restarted := recordLogs(slog.New(slog.NewJSONHandler(io.Discard, nil)), config.ServerConfig{AdminToken: "token"})
	restarted.add(logRecord{Message: "after restart", level: slog.LevelInfo})
	staleApp := &application{logBuffer: restarted}
	response := httptest.NewRecorder()
	staleApp.logs(response, httptest.NewRequest(http.MethodGet, "/api/v1/logs?cursor="+cursor, nil))
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "invalid_query") {
		t.Fatalf("cursor from another process: %d %s", response.Code, response.Body)
	}
}
