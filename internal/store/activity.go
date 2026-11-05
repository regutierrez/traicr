package store

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Activity counts sessions for the home page. A session is one card of the session
// list: a top-level Trace with its Child Traces. Each count is of distinct sessions,
// so Trace Revisions and repeated Event observations never add to it. A session is
// active in a range when any of its Events was observed in it, the rule the list's
// after and before filters use, so a count matches the list a row filters to.
type Activity struct {
	// Sessions is every session active in the range, or every session without one.
	Sessions int64 `json:"sessions"`
	// Harnesses puts each session under one harness, so their counts add up to Sessions.
	Harnesses []ActivityCount `json:"harnesses"`
	// Models counts a session under every model recorded for it in the range, so a
	// session that switched models counts more than once.
	Models []ActivityCount `json:"models"`
	// WithoutModel counts sessions with no model recorded in the range.
	WithoutModel int64 `json:"without_model"`
	// Days counts the sessions active on each local day, oldest first, leaving out
	// days with none. A session active on three days counts on each of them.
	Days []ActivityDay `json:"days"`
}

// ActivityCount is the number of sessions for one harness, model, or day.
type ActivityCount struct {
	Name     string `json:"name"`
	Sessions int64  `json:"sessions"`
}

type ActivityDay struct {
	Date     string `json:"date"`
	Sessions int64  `json:"sessions"`
}

// Activity reads the sessions active between after and before, which take the same
// values as the search filters; empty means unbounded. Days are calendar days in
// location.
func (s *Store) Activity(ctx context.Context, after, before string, location *time.Location) (Activity, error) {
	after, err := searchDate(after, false)
	if err != nil {
		return Activity{}, fmt.Errorf("%w: after must be YYYY-MM-DD or an RFC 3339 timestamp", ErrInvalidQuery)
	}
	before, err = searchDate(before, true)
	if err != nil {
		return Activity{}, fmt.Errorf("%w: before must be YYYY-MM-DD or an RFC 3339 timestamp", ErrInvalidQuery)
	}
	// One snapshot, so a Trace an import adds between the two reads has a session.
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Activity{}, err
	}
	defer tx.Rollback()
	sessionOf := map[int64]int64{}
	harnessOf := map[int64]string{}
	rows, err := tx.QueryContext(ctx, lineage+"SELECT t.id,COALESCE(l.root,t.id),t.harness FROM traces t LEFT JOIN lineage l ON l.id=t.id")
	if err != nil {
		return Activity{}, err
	}
	for rows.Next() {
		var trace, session int64
		var harness string
		if err := rows.Scan(&trace, &session, &harness); err != nil {
			rows.Close()
			return Activity{}, err
		}
		sessionOf[trace] = session
		if trace == session {
			harnessOf[session] = harness
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Activity{}, err
	}

	// One row per Trace, model, and 15-minute slot it was active in. Every UTC
	// offset in use is a whole number of quarter hours, so a slot never spans two
	// local days.
	where, args := []string{"1=1"}, []any{}
	if after != "" {
		where, args = append(where, "o.event_time>=?"), append(args, after)
	}
	if before != "" {
		where, args = append(where, "o.event_time<=?"), append(args, before)
	}
	rows, err = tx.QueryContext(ctx, `SELECT DISTINCT e.trace_id,o.model,
		substr(o.event_time,1,14)||printf('%02d',CAST(substr(o.event_time,15,2) AS INTEGER)/15*15)
		FROM event_observations o JOIN events e ON e.id=o.event_id WHERE `+strings.Join(where, " AND "), args...)
	if err != nil {
		return Activity{}, err
	}
	active := map[int64]bool{}
	withModel := map[int64]bool{}
	models := map[string]map[int64]bool{}
	days := map[string]map[int64]bool{}
	for rows.Next() {
		var trace int64
		var model, slot string
		if err := rows.Scan(&trace, &model, &slot); err != nil {
			rows.Close()
			return Activity{}, err
		}
		session := sessionOf[trace]
		active[session] = true
		if model != "" {
			withModel[session] = true
			addSession(models, model, session)
		}
		// Event times that are not RFC 3339 are kept as recorded; they have no day.
		if start, err := time.Parse("2006-01-02T15:04", slot); err == nil {
			addSession(days, start.In(location).Format(time.DateOnly), session)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Activity{}, err
	}

	// Without a range every session counts, as the list shows them all, including
	// sessions whose Events carry no time.
	if after == "" && before == "" {
		for session := range harnessOf {
			active[session] = true
		}
	}
	harnesses := map[string]map[int64]bool{}
	for session := range active {
		addSession(harnesses, harnessOf[session], session)
	}
	activity := Activity{
		Sessions:     int64(len(active)),
		Harnesses:    countSessions(harnesses),
		Models:       countSessions(models),
		WithoutModel: int64(len(active) - len(withModel)),
		Days:         make([]ActivityDay, 0, len(days)),
	}
	for date, sessions := range days {
		activity.Days = append(activity.Days, ActivityDay{Date: date, Sessions: int64(len(sessions))})
	}
	slices.SortFunc(activity.Days, func(a, b ActivityDay) int { return cmp.Compare(a.Date, b.Date) })
	return activity, nil
}

func addSession(groups map[string]map[int64]bool, name string, session int64) {
	if groups[name] == nil {
		groups[name] = map[int64]bool{}
	}
	groups[name][session] = true
}

// countSessions lists groups largest first.
func countSessions(groups map[string]map[int64]bool) []ActivityCount {
	counts := make([]ActivityCount, 0, len(groups))
	for name, sessions := range groups {
		counts = append(counts, ActivityCount{Name: name, Sessions: int64(len(sessions))})
	}
	slices.SortFunc(counts, func(a, b ActivityCount) int {
		return cmp.Or(cmp.Compare(b.Sessions, a.Sessions), cmp.Compare(a.Name, b.Name))
	})
	return counts
}
