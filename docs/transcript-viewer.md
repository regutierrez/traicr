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

Conversation is native Svelte. It uses the same dark, T3 Code-like tokens as the
rest of the embedded app: a turn column, work rows that fold thinking and
tools, and client-side navigation through the existing rail. The vendored Pi
export DOM is gone. Adapter logic that groups normalized blocks into turns,
pairs tool results with their calls, and guards parent cycles lives in
`web/app/src/lib/transcript/`.

A trace opens on two tabs. **Overview** (the default, `/traces/{id}`) is the
session at a glance: harness, model, thinking level, start and last activity,
directory, repository, Amp session facts, counts of prompts, replies, thoughts,
tool calls, and compactions, tool use by name with failures, recorded token
usage, branches, related traces, and the archive (revisions, native export,
delete). **Trace** (`/traces/{id}/trace`) is the conversation: one linear
stream, always the selected path. User messages are
right-aligned bubbles; agent replies are unfilled prose. Runs of three or more
tool and thought steps fold into one `Worked for … · Ran 3 commands …` row;
shorter runs and steps inside an open fold stay quiet rows with traces.com
icons and labels (`Ran command`, `Read file`, `Thought`, `Used` leftovers).
Each Overview count links to Trace with `?filter=`, which shows only that kind
of row under a "Showing only …" bar. Older links with `event`, `key`,
`leafId`, `targetId`, or `debug` on `/traces/{id}` redirect to Trace, and
`/traces/{id}/records` redirects to Overview. Raw records stay on the API and
the `inspect-trace` script, not a tab.

Forks do not grow a tree pane. Branches stay in the index: the count comes
from the parent graph, and choosing Branches lists leaf paths so a different
path can be selected without changing the page shape. Existing event/key links
resolve to their containing turn. `E` expands or collapses every work row; single thoughts and tools open one at a time.

Display features depend on the available data, not the harness name. Tool
bodies start collapsed. Opened, a tool shows the command, the file path, the
edit as a diff, the output, and the exit code only when it failed; raw
arguments appear only for tools with nothing more readable. Shell tools (`bash`, `Bash`,
`shell_command`) keep their recorded names in the adapter and expand to the
command; the chip says `Ran` plus that command. File tools (including Amp's `edit_file` and `create_file`) show paths,
highlighted code, and edits as diffs. Empty reasoning blocks explain that the export omitted
their text.

Display features depend on the available data, not the harness name. Archived
images use local previews; external image references stay links and unavailable
images have a readable notice. Source-specific collection, normalization, and
attachment retrieval remain separate; the renderer does not recover missing
bytes. Conversation shows the loaded path from the first message. Tool
results are paired within the selected branch.

This is a view of Traicr's merged normalized records, **not a new native source
parser**. Overview keeps revisions, parsing warnings, related sessions, and
deletion; source inspection stays reachable from revision links and the API.

## Harness-only extras (flagged, not a second viewer)

These records still render in the shared stream or header. They are not a
reason to change the layout. Decide later whether any of them need a dedicated
chip or should stay as a quiet note.

Shared stream, when the Event has the data:

- Compaction, hidden context, and `system` role messages (Pi system prompts,
  Claude Code injected context) expand in place as chips.
- Pi user-run commands (`bashExecution`) render as a user row with a `bash`
  tool call and its output and exit code.
- Skill blocks (`<skill>` in Pi user text, Amp `skill` tool) stay a chip plus
  the user remainder.
- Child traces (`create_thread`, `fromExecutorThreadID`, the Claude Code `Agent`
  call that names the subagent's `agentId`, leftover children) stay one quiet
  row that opens the child or says it was never collected. A child's header
  links back to its parent. The session list shows a child under its top
  collected ancestor; a search match inside it says which subagent matched.
- Unknown native types and uncommon stop reasons stay a note plus JSON/text.
Amp-only today:

- Archive view (merged vs one revision) still sits on the workspace header.
  Other harnesses have no equivalent control, so the header is not identical.
- `openAIResponsePhase` (`commentary`, `final_answer`) is retained on the
  Event and not drawn as its own chip.
- `fromAutomation` is a quiet "Automation message" note.
- `oracle`, `Task`, `create_thread`, and `apply_patch` use the shared chip
  row (`Oracle`, `Task · …`, `Thread …`, `Edit file`). traces.com also has
  Process and Cron chips; no current fixture emits those tool names.
- Native JSON download sits under its revision on Overview.

Pi / Claude parent-graph extras:

- `branch_summary` is a Thought-style chip on the path that contains it.
  Switching paths is the Branch picker above the stream (or the Branches list
  on Overview), not a sidebar tree.
- `model_change` and `thinking_level_change` sit in the workspace header
  and on Overview, not in the stream. Empty thinking levels
  are omitted. The model label uses the traces.com display name
  (`gpt-6-astra` → `GPT 6 Astra`).
- Pi `custom` records are extension sidecars (`plannotator`,
  `pi-rename-titles`, `context:skill_loaded`, `tps-stats`). They stay
  quiet notes: `(used pi-extension <name>)`. They are not Turns.

If a later harness adds Process, Cron, or another traces.com kind, map it
onto the existing kind list rather than adding a per-harness pane.

## Amp exports

Amp normalizer version 6 keys each message by its `protocolMessageID`. Amp's
numeric `messageId` is only the message's position, so it moves when a later
export inserts or removes a message. Tool results are keyed by their call ID and
other blocks by their number among blocks of the same kind, because later exports
can drop thinking blocks. Keys from earlier versions stay as aliases. The
normalizer retains native block order,
tool answers and errors, summary blocks, image references, usage, timing, execution
state, origin, phase, and environment metadata. Prompts and written replies show when
they were sent while hovered; per-reply tokens are summed on Overview instead. Overview lists the session's agent mode, branch and commit,
client, executor, and skills. Unknown content has a visible
JSON fallback. Hidden context, summaries, tool answers, diffs, and arguments
can be expanded independently.
Reasoning signatures are not interpreted as plaintext. Tool status describes the
recorded export, not a process that Traicr is monitoring.

The archive view selector separates merged history from an individual revision.
Path deep-links retain the selected revision.
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
so revision warnings on Overview also explain images missing from older archives.
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
keeps requesting pages until the selected transcript is complete, then
renders the path from the first message. Overview counts cover loaded records; archive search covers all indexed records.
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
