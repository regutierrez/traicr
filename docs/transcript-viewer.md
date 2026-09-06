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
0.85.1. The viewer uses Traicr's shared light palette, green accents, and body
typography, with monospace text reserved for code and the tree. A harness badge
appears beside the session heading. The Pi layout, searchable branch tree,
filters, Markdown/code rendering, expandable
tool output, copy links, sidebar resizing, mobile navigation, and T/O controls
are retained.

Traicr loads all event pages through a cookie-authenticated route before building
the viewer. The adapter groups normalized message blocks into turns and connects
tool results to their calls. Amp normalizer version 2 fixes native tool-use ID
pairing; the existing reindex worker rebuilds older Amp revisions on deployment.
The viewer preserves known parent branches and guards
against cycles. Existing event/key links resolve to their containing turn.

This is a view of Traicr's merged normalized records, **not a new native source
parser**. Unknown source records, original inline image bytes, tool-specific
metadata, and usage not retained by the current normalizers are not invented.
The viewer does not show fabricated token counts or costs. Session details keeps
all revisions, parsing warnings, related sessions, source inspection, and deletion.
“Viewer JSONL” downloads the adapted viewer data, not a lossless native export.

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
node --test web/static/pi-transcript/transcript-data.test.mjs
node --check web/static/pi-transcript/pi-transcript.js
```

Browser checks should cover session cards, search grouping, both Amp and Pi
transcripts, multiple event pages, branch navigation, tool expansion, T/O keys,
deep links, malicious Markdown, and mobile sidebar controls.
