# Transcript browsing and Conversation

The browser homepage lists sessions, not individual events. Each row opens the
whole transcript in the Svelte Conversation view. Text search still searches
event observations, but returns one card per matching session, with a matching
excerpt. Cards sort by session update time, then ID. Session-level cursors
prevent repeated matches from appearing on separate pages. Sessions without
normalized events remain visible when browsing.

The existing `/api/v1/search` contract still returns event results. The browser
uses a separate session-card query; API clients do not need to change.

## Viewer

Conversation is native Svelte. It uses the same dark, Cursor-like tokens as the
rest of the embedded app: a left-aligned turn column, collapsed thinking and
tools, and client-side navigation through the existing rail. The vendored Pi
export DOM is gone. Adapter logic that groups normalized blocks into turns,
pairs tool results with their calls, and guards parent cycles lives in
`web/app/src/lib/transcript/`.

A conversation tree appears only when the Event parent graph forks. Linear
graphs stay a turn list. Existing event/key links resolve to their containing
turn. `T` and `O` toggle thinking and tools. Amp's archive-view selector stays
on the workspace header; native export download stays on Details.

Every harness uses the same message and tool components. Tool arguments and
output start collapsed. Shell tools (`bash`, `Bash`, `shell_command`) show
commands directly while retaining their original names. File tools show paths,
highlighted code, and requested edits. Skills, model changes, compactions,
branch summaries, and custom records remain visible. Empty reasoning blocks
explain that the export omitted their text.

Display features depend on the available data, not the harness name. Archived
images use local previews; external image references stay links and unavailable
images have a readable notice. Source-specific collection, normalization, and
attachment retrieval remain separate; the renderer does not recover missing
bytes. Conversation renders at most 200 entries at once, with earlier/later
navigation. Tool results are paired within the selected branch.

This is a view of Traicr's merged normalized records, **not a new native source
parser**. Details keeps all revisions, parsing warnings, related sessions,
source inspection, and deletion.

## Amp exports

Amp normalizer version 4 retains numeric message identity, native block order,
tool answers and errors, summary blocks, image references, usage, timing, execution
state, origin, phase, and environment metadata. Unknown content has a visible
JSON fallback. Hidden context, summaries, tool answers, diffs, and arguments
can be expanded independently. Source inspection remains in Details.
Reasoning signatures are not interpreted as plaintext. Tool status describes the
recorded export, not a process that Traicr is monitoring.

The archive view selector separates merged history from an individual revision.
Copied message links retain the selected revision.
Native JSON downloads retain the exact exported bytes for that revision. Source
inspection identifies the revision supplying each observation. Spawned
threads, incoming messages, and references have distinct labels with local trace
lookup and original Amp links. A missing local trace produces an explicit empty
state; it does not create a child trace or imply the full subagent transcript is
present.

The Amp collector downloads hosted images with `amp files get`, using the source
machine's existing Amp login. User image blocks and tool-result images, including
`run.progress.displayImages`, are retained under `source/attachments/`. The
export JSON stays byte-for-byte unchanged. Repeated URLs with different media
fragments share one download within a trace. Only HTTPS attachments under
`ampcode.com/user-content/attachments/` are eligible; arbitrary external URLs
and local paths are never fetched. Failed downloads produce collection and
revision warnings, preserve the transcript, and are retried on later collection.
Reprocessing derives missing-image warnings from the export and retained files,
so Details also explains images missing from older archives.
Older Amp CLIs without `files get` need an update to collect hosted images.

Downloaded images have a separate 512 MiB total budget per trace and the existing
30-second command timeout per download. These are Traicr limits, not Amp limits.
Partial and over-budget downloads are discarded. Inline images already in the
export need no download. Recollect and upload with the updated collector to add
hosted images to existing traces; reindexing alone cannot retrieve missing bytes.
Retained images affect the revision digest, so adding them creates a new revision
even if the native export has not changed. When native update and event timestamps
tie, merged history prefers the observation with more archived images, before
the existing digest tie-break. Selected revisions still show only their own files.

Archived PNG, JPEG, GIF, and WebP images up to 10 MiB can be previewed through
authenticated endpoints without Amp access. Larger downloaded originals remain
downloadable through the retained-file endpoint. External references without
retained bytes remain links and may require Amp authentication. The server never
downloads missing images. SVG and other active content are not previewed.

Amp collection, normalization, and source inspection share a 512 MiB export
limit. Oversized collected exports produce an explicit collection warning; an
oversized imported source is retained with a normalization diagnostic and can
still be downloaded. Other normalizers retain their 64 MiB budget. JSON parsing
uses memory proportional to export size, so near-limit exports need substantially
more than 512 MiB of server memory. The browser loads 200 records per request and
renders at most 200 conversation entries at once. Load-more and earlier/later
controls retain access to the rest. Sidebar search and loaded-record counts cover
loaded records; archive search covers all indexed records. Deep links load pages
until their target is found. Other harnesses retain their existing all-pages
loading.
If an import or rebuild changes history between pages, the viewer requests a
reload rather than claiming that the incomplete history is fully loaded.
Transcript cursors use the latest normalizer-run ID as their generation. Writes
that change transcript events must record a normalizer run in the same transaction;
each page reads the generation and events from one database snapshot.

On server startup, the existing reindex worker rebuilds older Amp revisions from
retained exports. Rebuilds record aliases from old hashed event keys and IDs to
stable native keys so event, key, target, leaf, and source-inspection links
continue to resolve. Reindexing does not require recollection for transcript
details. Recollect with `traicr collect --all` using the updated collector and
upload to fill missing descriptor-level repository metadata even when the export
bytes are unchanged. Existing values and metadata from newer revisions are not
overwritten. Exported repository URL/ref/commit is already visible in rebuilt
environment details.

## Security and attribution

Markdown and syntax highlighting use Marked and Highlight.js from the Svelte app
dependencies. All assets are served from the embedded UI. Markdown images become
explicit links rather than automatic network requests. Tool arguments and source
text are escaped. Authentication still protects transcript data.

## Checks

```sh
GOTOOLCHAIN=auto go test ./internal/store ./internal/server
cd web/app && npm test
```

Browser checks should cover session rows, search grouping, Amp, Claude Code, and Pi
transcripts, multiple event pages, branch navigation, tool expansion, T/O keys,
deep links, malicious Markdown, and client-side navigation in and out of a trace.
