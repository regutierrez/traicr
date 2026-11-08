package store

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// Wrapped is a playful year-in-review of the archive, in the spirit of Spotify
// Wrapped. Each Event counts once, through its preferred observation, so Trace
// Revisions never add to a number. Sessions are the cards of the session list: a
// top-level Trace with its Child Traces, as Activity counts them.
type Wrapped struct {
	Sessions int64 `json:"sessions"`
	// Prompts are user messages a person typed. Amp also records messages from other
	// threads, automations, and plugins as user messages; those are left out.
	Prompts   int64 `json:"prompts"`
	ToolCalls int64 `json:"tool_calls"`
	// ActiveHours counts the quarter hours with any Event, so idle gaps do not count.
	ActiveHours float64 `json:"active_hours"`
	// ActiveDays and LongestStreak count local days with a prompt, so automations
	// that run while nobody is at the keyboard do not add to them.
	ActiveDays    int64 `json:"active_days"`
	LongestStreak int64 `json:"longest_streak"`
	// Hours and Weekdays count prompts by local hour and weekday, Sunday first.
	Hours      [24]int64       `json:"hours"`
	Weekdays   [7]int64        `json:"weekdays"`
	BusiestDay *WrappedDay     `json:"busiest_day,omitempty"`
	Tools      []WrappedCount  `json:"tools"`
	Files      []WrappedCount  `json:"files"`
	Commands   []WrappedCount  `json:"commands"`
	Models     []ActivityCount `json:"models"`
	RabbitHole *WrappedTurn    `json:"rabbit_hole,omitempty"`
	Marathon   *WrappedSession `json:"marathon,omitempty"`
	Polite     int64           `json:"polite_prompts"`
	Shortest   *WrappedPrompt  `json:"shortest_prompt,omitempty"`
	// Latest is the prompt latest in the night, counting a day as ending at 5 AM.
	Latest  *WrappedPrompt `json:"latest_prompt,omitempty"`
	Longest *WrappedPrompt `json:"longest_prompt,omitempty"`
}

type WrappedCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type WrappedDay struct {
	Date    string `json:"date"`
	Prompts int64  `json:"prompts"`
}

// WrappedTurn is the prompt after which the agent made the most tool calls before
// the next prompt.
type WrappedTurn struct {
	TraceID   int64   `json:"trace_id"`
	Prompt    string  `json:"prompt"`
	ToolCalls int64   `json:"tool_calls"`
	Minutes   float64 `json:"minutes"`
}

type WrappedSession struct {
	TraceID     int64   `json:"trace_id"`
	Title       string  `json:"title"`
	ActiveHours float64 `json:"active_hours"`
}

type WrappedPrompt struct {
	TraceID int64  `json:"trace_id"`
	Text    string `json:"text"`
	Chars   int    `json:"chars"`
	At      string `json:"at,omitempty"`
}

var politeWords = regexp.MustCompile(`(?i)\b(please|pls|plz|thanks?|thx)\b`)

// Tools that take a directory or a pattern in their path argument, not a file.
var directoryTools = regexp.MustCompile(`(?i)grep|glob|search|find|list|ls`)

// Wrapped reads the recap for Events between after and before, which take the same
// values as the search filters; empty means unbounded. Hours and days are in location.
func (s *Store) Wrapped(ctx context.Context, after, before string, location *time.Location) (Wrapped, error) {
	after, err := searchDate(after, false)
	if err != nil {
		return Wrapped{}, fmt.Errorf("%w: after must be YYYY-MM-DD or an RFC 3339 timestamp", ErrInvalidQuery)
	}
	before, err = searchDate(before, true)
	if err != nil {
		return Wrapped{}, fmt.Errorf("%w: before must be YYYY-MM-DD or an RFC 3339 timestamp", ErrInvalidQuery)
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Wrapped{}, err
	}
	defer tx.Rollback()

	sessionOf := map[int64]int64{}
	titles := map[int64]string{}
	rows, err := tx.QueryContext(ctx, lineage+"SELECT t.id,COALESCE(l.root,t.id),"+displayTitle+" FROM traces t LEFT JOIN lineage l ON l.id=t.id")
	if err != nil {
		return Wrapped{}, err
	}
	for rows.Next() {
		var trace, session int64
		var title string
		if err := rows.Scan(&trace, &session, &title); err != nil {
			rows.Close()
			return Wrapped{}, err
		}
		sessionOf[trace], titles[trace] = session, title
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Wrapped{}, err
	}

	where, args := []string{"1=1"}, []any{}
	if after != "" {
		where, args = append(where, "o.event_time>=?"), append(args, after)
	}
	if before != "" {
		where, args = append(where, "o.event_time<=?"), append(args, before)
	}
	// Tool inputs are JSON objects; only the path and command are read from them.
	// Reading and parsing JSON is the slow part, so each CASE parses only the rows it
	// is about (SQLite evaluates both sides of AND in a result column), each value is
	// parsed once, and tool results, the largest rows, are never read.
	rows, err = tx.QueryContext(ctx, `SELECT e.trace_id,o.kind,o.role,o.tool,o.model,o.event_time,
		CASE WHEN o.kind='message' AND o.role='user' THEN o.text ELSE '' END,
		CASE WHEN o.kind='tool_call' AND json_valid(o.text) THEN json_extract(o.text,'$.path','$.file_path','$.filePath','$.target_file','$.cmd','$.command') ELSE '' END,
		CASE WHEN o.kind='message' AND o.role='user' THEN COALESCE(json_extract(o.event_json,'$.metadata.message_meta'),'') ELSE '' END
		FROM events e JOIN event_observations o ON o.id=e.preferred_observation_id
		WHERE `+strings.Join(where, " AND ")+` ORDER BY e.trace_id,e.sort_time,e.sort_key,e.id`, args...)
	if err != nil {
		return Wrapped{}, err
	}
	var wrapped Wrapped
	sessions := map[int64]bool{}
	slots := map[int64]map[string]bool{}
	allSlots := map[string]bool{}
	promptDays := map[string]int64{}
	tools, files, commands := map[string]int64{}, map[string]int64{}, map[string]int64{}
	models := map[string]map[int64]bool{}
	var turn *WrappedTurn
	var turnStart, turnLast time.Time
	var latestClock string
	closeTurn := func() {
		if turn == nil || turn.ToolCalls == 0 {
			turn = nil
			return
		}
		turn.Minutes = turnLast.Sub(turnStart).Minutes()
		if hole := wrapped.RabbitHole; hole == nil || turn.ToolCalls > hole.ToolCalls || turn.ToolCalls == hole.ToolCalls && turn.Minutes > hole.Minutes {
			wrapped.RabbitHole = turn
		}
		turn = nil
	}
	lastTrace := int64(0)
	for rows.Next() {
		var trace int64
		var kind, role, tool, model, eventTime, prompt, arguments, meta string
		if err := rows.Scan(&trace, &kind, &role, &tool, &model, &eventTime, &prompt, &arguments, &meta); err != nil {
			rows.Close()
			return Wrapped{}, err
		}
		if trace != lastTrace {
			closeTurn()
			lastTrace = trace
		}
		session := sessionOf[trace]
		sessions[session] = true
		if model != "" {
			addSession(models, model, session)
		}
		at, timeErr := time.Parse(time.RFC3339Nano, eventTime)
		local := at.In(location)
		if timeErr == nil {
			slot := at.UTC().Truncate(15 * time.Minute).Format(time.RFC3339)
			if slots[session] == nil {
				slots[session] = map[string]bool{}
			}
			slots[session][slot], allSlots[slot] = true, true
		}
		switch {
		case kind == "message" && role == "user" && automated(meta):
			// A message another thread, an automation, or a plugin sent ends the turn,
			// but it is not a prompt you typed.
			closeTurn()
		case kind == "message" && role == "user":
			closeTurn()
			wrapped.Prompts++
			text := strings.TrimSpace(prompt)
			chars := utf8.RuneCountInString(text)
			if timeErr == nil {
				wrapped.Hours[local.Hour()]++
				wrapped.Weekdays[local.Weekday()]++
				promptDays[local.Format(time.DateOnly)]++
				turn, turnStart, turnLast = &WrappedTurn{TraceID: trace, Prompt: clip(prompt, 240)}, at, at
				if clock := nightClock(local); wrapped.Latest == nil || clock > latestClock {
					wrapped.Latest, latestClock = &WrappedPrompt{TraceID: trace, Text: clip(text, 160), Chars: chars, At: eventTime}, clock
				}
			}
			if politeWords.MatchString(prompt) {
				wrapped.Polite++
			}
			if text != "" {
				if wrapped.Shortest == nil || chars < wrapped.Shortest.Chars {
					wrapped.Shortest = &WrappedPrompt{TraceID: trace, Text: text, Chars: chars}
				}
				if wrapped.Longest == nil || chars > wrapped.Longest.Chars {
					wrapped.Longest = &WrappedPrompt{TraceID: trace, Text: clip(text, 240), Chars: chars}
				}
			}
		case kind == "tool_call":
			filePath, command := toolArguments(arguments)
			wrapped.ToolCalls++
			tools[cmp.Or(tool, "unknown")]++
			if filePath = strings.TrimPrefix(filePath, "file://"); strings.HasPrefix(filePath, "/") && !directoryTools.MatchString(tool) {
				files[path.Clean(filePath)]++
			}
			if command = strings.TrimSpace(command); command != "" {
				commands[clip(command, 120)]++
			}
			if turn != nil {
				turn.ToolCalls++
			}
		}
		if turn != nil && timeErr == nil && at.After(turnLast) {
			turnLast = at
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Wrapped{}, err
	}
	closeTurn()

	wrapped.Sessions = int64(len(sessions))
	wrapped.ActiveHours = float64(len(allSlots)) / 4
	wrapped.ActiveDays = int64(len(promptDays))
	wrapped.LongestStreak = longestStreak(promptDays)
	// In sorted order, so the earliest day and the oldest session win a tie.
	for _, date := range slices.Sorted(maps.Keys(promptDays)) {
		if day := wrapped.BusiestDay; day == nil || promptDays[date] > day.Prompts {
			wrapped.BusiestDay = &WrappedDay{Date: date, Prompts: promptDays[date]}
		}
	}
	for _, session := range slices.Sorted(maps.Keys(slots)) {
		hours := float64(len(slots[session])) / 4
		if top := wrapped.Marathon; top == nil || hours > top.ActiveHours {
			wrapped.Marathon = &WrappedSession{TraceID: session, Title: titles[session], ActiveHours: hours}
		}
	}
	wrapped.Tools = topCounts(tools, 10)
	wrapped.Files = topCounts(files, 10)
	wrapped.Commands = topCounts(commands, 5)
	wrapped.Models = countSessions(models)
	wrapped.Models = wrapped.Models[:min(5, len(wrapped.Models))]
	return wrapped, nil
}

// nightClock is the time of day counted from 5 AM, as 15:04:05, so the strings
// compare in night order: 2 AM (21:00:00) is later than 11 PM (18:00:00).
func nightClock(local time.Time) string {
	return local.Add(-5 * time.Hour).Format(time.TimeOnly)
}

// toolArguments reads the file path and shell command from the array json_extract
// returns for path, file_path, filePath, target_file, cmd, and command.
// Text that is not JSON was extracted as an empty string and has neither.
func toolArguments(extracted string) (filePath, command string) {
	var values []any
	if json.Unmarshal([]byte(extracted), &values) != nil {
		return "", ""
	}
	for _, value := range values[:4] {
		if text, ok := value.(string); ok && text != "" {
			filePath = text
			break
		}
	}
	return filePath, commandText(values[4:])
}

// commandText is the first command given as a string or as an argument list.
func commandText(values []any) string {
	for _, value := range values {
		switch value := value.(type) {
		case string:
			if value != "" {
				return value
			}
		case []any:
			parts := make([]string, 0, len(value))
			for _, part := range value {
				if text, ok := part.(string); ok {
					parts = append(parts, text)
				}
			}
			if command := strings.Join(parts, " "); command != "" {
				return command
			}
		}
	}
	return ""
}

// automated reports whether Amp recorded the user message as sent by another
// thread, an automation, or a plugin rather than typed by a person.
func automated(meta string) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal([]byte(meta), &fields) != nil {
		return false
	}
	for _, key := range []string{"fromExecutorThreadID", "fromAutomation", "generatedByPlugin", "fromPuck"} {
		if _, ok := fields[key]; ok {
			return true
		}
	}
	return false
}

// longestStreak is the most consecutive calendar days in days.
func longestStreak(days map[string]int64) int64 {
	var best int64
	for day := range days {
		date, err := time.Parse(time.DateOnly, day)
		if _, continued := days[date.AddDate(0, 0, -1).Format(time.DateOnly)]; err != nil || continued {
			continue
		}
		length := int64(0)
		for days[date.Format(time.DateOnly)] > 0 {
			length++
			date = date.AddDate(0, 0, 1)
		}
		best = max(best, length)
	}
	return best
}

func topCounts(counts map[string]int64, limit int) []WrappedCount {
	top := make([]WrappedCount, 0, len(counts))
	for name, count := range counts {
		top = append(top, WrappedCount{Name: name, Count: count})
	}
	slices.SortFunc(top, func(a, b WrappedCount) int {
		return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(a.Name, b.Name))
	})
	return top[:min(limit, len(top))]
}

func clip(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	return string([]rune(text)[:limit-1]) + "…"
}
