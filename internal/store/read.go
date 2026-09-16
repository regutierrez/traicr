package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/regutierrez/traicr/internal/domain"
)

func (s *Store) TraceID(ctx context.Context, harness, nativeID string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, "SELECT id FROM traces WHERE harness=? AND native_trace_id=?", harness, nativeID).Scan(&id)
	return id, err
}

func (s *Store) Trace(ctx context.Context, id int64) (Trace, error) {
	var trace Trace
	err := s.db.QueryRowContext(ctx, `SELECT t.id,t.harness,t.native_trace_id,t.title,t.working_directory,COALESCE(r.remote,''),t.parent_native_trace_id,t.created_at,t.updated_at FROM traces t LEFT JOIN repositories r ON r.id=t.repository_id WHERE t.id=?`, id).Scan(&trace.ID, &trace.Harness, &trace.NativeTraceID, &trace.Title, &trace.WorkingDirectory, &trace.Repository, &trace.ParentNativeTraceID, &trace.CreatedAt, &trace.UpdatedAt)
	if err != nil {
		return Trace{}, err
	}
	trace.Revisions = []Revision{}
	trace.Parents = []TraceRef{}
	trace.Children = []TraceRef{}
	rows, err := s.db.QueryContext(ctx, `SELECT r.id,r.digest,r.native_updated_at,r.collected_at,r.adapter,r.status,r.normalizer_version,COALESCE((SELECT diagnostics_json FROM normalizer_runs n WHERE n.revision_id=r.id ORDER BY n.id DESC LIMIT 1),'[]') FROM trace_revisions r WHERE r.trace_id=? ORDER BY r.id DESC`, id)
	if err != nil {
		return Trace{}, err
	}
	for rows.Next() {
		var revision Revision
		var diagnostics []byte
		if err = rows.Scan(&revision.ID, &revision.Digest, &revision.NativeUpdatedAt, &revision.CollectedAt, &revision.Adapter, &revision.Status, &revision.NormalizerVersion, &diagnostics); err != nil {
			rows.Close()
			return Trace{}, err
		}
		if err = json.Unmarshal(diagnostics, &revision.Diagnostics); err != nil {
			rows.Close()
			return Trace{}, err
		}
		trace.Revisions = append(trace.Revisions, revision)
	}
	if err = rows.Close(); err != nil {
		return Trace{}, err
	}
	if trace.ParentNativeTraceID != "" {
		var parent TraceRef
		err = s.db.QueryRowContext(ctx, "SELECT id,native_trace_id,title FROM traces WHERE harness=? AND native_trace_id=?", trace.Harness, trace.ParentNativeTraceID).Scan(&parent.ID, &parent.NativeTraceID, &parent.Title)
		if err == nil {
			trace.Parents = append(trace.Parents, parent)
		} else if !errors.Is(err, sql.ErrNoRows) {
			return Trace{}, err
		}
	}
	rows, err = s.db.QueryContext(ctx, "SELECT id,native_trace_id,title FROM traces WHERE harness=? AND parent_native_trace_id=? ORDER BY id", trace.Harness, trace.NativeTraceID)
	if err != nil {
		return Trace{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var child TraceRef
		if err = rows.Scan(&child.ID, &child.NativeTraceID, &child.Title); err != nil {
			return Trace{}, err
		}
		trace.Children = append(trace.Children, child)
	}
	return trace, rows.Err()
}

func (s *Store) Events(ctx context.Context, id int64, encodedCursor string, limit int) (EventPage, error) {
	if err := s.requireTrace(ctx, id); err != nil {
		return EventPage{}, err
	}
	position, err := s.decodeEventCursor(ctx, id, encodedCursor)
	if err != nil {
		return EventPage{}, err
	}
	return s.eventsAt(ctx, id, position, limit)
}

func (s *Store) EventsFrom(ctx context.Context, traceID, eventID int64, limit int) (EventPage, error) {
	position, err := s.eventPosition(ctx, traceID, "id=?", eventID)
	if err != nil {
		return EventPage{}, err
	}
	return s.eventsAt(ctx, traceID, position, limit)
}

func (s *Store) EventsFromKey(ctx context.Context, traceID int64, eventKey string, limit int) (EventPage, error) {
	position, err := s.eventPosition(ctx, traceID, "event_key=?", eventKey)
	if err != nil {
		return EventPage{}, err
	}
	return s.eventsAt(ctx, traceID, position, limit)
}

func (s *Store) eventsAt(ctx context.Context, traceID int64, position eventCursor, limit int) (EventPage, error) {
	limit = pageLimit(limit)
	rows, err := s.db.QueryContext(ctx, `SELECT e.id,e.sort_time,e.sort_key,o.event_json,(SELECT COUNT(*) FROM event_observations x WHERE x.event_id=e.id) FROM events e JOIN event_observations o ON o.id=e.preferred_observation_id WHERE e.trace_id=? AND e.id<=? AND (e.sort_time>? OR (e.sort_time=? AND e.sort_key>?) OR (e.sort_time=? AND e.sort_key=? AND e.id>=?)) ORDER BY e.sort_time,e.sort_key,e.id LIMIT ?`, traceID, position.MaxID, position.Time, position.Time, position.Key, position.Time, position.Key, position.ID, limit+1)
	if err != nil {
		return EventPage{}, err
	}
	defer rows.Close()
	page := EventPage{Events: []Event{}}
	var lastTime, lastKey string
	var lastID int64
	for rows.Next() {
		var event Event
		var eventID int64
		var sortTime, sortKey string
		var encoded []byte
		if err = rows.Scan(&eventID, &sortTime, &sortKey, &encoded, &event.ObservationCount); err != nil {
			return EventPage{}, err
		}
		if err = json.Unmarshal(encoded, &event.Event); err != nil {
			return EventPage{}, err
		}
		event.ID = eventID
		event.TraceID = traceID
		page.Events = append(page.Events, event)
		if len(page.Events) <= limit {
			lastTime, lastKey, lastID = sortTime, sortKey, eventID
		}
	}
	if len(page.Events) > limit {
		page.Events = page.Events[:limit]
		page.NextCursor = encodeEventCursor(eventCursor{MaxID: position.MaxID, Time: lastTime, Key: lastKey, ID: lastID + 1})
	}
	return page, rows.Err()
}

func (s *Store) decodeEventCursor(ctx context.Context, traceID int64, encoded string) (eventCursor, error) {
	if encoded != "" {
		b, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			return eventCursor{}, fmt.Errorf("%w: invalid cursor", ErrInvalidQuery)
		}
		var position eventCursor
		if err = json.Unmarshal(b, &position); err != nil || position.MaxID <= 0 || position.ID < 0 {
			return eventCursor{}, fmt.Errorf("%w: invalid cursor", ErrInvalidQuery)
		}
		return position, nil
	}
	var maximum int64
	if err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(id),0) FROM events WHERE trace_id=?", traceID).Scan(&maximum); err != nil {
		return eventCursor{}, err
	}
	return eventCursor{MaxID: maximum}, nil
}

func (s *Store) eventPosition(ctx context.Context, traceID int64, condition string, value any) (eventCursor, error) {
	var position eventCursor
	err := s.db.QueryRowContext(ctx, "SELECT sort_time,sort_key,id FROM events WHERE trace_id=? AND "+condition, traceID, value).Scan(&position.Time, &position.Key, &position.ID)
	if errors.Is(err, sql.ErrNoRows) {
		aliasCondition := "a.legacy_key=?"
		if condition == "id=?" {
			aliasCondition = "a.legacy_id=?"
		}
		err = s.db.QueryRowContext(ctx, "SELECT e.sort_time,e.sort_key,e.id FROM event_aliases a JOIN events e ON e.trace_id=a.trace_id AND e.event_key=a.event_key WHERE a.trace_id=? AND "+aliasCondition, traceID, value).Scan(&position.Time, &position.Key, &position.ID)
	}
	if err != nil {
		return eventCursor{}, err
	}
	if err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(id),0) FROM events WHERE trace_id=?", traceID).Scan(&position.MaxID); err != nil {
		return eventCursor{}, err
	}
	return position, nil
}

func encodeEventCursor(position eventCursor) string {
	b, _ := json.Marshal(position)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (s *Store) Sources(ctx context.Context, eventID int64) ([]Source, error) {
	var resolvedID int64
	err := s.db.QueryRowContext(ctx, "SELECT id FROM events WHERE id=?", eventID).Scan(&resolvedID)
	if errors.Is(err, sql.ErrNoRows) {
		err = s.db.QueryRowContext(ctx, `SELECT e.id FROM event_aliases a JOIN events e ON e.trace_id=a.trace_id AND e.event_key=a.event_key WHERE a.legacy_id=?`, eventID).Scan(&resolvedID)
	}
	if err != nil {
		return nil, err
	}
	eventID = resolvedID
	rows, err := s.db.QueryContext(ctx, `SELECT o.id,r.id,r.digest,o.event_json,COALESCE(src.path,''),COALESCE(src.line,0),COALESCE(ro.object_digest,''),COALESCE(obj.size,0) FROM event_observations o JOIN observation_revisions x ON x.observation_id=o.id JOIN trace_revisions r ON r.id=x.revision_id LEFT JOIN observation_sources src ON src.observation_id=o.id AND src.revision_id=r.id LEFT JOIN revision_objects ro ON ro.revision_id=r.id AND ro.relative_path=src.path LEFT JOIN source_objects obj ON obj.digest=ro.object_digest WHERE o.event_id=? ORDER BY o.id,r.id,src.ordinal`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sources := []Source{}
	for rows.Next() {
		var observationID, revisionID int64
		var revisionDigest string
		var encoded []byte
		var source Source
		if err = rows.Scan(&observationID, &revisionID, &revisionDigest, &encoded, &source.Path, &source.Line, &source.ObjectDigest, &source.Size); err != nil {
			return nil, err
		}
		var event domain.Event
		if err = json.Unmarshal(encoded, &event); err != nil {
			return nil, err
		}
		source.ObservationID = observationID
		source.RevisionID = revisionID
		source.RevisionDigest = revisionDigest
		source.Event = event
		sources = append(sources, source)
	}
	return sources, rows.Err()
}
func (s *Store) RevisionSources(ctx context.Context, revisionID int64) ([]SourceObject, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx, "SELECT 1 FROM trace_revisions WHERE id=?", revisionID).Scan(&exists); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT ro.relative_path,ro.object_digest,so.size FROM revision_objects ro JOIN source_objects so ON so.digest=ro.object_digest WHERE ro.revision_id=? ORDER BY ro.relative_path`, revisionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	objects := []SourceObject{}
	for rows.Next() {
		object := SourceObject{RevisionID: revisionID}
		if err = rows.Scan(&object.Path, &object.Digest, &object.Size); err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	return objects, rows.Err()
}

func (s *Store) OpenSource(ctx context.Context, revisionID int64, path string) (io.ReadCloser, error) {
	var objectPath string
	err := s.db.QueryRowContext(ctx, `SELECT so.path FROM revision_objects ro JOIN source_objects so ON so.digest=ro.object_digest WHERE ro.revision_id=? AND ro.relative_path=?`, revisionID, path).Scan(&objectPath)
	if err != nil {
		return nil, err
	}
	return os.Open(filepath.Join(s.objects, objectPath))
}

func (s *Store) Imports(ctx context.Context, encodedCursor string, limit int) (ImportPage, error) {
	position, err := decodeCursor(ctx, s.db, encodedCursor, "imports", "1=1")
	if err != nil {
		return ImportPage{}, err
	}
	limit = pageLimit(limit)
	rows, err := s.db.QueryContext(ctx, `SELECT i.id,i.created_at,m.id,m.hostname,m.os,m.arch,i.report_json FROM imports i JOIN source_machines m ON m.id=i.machine_id WHERE i.id<=? AND i.id<? ORDER BY i.id DESC LIMIT ?`, position.Max, position.Last, limit+1)
	if err != nil {
		return ImportPage{}, err
	}
	defer rows.Close()
	page := ImportPage{Imports: []domain.ImportReport{}}
	for rows.Next() {
		var report domain.ImportReport
		var encoded []byte
		if err = rows.Scan(&report.ID, &report.CreatedAt, &report.Machine.ID, &report.Machine.Hostname, &report.Machine.OS, &report.Machine.Arch, &encoded); err != nil {
			return ImportPage{}, err
		}
		if len(encoded) > 0 {
			if err = json.Unmarshal(encoded, &report); err != nil {
				return ImportPage{}, err
			}
		}
		page.Imports = append(page.Imports, report)
	}
	if len(page.Imports) > limit {
		page.Imports = page.Imports[:limit]
		page.NextCursor = encodeCursor(cursor{Last: page.Imports[len(page.Imports)-1].ID, Max: position.Max})
	}
	return page, rows.Err()
}

func (s *Store) ImportReport(ctx context.Context, id int64) (domain.ImportReport, error) {
	var report domain.ImportReport
	var encoded []byte
	if err := s.db.QueryRowContext(ctx, `SELECT i.id,i.created_at,m.id,m.hostname,m.os,m.arch,i.report_json FROM imports i JOIN source_machines m ON m.id=i.machine_id WHERE i.id=?`, id).Scan(&report.ID, &report.CreatedAt, &report.Machine.ID, &report.Machine.Hostname, &report.Machine.OS, &report.Machine.Arch, &encoded); err != nil {
		return domain.ImportReport{}, err
	}
	if len(encoded) > 0 {
		if err := json.Unmarshal(encoded, &report); err != nil {
			return domain.ImportReport{}, err
		}
	}
	return report, nil
}

func (s *Store) Machines(ctx context.Context) ([]Machine, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,hostname,os,arch,first_seen,last_seen FROM source_machines ORDER BY last_seen DESC,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	machines := []Machine{}
	for rows.Next() {
		var machine Machine
		if err = rows.Scan(&machine.ID, &machine.Hostname, &machine.OS, &machine.Arch, &machine.FirstSeen, &machine.LastSeen); err != nil {
			return nil, err
		}
		machines = append(machines, machine)
	}
	return machines, rows.Err()
}

func decodeCursor(ctx context.Context, db *sql.DB, encoded, table, condition string, args ...any) (cursor, error) {
	if encoded != "" {
		b, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			return cursor{}, fmt.Errorf("%w: invalid cursor", ErrInvalidQuery)
		}
		var position cursor
		if err = json.Unmarshal(b, &position); err != nil || position.Last <= 0 || position.Max <= 0 {
			return cursor{}, fmt.Errorf("%w: invalid cursor", ErrInvalidQuery)
		}
		return position, nil
	}
	query := fmt.Sprintf("SELECT COALESCE(MAX(id),0) FROM %s WHERE %s", table, condition)
	var maximum int64
	if err := db.QueryRowContext(ctx, query, args...).Scan(&maximum); err != nil {
		return cursor{}, err
	}
	return cursor{Last: maximum + 1, Max: maximum}, nil
}

func encodeCursor(position cursor) string {
	b, _ := json.Marshal(position)
	return base64.RawURLEncoding.EncodeToString(b)
}

func pageLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	return min(limit, 200)
}

func (s *Store) requireTrace(ctx context.Context, id int64) error {
	var exists int
	return s.db.QueryRowContext(ctx, "SELECT 1 FROM traces WHERE id=?", id).Scan(&exists)
}
