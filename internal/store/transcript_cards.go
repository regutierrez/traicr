package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

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
}

// TranscriptPage uses session-level cursors so repeated matches cannot duplicate cards.
type TranscriptPage struct {
	Cards      []TranscriptCard
	NextCursor string
}

type transcriptCursor struct {
	ID      int64  `json:"id"`
	Updated string `json:"updated"`
	MaxID   int64  `json:"max_id"`
}

func (s *Store) TranscriptCards(ctx context.Context, query SearchQuery) (TranscriptPage, error) {
	encoded := query.Cursor
	query.Cursor = ""
	prepared, err := compileSearch(query)
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
	joins := " FROM traces t LEFT JOIN repositories repo ON repo.id=t.repository_id"
	where := []string{"t.id<=?"}
	args := []any{position.MaxID}
	if position.ID != 0 {
		where = append(where, "(t.updated_at<? OR (t.updated_at=? AND t.id<?))")
		args = append(args, position.Updated, position.Updated, position.ID)
	}
	text := "''"
	needsEvents := query.Query != "" || query.Model != "" || query.Role != "" || query.Kind != "" || query.Tool != "" || query.After != "" || query.Before != ""
	if needsEvents {
		joins += " JOIN events e ON e.trace_id=t.id JOIN event_observations o ON o.event_id=e.id JOIN observation_revisions ox ON ox.observation_id=o.id"
		text = "ox.searchable_text"
		if prepared.index != "" {
			where = append(where, "EXISTS(SELECT 1 FROM search_chunks sc JOIN "+prepared.index+" ON "+prepared.index+".rowid=sc.id WHERE sc.observation_id=o.id AND sc.revision_id=ox.revision_id AND "+prepared.index+" MATCH ?)")
			args = append(args, prepared.indexQuery)
		}
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
		where = append(where, "(repo.remote=? OR EXISTS(SELECT 1 FROM repository_paths rp WHERE rp.repository_id=repo.id AND rp.path=?))")
		args = append(args, query.Repository, query.Repository)
	}
	if query.Machine != "" {
		if needsEvents {
			where = append(where, "EXISTS(SELECT 1 FROM revision_machines rm WHERE rm.revision_id=ox.revision_id AND rm.machine_id=?)")
		} else {
			where = append(where, "EXISTS(SELECT 1 FROM trace_revisions rev JOIN revision_machines rm ON rm.revision_id=rev.id WHERE rev.trace_id=t.id AND rm.machine_id=?)")
		}
		args = append(args, query.Machine)
	}
	rows, err := s.db.QueryContext(ctx, "SELECT t.id,t.title,t.harness,t.native_trace_id,COALESCE(repo.remote,repo.root,''),t.updated_at,"+text+joins+" WHERE "+strings.Join(where, " AND ")+" ORDER BY t.updated_at DESC,t.id DESC", args...)
	if err != nil {
		return TranscriptPage{}, err
	}
	page := TranscriptPage{Cards: []TranscriptCard{}}
	limit := pageLimit(query.Limit)
	var lastID int64
	for rows.Next() {
		var card TranscriptCard
		var matchedText string
		if err := rows.Scan(&card.ID, &card.Title, &card.Harness, &card.NativeTraceID, &card.Repository, &card.UpdatedAt, &matchedText); err != nil {
			rows.Close()
			return TranscriptPage{}, err
		}
		if card.ID == lastID || !searchMatches(prepared, matchedText) {
			continue
		}
		lastID = card.ID
		if query.Query != "" {
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
	if len(page.Cards) > limit {
		page.Cards = page.Cards[:limit]
		last := page.Cards[len(page.Cards)-1]
		b, _ := json.Marshal(transcriptCursor{ID: last.ID, Updated: last.UpdatedAt, MaxID: position.MaxID})
		page.NextCursor = base64.RawURLEncoding.EncodeToString(b)
	}
	for i := range page.Cards {
		card := &page.Cards[i]
		err := s.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM events WHERE trace_id=?),(SELECT COUNT(*) FROM trace_revisions WHERE trace_id=?),COALESCE((SELECT substr(o.text,1,320) FROM events e JOIN event_observations o ON o.id=e.preferred_observation_id WHERE e.trace_id=? AND o.role='user' AND o.kind='message' ORDER BY e.sort_time,e.sort_key,e.id LIMIT 1),'')`, card.ID, card.ID, card.ID).Scan(&card.EventCount, &card.RevisionCount, &card.Preview)
		if err != nil {
			return TranscriptPage{}, err
		}
	}
	return page, nil
}
