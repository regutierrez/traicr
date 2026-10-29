package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
)

var ErrTranscriptChanged = errors.New("transcript changed; reload the page to continue")

// transcriptObservation picks each event's observation: the preferred one for
// the merged view (revision 0), otherwise the one the revision recorded.
const transcriptObservation = `CASE WHEN ?=0 THEN e.preferred_observation_id ELSE
 (SELECT x.observation_id FROM observation_revisions x JOIN event_observations v ON v.id=x.observation_id WHERE x.revision_id=? AND v.event_id=e.id ORDER BY v.digest DESC LIMIT 1) END`

// transcriptOrderCapacity is how many transcript orders stay cached. A reader
// pages through one transcript at a time, so a few cover concurrent readers.
const transcriptOrderCapacity = 8

type transcriptOrderKey struct{ trace, revision, generation int64 }

// transcriptOrders caches the event order of recently read transcripts.
// Sorting reads every event's JSON, so sorting once per page made reading a
// whole trace cost the square of its size. The generation in the key changes
// whenever a revision of the trace is normalized, so a cached order is never stale.
type transcriptOrders struct {
	mu    sync.Mutex
	keys  []transcriptOrderKey
	items map[transcriptOrderKey][]int64
}

func (c *transcriptOrders) get(key transcriptOrderKey) ([]int64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ids, ok := c.items[key]
	return ids, ok
}

func (c *transcriptOrders) put(key transcriptOrderKey, ids []int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items == nil {
		c.items = map[transcriptOrderKey][]int64{}
	}
	if _, ok := c.items[key]; ok {
		return
	}
	if len(c.keys) == transcriptOrderCapacity {
		delete(c.items, c.keys[0])
		c.keys = slices.Delete(c.keys, 0, 1)
	}
	c.keys = append(c.keys, key)
	c.items[key] = ids
}

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
	order, err := s.transcriptOrder(ctx, tx, transcriptOrderKey{traceID, revisionID, generation})
	if err != nil {
		return EventPage{}, err
	}
	ids := order[min(position.Offset, int64(len(order))):min(position.Offset+int64(limit), int64(len(order)))]
	page := EventPage{Events: make([]Event, len(ids))}
	if len(ids) > 0 {
		if err := s.transcriptRows(ctx, tx, traceID, revisionID, ids, page.Events); err != nil {
			return EventPage{}, err
		}
	}
	if position.Offset+int64(limit) < int64(len(order)) {
		position.Offset += int64(limit)
		data, _ := json.Marshal(position)
		page.NextCursor = base64.RawURLEncoding.EncodeToString(data)
	}
	return page, nil
}

// transcriptOrder returns the trace's event IDs in transcript order: Amp's
// native message and block order where recorded, otherwise time.
func (s *Store) transcriptOrder(ctx context.Context, tx *sql.Tx, key transcriptOrderKey) ([]int64, error) {
	if ids, ok := s.orders.get(key); ok {
		return ids, nil
	}
	rows, err := tx.QueryContext(ctx, `
SELECT e.id FROM events e JOIN event_observations o ON o.id=`+transcriptObservation+`
WHERE e.trace_id=?
ORDER BY COALESCE(json_extract(o.event_json,'$.metadata.transcript_order'),0),
 CASE WHEN json_extract(o.event_json,'$.metadata.transcript_order') IS NULL THEN e.sort_time ELSE '' END,
 COALESCE(json_extract(o.event_json,'$.metadata.transcript_block'),0),e.sort_key,e.id`, key.revision, key.revision, key.trace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.orders.put(key, ids)
	return ids, nil
}

// transcriptRows fills events with the rows for ids, in the order of ids.
func (s *Store) transcriptRows(ctx context.Context, tx *sql.Tx, traceID, revisionID int64, ids []int64, events []Event) error {
	index := make(map[int64]int, len(ids))
	for i, id := range ids {
		index[id] = i
	}
	list, _ := json.Marshal(ids)
	rows, err := tx.QueryContext(ctx, `
SELECT e.id,o.event_json,
 COALESCE((SELECT x.revision_id FROM observation_revisions x JOIN trace_revisions r ON r.id=x.revision_id WHERE x.observation_id=o.id AND (?=0 OR x.revision_id=?) ORDER BY r.native_updated_at DESC,r.id DESC LIMIT 1),0),
 (SELECT json_group_array(a.legacy_key) FROM event_aliases a WHERE a.trace_id=e.trace_id AND a.event_key=e.event_key),
 (SELECT json_group_array(CAST(a.legacy_id AS TEXT)) FROM event_aliases a WHERE a.trace_id=e.trace_id AND a.event_key=e.event_key)
FROM events e JOIN event_observations o ON o.id=`+transcriptObservation+`
WHERE e.id IN (SELECT value FROM json_each(?))`, revisionID, revisionID, revisionID, revisionID, string(list))
	if err != nil {
		return err
	}
	defer rows.Close()
	found := 0
	for rows.Next() {
		var event Event
		var data, keys, oldIDs []byte
		if err := rows.Scan(&event.ID, &data, &event.RevisionID, &keys, &oldIDs); err != nil {
			return err
		}
		id := event.ID
		if err := json.Unmarshal(data, &event.Event); err != nil {
			return err
		}
		event.ID, event.TraceID = id, traceID
		if err := json.Unmarshal(keys, &event.Aliases); err != nil {
			return err
		}
		var legacyIDs []string
		if err := json.Unmarshal(oldIDs, &legacyIDs); err != nil {
			return err
		}
		event.Aliases = append(event.Aliases, legacyIDs...)
		events[index[id]] = event
		found++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if found != len(ids) {
		return fmt.Errorf("transcript page found %d of %d events", found, len(ids))
	}
	return nil
}
