package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/regutierrez/traicr/internal/domain"
)

func (s *Store) DeleteTrace(ctx context.Context, id int64) error {
	select {
	case s.writer <- struct{}{}:
		defer func() { <-s.writer }()
	case <-ctx.Done():
		return ctx.Err()
	}
	result, err := s.db.ExecContext(ctx, "DELETE FROM traces WHERE id=?", id)
	if err != nil {
		return err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if deleted == 0 {
		return sql.ErrNoRows
	}
	return s.gc(ctx)
}

func (s *Store) GC(ctx context.Context) error {
	select {
	case s.writer <- struct{}{}:
		defer func() { <-s.writer }()
	case <-ctx.Done():
		return ctx.Err()
	}
	return s.gc(ctx)
}

func (s *Store) gc(ctx context.Context) error {
	type unused struct{ digest, path string }
	for {
		rows, err := s.db.QueryContext(ctx, "SELECT digest,path FROM source_objects WHERE NOT EXISTS(SELECT 1 FROM revision_objects WHERE object_digest=source_objects.digest) LIMIT 100")
		if err != nil {
			return err
		}
		var objects []unused
		for rows.Next() {
			var object unused
			if err = rows.Scan(&object.digest, &object.path); err != nil {
				rows.Close()
				return err
			}
			objects = append(objects, object)
		}
		if err = errors.Join(rows.Err(), rows.Close()); err != nil {
			return err
		}
		for _, object := range objects {
			if err = os.Remove(filepath.Join(s.objects, object.path)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if _, err = s.db.ExecContext(ctx, "DELETE FROM source_objects WHERE digest=? AND NOT EXISTS(SELECT 1 FROM revision_objects WHERE object_digest=?)", object.digest, object.digest); err != nil {
				return err
			}
		}
		if len(objects) < 100 {
			break
		}
	}
	return filepath.WalkDir(s.objects, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(s.objects, path)
		if err != nil {
			return err
		}
		var referenced bool
		if err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM source_objects WHERE path=?)", relative).Scan(&referenced); err != nil {
			return err
		}
		if !referenced {
			return os.Remove(path)
		}
		return nil
	})
}

func (s *Store) Renormalize(ctx context.Context, version func(string) int, normalize func(context.Context, domain.Descriptor, fs.FS) (domain.Normalization, error)) error {
	if version == nil || normalize == nil {
		return errors.New("version and normalize functions are required")
	}
	type candidate struct {
		revisionID, traceID int64
		descriptor          domain.Descriptor
		currentVersion      int
	}
	var maximum int64
	if err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(id),0) FROM trace_revisions").Scan(&maximum); err != nil {
		return err
	}
	for last := int64(0); last < maximum; {
		rows, err := s.db.QueryContext(ctx, `SELECT r.id,r.trace_id,r.digest,r.native_updated_at,r.adapter,r.normalizer_version,t.harness,t.native_trace_id,t.parent_native_trace_id,t.title,t.working_directory,COALESCE(repo.remote,''),COALESCE(repo.root,'') FROM trace_revisions r JOIN traces t ON t.id=r.trace_id LEFT JOIN repositories repo ON repo.id=t.repository_id WHERE r.id>? AND r.id<=? ORDER BY r.id LIMIT 100`, last, maximum)
		if err != nil {
			return err
		}
		var candidates []candidate
		scanned := false
		for rows.Next() {
			scanned = true
			var item candidate
			if err = rows.Scan(&item.revisionID, &item.traceID, &item.descriptor.RevisionDigest, &item.descriptor.NativeUpdatedAt, &item.descriptor.Adapter, &item.currentVersion, &item.descriptor.Harness, &item.descriptor.NativeTraceID, &item.descriptor.ParentNativeTraceID, &item.descriptor.Title, &item.descriptor.WorkingDirectory, &item.descriptor.Repository.Remote, &item.descriptor.Repository.Root); err != nil {
				rows.Close()
				return err
			}
			last = item.revisionID
			if item.currentVersion < version(item.descriptor.Harness) {
				candidates = append(candidates, item)
			}
		}
		if err = errors.Join(rows.Err(), rows.Close()); err != nil {
			return err
		}
		if !scanned {
			last = maximum
		}
		for _, item := range candidates {
			if err = ctx.Err(); err != nil {
				return err
			}
			if err = s.lockWriter(ctx); err != nil {
				return err
			}
			files, fileErr := s.revisionFiles(ctx, item.revisionID)
			if fileErr != nil {
				s.unlockWriter()
				return fileErr
			}
			for path, objectPath := range files {
				info, statErr := os.Stat(objectPath)
				if statErr != nil {
					s.unlockWriter()
					return statErr
				}
				item.descriptor.Files = append(item.descriptor.Files, domain.File{Path: path, Size: info.Size()})
			}
			normalization, normalizeErr := normalizeResult(ctx, item.descriptor, objectFS{files: files}, normalize)
			if normalization.Version == 0 {
				normalization.Version = version(item.descriptor.Harness)
			}
			if normalizeErr != nil || normalization.Status == "failed" || normalization.Status == "unsupported" {
				err = s.recordRun(ctx, item.revisionID, normalization.Version, normalization.Status, normalization.Warnings)
			} else {
				labels := strings.Join([]string{item.descriptor.Harness, item.descriptor.Title, item.descriptor.WorkingDirectory, item.descriptor.Repository.Remote, item.descriptor.Repository.Root}, "\n")
				_, err = s.replaceRevisionEvents(ctx, item.traceID, item.revisionID, labels, normalization)
			}
			s.unlockWriter()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) lockWriter(ctx context.Context) error {
	select {
	case s.writer <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Store) unlockWriter() {
	<-s.writer
}

func (s *Store) revisionFiles(ctx context.Context, revisionID int64) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT ro.relative_path,so.path FROM revision_objects ro JOIN source_objects so ON so.digest=ro.object_digest WHERE ro.revision_id=?`, revisionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	files := map[string]string{}
	for rows.Next() {
		var name, path string
		if err = rows.Scan(&name, &path); err != nil {
			return nil, err
		}
		files[name] = filepath.Join(s.objects, path)
	}
	return files, rows.Err()
}

type objectFS struct{ files map[string]string }

func (f objectFS) Open(name string) (fs.File, error) {
	path, ok := f.files[name]
	if !ok || !fs.ValidPath(name) {
		return nil, fs.ErrNotExist
	}
	return os.Open(path)
}

func (s *Store) recordRun(ctx context.Context, revisionID int64, version int, status string, warnings []domain.Warning) error {
	diagnostics, _ := json.Marshal(warnings)
	_, err := s.db.ExecContext(ctx, "INSERT INTO normalizer_runs(revision_id,version,status,diagnostics_json,created_at) VALUES(?,?,?,?,?)", revisionID, version, status, diagnostics, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) replaceRevisionEvents(ctx context.Context, traceID, revisionID int64, labels string, normalization domain.Normalization) ([]domain.Warning, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for _, event := range normalization.Events {
		for _, key := range event.LegacyKeys {
			if _, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO event_aliases(trace_id,legacy_key,legacy_id,event_key) SELECT trace_id,event_key,id,? FROM events WHERE trace_id=? AND event_key=?`, event.Key, traceID, key); err != nil {
				return nil, err
			}
		}
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM observation_revisions WHERE revision_id=?", revisionID); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM event_observations WHERE event_id IN (SELECT id FROM events WHERE trace_id=?) AND NOT EXISTS(SELECT 1 FROM observation_revisions WHERE observation_id=event_observations.id)", traceID); err != nil {
		return nil, err
	}
	conflicts, err := s.insertEvents(ctx, tx, traceID, revisionID, labels, normalization.Events)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE events SET preferred_observation_id=(SELECT o.id FROM event_observations o JOIN observation_revisions x ON x.observation_id=o.id JOIN trace_revisions r ON r.id=x.revision_id WHERE o.event_id=events.id ORDER BY r.native_updated_at DESC,o.event_time DESC,(SELECT COUNT(*) FROM json_each(o.event_json,'$.attachments') WHERE json_extract(value,'$.archived_path')<>'') DESC,o.digest DESC LIMIT 1) WHERE trace_id=?`, traceID); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM events WHERE trace_id=? AND NOT EXISTS(SELECT 1 FROM event_observations WHERE event_id=events.id)", traceID); err != nil {
		return nil, err
	}
	diagnostics, _ := json.Marshal(append(normalization.Warnings, conflicts...))
	if _, err = tx.ExecContext(ctx, "UPDATE trace_revisions SET status=?,normalizer_version=? WHERE id=?", normalization.Status, normalization.Version, revisionID); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO normalizer_runs(revision_id,version,status,diagnostics_json,created_at) VALUES(?,?,?,?,?)", revisionID, normalization.Version, normalization.Status, diagnostics, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return nil, err
	}
	return conflicts, tx.Commit()
}
