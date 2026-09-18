# Transcript browsing and Pi viewer

The browser homepage lists sessions, not individual events. Each card opens the
whole transcript. Text search still searches event observations, but returns one
card per matching session, with a matching excerpt. Cards sort by session update
time, then ID. Session-level cursors prevent repeated matches from appearing on
separate pages. Sessions without normalized events remain visible when browsing.

The existing `/api/v1/search` contract still returns event results. The browser
uses a separate session-card query; API clients do not need to change.

## Viewer

`web/static/pi-transcript/` adapts the HTML export viewer from Pi coding agent
0.85.1. The viewer uses Traicr's shared theme tokens and body typography, with monospace text reserved for code and the tree. A harness badge
appears beside the session heading. The Pi layout, searchable branch tree,
filters, Markdown/code rendering, expandable
tool output, copy links, sidebar resizing, mobile navigation, and T/O controls
are retained.

The adapter groups normalized message blocks into turns and connects tool results
to their calls. The viewer preserves known parent branches and guards against
cycles. Existing event/key links resolve to their containing turn.

Every harness uses the same message and tool renderer in `transcript-render.js`.
Tool arguments and output start collapsed in separate dropdowns. Shell tools
(`bash`, `Bash`, `shell_command`) show commands directly while retaining their
original names. File tools show paths, highlighted code, and requested edits.
Skills, model changes, compactions, branch summaries, and custom records remain
visible. Empty reasoning blocks explain that the export omitted their text.
Deep branch indentation is capped so sidebar labels remain visible. Per-message
source panels are omitted from the conversation.

Display features depend on the available data, not the harness name. Archived
images use local previews; external image references stay links and unavailable
images have a readable notice. Source-specific collection, normalization, and
attachment retrieval remain separate; the renderer does not recover missing bytes.
Every harness renders at most 200 entries at once, with earlier/later navigation.
Tool results are paired within the selected branch.

This is a view of Traicr's merged normalized records, **not a new native source
parser**. Session details keeps all revisions, parsing warnings, related sessions,
source inspection, and deletion. “Viewer JSONL” downloads the adapted, loaded
viewer data, not a lossless native export.

## Amp exports

Amp normalizer version 4 retains numeric message identity, native block order,
tool answers and errors, summary blocks, image references, usage, timing, execution
state, origin, phase, and environment metadata. Unknown content has a visible
JSON fallback. Hidden context, summaries, tool answers, diffs, and arguments
can be expanded independently. Source inspection remains in Session details.
Reasoning signatures are not interpreted as plaintext. Tool status describes the
recorded export, not a process that Traicr is monitoring.

Usage totals count each loaded assistant message once across its content blocks,
using the preferred observation in merged history or only the selected revision.
Missing counts are unknown or partial, never invented zeros or prices. Original
usage and block timing fields remain available through source inspection. Timestamp
precedence is message `timestamp`/`createdAt`, `meta.sentAt`, block
`startTime`/`finalTime`, then usage timestamp.

The archive view selector separates merged history from an individual revision.
Copied message links retain the selected revision.
Native JSON downloads retain the exact exported bytes for that revision. Source
inspection identifies the revision supplying each observation. Spawned
threads, incoming messages, and references have distinct labels with local trace
lookup and original Amp links. A missing local trace produces an explicit 404;
it does not create a child trace or imply the full subagent transcript is present.

The Amp collector downloads hosted images with `amp files get`, using the source
machine's existing Amp login. User image blocks and tool-result images, including
`run.progress.displayImages`, are retained under `source/attachments/`. The
export JSON stays byte-for-byte unchanged. Repeated URLs with different media
fragments share one download within a trace. Only HTTPS attachments under
`ampcode.com/user-content/attachments/` are eligible; arbitrary external URLs
and local paths are never fetched. Failed downloads produce collection and
revision warnings, preserve the transcript, and are retried on later collection.
Reprocessing derives missing-image warnings from the export and retained files,
so Session details also explains images missing from older archives.
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
controls retain access to the rest. Sidebar search and usage totals cover loaded
records; archive search covers all indexed records. Deep links load pages until
their target is found. Other harnesses retain their existing all-pages loading.
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

Pi viewer sources: <https://github.com/earendil-works/pi-mono>, package
`@earendil-works/pi-coding-agent`, `dist/core/export-html/`.

Vendored dependencies:

- Pi viewer 0.85.1: MIT, see `web/static/pi-transcript/pi-LICENSE.txt`.
- Marked 18.0.5: MIT and Markdown attribution, see
  `web/static/pi-transcript/marked-LICENSE.txt`.
- Highlight.js 11.9.0: BSD-3-Clause, see
  `web/static/pi-transcript/highlight-LICENSE.txt`.

All assets are served locally. No npm runtime or Node build is needed for the
server image. Inline click handlers were replaced with delegated event handlers
so Traicr's existing Content Security Policy remains unchanged. Markdown images
become explicit links rather than automatic network requests. Tool arguments
and source text are escaped. Authentication still protects transcript data.

## Checks

```sh
GOTOOLCHAIN=auto go test ./internal/store ./internal/server
node --test web/static/pi-transcript/*.test.mjs
node --check web/static/pi-transcript/pi-transcript.js
```

The Node commands are manual checks. They are not part of `make all` or CI.

Browser checks should cover session cards, search grouping, Amp, Claude Code, and Pi
transcripts, multiple event pages, branch navigation, tool expansion, T/O keys,
deep links, malicious Markdown, and mobile sidebar controls.
