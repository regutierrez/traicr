package store

import (
	"database/sql"
	"encoding/json"

	"github.com/regutierrez/traicr/internal/domain"
)

type Store struct {
	db      *sql.DB
	dataDir string
	objects string
	writer  chan struct{}
}

type SearchQuery struct {
	Query      string `json:"query"`
	Mode       string `json:"mode"`
	Harness    string `json:"harness"`
	Model      string `json:"model"`
	Machine    string `json:"machine"`
	Repository string `json:"repository"`
	After      string `json:"after"`
	Before     string `json:"before"`
	Role       string `json:"role"`
	Kind       string `json:"kind"`
	Tool       string `json:"tool"`
	Cursor     string `json:"cursor"`
	Limit      int    `json:"limit"`
}

type SearchResult struct {
	EventID    int64  `json:"event_id"`
	TraceID    int64  `json:"trace_id"`
	TraceTitle string `json:"trace_title"`
	Harness    string `json:"harness"`
	Repository string `json:"repository,omitempty"`
	Kind       string `json:"kind"`
	Role       string `json:"role,omitempty"`
	Model      string `json:"model,omitempty"`
	Tool       string `json:"tool,omitempty"`
	Timestamp  string `json:"timestamp,omitempty"`
	Snippet    string `json:"snippet"`
}

type SearchPage struct {
	Results    []SearchResult `json:"results"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

type Trace struct {
	ID                  int64      `json:"id"`
	Harness             string     `json:"harness"`
	NativeTraceID       string     `json:"native_trace_id"`
	Title               string     `json:"title,omitempty"`
	WorkingDirectory    string     `json:"working_directory,omitempty"`
	Repository          string     `json:"repository,omitempty"`
	ParentNativeTraceID string     `json:"parent_native_trace_id,omitempty"`
	CreatedAt           string     `json:"created_at"`
	UpdatedAt           string     `json:"updated_at"`
	Revisions           []Revision `json:"revisions"`
	Parents             []TraceRef `json:"parents"`
	Children            []TraceRef `json:"children"`
}

type TraceRef struct {
	ID            int64  `json:"id"`
	NativeTraceID string `json:"native_trace_id"`
	Title         string `json:"title,omitempty"`
}

type Revision struct {
	ID                int64            `json:"id"`
	Digest            string           `json:"digest"`
	NativeUpdatedAt   string           `json:"native_updated_at,omitempty"`
	CollectedAt       string           `json:"collected_at"`
	Adapter           string           `json:"adapter"`
	Status            string           `json:"status"`
	NormalizerVersion int              `json:"normalizer_version"`
	Diagnostics       []domain.Warning `json:"diagnostics,omitempty"`
}

type Event struct {
	domain.Event
	ObservationCount int      `json:"observation_count"`
	RevisionID       int64    `json:"revision_id,omitempty"`
	Aliases          []string `json:"aliases,omitempty"`
}

type EventPage struct {
	Events     []Event `json:"events"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

type Source struct {
	ObservationID  int64        `json:"observation_id"`
	RevisionID     int64        `json:"revision_id"`
	RevisionDigest string       `json:"revision_digest"`
	Path           string       `json:"path,omitempty"`
	Line           int          `json:"line,omitempty"`
	ObjectDigest   string       `json:"object_digest,omitempty"`
	Size           int64        `json:"size,omitempty"`
	Event          domain.Event `json:"event"`
}

type SourceObject struct {
	RevisionID int64  `json:"revision_id"`
	Path       string `json:"path"`
	Digest     string `json:"digest"`
	Size       int64  `json:"size"`
}

type ImportPage struct {
	Imports    []domain.ImportReport `json:"imports"`
	NextCursor string                `json:"next_cursor,omitempty"`
}

type Machine struct {
	domain.Machine
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
}

type cursor struct {
	Last int64 `json:"last"`
	Max  int64 `json:"max"`
}

type eventCursor struct {
	MaxID int64  `json:"max_id"`
	Time  string `json:"time"`
	Key   string `json:"key"`
	ID    int64  `json:"id"`
}

func observationDigest(event domain.Event) (string, []byte, error) {
	b, err := json.Marshal(event)
	if err != nil {
		return "", nil, err
	}
	return digestBytes(b), b, nil
}
