package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/regutierrez/traicr/internal/domain"
	searchtext "github.com/regutierrez/traicr/internal/search"
	"github.com/regutierrez/traicr/migrations"
	_ "modernc.org/sqlite"
)

func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(dataDir, "objects", "sha256"), 0o700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "tmp"), 0o700); err != nil {
		return nil, err
	}
	dsn := "file:" + filepath.Join(dataDir, "traicr.db") + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(8)
	if err = db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure sqlite: %w", err)
	}
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		db.Close()
		return nil, err
	}
	if version == 0 {
		tx, txErr := db.Begin()
		if txErr == nil {
			_, txErr = tx.Exec(migrations.Initial)
		}
		if txErr == nil {
			txErr = tx.Commit()
		} else if tx != nil {
			tx.Rollback()
		}
		if txErr != nil {
			db.Close()
			return nil, fmt.Errorf("migrate sqlite: %w", txErr)
		}
	} else if version != 1 {
		db.Close()
		return nil, fmt.Errorf("unsupported database version %d", version)
	}
	return &Store{db: db, dataDir: dataDir, objects: filepath.Join(dataDir, "objects", "sha256"), writer: make(chan struct{}, 1)}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Import(ctx context.Context, manifest domain.Manifest, sources fs.FS, normalize func(context.Context, domain.Descriptor, fs.FS) (domain.Normalization, error), progress func(domain.Progress)) (domain.ImportReport, error) {
	if normalize == nil {
		return domain.ImportReport{}, errors.New("normalize function is required")
	}
	if progress == nil {
		progress = func(domain.Progress) {}
	}
	select {
	case s.writer <- struct{}{}:
		defer func() { <-s.writer }()
	case <-ctx.Done():
		return domain.ImportReport{}, ctx.Err()
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	report := domain.ImportReport{CreatedAt: now, Machine: manifest.SourceMachine, Traces: make([]domain.TraceOutcome, 0, len(manifest.Traces))}
	if err := s.beginImport(ctx, &report); err != nil {
		return report, err
	}
	progress(domain.Progress{Phase: "normalizing", Total: len(manifest.Traces)})
	for i, descriptor := range manifest.Traces {
		outcome := s.importTrace(ctx, manifest, descriptor, sources, normalize)
		report.Traces = append(report.Traces, outcome)
		countOutcome(&report, outcome.Status)
		progress(domain.Progress{Phase: "normalizing", Completed: i + 1, Total: len(manifest.Traces), Outcome: &outcome})
	}
	b, err := json.Marshal(report)
	if err == nil {
		_, err = s.db.ExecContext(ctx, "UPDATE imports SET report_json=? WHERE id=?", b, report.ID)
	}
	if err != nil {
		return report, err
	}
	progress(domain.Progress{Phase: "complete", Completed: len(manifest.Traces), Total: len(manifest.Traces), Report: &report})
	return report, nil
}

func (s *Store) beginImport(ctx context.Context, report *domain.ImportReport) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	m := report.Machine
	_, err = tx.ExecContext(ctx, `INSERT INTO source_machines(id,hostname,os,arch,first_seen,last_seen) VALUES(?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET hostname=excluded.hostname,os=excluded.os,arch=excluded.arch,last_seen=excluded.last_seen`, m.ID, m.Hostname, m.OS, m.Arch, report.CreatedAt, report.CreatedAt)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO imports(created_at,machine_id) VALUES(?,?)", report.CreatedAt, m.ID)
	if err != nil {
		return err
	}
	report.ID, err = result.LastInsertId()
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) importTrace(ctx context.Context, manifest domain.Manifest, descriptor domain.Descriptor, sources fs.FS, normalize func(context.Context, domain.Descriptor, fs.FS) (domain.Normalization, error)) domain.TraceOutcome {
	outcome := domain.TraceOutcome{Harness: descriptor.Harness, NativeTraceID: descriptor.NativeTraceID, RevisionDigest: descriptor.RevisionDigest, Warnings: append([]domain.Warning(nil), descriptor.Warnings...)}
	traceFS, err := fs.Sub(sources, descriptor.Path)
	if err != nil {
		outcome.Status, outcome.Error = "failed", err.Error()
		return outcome
	}
	objects, err := s.storeObjects(ctx, descriptor, traceFS)
	if err != nil {
		outcome.Status, outcome.Error = "failed", err.Error()
		return outcome
	}
	var existingID, existingTraceID int64
	var existingStatus string
	err = s.db.QueryRowContext(ctx, `SELECT r.id,t.id,r.status FROM trace_revisions r JOIN traces t ON t.id=r.trace_id WHERE t.harness=? AND t.native_trace_id=? AND r.digest=?`, descriptor.Harness, descriptor.NativeTraceID, descriptor.RevisionDigest).Scan(&existingID, &existingTraceID, &existingStatus)
	if err == nil {
		_, err = s.db.ExecContext(ctx, "INSERT OR IGNORE INTO revision_machines(revision_id,machine_id) VALUES(?,?)", existingID, manifest.SourceMachine.ID)
		outcome.TraceID = existingTraceID
		if err != nil {
			outcome.Status, outcome.Error = "failed", err.Error()
			return outcome
		}
		updated, metadataErr := s.backfillMissingTraceMetadata(ctx, existingTraceID, existingID, descriptor)
		if metadataErr != nil {
			outcome.Status, outcome.Error = "failed", metadataErr.Error()
			return outcome
		}
		if normalizationSuccessful(existingStatus) {
			outcome.Status = "unchanged"
			if updated {
				outcome.Status = "updated"
			}
			return outcome
		}
		normalization, normalizeErr := normalizeResult(ctx, descriptor, traceFS, normalize)
		outcome.Warnings = append(outcome.Warnings, normalization.Warnings...)
		if normalizeErr != nil {
			outcome.Error = normalizeErr.Error()
		}
		if normalization.Status == "failed" || normalization.Status == "unsupported" {
			if runErr := s.recordRun(ctx, existingID, normalization.Version, normalization.Status, normalization.Warnings); runErr != nil {
				outcome.Status, outcome.Error = "failed", runErr.Error()
				return outcome
			}
			outcome.Status = normalization.Status
			return outcome
		}
		labels := strings.Join([]string{descriptor.Harness, descriptor.Title, descriptor.WorkingDirectory, descriptor.Repository.Remote, descriptor.Repository.Root}, "\n")
		conflicts, replaceErr := s.replaceRevisionEvents(ctx, existingTraceID, existingID, labels, normalization)
		outcome.Warnings = append(outcome.Warnings, conflicts...)
		if replaceErr != nil {
			outcome.Status, outcome.Error = "failed", replaceErr.Error()
			return outcome
		}
		if normalizationSuccessful(normalization.Status) {
			outcome.Status = "updated"
		} else {
			outcome.Status = normalization.Status
		}
		return outcome
	}
	if !errors.Is(err, sql.ErrNoRows) {
		outcome.Status, outcome.Error = "failed", err.Error()
		return outcome
	}
	normalization, normalizeErr := normalizeResult(ctx, descriptor, traceFS, normalize)
	traceID, isNew, conflicts, err := s.commitRevision(ctx, manifest, descriptor, normalization, objects)
	outcome.TraceID = traceID
	outcome.Warnings = append(outcome.Warnings, normalization.Warnings...)
	outcome.Warnings = append(outcome.Warnings, conflicts...)
	if err != nil {
		outcome.Status, outcome.Error = "failed", err.Error()
		return outcome
	}
	switch normalization.Status {
	case "partially_parsed", "unsupported", "failed":
		outcome.Status = normalization.Status
	default:
		if isNew {
			outcome.Status = "imported"
		} else {
			outcome.Status = "updated"
		}
	}
	if normalizeErr != nil {
		outcome.Error = normalizeErr.Error()
	}
	return outcome
}

// backfillMissingTraceMetadata fills collector metadata added after a revision was first imported.
func (s *Store) backfillMissingTraceMetadata(ctx context.Context, traceID, revisionID int64, descriptor domain.Descriptor) (bool, error) {
	if descriptor.Title == "" && descriptor.WorkingDirectory == "" {
		return false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var searchableMetadata []string
	if descriptor.Title != "" {
		result, err := tx.ExecContext(ctx, `UPDATE traces SET title=? WHERE id=? AND title='' AND updated_at<=?`, descriptor.Title, traceID, canonicalTime(descriptor.NativeUpdatedAt))
		if err != nil {
			return false, err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return false, err
		}
		if changed != 0 {
			searchableMetadata = append(searchableMetadata, descriptor.Title)
		}
	}
	if descriptor.WorkingDirectory != "" {
		result, err := tx.ExecContext(ctx, `UPDATE traces SET working_directory=? WHERE id=? AND working_directory='' AND updated_at<=?`, descriptor.WorkingDirectory, traceID, canonicalTime(descriptor.NativeUpdatedAt))
		if err != nil {
			return false, err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return false, err
		}
		if changed != 0 {
			searchableMetadata = append(searchableMetadata, descriptor.WorkingDirectory)
		}
	}
	if len(searchableMetadata) == 0 {
		return false, nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE observation_revisions SET searchable_text=searchable_text || char(10) || ? WHERE revision_id=?`, strings.Join(searchableMetadata, "\n"), revisionID); err != nil {
		return false, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT x.observation_id,o.event_id,x.searchable_text FROM observation_revisions x JOIN event_observations o ON o.id=x.observation_id WHERE x.revision_id=?`, revisionID)
	if err != nil {
		return false, err
	}
	type metadataSearchText struct {
		observationID, eventID int64
		text                   string
	}
	var texts []metadataSearchText
	for rows.Next() {
		var text metadataSearchText
		if err := rows.Scan(&text.observationID, &text.eventID, &text.text); err != nil {
			rows.Close()
			return false, err
		}
		texts = append(texts, text)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM search_chunks WHERE revision_id=?", revisionID); err != nil {
		return false, err
	}
	for _, text := range texts {
		for _, chunk := range searchtext.Chunks(text.text) {
			if _, err := tx.ExecContext(ctx, "INSERT INTO search_chunks(observation_id,revision_id,event_id,content) VALUES(?,?,?,?)", text.observationID, revisionID, text.eventID, chunk); err != nil {
				return false, err
			}
		}
	}
	return true, tx.Commit()
}

func normalizeResult(ctx context.Context, descriptor domain.Descriptor, source fs.FS, normalize func(context.Context, domain.Descriptor, fs.FS) (domain.Normalization, error)) (domain.Normalization, error) {
	normalization, normalizeErr := normalize(ctx, descriptor, source)
	if normalizeErr != nil {
		normalization.Status = "failed"
		normalization.Warnings = append(normalization.Warnings, domain.Warning{Code: "normalizer_error", Message: normalizeErr.Error()})
	}
	if normalization.Status == "" {
		normalization.Status = "normalized"
	}
	if err := validateEvents(normalization.Events); err != nil {
		normalization.Status = "failed"
		normalization.Events = nil
		normalization.Warnings = append(normalization.Warnings, domain.Warning{Code: "invalid_event", Message: err.Error()})
	}
	return normalization, normalizeErr
}

func normalizationSuccessful(status string) bool {
	return status == "normalized"
}

type storedObject struct {
	path   string
	digest string
	size   int64
}

func (s *Store) storeObjects(ctx context.Context, descriptor domain.Descriptor, source fs.FS) ([]storedObject, error) {
	objects := make([]storedObject, 0, len(descriptor.Files))
	for _, file := range descriptor.Files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		opened, err := source.Open(file.Path)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", file.Path, err)
		}
		object, err := s.writeObject(ctx, opened, file.Path, file.SHA256, file.Size)
		opened.Close()
		if err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	return objects, nil
}

func (s *Store) writeObject(ctx context.Context, source io.Reader, relativePath, expected string, expectedSize int64) (storedObject, error) {
	digest := strings.TrimPrefix(expected, "sha256:")
	if len(digest) != sha256.Size*2 {
		return storedObject{}, fmt.Errorf("invalid digest for %s", relativePath)
	}
	destination := filepath.Join(s.objects, digest[:2], digest[2:])
	if info, err := os.Stat(destination); err == nil {
		if info.Size() != expectedSize {
			return storedObject{}, fmt.Errorf("stored object size mismatch for %s", relativePath)
		}
		return storedObject{path: relativePath, digest: digest, size: expectedSize}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return storedObject{}, err
	}
	temporary, err := os.CreateTemp(filepath.Join(s.dataDir, "tmp"), "object-")
	if err != nil {
		return storedObject{}, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	defer temporary.Close()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(temporary, hash), contextReader{ctx: ctx, reader: source})
	if err != nil {
		return storedObject{}, err
	}
	if err = temporary.Sync(); err != nil {
		return storedObject{}, err
	}
	if err = temporary.Close(); err != nil {
		return storedObject{}, err
	}
	actualDigest := hex.EncodeToString(hash.Sum(nil))
	if digest != actualDigest {
		return storedObject{}, fmt.Errorf("digest mismatch for %s", relativePath)
	}
	if size != expectedSize {
		return storedObject{}, fmt.Errorf("size mismatch for %s", relativePath)
	}
	if err = os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return storedObject{}, err
	}
	if err = os.Rename(temporaryPath, destination); err != nil {
		return storedObject{}, err
	}
	directory, err := os.Open(filepath.Dir(destination))
	if err != nil {
		return storedObject{}, err
	}
	err = directory.Sync()
	directory.Close()
	if err != nil {
		return storedObject{}, err
	}
	return storedObject{path: relativePath, digest: digest, size: size}, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

func (s *Store) commitRevision(ctx context.Context, manifest domain.Manifest, descriptor domain.Descriptor, normalization domain.Normalization, objects []storedObject) (int64, bool, []domain.Warning, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, nil, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var repositoryID any
	if descriptor.Repository.Remote != "" || descriptor.Repository.Root != "" {
		identity := descriptor.Repository.Remote
		if identity == "" {
			identity = "path:" + manifest.SourceMachine.ID + ":" + descriptor.Repository.Root
		}
		_, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO repositories(identity,remote,root) VALUES(?,?,?)", identity, descriptor.Repository.Remote, descriptor.Repository.Root)
		if err != nil {
			return 0, false, nil, err
		}
		var id int64
		if err = tx.QueryRowContext(ctx, "SELECT id FROM repositories WHERE identity=?", identity).Scan(&id); err != nil {
			return 0, false, nil, err
		}
		if descriptor.Repository.Root != "" {
			if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO repository_paths(repository_id,path) VALUES(?,?)", id, descriptor.Repository.Root); err != nil {
				return 0, false, nil, err
			}
		}
		repositoryID = id
	}
	metadataTime := canonicalTime(descriptor.NativeUpdatedAt)
	if metadataTime == "" {
		metadataTime = now
	}
	result, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO traces(harness,native_trace_id,title,working_directory,repository_id,parent_native_trace_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, descriptor.Harness, descriptor.NativeTraceID, descriptor.Title, descriptor.WorkingDirectory, repositoryID, descriptor.ParentNativeTraceID, now, metadataTime)
	if err != nil {
		return 0, false, nil, err
	}
	inserted, _ := result.RowsAffected()
	var traceID int64
	if err = tx.QueryRowContext(ctx, "SELECT id FROM traces WHERE harness=? AND native_trace_id=?", descriptor.Harness, descriptor.NativeTraceID).Scan(&traceID); err != nil {
		return 0, false, nil, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE traces SET title=CASE WHEN ? >= updated_at THEN ? ELSE title END, working_directory=CASE WHEN ? >= updated_at THEN ? ELSE working_directory END, repository_id=CASE WHEN ? >= updated_at THEN ? ELSE repository_id END, parent_native_trace_id=CASE WHEN ? >= updated_at THEN ? ELSE parent_native_trace_id END, updated_at=MAX(updated_at,?) WHERE id=?`, metadataTime, descriptor.Title, metadataTime, descriptor.WorkingDirectory, metadataTime, repositoryID, metadataTime, descriptor.ParentNativeTraceID, metadataTime, traceID)
	if err != nil {
		return 0, false, nil, err
	}
	result, err = tx.ExecContext(ctx, `INSERT INTO trace_revisions(trace_id,digest,native_updated_at,collected_at,adapter,status,normalizer_version) VALUES(?,?,?,?,?,?,?)`, traceID, descriptor.RevisionDigest, metadataTime, canonicalTime(manifest.CreatedAt), descriptor.Adapter, normalization.Status, normalization.Version)
	if err != nil {
		return 0, false, nil, err
	}
	revisionID, _ := result.LastInsertId()
	if _, err = tx.ExecContext(ctx, "INSERT INTO revision_machines(revision_id,machine_id) VALUES(?,?)", revisionID, manifest.SourceMachine.ID); err != nil {
		return 0, false, nil, err
	}
	for _, object := range objects {
		objectPath := filepath.Join(object.digest[:2], object.digest[2:])
		if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO source_objects(digest,size,path) VALUES(?,?,?)", object.digest, object.size, objectPath); err != nil {
			return 0, false, nil, err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO revision_objects(revision_id,relative_path,object_digest) VALUES(?,?,?)", revisionID, object.path, object.digest); err != nil {
			return 0, false, nil, err
		}
	}
	labels := strings.Join([]string{descriptor.Harness, descriptor.Title, descriptor.WorkingDirectory, descriptor.Repository.Remote, descriptor.Repository.Root}, "\n")
	conflicts, err := s.insertEvents(ctx, tx, traceID, revisionID, labels, normalization.Events)
	if err != nil {
		return 0, false, nil, err
	}
	diagnostics, _ := json.Marshal(append(normalization.Warnings, conflicts...))
	_, err = tx.ExecContext(ctx, "INSERT INTO normalizer_runs(revision_id,version,status,diagnostics_json,created_at) VALUES(?,?,?,?,?)", revisionID, normalization.Version, normalization.Status, diagnostics, now)
	if err != nil {
		return 0, false, nil, err
	}
	return traceID, inserted == 1, conflicts, tx.Commit()
}

func (s *Store) insertEvents(ctx context.Context, tx *sql.Tx, traceID, revisionID int64, labels string, events []domain.Event) ([]domain.Warning, error) {
	var conflicts []domain.Warning
	// Identical records can repeat within one revision (Cursor Agent retries carry no
	// ids, so they hash to the same key). They map to one observation whose source
	// references accumulate under consecutive ordinals instead of failing the import.
	ordinals := map[int64]int{}
	for _, event := range events {
		if event.Key == "" {
			return nil, errors.New("normalized event key is required")
		}
		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO events(trace_id,event_key,sort_time,sort_key) VALUES(?,?,?,?)", traceID, event.Key, canonicalTime(event.Timestamp), event.Key); err != nil {
			return nil, err
		}
		var eventID int64
		if err := tx.QueryRowContext(ctx, "SELECT id FROM events WHERE trace_id=? AND event_key=?", traceID, event.Key).Scan(&eventID); err != nil {
			return nil, err
		}
		sourceRefs := append([]domain.SourceRef(nil), event.Sources...)
		searchable := searchableText(event) + "\n" + labels
		event.ID = 0
		event.TraceID = 0
		event.Sources = nil
		digest, encoded, err := observationDigest(event)
		if err != nil {
			return nil, err
		}
		var differs bool
		if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM event_observations WHERE event_id=? AND digest<>?)", eventID, digest).Scan(&differs); err != nil {
			return nil, err
		}
		if differs {
			conflicts = append(conflicts, domain.Warning{Code: "conflicting_observation", Message: "conflicting observations retained for event " + event.Key})
		}
		_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO event_observations(event_id,digest,parent_key,branch,kind,role,model,provider,tool,call_id,event_time,text,event_json) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, eventID, digest, event.ParentKey, event.Branch, event.Kind, event.Role, event.Model, event.Provider, event.Tool, event.CallID, canonicalTime(event.Timestamp), event.Text, encoded)
		if err != nil {
			return nil, err
		}
		var observationID int64
		if err = tx.QueryRowContext(ctx, "SELECT id FROM event_observations WHERE event_id=? AND digest=?", eventID, digest).Scan(&observationID); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO observation_revisions(observation_id,revision_id,searchable_text) VALUES(?,?,?)", observationID, revisionID, searchable); err != nil {
			return nil, err
		}
		next, repeated := ordinals[observationID]
		for _, source := range sourceRefs {
			if _, err = tx.ExecContext(ctx, "INSERT INTO observation_sources(observation_id,revision_id,ordinal,path,line) VALUES(?,?,?,?,?)", observationID, revisionID, next, source.Path, source.Line); err != nil {
				return nil, err
			}
			next++
		}
		ordinals[observationID] = next
		if repeated {
			conflicts = append(conflicts, domain.Warning{Code: "duplicate_event", Message: "identical repeated event " + event.Key + " was retained once with its source references merged"})
			continue
		}
		for _, chunk := range searchtext.Chunks(searchable) {
			if _, err = tx.ExecContext(ctx, "INSERT INTO search_chunks(observation_id,revision_id,event_id,content) VALUES(?,?,?,?)", observationID, revisionID, eventID, chunk); err != nil {
				return nil, err
			}
		}
		if _, err = tx.ExecContext(ctx, `UPDATE events SET preferred_observation_id=(SELECT o.id FROM event_observations o JOIN observation_revisions x ON x.observation_id=o.id JOIN trace_revisions r ON r.id=x.revision_id WHERE o.event_id=events.id ORDER BY r.native_updated_at DESC,o.event_time DESC,o.digest DESC LIMIT 1) WHERE id=?`, eventID); err != nil {
			return nil, err
		}
	}
	return conflicts, nil
}

func searchableText(event domain.Event) string {
	parts := []string{event.Text, event.Model, event.Provider, event.Tool, event.Kind, event.Role}
	for _, attachment := range event.Attachments {
		parts = append(parts, attachment.Name, attachment.Path, attachment.MediaType)
	}
	for _, source := range event.Sources {
		parts = append(parts, source.Path)
	}
	return strings.Join(parts, "\n")
}

func countOutcome(report *domain.ImportReport, status string) {
	switch status {
	case "imported":
		report.Imported++
	case "updated":
		report.Updated++
	case "unchanged":
		report.Unchanged++
	case "partially_parsed":
		report.Partial++
	case "unsupported":
		report.Unsupported++
	default:
		report.Failed++
	}
}

func digestBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func validateEvents(events []domain.Event) error {
	for _, event := range events {
		if event.Key == "" {
			return errors.New("normalized event key is required")
		}
	}
	return nil
}

func canonicalTime(value string) string {
	if value == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return parsed.UTC().Format("2006-01-02T15:04:05.000000000Z")
}
