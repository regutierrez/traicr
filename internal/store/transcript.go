package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrTranscriptChanged = errors.New("transcript changed; reload the page to continue")

func (s *Store) TranscriptEvents(ctx context.Context, traceID, revisionID int64, cursor string, limit int) (EventPage, error) {
	if err := s.requireTrace(ctx, traceID); err != nil {
		return EventPage{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return EventPage{}, err
	}
	defer tx.Rollback()
	if revisionID != 0 {
		var exists int
		if err := tx.QueryRowContext(ctx, "SELECT 1 FROM trace_revisions WHERE id=? AND trace_id=?", revisionID, traceID).Scan(&exists); err != nil {
			return EventPage{}, err
		}
	}
	var generation int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(n.id),0) FROM normalizer_runs n JOIN trace_revisions r ON r.id=n.revision_id WHERE r.trace_id=? AND (?=0 OR r.id=?)`, traceID, revisionID, revisionID).Scan(&generation); err != nil {
		return EventPage{}, err
	}
	position := struct{ Trace, Revision, Offset, Generation int64 }{Trace: traceID, Revision: revisionID, Generation: generation}
	if cursor != "" {
		position.Generation = 0
		data, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil || json.Unmarshal(data, &position) != nil || position.Trace != traceID || position.Revision != revisionID || position.Offset < 0 {
			return EventPage{}, fmt.Errorf("%w: invalid transcript cursor", ErrInvalidQuery)
		}
		if position.Generation != generation {
			return EventPage{}, ErrTranscriptChanged
		}
	}
	limit = pageLimit(limit)
	rows, err := tx.QueryContext(ctx, `
SELECT e.id,o.event_json,
 COALESCE((SELECT x.revision_id FROM observation_revisions x JOIN trace_revisions r ON r.id=x.revision_id WHERE x.observation_id=o.id AND (?=0 OR x.revision_id=?) ORDER BY r.native_updated_at DESC,r.id DESC LIMIT 1),0),
 (SELECT json_group_array(a.legacy_key) FROM event_aliases a WHERE a.trace_id=e.trace_id AND a.event_key=e.event_key),
 (SELECT json_group_array(CAST(a.legacy_id AS TEXT)) FROM event_aliases a WHERE a.trace_id=e.trace_id AND a.event_key=e.event_key)
FROM events e JOIN event_observations o ON o.id=CASE WHEN ?=0 THEN e.preferred_observation_id ELSE
 (SELECT x.observation_id FROM observation_revisions x JOIN event_observations v ON v.id=x.observation_id WHERE x.revision_id=? AND v.event_id=e.id ORDER BY v.digest DESC LIMIT 1) END
WHERE e.trace_id=?
ORDER BY COALESCE(json_extract(o.event_json,'$.metadata.transcript_order'),0),
 CASE WHEN json_extract(o.event_json,'$.metadata.transcript_order') IS NULL THEN e.sort_time ELSE '' END,
 COALESCE(json_extract(o.event_json,'$.metadata.transcript_block'),0),e.sort_key,e.id
LIMIT ? OFFSET ?`, revisionID, revisionID, revisionID, revisionID, traceID, limit+1, position.Offset)
	if err != nil {
		return EventPage{}, err
	}
	defer rows.Close()
	page := EventPage{Events: []Event{}}
	for rows.Next() {
		var event Event
		var data, keys, ids []byte
		if err := rows.Scan(&event.ID, &data, &event.RevisionID, &keys, &ids); err != nil {
			return EventPage{}, err
		}
		id := event.ID
		if err := json.Unmarshal(data, &event.Event); err != nil {
			return EventPage{}, err
		}
		event.ID, event.TraceID = id, traceID
		if err := json.Unmarshal(keys, &event.Aliases); err != nil {
			return EventPage{}, err
		}
		var oldIDs []string
		if err := json.Unmarshal(ids, &oldIDs); err != nil {
			return EventPage{}, err
		}
		event.Aliases = append(event.Aliases, oldIDs...)
		page.Events = append(page.Events, event)
	}
	if err := rows.Err(); err != nil {
		return EventPage{}, err
	}
	if len(page.Events) > limit {
		page.Events = page.Events[:limit]
		position.Offset += int64(limit)
		data, _ := json.Marshal(position)
		page.NextCursor = base64.RawURLEncoding.EncodeToString(data)
	}
	return page, nil
}
