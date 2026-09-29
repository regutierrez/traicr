package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/regutierrez/traicr/internal/domain"
	searchtext "github.com/regutierrez/traicr/internal/search"
)

// ErrInvalidTitle reports a title the archive will not store.
var ErrInvalidTitle = errors.New("invalid title")

const maxTitleRunes = 200

// displayTitle is the title every reader shows: a manual override, then the
// collected title, then a model-generated title. Imports only write traces.title.
const displayTitle = "COALESCE(NULLIF(t.title_override,''),NULLIF(t.title,''),t.generated_title)"

// searchLabels is the trace metadata appended to each Event's search text.
// Every title a trace has stays searchable, whichever one is displayed.
func searchLabels(descriptor domain.Descriptor, extraTitles ...string) string {
	parts := []string{descriptor.Harness, descriptor.Title, descriptor.WorkingDirectory, descriptor.Repository.Remote, descriptor.Repository.Root}
	for _, title := range extraTitles {
		if title != "" {
			parts = append(parts, title)
		}
	}
	return strings.Join(parts, "\n")
}

type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// storedTitles returns the titles imports never write: the override and the generated title.
func storedTitles(ctx context.Context, db rowQuerier, traceID int64) (override, generated string, err error) {
	err = db.QueryRowContext(ctx, "SELECT title_override,generated_title FROM traces WHERE id=?", traceID).Scan(&override, &generated)
	return override, generated, err
}

// currentSearchLabels builds search labels from the trace row as it is now.
// Callers must hold the writer lock so a concurrent rename cannot be lost.
func currentSearchLabels(ctx context.Context, db rowQuerier, traceID int64) (string, error) {
	var descriptor domain.Descriptor
	var override, generated string
	if err := db.QueryRowContext(ctx, `SELECT t.harness,t.title,t.title_override,t.generated_title,t.working_directory,COALESCE(repo.remote,''),COALESCE(repo.root,'') FROM traces t LEFT JOIN repositories repo ON repo.id=t.repository_id WHERE t.id=?`, traceID).Scan(&descriptor.Harness, &descriptor.Title, &override, &generated, &descriptor.WorkingDirectory, &descriptor.Repository.Remote, &descriptor.Repository.Root); err != nil {
		return "", err
	}
	return searchLabels(descriptor, override, generated), nil
}

func validateTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if !utf8.ValidString(title) || utf8.RuneCountInString(title) > maxTitleRunes {
		return "", fmt.Errorf("%w: title must be valid UTF-8 and at most %d characters", ErrInvalidTitle, maxTitleRunes)
	}
	if strings.IndexFunc(title, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("%w: title must be one line without control characters", ErrInvalidTitle)
	}
	return title, nil
}

// SetTitleOverride stores a manual title, or clears it when title is blank,
// and rebuilds the trace's search text so the new title is searchable.
func (s *Store) SetTitleOverride(ctx context.Context, id int64, title string) (Trace, error) {
	title, err := validateTitle(title)
	if err != nil {
		return Trace{}, err
	}
	if err := s.lockWriter(ctx); err != nil {
		return Trace{}, err
	}
	err = s.setTitleOverride(ctx, id, title)
	s.unlockWriter()
	if err != nil {
		return Trace{}, err
	}
	return s.Trace(ctx, id)
}

func (s *Store) setTitleOverride(ctx context.Context, id int64, title string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "UPDATE traces SET title_override=? WHERE id=?", title, id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return sql.ErrNoRows
	}
	if err := reindexTraceSearch(ctx, tx, id); err != nil {
		return err
	}
	return tx.Commit()
}

// reindexTraceSearch rewrites the search text of every revision of a trace
// from its stored observations and the trace's current metadata. It works one
// revision at a time so a long trace is never held in memory at once.
func reindexTraceSearch(ctx context.Context, tx *sql.Tx, traceID int64) error {
	labels, err := currentSearchLabels(ctx, tx, traceID)
	if err != nil {
		return err
	}
	revisions, err := queryIDs(ctx, tx, "SELECT id FROM trace_revisions WHERE trace_id=? ORDER BY id", traceID)
	if err != nil {
		return err
	}
	for _, revisionID := range revisions {
		if err := reindexRevisionSearch(ctx, tx, revisionID, labels); err != nil {
			return err
		}
	}
	return nil
}

func reindexRevisionSearch(ctx context.Context, tx *sql.Tx, revisionID int64, labels string) error {
	sources := map[int64][]domain.SourceRef{}
	rows, err := tx.QueryContext(ctx, "SELECT observation_id,path,line FROM observation_sources WHERE revision_id=? ORDER BY observation_id,ordinal", revisionID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var observationID int64
		var source domain.SourceRef
		if err := rows.Scan(&observationID, &source.Path, &source.Line); err != nil {
			rows.Close()
			return err
		}
		sources[observationID] = append(sources[observationID], source)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	type observation struct {
		id, eventID int64
		text        string
	}
	var observations []observation
	rows, err = tx.QueryContext(ctx, "SELECT o.id,o.event_id,o.event_json FROM observation_revisions x JOIN event_observations o ON o.id=x.observation_id WHERE x.revision_id=?", revisionID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item observation
		var encoded []byte
		if err := rows.Scan(&item.id, &item.eventID, &encoded); err != nil {
			rows.Close()
			return err
		}
		var event domain.Event
		if err := json.Unmarshal(encoded, &event); err != nil {
			rows.Close()
			return err
		}
		event.Sources = sources[item.id]
		item.text = searchableText(event) + "\n" + labels
		observations = append(observations, item)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM search_chunks WHERE revision_id=?", revisionID); err != nil {
		return err
	}
	for _, item := range observations {
		if _, err := tx.ExecContext(ctx, "UPDATE observation_revisions SET searchable_text=? WHERE observation_id=? AND revision_id=?", item.text, item.id, revisionID); err != nil {
			return err
		}
		for _, chunk := range searchtext.Chunks(item.text) {
			if _, err := tx.ExecContext(ctx, "INSERT INTO search_chunks(observation_id,revision_id,event_id,content) VALUES(?,?,?,?)", item.id, revisionID, item.eventID, chunk); err != nil {
				return err
			}
		}
	}
	return nil
}

func queryIDs(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// TitleCandidate is a trace that needs a generated title for its latest revision.
type TitleCandidate struct {
	TraceID    int64
	RevisionID int64
}

// TitleCandidates lists traces with no manual or collected title whose latest
// revision has not been titled yet and that have been idle since idleBefore,
// newest first. A trace is offered again only after a new revision arrives.
func (s *Store) TitleCandidates(ctx context.Context, idleBefore time.Time, limit int) ([]TitleCandidate, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,latest FROM (
		SELECT t.id,t.updated_at,t.generated_title_revision_id,(SELECT MAX(r.id) FROM trace_revisions r WHERE r.trace_id=t.id) AS latest
		FROM traces t WHERE t.title_override='' AND t.title='' AND t.updated_at<?
	) WHERE latest>generated_title_revision_id ORDER BY updated_at DESC,id DESC LIMIT ?`, idleBefore.UTC().Format("2006-01-02T15:04:05.000000000Z"), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var candidates []TitleCandidate
	for rows.Next() {
		var candidate TitleCandidate
		if err := rows.Scan(&candidate.TraceID, &candidate.RevisionID); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

// TitleSource is the part of a trace a title is generated from.
type TitleSource struct {
	Harness          string
	WorkingDirectory string
	Repository       string
	// UserMessages are the user's messages in transcript order, each cut to
	// maxTitleSourceRunes. The limit is generous because harnesses prefix the
	// user's words with long injected context that is only removed later.
	UserMessages []string
	// LastAssistantMessage is the final assistant message, cut the same way.
	LastAssistantMessage string
}

const maxTitleSourceRunes = 64000

func (s *Store) TitleSource(ctx context.Context, traceID int64) (TitleSource, error) {
	var source TitleSource
	if err := s.db.QueryRowContext(ctx, `SELECT t.harness,t.working_directory,COALESCE(NULLIF(repo.remote,''),repo.root,'') FROM traces t LEFT JOIN repositories repo ON repo.id=t.repository_id WHERE t.id=?`, traceID).Scan(&source.Harness, &source.WorkingDirectory, &source.Repository); err != nil {
		return TitleSource{}, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT o.role,substr(o.text,1,?) FROM events e JOIN event_observations o ON o.id=e.preferred_observation_id WHERE e.trace_id=? AND o.kind='message' AND o.role IN ('user','assistant') AND o.text<>''
		ORDER BY COALESCE(json_extract(o.event_json,'$.metadata.transcript_order'),0),
		CASE WHEN json_extract(o.event_json,'$.metadata.transcript_order') IS NULL THEN e.sort_time ELSE '' END,
		COALESCE(json_extract(o.event_json,'$.metadata.transcript_block'),0),
		(SELECT MIN(src.line) FROM observation_sources src WHERE src.observation_id=o.id),e.sort_key,e.id`, maxTitleSourceRunes, traceID)
	if err != nil {
		return TitleSource{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var role, text string
		if err := rows.Scan(&role, &text); err != nil {
			return TitleSource{}, err
		}
		if role == "user" {
			source.UserMessages = append(source.UserMessages, text)
		} else {
			source.LastAssistantMessage = text
		}
	}
	return source, rows.Err()
}

// SetGeneratedTitle records the title generated for a trace's revision. An
// empty title records that the revision was tried and produced nothing, so the
// trace is not offered again until a newer revision arrives. Results for a
// revision older than the one already recorded are ignored.
func (s *Store) SetGeneratedTitle(ctx context.Context, id, revisionID int64, title string) error {
	title, err := validateTitle(title)
	if err != nil {
		return err
	}
	if err := s.lockWriter(ctx); err != nil {
		return err
	}
	defer s.unlockWriter()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var previous string
	if err := tx.QueryRowContext(ctx, "SELECT generated_title FROM traces WHERE id=?", id).Scan(&previous); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, "UPDATE traces SET generated_title=?,generated_title_revision_id=? WHERE id=? AND generated_title_revision_id<?", title, revisionID, id, revisionID)
	if err != nil {
		return err
	}
	if changed, err := result.RowsAffected(); err != nil || changed == 0 {
		return err
	}
	if title != previous {
		if err := reindexTraceSearch(ctx, tx, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
