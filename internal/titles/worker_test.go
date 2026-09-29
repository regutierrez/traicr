package titles

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/regutierrez/traicr/internal/domain"
	"github.com/regutierrez/traicr/internal/store"
)

type fakeNamer struct {
	prompts []string
	reply   func(traceID int64) (string, error)
}

func (f *fakeNamer) Generate(_ context.Context, traceID int64, prompt string) (string, error) {
	f.prompts = append(f.prompts, prompt)
	return f.reply(traceID)
}

func importTrace(t *testing.T, s *store.Store, nativeID, title string, messages ...string) int64 {
	t.Helper()
	content := []byte(nativeID)
	sum := sha256.Sum256(content)
	descriptor := domain.Descriptor{Path: "traces/000001", Harness: "synthetic", Adapter: "test", NativeTraceID: nativeID, NativeUpdatedAt: "2026-08-22T10:00:00Z", RevisionDigest: nativeID, Title: title, Files: []domain.File{{Path: "source/records.jsonl", Size: int64(len(content)), SHA256: hex.EncodeToString(sum[:])}}}
	manifest := domain.Manifest{FormatVersion: 1, CreatedAt: "2026-08-22T10:00:00Z", SourceMachine: domain.Machine{ID: "machine", Hostname: "machine", OS: "linux", Arch: "amd64"}, Traces: []domain.Descriptor{descriptor}}
	var events []domain.Event
	for i, text := range messages {
		events = append(events, domain.Event{Key: nativeID + string(rune('a'+i)), Kind: "message", Role: "user", Text: text})
	}
	report, err := s.Import(context.Background(), manifest, fstest.MapFS{"traces/000001/source/records.jsonl": &fstest.MapFile{Data: content}}, func(context.Context, domain.Descriptor, fs.FS) (domain.Normalization, error) {
		return domain.Normalization{Version: 1, Status: "normalized", Events: events}, nil
	}, nil)
	if err != nil || report.Failed != 0 {
		t.Fatalf("import %s: %+v, %v", nativeID, report, err)
	}
	return report.Traces[0].TraceID
}

func testWorker(t *testing.T, namer Namer) (*Worker, *store.Store) {
	t.Helper()
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	worker := NewWorker(s, namer, slog.New(slog.NewTextHandler(io.Discard, nil)))
	worker.Pause = 0
	worker.Now = func() time.Time { return time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC) }
	return worker, s
}

func TestWorkerNamesUntitledTracesOnce(t *testing.T) {
	namer := &fakeNamer{reply: func(int64) (string, error) { return "Fix the login redirect", nil }}
	worker, s := testWorker(t, namer)
	untitled := importTrace(t, s, "untitled", "", "<timestamp>now</timestamp><user_query>why does login redirect forever?</user_query>")
	empty := importTrace(t, s, "empty", "", "<timestamp>only context</timestamp>")
	named := importTrace(t, s, "named", "Pi name", "hello")

	recorded, err := worker.RunOnce(context.Background())
	if err != nil || recorded != 2 {
		t.Fatalf("RunOnce = %d, %v; want the untitled and empty traces recorded", recorded, err)
	}
	if len(namer.prompts) != 1 || !strings.Contains(namer.prompts[0], "why does login redirect forever?") || strings.Contains(namer.prompts[0], "timestamp") {
		t.Fatalf("prompts = %q", namer.prompts)
	}
	for id, want := range map[int64]string{untitled: "Fix the login redirect", empty: "", named: "Pi name"} {
		if trace, err := s.Trace(context.Background(), id); err != nil || trace.Title != want {
			t.Errorf("trace %d title = %q, %v; want %q", id, trace.Title, err, want)
		}
	}
	if recorded, err := worker.RunOnce(context.Background()); err != nil || recorded != 0 || len(namer.prompts) != 1 {
		t.Fatalf("second pass = %d, %v, %d prompts; want nothing to do", recorded, err, len(namer.prompts))
	}
}

func TestWorkerStopsOnAPIFailuresAndSkipsRejectedTracesOnlyAfterSuccess(t *testing.T) {
	rejection := &RequestError{Status: 400, Message: "bad request"}
	namer := &fakeNamer{reply: func(int64) (string, error) { return "", rejection }}
	worker, s := testWorker(t, namer)
	first := importTrace(t, s, "first", "", "first task")
	second := importTrace(t, s, "second", "", "second task")

	// Before any success a 400 may mean a bad model name, so nothing is recorded.
	if recorded, err := worker.RunOnce(context.Background()); !errors.Is(err, rejection) || recorded != 0 {
		t.Fatalf("unproven rejection = %d, %v", recorded, err)
	}
	candidates, _ := s.TitleCandidates(context.Background(), worker.Now(), 10)
	if len(candidates) != 2 {
		t.Fatalf("candidates after unproven rejection = %+v", candidates)
	}

	// Once the API has produced a title, a 400 is about that trace and it is skipped.
	namer.reply = func(id int64) (string, error) {
		if id == second {
			return "Second task title", nil
		}
		return "", rejection
	}
	if recorded, err := worker.RunOnce(context.Background()); err != nil || recorded != 2 {
		t.Fatalf("RunOnce = %d, %v", recorded, err)
	}
	if trace, _ := s.Trace(context.Background(), first); trace.Title != "" || trace.GeneratedTitle != "" {
		t.Fatalf("rejected trace = %+v", trace)
	}

	// Auth and server errors stop the batch without recording anything.
	third := importTrace(t, s, "third", "", "third task")
	namer.reply = func(int64) (string, error) { return "", &RequestError{Status: 401, Message: "bad key"} }
	if recorded, err := worker.RunOnce(context.Background()); err == nil || recorded != 0 {
		t.Fatalf("auth failure = %d, %v", recorded, err)
	}
	candidates, _ = s.TitleCandidates(context.Background(), worker.Now(), 10)
	if len(candidates) != 1 || candidates[0].TraceID != third {
		t.Fatalf("candidates after auth failure = %+v", candidates)
	}
}

func TestPromptKeepsTheRequestAfterALongInjectedBlock(t *testing.T) {
	_, s := testWorker(t, &fakeNamer{})
	skill := "<skill name=\"review\" location=\"/skills/review\">\n" + strings.Repeat("Skill instructions the user did not write. ", 150) + "\n</skill>\n\nreview the traicr title worker"
	id := importTrace(t, s, "skill", "", skill)
	source, err := s.TitleSource(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	prompt := Prompt(source)
	if !strings.Contains(prompt, "review the traicr title worker") || strings.Contains(prompt, "Skill instructions") {
		t.Fatalf("prompt lost the request or kept the skill body:\n%s", prompt)
	}
}
