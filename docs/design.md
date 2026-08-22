# Traicr System Design

Status: accepted design, 2026-08-22

## Purpose

Traicr is a private, single-user archive for AI coding traces collected from several personal macOS and Linux machines. It preserves the harness-native records, presents a common trace view, and searches across harnesses, models, repositories, machines, dates, roles, tools, and event types.

The canonical domain language is defined in [CONTEXT.md](../CONTEXT.md).

## Goals

- Collect Amp, Cursor editor, `cursor-agent`, Claude Code, Codex, Pi, Grok Build, and OpenCode traces.
- Run collection manually from an installed, standalone binary on each source machine.
- Preserve enough native data to repair normalizers without recollecting source machines.
- Keep one logical trace when the same session is observed on several machines or at several times.
- Preserve branches, child traces, tool calls, tool results, reasoning, diffs, files, models, usage, and timestamps when the harness provides them.
- Search normalized text with full-text, exact, and regular-expression modes.
- Deploy one Docker application on a Debian server without PostgreSQL, object storage, or a separate search service.
- Handle an expected archive of about 50 GB without reading an entire upload or result set into memory.

## Non-goals

- Multiple users, teams, permissions, or trace sharing
- Windows collection
- Automatic or scheduled collection
- Server-controlled SSH collection
- Automatic redaction or review before upload
- Trace editing or annotation
- Dashboards and charts
- Browser-based trace collection
- Automatic collector updates
- Backups
- Application-managed TLS
- Training-data generation

## System shape

```text
┌──────────────────────┐
│ Source machine       │
│                      │
│ Harnesses            │
│ Collector adapters   │
│ Collection State     │
└──────────┬───────────┘
           │ Trace ZIPs
           ▼
┌──────────────────────────────────────────────┐
│ Traicr server                                │
│                                              │
│ Import validation ──▶ Source Record storage │
│          │                                   │
│          ▼                                   │
│ Harness Normalizers ──▶ SQLite Events + FTS │
│                                  │           │
│                           API + Web UI       │
└──────────────────────────────────────────────┘
```

Traicr is one Go repository with two applications:

- `traicr`: the collector installed on source machines
- `traicr-server`: the HTTP server packaged in Docker

The applications share Trace ZIP contracts, normalized Event types, validation rules, version information, and sanitized test fixtures. Harness collection belongs only to the collector. Harness normalization belongs only to the server.

## Collector

### Lifecycle

The collector is a standalone binary. It has no daemon, scheduler, embedded server, or required Go installation.

On first use it automatically creates:

- A stable random source-machine ID
- A machine record whose displayed name is always the current hostname
- A local configuration file
- Local Collection State

There is no `init` or machine-renaming command.

Planned commands:

```text
traicr sources
traicr collect --output ./exports
traicr collect --harness amp --output ./exports
traicr collect --all --output ./exports
traicr login http://traicr-server:8080
traicr upload ./exports/*.zip
traicr version
```

`sources` reports detected harnesses, source locations, available trace counts, and collection warnings without writing an archive.

`collect` includes traces that are new or changed since their last successful upload from that machine. `--all` ignores Collection State. Collection and upload remain separate so an archive can be inspected or transferred before sending it.

`login` stores the server URL and admin token in a user-only configuration file. It does not create a server-side user.

### Collection State

Collection State maps a harness-native trace identity to the content digest most recently acknowledged by the server. Creating a Trace ZIP does not advance this state. `upload` advances it only for revisions reported as imported, updated, or unchanged in a successful Import Report. Failed traces remain eligible for the next collection.

Collection State is an optimization, not authoritative data. Deleting it causes a complete recollection; server deduplication prevents duplicates.

### Source adapter policy

Collectors prefer supported, read-only harness commands because native storage formats change. They read local storage when no reliable bulk export exists. They never resume a session, send a prompt, mutate a harness database, or interpolate trace content into a shell command.

| Harness | Preferred source | Fallback or special handling |
| --- | --- | --- |
| Amp | `amp threads list --json` and `amp threads export` | Amp threads are server-backed; local logs are not a transcript source |
| Cursor editor | Read relevant SQLite rows using a read-only connection or SQLite backup | Preserve table, key, and original value; support known Chat and Composer generations |
| `cursor-agent` | Agent transcript JSONL plus its session metadata | Treat transcript and metadata as one source; tolerate an incomplete final line |
| Claude Code | Project session JSONL | Preserve parent links, tool-call IDs, branches, child-agent records, unknown entries, and incomplete tails |
| Codex | `codex app-server` thread, turn, and item APIs | Raw rollout parsing is a compatibility fallback, not the primary contract |
| Pi | Documented JSONL export or session format | Preserve the parent graph, model changes, compactions, images, and extension records |
| Grok Build | `grok export` | If needed, snapshot the complete native session directory rather than selected files |
| OpenCode | `opencode session list --format json` and `opencode export` | CLI and desktop share session storage; do not scrape desktop UI state |

An unavailable harness command, unreadable source, live-write race, unknown record, or partially collected session becomes a warning in the Trace ZIP. The collector does not silently drop it.

### Repository identity

The collector preserves the native working directory and detects the repository root and Git remotes when available. It strips credentials from remote URLs before writing them. The server groups paths from different machines by normalized remote URL. Repositories without a remote remain distinct until a later aliasing feature is explicitly designed.

## Trace ZIP contract

### Archive splitting

Collection writes numbered ZIP64 archives with a soft target of 2 GB:

```text
traicr-macbook-20260822-001.zip
traicr-macbook-20260822-002.zip
```

One trace is never split between archives. A single trace may exceed the target. Archives contain complete revisions, so every ZIP can be imported independently and retried safely.

### Layout

```text
manifest.json
traces/
  000001/
    descriptor.json
    source/
      records.jsonl
      attachments/
        ...
  000002/
    descriptor.json
    source/
      export.json
```

The exact native payload is harness-specific. JSON, JSONL, extracted SQLite rows, exported files, and attachments are all allowed. Cursor extraction wraps each row with its original database table and key instead of uploading an entire application database.

### Manifest

The versioned manifest records the collector and source-machine context plus one descriptor for every trace:

```json
{
  "format_version": 1,
  "collector_version": "0.1.0",
  "created_at": "2026-08-22T12:00:00Z",
  "source_machine": {
    "id": "01K...",
    "hostname": "macbook",
    "os": "darwin",
    "arch": "arm64"
  },
  "traces": [
    {
      "path": "traces/000001",
      "harness": "claude-code",
      "adapter": "claude-code-jsonl",
      "native_trace_id": "4d1a...",
      "native_updated_at": "2026-08-22T11:59:00Z",
      "revision_digest": "sha256:...",
      "parent_native_trace_id": null,
      "files": [
        {
          "path": "source/records.jsonl",
          "size": 12345,
          "sha256": "..."
        }
      ],
      "warnings": []
    }
  ]
}
```

The revision digest is calculated from sorted relative file paths and their content digests. ZIP timestamps, compression settings, archive names, and entry order do not affect revision identity.

### Validation

The server rejects an entire archive when its container is unsafe or unverifiable:

- Missing or unsupported manifest version
- Absolute paths, path traversal, duplicate paths, links, or non-regular files
- Missing files, size mismatches, or digest mismatches
- Archive size, expanded size, file-count, or nesting limits exceeded
- Corrupt ZIP structure

Unknown harness records do not invalidate an otherwise safe archive. The server retains them and reports a partial or unsupported normalization status.

## Import protocol

The collector sends each archive to:

```text
POST /api/v1/imports
Authorization: Bearer <admin token>
Content-Type: application/zip
```

The CLI calculates upload-byte progress while sending the request. After the upload reaches the server, the same HTTP response streams newline-delimited JSON:

```jsonl
{"phase":"validating"}
{"phase":"normalizing","completed":12,"total":100}
{"phase":"indexing","completed":85,"total":100}
{"phase":"complete","report":{"imported":80,"updated":15,"unchanged":5,"failed":0}}
```

There is no polling, SSE, resumable-upload protocol, or second status connection. If the connection or server fails, the collector retries the complete ZIP. Revision and Event deduplication makes retries safe.

The server processes traces independently after archive-level validation. A malformed trace does not roll back valid traces from the same ZIP. Every trace receives one of these outcomes:

- `imported`
- `updated`
- `unchanged`
- `partially_parsed`
- `unsupported`
- `failed`

The final Import Report and warnings are retained for the Imports page. Temporary upload files are removed after processing or failure.

## Storage

### Filesystem layout

One Docker volume contains all persistent state:

```text
/data/
  traicr.db
  traicr.db-wal
  traicr.db-shm
  objects/
    sha256/
      ab/
        cd...
  tmp/
```

Source files are stored by SHA-256 digest. Writes use a temporary file, verification, and atomic rename. Revisions reference objects through SQLite. The complete incoming ZIP is not retained after its trace objects are committed.

Hard deletion removes database references first. A mark-and-sweep pass deletes objects no remaining revision references. This avoids deleting a shared object and recovers files left behind by an interrupted import.

### SQLite

SQLite runs in WAL mode with foreign keys enabled and a busy timeout. The application uses one bounded import writer and short transactions so searches remain available during ingestion. The database and its WAL files stay on the same local Docker volume.

Core tables:

| Table | Purpose |
| --- | --- |
| `source_machines` | Stable machine IDs, observed hostnames, OS, architecture, and timestamps |
| `imports` | Archive metadata, status, counts, warnings, and final report |
| `repositories` | Normalized Git identity and observed local paths |
| `traces` | Logical trace identity, harness, native ID, parent trace, and summary metadata |
| `trace_revisions` | Revision digest, source machine, native times, collection time, and parse status |
| `source_objects` | Content digest, size, media type, and filesystem location |
| `revision_objects` | Relative source paths and object references for a revision |
| `events` | Logical normalized Events and their parent, branch, model, tool, role, and time fields |
| `event_observations` | Distinct normalized forms observed across revisions |
| `search_chunks` | Bounded human-readable text chunks associated with Events |
| `normalizer_runs` | Harness Normalizer version, outcome, and warnings per revision |

FTS5 virtual tables index full-text and trigram search data derived from `search_chunks`.

### Identity and deduplication

A trace is identified by its harness namespace and native trace ID. Source-machine identity is provenance and never part of trace identity.

A revision is identified by the digest of its Source Record files. Uploading the same revision from any machine is a no-op apart from recording the observation in the Import Report.

A Harness Normalizer supplies a stable Event key from the native Event ID and Event kind. When no native ID exists, it uses a deterministic fingerprint from stable native fields. Import order is never part of an identity.

Events observed across revisions merge into one logical trace. Identical observations collapse. Conflicting observations remain associated with the same logical Event and produce a warning rather than losing either Source Record. The viewer chooses a deterministic preferred observation using native update time, then native Event time, then content digest; search results deduplicate by logical Event.

Branches retain native parent relationships. A session spawned by another session is a Child Trace linked through its parent trace and originating Event rather than flattened into the parent timeline.

## Harness normalization

Each Harness Normalizer converts one source format into common trace metadata, Events, relationships, searchable text, and warnings. Normalizers preserve unknown records as Source Records even when they cannot produce an Event.

Every normalizer has an explicit version. An import records the version used. When a server deployment increases a normalizer version, one background worker rebuilds affected revisions. Existing Events remain searchable until replacement data for a revision is complete and can be swapped in one transaction. A failed rebuild leaves the previous Events active and records a warning.

Normalizers produce these Event kinds where native data permits:

- User, assistant, system, and reasoning messages
- Tool calls and tool results linked by call ID
- File context and attachments
- Proposed and applied edits or diffs
- Model and mode changes
- Compaction and summary records
- Usage and cost records
- Branch and child-trace relationships
- Unknown native records with searchable text when safe

## Search

### Indexed content

Traicr indexes all human-readable text without silently truncating it:

- Prompts, responses, reasoning, and summaries
- Tool names, arguments, results, and errors
- Diffs and code context
- File paths and attachment metadata
- Model, provider, mode, and repository labels

Large text values are split into bounded UTF-8 chunks with enough overlap to preserve matches across boundaries. Binary data, images, embeddings, and base64 payloads remain in Source Records but are not text-indexed.

### Search modes

- Full-text uses an FTS5 word index for terms, quoted phrases, ranking, and snippets.
- Exact search uses a trigram index for substring matching, including code and paths.
- Regular-expression search uses Go's bounded, linear-time regular-expression engine. It narrows candidates through filters and extractable literals when possible; expressions without a useful literal scan filtered text chunks and can be cancelled.

All modes support filters for harness, model, source machine, repository, date, role, Event kind, and tool. The API uses ordinary query parameters; Traicr has no custom query language.

Search results identify the logical Event, show a highlighted snippet and surrounding context, and open the complete trace. Matching observations from several revisions do not create duplicate result rows.

## HTTP API

The first API is versioned under `/api/v1`:

| Method and path | Purpose |
| --- | --- |
| `POST /api/v1/imports` | Upload and process a Trace ZIP with streamed NDJSON progress |
| `GET /api/v1/imports` | List Import Reports |
| `GET /api/v1/imports/{id}` | Read one Import Report |
| `GET /api/v1/search` | Search Events with explicit mode and filters |
| `GET /api/v1/traces/{id}` | Read trace metadata and relationships |
| `GET /api/v1/traces/{id}/events` | Page through a trace's Events and branches |
| `GET /api/v1/events/{id}/sources` | Inspect supporting Source Records and observations |
| `DELETE /api/v1/traces/{id}` | Permanently delete a trace and unreferenced source objects |
| `GET /healthz` | Process liveness |

List endpoints use stable cursor pagination rather than offset pagination. Error responses have a versioned JSON shape and never include Source Record content unless the authenticated caller explicitly requests it.

## Web UI

The server renders HTML with Go templates and uses HTMX for targeted interactions. There is no Node.js build, client-side application router, or separate frontend service.

Initial pages:

1. Search with mode selection, filters, snippets, and shareable URL parameters
2. Trace viewer with branches, linked Child Traces, models, tools, files, and Event context
3. Source Record inspector
4. Imports and Import Reports
5. Source Machines
6. Permanent trace deletion with explicit confirmation

The first version does not upload archives through the browser. Upload remains a collector command.

## Authentication and network

Traicr has one admin token supplied through `TRAICR_ADMIN_TOKEN`. API clients use it as a bearer token. Browser login exchanges it for a signed, HTTP-only, same-site session cookie. There are no users, roles, invitations, password resets, or registration routes.

The server listens on plain HTTP. Initially Docker publishes the port only on the Debian host's Tailscale address. Tailscale encrypts transport between personal machines and the server. Caddy may later terminate TLS and proxy to the same HTTP listener without changing Traicr's storage or API design.

All state-changing browser actions require same-origin requests and CSRF protection. Source text is escaped when rendered. Traicr never automatically fetches URLs found in Source Records.

## Deployment

The deployment consists of one application image and one persistent volume:

```text
Debian host
├── Tailscale
├── Docker
│   └── traicr-server
│       ├── HTTP :8080
│       └── /data volume
└── Caddy, optional later
```

The container runs one server process. It does not include PostgreSQL, Redis, Elasticsearch, a queue, nginx, Caddy, or S3-compatible storage.

Configuration is limited to the listen address, data directory, admin token, cookie mode for a future HTTPS proxy, log level, and safety limits for uploads and expanded archives.

## Capacity

The design target is approximately 50 GB of collected Source Records and millions of Events. The server should have at least 100 GB free, with 150 GB preferred for Source Records, normalized data, FTS indexes, temporary imports, and database maintenance.

Collection, upload, ZIP validation, hashing, normalization, indexing, API pagination, and deletion all operate incrementally. No normal path reads a complete archive, trace corpus, or search result set into memory.

Backups are deliberately outside the first version.

## Failure behavior

- Exact re-upload: report `unchanged` without duplicating a trace, revision, Event, or object.
- Older revision uploaded later: merge previously unseen Events without replacing a more complete trace.
- One malformed trace: import valid peers and report that trace as failed.
- Unsafe or corrupt archive: reject the complete archive before importing traces.
- Unknown source version: retain Source Records and mark it unsupported or partially parsed.
- Normalizer failure: retain Source Records and the previous successful Events.
- Collector interruption: leave the previous Collection State unchanged.
- Upload interruption: retry the complete numbered ZIP.
- Server restart during import: retry the ZIP; committed traces remain deduplicated.
- Source file changes during collection: use native export or database snapshot semantics where available and emit a warning when a stable snapshot cannot be proven.
- Hard deletion interruption: database references remain authoritative and object garbage collection safely resumes.

## Security boundaries

Traicr intentionally stores unredacted prompts, responses, reasoning, code, paths, diffs, tool input, tool output, and attachments. The server is trusted and reachable only through the owner's network path.

Even within that trust model, Traicr must:

- Require authentication for every trace, search, import, and deletion endpoint
- Compare credentials without timing-dependent string comparison
- Never log tokens or Source Record bodies
- Escape rendered content and send restrictive browser security headers
- Reject unsafe ZIP paths and resource-exhaustion archives
- Use fixed executable names and argument arrays rather than shell interpolation
- Read harness SQLite databases through read-only or backup-safe connections
- Strip credentials from Git remotes
- Avoid fetching attachment URLs or executing imported content
- Run the Docker container as a non-root user
