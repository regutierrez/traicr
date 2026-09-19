# Traicr

A private archive for collecting, searching, and reading one person's AI coding traces from multiple personal machines, including the file edits and commits those traces produced.

## Language

**Trace Archive**:
The private service that stores and searches traces collected from all source machines.

**Harness**:
The AI coding application or runtime that owns a session, such as Amp, Cursor, Claude Code, Codex, Pi, Grok Build, or OpenCode.

**Trace**:
One native harness session or thread, including its messages, tool calls, results, branches, and metadata.
_Avoid_: Conversation, chat

**Trace Revision**:
A distinct collected state of a trace. Events observed across its revisions merge into one trace in search results and history.

**Trace ZIP**:
A portable archive containing one or more traces as Source Records and the information required to import them.
_Avoid_: Trace pack, export bundle

**Source Record**:
A lossless harness-specific item collected from a trace, preserving its native field names, identifiers, and values.
_Avoid_: Source-aware record

**Harness Normalizer**:
The server component for one harness that converts its Source Records into common Events.
_Avoid_: Harness parser

**Event**:
A common searchable representation derived from Source Records, such as a message, tool call, tool result, or model change.

**Turn**:
A user Event and the assistant Events that follow it until the next user Event.
_Avoid_: Bubble, message group

**Branch**:
An alternative continuation within a trace.

**Child Trace**:
A distinct trace spawned from another trace, linked to its parent rather than flattened into it.

**Model**:
The AI model used for an event, independently of the harness that produced the trace.

**Source Machine**:
A personal macOS or Linux machine from which traces are collected. It has a stable internal identity, uses its hostname as its name, and does not define trace identity.

**Collection State**:
A source machine's local record of trace revisions it has already collected. It avoids repeated work but is not the authoritative archive.
_Avoid_: Collection ledger

**Import**:
The server's processing of one Trace ZIP into trace revisions, Source Records, and Events.

**Import Report**:
The outcome of an import, listing imported, unchanged, updated, and failed traces.

**Repository**:
One codebase associated with traces across source machines, independently of the local path used on each machine.

**Patch**:
One recorded file edit produced by a tool call in a trace, typically a write or apply-patch on a single path.
_Avoid_: Change burst, hunk (unless referring to a diff region inside a Patch)

**Anchor**:
A recorded link between a commit in a Repository and the Trace that produced it. It lives in the archive, not in the repository other people clone.
_Avoid_: Git note, git trail, survival

**Signal**:
A deterministic finding derived from a Trace's Events, such as a tool error, a repeated command, an oversized tool result, or a file read many times in a short window.
_Avoid_: Evaluation, score, annotation
