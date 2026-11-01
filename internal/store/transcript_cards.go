package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	searchtext "github.com/regutierrez/traicr/internal/search"
)

// TranscriptCard describes a whole session, not an individual matching event.
type TranscriptCard struct {
	ID            int64
	Title         string
	Harness       string
	NativeTraceID string
	Repository    string
	UpdatedAt     string
	MatchSnippet  string
	Preview       string
	EventCount    int
	RevisionCount int
	// MatchedChild is the Child Trace whose Events matched, when the match was not in
	// the Trace itself. Child Traces are listed under their top ancestor.
	MatchedChildID    int64
	MatchedChildTitle string
	// MatchedEventID is the earliest Event of the matched Trace whose text matched q,
	// so a result can open at that message. It is 0 without q.
	MatchedEventID int64
}

// TranscriptPage uses session-level cursors so repeated matches cannot duplicate cards.
type TranscriptPage struct {
	Cards      []TranscriptCard
	NextCursor string
	// Partial means the search budget ran out before the page filled; NextCursor
	// continues the search after the last session it read.
	Partial bool
}

type transcriptCursor struct {
	ID      int64  `json:"id"`
	Updated string `json:"updated"`
	MaxID   int64  `json:"max_id"`
}

// TranscriptCards searches retained observations but returns one card per transcript.
func (s *Store) TranscriptCards(ctx context.Context, query SearchQuery) (TranscriptPage, error) {
	encoded := query.Cursor
	query.Cursor = ""
	prepared, err := s.prepareSearch(ctx, query)
	if err != nil {
		return TranscriptPage{}, err
	}
	query = prepared.query
	var position transcriptCursor
	if encoded != "" {
		b, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			return TranscriptPage{}, fmt.Errorf("%w: invalid transcript cursor", ErrInvalidQuery)
		}
		if err := json.Unmarshal(b, &position); err != nil || position.ID <= 0 || position.MaxID < position.ID || position.Updated == "" {
			return TranscriptPage{}, fmt.Errorf("%w: invalid transcript cursor", ErrInvalidQuery)
		}
	} else if err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(id),0) FROM traces").Scan(&position.MaxID); err != nil {
		return TranscriptPage{}, err
	}
	// Each Trace belongs to the card of its top collected ancestor (r). A child whose
	// parent was never collected, or whose parents form a loop, keeps its own card.
	// The walk goes up from children so every step is a (harness, native_trace_id) lookup.
	const lineage = `WITH RECURSIVE up(id,ancestor) AS (
		SELECT c.id,p.id FROM traces c JOIN traces p ON p.harness=c.harness AND p.native_trace_id=c.parent_native_trace_id WHERE c.parent_native_trace_id<>''
		UNION SELECT u.id,p.id FROM up u JOIN traces a ON a.id=u.ancestor JOIN traces p ON p.harness=a.harness AND p.native_trace_id=a.parent_native_trace_id WHERE a.parent_native_trace_id<>''),
	lineage(id,root) AS (SELECT u.id,u.ancestor FROM up u JOIN traces a ON a.id=u.ancestor
		WHERE NOT EXISTS(SELECT 1 FROM traces p WHERE p.harness=a.harness AND p.native_trace_id=a.parent_native_trace_id)) `
	// The query returns small candidate rows in card order; text and card details are
	// read only for the cards the page keeps. A search starts from the matching index
	// chunks, or from the matching observations when no index applies, never from
	// every Trace: checking each Trace's observations for a match took over 30 seconds
	// on a 1.5M-observation archive.
	var from, event, observation, revision string
	where := []string{"r.id<=?", "t.id<=?"}
	args := []any{position.MaxID, position.MaxID}
	needsEvents := query.Query != "" || query.Model != "" || query.Role != "" || query.Kind != "" || query.Tool != "" || query.After != "" || query.Before != ""
	needsObservation := query.Model != "" || query.Role != "" || query.Kind != "" || query.Tool != "" || query.After != "" || query.Before != ""
	switch {
	case !needsEvents:
		// Browsing lists top-level Traces; their children open from the parent.
		from, event, observation, revision = " FROM traces t", "0", "0", "0"
		where = append(where, "t.id=r.id")
	case prepared.index != "":
		from = " FROM " + prepared.index + " JOIN search_chunks sc ON sc.id=" + prepared.index + ".rowid JOIN events e ON e.id=sc.event_id JOIN traces t ON t.id=e.trace_id"
		event, observation, revision = "e.id", "sc.observation_id", "sc.revision_id"
		if needsObservation {
			from += " JOIN event_observations o ON o.id=sc.observation_id"
		}
		where = append(where, prepared.index+" MATCH ?")
		args = append(args, prepared.indexQuery)
	default:
		from = " FROM event_observations o JOIN observation_revisions ox ON ox.observation_id=o.id JOIN events e ON e.id=o.event_id JOIN traces t ON t.id=e.trace_id"
		event, observation, revision = "e.id", "o.id", "ox.revision_id"
	}
	from += " LEFT JOIN lineage l ON l.id=t.id JOIN traces r ON r.id=COALESCE(l.root,t.id)"
	if position.ID != 0 {
		where = append(where, "(r.updated_at<? OR (r.updated_at=? AND r.id<?))")
		args = append(args, position.Updated, position.Updated, position.ID)
	}
	for _, f := range []struct{ value, clause string }{
		{query.Harness, "t.harness=?"}, {query.Model, "o.model=?"}, {query.Role, "o.role=?"}, {query.Kind, "o.kind=?"}, {query.Tool, "o.tool=?"}, {query.After, "o.event_time>=?"}, {query.Before, "o.event_time<=?"},
	} {
		if f.value != "" {
			where = append(where, f.clause)
			args = append(args, f.value)
		}
	}
	if query.Repository != "" {
		from += " LEFT JOIN repositories repo ON repo.id=r.repository_id"
		where = append(where, "(repo.remote=? OR EXISTS(SELECT 1 FROM repository_paths rp WHERE rp.repository_id=repo.id AND rp.path=?))")
		args = append(args, query.Repository, query.Repository)
	}
	if query.Machine != "" {
		if needsEvents {
			where = append(where, "EXISTS(SELECT 1 FROM revision_machines rm WHERE rm.revision_id="+revision+" AND rm.machine_id=?)")
		} else {
			where = append(where, "EXISTS(SELECT 1 FROM trace_revisions rev JOIN revision_machines rm ON rm.revision_id=rev.id WHERE rev.trace_id=t.id AND rm.machine_id=?)")
		}
		args = append(args, query.Machine)
	}
	// Within a card, a match in the Trace itself comes first, then the earliest Event.
	order := " ORDER BY r.updated_at DESC,r.id DESC,t.id=r.id DESC"
	if needsEvents {
		order += ",e.id"
	}
	rows, err := s.db.QueryContext(ctx, lineage+"SELECT r.id,r.updated_at,t.id,"+event+","+observation+","+revision+from+" WHERE "+strings.Join(where, " AND ")+order, args...)
	if err != nil {
		return TranscriptPage{}, err
	}
	page := TranscriptPage{Cards: []TranscriptCard{}}
	limit := pageLimit(query.Limit)
	deadline := time.Now().Add(s.searchBudget)
	var lastID int64
	// reading is the card whose rows are being read; read is the last card whose
	// rows were all read, where a partial page resumes.
	var reading, read transcriptCursor
	for rows.Next() {
		var card TranscriptCard
		var matchedText string
		var matchedID, eventID, observationID, revisionID int64
		if err := rows.Scan(&card.ID, &card.UpdatedAt, &matchedID, &eventID, &observationID, &revisionID); err != nil {
			rows.Close()
			return TranscriptPage{}, err
		}
		if card.ID != reading.ID {
			if reading.ID != 0 {
				read = reading
				// Stopping only between cards guarantees every page moves forward.
				if time.Now().After(deadline) {
					page.Partial = true
					break
				}
			}
			reading = transcriptCursor{ID: card.ID, Updated: card.UpdatedAt}
		}
		if card.ID == lastID {
			continue
		}
		if query.Query != "" {
			if err := s.db.QueryRowContext(ctx, "SELECT searchable_text FROM observation_revisions WHERE observation_id=? AND revision_id=?", observationID, revisionID).Scan(&matchedText); err != nil {
				rows.Close()
				return TranscriptPage{}, err
			}
			if !searchMatches(prepared, matchedText) {
				continue
			}
		}
		lastID = card.ID
		if matchedID != card.ID {
			card.MatchedChildID = matchedID
		}
		if query.Query != "" {
			card.MatchedEventID = eventID
			if prepared.expression != nil {
				location := prepared.expression.FindStringIndex(matchedText)
				card.MatchSnippet = searchtext.SnippetRange(matchedText, location[0], location[1])
			} else {
				card.MatchSnippet = searchtext.Snippet(matchedText, strings.Trim(query.Query, `"`))
			}
		}
		page.Cards = append(page.Cards, card)
		if len(page.Cards) > limit {
			break
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return TranscriptPage{}, err
	}
	more := len(page.Cards) > limit
	if more {
		page.Cards = page.Cards[:limit]
		last := page.Cards[len(page.Cards)-1]
		read = transcriptCursor{ID: last.ID, Updated: last.UpdatedAt}
	}
	if more || page.Partial {
		read.MaxID = position.MaxID
		b, _ := json.Marshal(read)
		page.NextCursor = base64.RawURLEncoding.EncodeToString(b)
	}
	for i := range page.Cards {
		card := &page.Cards[i]
		err := s.db.QueryRowContext(ctx, `SELECT `+displayTitle+`,t.harness,t.native_trace_id,COALESCE(repo.remote,repo.root,''),(SELECT COUNT(*) FROM events WHERE trace_id=t.id),(SELECT COUNT(*) FROM trace_revisions WHERE trace_id=t.id),COALESCE((SELECT substr(o.text,1,320) FROM events e JOIN event_observations o ON o.id=e.preferred_observation_id WHERE e.trace_id=t.id AND o.role='user' AND o.kind='message' ORDER BY e.sort_time,e.sort_key,e.id LIMIT 1),'') FROM traces t LEFT JOIN repositories repo ON repo.id=t.repository_id WHERE t.id=?`, card.ID).Scan(&card.Title, &card.Harness, &card.NativeTraceID, &card.Repository, &card.EventCount, &card.RevisionCount, &card.Preview)
		if err != nil {
			return TranscriptPage{}, err
		}
		if card.MatchedChildID != 0 {
			if err := s.db.QueryRowContext(ctx, "SELECT "+displayTitle+" FROM traces t WHERE t.id=?", card.MatchedChildID).Scan(&card.MatchedChildTitle); err != nil {
				return TranscriptPage{}, err
			}
		}
	}
	return page, nil
}
