package domain

import "encoding/json"

const FormatVersion = 1
const MaxAmpExportBytes = 512 << 20

type Machine struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
}

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type File struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Repository struct {
	Remote string `json:"remote,omitempty"`
	Root   string `json:"root,omitempty"`
}

type Descriptor struct {
	Path                string     `json:"path"`
	Harness             string     `json:"harness"`
	Adapter             string     `json:"adapter"`
	NativeTraceID       string     `json:"native_trace_id"`
	NativeUpdatedAt     string     `json:"native_updated_at,omitempty"`
	RevisionDigest      string     `json:"revision_digest"`
	ParentNativeTraceID string     `json:"parent_native_trace_id,omitempty"`
	Title               string     `json:"title,omitempty"`
	WorkingDirectory    string     `json:"working_directory,omitempty"`
	Repository          Repository `json:"repository,omitempty"`
	Files               []File     `json:"files"`
	Warnings            []Warning  `json:"warnings,omitempty"`
}

type Manifest struct {
	FormatVersion    int          `json:"format_version"`
	CollectorVersion string       `json:"collector_version"`
	CreatedAt        string       `json:"created_at"`
	SourceMachine    Machine      `json:"source_machine"`
	Traces           []Descriptor `json:"traces"`
}

type SourceRef struct {
	Path string `json:"path"`
	Line int    `json:"line,omitempty"`
}

type Attachment struct {
	Name          string `json:"name,omitempty"`
	MediaType     string `json:"media_type,omitempty"`
	Path          string `json:"path,omitempty"`
	URL           string `json:"url,omitempty"`
	SourcePointer string `json:"source_pointer,omitempty"`
	Inline        bool   `json:"inline,omitempty"`
	ArchivedPath  string `json:"archived_path,omitempty"`
	Size          int64  `json:"size,omitempty"`
}

type Event struct {
	ID          int64           `json:"id,omitempty"`
	TraceID     int64           `json:"trace_id,omitempty"`
	Key         string          `json:"key"`
	ParentKey   string          `json:"parent_key,omitempty"`
	Branch      string          `json:"branch,omitempty"`
	Kind        string          `json:"kind"`
	Role        string          `json:"role,omitempty"`
	Model       string          `json:"model,omitempty"`
	Provider    string          `json:"provider,omitempty"`
	Tool        string          `json:"tool,omitempty"`
	CallID      string          `json:"call_id,omitempty"`
	Timestamp   string          `json:"timestamp,omitempty"`
	Text        string          `json:"text"`
	Sources     []SourceRef     `json:"sources,omitempty"`
	Attachments []Attachment    `json:"attachments,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	LegacyKeys  []string        `json:"-"`
}

type Normalization struct {
	Version  int       `json:"version"`
	Status   string    `json:"status"`
	Events   []Event   `json:"events"`
	Warnings []Warning `json:"warnings,omitempty"`
	// Title is a name the harness recorded in the session itself. It fills an
	// empty trace title; it never replaces one the collector supplied.
	Title string `json:"title,omitempty"`
}

type TraceOutcome struct {
	TraceID       int64  `json:"trace_id,omitempty"`
	Harness       string `json:"harness"`
	NativeTraceID string `json:"native_trace_id"`
	// Title is the trace's current display title, filled when a stored report is read.
	Title          string    `json:"title,omitempty"`
	RevisionDigest string    `json:"revision_digest"`
	Status         string    `json:"status"`
	Warnings       []Warning `json:"warnings,omitempty"`
	Error          string    `json:"error,omitempty"`
}

type ImportReport struct {
	ID          int64   `json:"id"`
	CreatedAt   string  `json:"created_at"`
	Machine     Machine `json:"source_machine"`
	Imported    int     `json:"imported"`
	Updated     int     `json:"updated"`
	Unchanged   int     `json:"unchanged"`
	Partial     int     `json:"partially_parsed"`
	Unsupported int     `json:"unsupported"`
	Failed      int     `json:"failed"`
	// Complete is false while an import runs or after it was interrupted.
	// Readers derive it from whether the final report was stored.
	Complete bool           `json:"complete"`
	Traces   []TraceOutcome `json:"traces"`
}

type Progress struct {
	Phase     string        `json:"phase"`
	Completed int           `json:"completed,omitempty"`
	Total     int           `json:"total,omitempty"`
	Outcome   *TraceOutcome `json:"outcome,omitempty"`
	Report    *ImportReport `json:"report,omitempty"`
	Error     string        `json:"error,omitempty"`
}
