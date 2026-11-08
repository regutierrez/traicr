package store_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/regutierrez/traicr/internal/domain"
	"github.com/regutierrez/traicr/internal/store"
)

func TestWrappedCountsEachEventOnceInTheReadersTimeZone(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	call := func(key, tool, input, at string) domain.Event {
		return domain.Event{Key: key, Kind: "tool_call", Role: "assistant", Model: "opus", Tool: tool, Text: input, Timestamp: at}
	}
	prompt := func(key, text, at string) domain.Event {
		return domain.Event{Key: key, Kind: "message", Role: "user", Text: text, Timestamp: at}
	}
	first := []domain.Event{
		// 04:30 UTC on Nov 2 is 23:30 on Sunday Nov 1 in New York.
		prompt("p1", "fix the build please", "2026-11-02T04:30:00Z"),
		call("c1", "Read", `{"path":"/repo/main.go"}`, "2026-11-02T04:31:00Z"),
		call("c2", "Grep", `{"pattern":"x","path":"/repo"}`, "2026-11-02T04:32:00Z"),
		prompt("p2", "I am pleased", "2026-11-02T15:00:00Z"),
		call("c3", "edit_file", `{"path":"file:///repo/main.go"}`, "2026-11-02T15:01:00Z"),
		call("c4", "Bash", `{"command":"go test ./..."}`, "2026-11-02T15:20:00Z"),
		{Key: "r4", Kind: "tool_result", Role: "user", Tool: "Bash", Text: "FAIL", Timestamp: "2026-11-02T15:21:00Z"},
		call("c5", "Bash", `{"command":"go test ./..."}`, "2026-11-02T15:30:00Z"),
	}
	imports := []struct {
		id, parent, revision string
		events               []domain.Event
	}{
		// A second revision records the same Events: nothing counts twice.
		{"a", "", "a-1", first},
		{"a", "", "a-2", first},
		// The subagent's calls belong to no prompt of its parent, so they do not
		// lengthen the parent's last turn, but its session is the parent's.
		{"a-child", "a", "child-1", []domain.Event{
			call("k1", "Write", `{"file_path":"/repo/notes.md"}`, "2026-11-02T15:40:00Z"),
			call("k2", "Read", `{"path":"relative.txt"}`, "2026-11-02T15:41:00Z"),
			call("k3", "Read", "not json", "2026-11-02T15:42:00Z"),
		}},
		// 06:00 UTC is 1 AM in New York: later in the night than 11:30 PM.
		{"b", "", "b-1", []domain.Event{
			prompt("p", "go", "2026-11-04T06:00:00Z"),
			call("c", "Read", `{"path":"/repo/main.go"}`, "2026-11-04T06:00:30Z"),
		}},
		// Another thread's message, in a second subagent of a, is not a prompt: it is
		// not the shortest prompt, the call after it belongs to no turn, and its day
		// has no prompt, so it does not join Nov 1 and 2 to Nov 4 in a streak.
		{"a-relay", "a", "relay-1", []domain.Event{
			{Key: "relay", Kind: "message", Role: "user", Text: "a", Timestamp: "2026-11-03T15:35:00Z", Metadata: []byte(`{"message_meta":{"fromExecutorThreadID":"T-other"}}`)},
			call("c6", "Read", `{}`, "2026-11-03T15:36:00Z"),
		}},
	}
	for _, trace := range imports {
		input := traceInput("machine", trace.revision, "2026-11-05T00:00:00Z", trace.id, trace.events)
		input.manifest.Traces[0].NativeTraceID = trace.id
		input.manifest.Traces[0].ParentNativeTraceID = trace.parent
		if _, err := s.Import(ctx, input.manifest, input.files, normalizer(trace.events, "normalized", 1), nil); err != nil {
			t.Fatal(err)
		}
	}
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.Wrapped(ctx, "", "", newYork)
	if err != nil {
		t.Fatal(err)
	}
	if line := fmt.Sprint(got.Sessions, got.Prompts, got.ToolCalls, got.Polite, got.ActiveDays, got.LongestStreak); line != "2 3 10 1 3 2" {
		t.Errorf("sessions prompts calls polite days streak = %s", line)
	}
	if got.Hours[23] != 1 || got.Hours[10] != 1 || got.Hours[1] != 1 || got.Weekdays[0] != 1 || got.Weekdays[1] != 1 || got.Weekdays[3] != 1 {
		t.Errorf("hours %v weekdays %v", got.Hours, got.Weekdays)
	}
	// Prompts record no model, and an empty model is not one.
	if line := fmt.Sprint(got.Tools, got.Files, got.Commands, got.Models); line !=
		"[{Read 5} {Bash 2} {Grep 1} {Write 1} {edit_file 1}] [{/repo/main.go 3} {/repo/notes.md 1}] [{go test ./... 2}] [{opus 2}]" {
		t.Errorf("tools files commands models = %s", line)
	}
	if hole := got.RabbitHole; hole == nil || hole.Prompt != "I am pleased" || hole.ToolCalls != 3 || hole.Minutes != 30 {
		t.Errorf("rabbit hole = %+v", hole)
	}
	if got.Latest == nil || got.Latest.Text != "go" {
		t.Errorf("latest prompt = %+v", got.Latest)
	}
	if got.Shortest == nil || got.Shortest.Text != "go" || got.Longest == nil || got.Longest.Text != "fix the build please" {
		t.Errorf("shortest %+v longest %+v", got.Shortest, got.Longest)
	}
	// Session a: Nov 2 04:30, 15:00, 15:15, 15:30 (shared with its subagent), Nov 3 15:30.
	if got.Marathon == nil || got.Marathon.Title != "a" || got.Marathon.ActiveHours != 1.25 || got.ActiveHours != 1.5 {
		t.Errorf("marathon %+v, active hours %v", got.Marathon, got.ActiveHours)
	}

	recent, err := s.Wrapped(ctx, "2026-11-04", "", newYork)
	if err != nil {
		t.Fatal(err)
	}
	if recent.Sessions != 1 || recent.Prompts != 1 || recent.ToolCalls != 1 {
		t.Errorf("recent = %+v", recent)
	}
	if _, err := s.Wrapped(ctx, "yesterday", "", time.UTC); !errors.Is(err, store.ErrInvalidQuery) {
		t.Fatalf("invalid date: %v", err)
	}
}

// Every record goes to whatever came first when two are equal: the earlier prompt,
// the earlier day, the older session.
func TestWrappedBreaksTiesInFavorOfWhatCameFirst(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	call := func(key, input, at string) domain.Event {
		return domain.Event{Key: key, Kind: "tool_call", Role: "assistant", Tool: "Bash", Text: input, Timestamp: at}
	}
	prompt := func(key, text, at string) domain.Event {
		return domain.Event{Key: key, Kind: "message", Role: "user", Text: text, Timestamp: at}
	}
	sameCommand := strings.Repeat("x", 120)
	early := []domain.Event{
		// Two turns of 3 calls over 30 minutes, then one of 2 calls over an hour.
		prompt("p1", "aa", "2026-03-01T01:00:00Z"),
		call("c1", `{}`, "2026-03-01T01:10:00Z"),
		call("c2", `{}`, "2026-03-01T01:20:00Z"),
		call("c3", `{}`, "2026-03-01T01:30:00Z"),
		prompt("p2", "bb", "2026-03-02T01:00:00Z"),
		call("c4", `{}`, "2026-03-02T01:10:00Z"),
		call("c5", `{}`, "2026-03-02T01:20:00Z"),
		call("c6", `{}`, "2026-03-02T01:30:00Z"),
		prompt("p3", strings.Repeat("a", 300), "2026-03-02T12:00:00Z"),
		call("c7", `{"command":["go","vet"]}`, "2026-03-02T12:30:00Z"),
		call("c8", `{"command":"`+sameCommand+`"}`, "2026-03-02T13:00:00Z"),
	}
	// As long a prompt as p3, on p1's day, and nine quarter hours like early.
	late := []domain.Event{prompt("p4", strings.Repeat("b", 300), "2026-03-01T12:00:00Z")}
	for minute := 15; minute <= 120; minute += 15 {
		at := time.Date(2026, 3, 1, 12, minute, 0, 0, time.UTC).Format(time.RFC3339)
		late = append(late, domain.Event{Key: at, Kind: "message", Role: "assistant", Text: "working", Timestamp: at})
	}
	for _, trace := range []struct {
		id     string
		events []domain.Event
	}{{"early", early}, {"late", late}} {
		input := traceInput("machine", trace.id+"-1", "2026-03-05T00:00:00Z", trace.id, trace.events)
		input.manifest.Traces[0].NativeTraceID = trace.id
		if _, err := s.Import(ctx, input.manifest, input.files, normalizer(trace.events, "normalized", 1), nil); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.Wrapped(ctx, "", "", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if hole := got.RabbitHole; hole == nil || hole.Prompt != "aa" || hole.ToolCalls != 3 || hole.Minutes != 30 {
		t.Errorf("rabbit hole = %+v, want the first of the two 3-call turns", hole)
	}
	// p1 and p2 are both 1 AM, the latest clock time; p4, the last prompt, is noon.
	if got.Latest == nil || got.Latest.Text != "aa" {
		t.Errorf("latest prompt = %+v", got.Latest)
	}
	if got.Shortest == nil || got.Shortest.Text != "aa" {
		t.Errorf("shortest prompt = %+v", got.Shortest)
	}
	if longest := got.Longest; longest == nil || longest.Chars != 300 || longest.Text != strings.Repeat("a", 239)+"…" {
		t.Errorf("longest prompt = %+v", longest)
	}
	if got.BusiestDay == nil || *got.BusiestDay != (store.WrappedDay{Date: "2026-03-01", Prompts: 2}) {
		t.Errorf("busiest day = %+v", got.BusiestDay)
	}
	if got.Marathon == nil || got.Marathon.Title != "early" || got.Marathon.ActiveHours != 2.25 {
		t.Errorf("marathon = %+v", got.Marathon)
	}
	// A command given as a list is joined; one exactly at the length limit is kept whole.
	if line := fmt.Sprint(got.Commands); line != "[{go vet 1} {"+sameCommand+" 1}]" {
		t.Errorf("commands = %s", line)
	}
}
