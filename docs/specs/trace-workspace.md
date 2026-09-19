# Trace workspace

## Problem Statement

I can collect traces into Traicr, but reading them is worse than the products this archive is meant to beat. The session list is a card grid. The transcript is a vendored Pi export viewer that treats every harness like Pi. I cannot see the files a session changed, the commits it produced, or whether the run looped or blew up a tool. I use Jujutsu and Git on repositories other people also use, so nothing about this may show up in their clone.

## Solution

Traicr stays a private single-user archive and becomes the place I read my own agent work. After I unlock it I get a dark, Cursor-like three-pane shell: a rail, a dense session list, and a workspace with Conversation, Changes, Timeline, and Details. Conversation is Turns in a column; a tree appears only when the Event graph forks. Changes lists Patches. Timeline is a log with Signal ticks and commit rows. Anchors live only in the archive. Insights wait until this workspace is not finicky.

This replaces the Linear-inspired, light-and-dark direction in issue #9. The reference is Cursor's agent list and chat workspace, not Linear and not the old Traicr paper/green theme.

## Visual language

The agreed aesthetic is **Cursor-like, dark-only, dense, and quiet**. Implementers treat this section as a constraint, not a later polish pass. Token-level pixel matching waits on screenshots; until those exist, use shadcn-svelte dark tokens tuned toward Cursor, not a new brand.

### Reference

- **Primary**: Cursor Cloud Agents / agent list, and Cursor's chat workspace (composer + agent transcript).
- **Not the reference**: Linear (issue #9), traces.com marketing, AgentTrace eval dashboards, the current Traicr paper/green tokens (`--paper`, `--ink`, `--green`), daisyUI, or the vendored Pi transcript chrome.
- A later screenshot pass may retune exact greys, accent, and radius. Do not invent a second theme while waiting.

### Surfaces and color

- Dark mode only. No light theme, no theme toggle, no `prefers-color-scheme` flip.
- Near-black application canvas. The rail is the darkest strip. The session list and workspace sit on a slightly lighter surface. Selected rows and sticky headers use a third, still-quiet lift — not a bright card.
- Hairline separators (`1px` border at ~10–12% white) instead of boxed cards, drop shadows, or tinted panels.
- One muted accent (Cursor-like blue/teal) for selection, focus, and the active rail icon. Semantic color is reserved for Signal chips and diff hunks: red/green only on `+/-` and errors; amber only on warnings.
- No decorative gradients, glass, or blur. Respect `prefers-reduced-motion`; if motion exists it is short and spatial (pane collapse), never ornamental.

### Type and density

- UI chrome is compact: ~13px body, 11–12px meta, tabular figures for counts and times.
- Conversation prose is the exception: comfortable reading measure (~68–80ch), not the chrome size.
- Titles truncate to one line in the list. Meta (harness, repo, model, duration, counts, relative time) is secondary and never competes with the title.
- Monospace for SHAs, paths, diffs, tool names, and code. Highlight.js uses a dark theme that matches the chrome.

### Shell

Three panes, hideable:

1. **Rail** (~48px): icon-only. Sessions, Repositories, Archive. Insights is omitted or visibly disabled until that ticket. Active icon: quiet fill + accent, not a badge count.
2. **Session list** (~320–360px): dense rows grouped by day (`Today`, `Yesterday`, weekday, then date). Not a card grid. `[` / `]` hide and show it; the collapsed state is remembered. Below ~1100px the list is a full page with a back chevron.
3. **Workspace**: Conversation / Changes / Timeline / Details. Sticky header with title, harness, counts, Signal chips (only when a Signal fired), Amp revision selector when relevant, and underline tabs — not pill cards.

`j` / `k` / `Enter` move the list. The open Trace is the URL. On a wide screen the workspace updates in place so the list stays visible.

### Conversation

- A single Turn column. User and assistant share a left-aligned document flow (Cursor chat), not chat bubbles and not a card stack.
- Thinking and tools start collapsed. A tool is one quiet row (`tool · path or status`); expand in place for args and result. `T` and `O` toggle thinking and tools.
- Token totals stay on Details, not in the Conversation header.
- A conversation tree exists only when the Event parent graph forks. Linear graphs must not grow a fake tree.
- A Child Trace is a compact card/row that opens that Trace in the same workspace. Do not dock a second transcript pane (that was issue #8 concept A / the Linear plan). A missing child says it was never collected.

### Changes

- File list on the left of the workspace (or a stacked list on narrow widths): path, tool, `+/-`.
- Selected file shows a unified diff. No split view in the first slice.
- Anchors for that path sit under the file as commit rows (short SHA, subject, time). Clicking a SHA opens the Repository page.

### Timeline

- A vertical log, not a Gantt and not swimlanes.
- Phase labels (`user`, `think`, `read`, `exec`, `write`) and Signal ticks in the gutter.
- Anchor commits interpolate as commit rows on the same clock.
- Clicking a row jumps to Conversation or Changes.

### What this is not

- The old Traicr paper/green theme, daisyUI, or shadcn-on-Go-templates.
- A card grid of sessions.
- Light mode or a third theme.
- A command palette (later ticket).
- Eval dashboards, score colors, or AgentTrace-style charts.
- Heavy headers, floating toolbars, or a permanently open right-hand inspector.

## User Stories

1. As the archive owner, I want to open Traicr and land on my sessions, so that I can pick up a recent Trace without searching first.
2. As the archive owner, I want a narrow rail for Sessions, Repositories, Insights, and Archive, so that I always know where I am.
3. As the archive owner, I want a dense session list grouped by day, so that I can scan the way I scan a Cursor agent list.
4. As the archive owner, I want each row to show harness, title (or first user message), repository, model, duration, message count, files-changed count, and relative time, so that I can recognize a session without opening it.
5. As the archive owner, I want a search box and filter chips on the list, so that I can narrow by harness, repository, or text.
6. As the archive owner, I want a match excerpt only while a text query is active, so that idle rows stay short.
7. As the archive owner, I want `j`/`k`/`Enter` on the list, so that I can move without the mouse.
8. As the archive owner, I want the workspace to update in place on a wide screen, so that the list stays visible.
9. As the archive owner, I want `[` and `]` to hide and show the list, so that Conversation can go full width.
10. As the archive owner, I want that list collapse remembered, so that I do not fight the layout every visit.
11. As the archive owner, I want a full-page list under 1100px with a back chevron into the workspace, so that a laptop still works.
12. As the archive owner, I want the URL to be the Trace when one is open, so that refresh keeps my place.
13. As the archive owner, I want a sticky workspace header with title, harness, counts, Signal chips, and tabs, so that I can change views without losing context.
14. As the archive owner, I want Conversation, Changes, Timeline, and Details as tabs, so that each job has a surface.
15. As the archive owner, I want Conversation to list Turns, so that I read a session the way I read a chat.
16. As the archive owner, I want thinking and tools collapsed until I open them, so that prose stays readable.
17. As the archive owner, I want `T` and `O` to toggle thinking and tools, so that I can skim or inspect without the mouse.
18. As the archive owner, I want token totals on Details, not in the Conversation header, so that the reading surface stays quiet.
19. As the archive owner, I want a conversation tree only when the Event parent graph forks, so that branched Pi sessions stay navigable and linear sessions do not grow a fake tree.
20. As the archive owner, I want clicking a tree node to show that path only, so that a Branch is readable.
21. As the archive owner, I want a Child Trace to appear as a card that opens that Trace in the same workspace, so that subagents stay honest links.
22. As the archive owner, I want a missing Child Trace to say it was never collected, so that I do not think the full child is present.
23. As the archive owner, I want Amp's revision selector on the header, so that I can see merged history or one Trace Revision.
24. As the archive owner, I want native Amp bytes available from Details, so that I can download the retained export without calling that sharing.
25. As the archive owner, I want Changes to list every Patch path, so that I can see what the session edited.
26. As the archive owner, I want a unified diff when the Patch has one, so that I can read the edit.
27. As the archive owner, I want tool args and result when there is no diff, so that unknown edit tools still show something true.
28. As the archive owner, I want Anchors for a path listed under that file, so that I can see which commit accepted it.
29. As the archive owner, I want clicking a commit to open that SHA on the Repository page, so that I can see every Trace on that commit.
30. As the archive owner, I want Timeline to be a vertical log with phase labels, so that I can scan user / think / read / exec / write.
31. As the archive owner, I want Signal ticks in the Timeline gutter, so that loops and errors are visible in place.
32. As the archive owner, I want commit rows on Timeline when an Anchor falls in the session window, so that edits and commits share one clock.
33. As the archive owner, I want clicking a Timeline row to jump to Conversation or Changes, so that the log is an index, not a dead view.
34. As the archive owner, I want Signal chips in the header only when a Signal fired, so that a clean run stays quiet.
35. As the archive owner, I want clicking a chip to jump to the first matching Timeline row, so that I can see why it fired.
36. As the archive owner, I want Details to keep revisions, warnings, sources, relations, and delete, so that the forensic view is not lost.
37. As the archive owner, I want a Repository page with remotes or roots, Traces, and Anchors, so that Changes has somewhere to send a SHA.
38. As the archive owner, I want copying a SHA and filtering to that commit, so that I can gather every Trace on it.
39. As the archive owner, I want Archive to group Imports and Machines, so that collection history is one place.
40. As the archive owner, I want Insights deferred until this workspace is boring, so that we do not polish charts on a broken reader.
41. As the archive owner, I want Patches derived from known edit tools on rebuild, so that old traces get a Changes tab without recollection.
42. As the archive owner, I want Signals derived from Events on rebuild or read, so that old traces get chips without recollection.
43. As the archive owner, I want Anchors from `git log` in the working directory at collect time, so that Git sessions link to commits.
44. As the archive owner, I want Anchors from described Jujutsu revisions only, so that working-copy snapshots are not treated as real commits.
45. As the archive owner, I want each Jujutsu Anchor to store the Git SHA and the change-id, so that a later rewrite still matches.
46. As the archive owner, I want no Git hook on Jujutsu repos, so that we do not pretend hooks fire.
47. As the archive owner, I want no notes or hooks written into remotes other people fetch, so that collaborators are not affected.
48. As the archive owner, I want the next `traicr collect --all` to backfill Anchors when the repo still exists, so that old traces can catch up.
49. As the archive owner, I want a Trace with no recorded cwd or a missing repo to stay without Anchors, so that we do not invent history.
50. As the archive owner, I want dark mode only, so that the chrome matches Cursor until I supply screenshots for tokens.
51. As the archive owner, I want the browser UI to be the embedded SvelteKit app, so that Go templates are gone.
52. As the archive owner, I want the Pi transcript DOM gone, so that Conversation is Svelte and shares one Event model with the other tabs.

## Implementation Decisions

- Land the open SvelteKit migration onto `main` first (PR 21's commits, retargeted). Go serves JSON under `/api/v1` and the static Svelte shell for every other browser GET. Fix the foundation e2e: unauthenticated `GET /` is the Svelte document, not the old Go login string.
- Close or abandon the daisyUI and shadcn-on-templates drafts; they are superseded.
- Rebuild Conversation as Svelte components. Keep the existing adapter that groups Events into Turns and pairs tool calls with results; port it to a typed module. Drop the vendored Pi DOM. Keep Marked and Highlight.js for Markdown and code.
- Show the conversation tree if and only if the Event parent graph has a fork. Linear graphs are a Turn list. This is data-driven, not harness-named, so Pi forks keep a tree and linear Pi looks like Cursor.
- Child Traces render as cards that swap the workspace to that Trace. A missing child is an explicit empty state. Do not inline child Turns.
- Amp revision selector stays on the workspace header. Native export download stays on Details. Other harnesses use merged history.
- Introduce Patch as a derived record from known edit tools at normalize time (write, edit, apply-patch, and the harness-specific names already recognized in the renderer). Persist enough to list path, tool, source Event, and a unified diff when the payload has old/new or patch text.
- Introduce Signal as a deterministic function over a Trace's Events: tool errors, repeated identical commands, oversized tool results, repeated file reads, repeated searches. Surface as header chips and Timeline gutter ticks. Do not persist a parallel event log if it can be derived on read from stored Events; persist only if read cost is too high after measurement.
- Introduce Anchor as an archive-only record: repository identity, commit SHA, optional Jujutsu change-id, subject, committer time, linked Trace. Collect at `traicr collect` by reading Git or Jujutsu history in the Trace working directory. Jujutsu: described revisions in the session window, exclude live `@`. Never write `refs/notes` or install a hook that collaborators would run.
- Timeline is a vertical Event log with a derived phase (`user`, `think`, `read`, `exec`, `write`) and interpolated Anchor rows. No swimlanes.
- Session list uses the existing session-card search seam (one row per Trace, session cursors, fulltext/exact/regex). Change presentation, not the query contract, until files-changed and duration need new stored fields; those fields are filled from Events/Patches when present.
- Repository page is a new browser route over existing Repository grouping plus Anchors.
- Insights is specified but not built in the first implementation slice.
- Dark-only shadcn-svelte (bits-ui) + Tailwind 4 tokens. Cursor-faithful token pass waits on screenshots. Until then, tune the default dark palette toward Cursor (near-black canvas, hairline borders, one muted accent) and delete the paper/green Traicr theme from any route the Svelte app owns.
- Issue #9 (Linear-inspired, light-and-dark, right-hand inspector, command palette first) is superseded. Keep its useful constraints: density, keyboard, fewer cards, `prefers-reduced-motion`. Drop light mode, Linear-as-reference, and the child-transcript side pane.
- Keyboard: `j`/`k`/`Enter` on the list, `[`/`]` for the list pane, `T`/`O` in Conversation. Command palette is a later ticket.

### Test seams

Prefer existing seams: HTTP JSON for traces and events, normalizer `Run` for Patch extraction, collector collect output for Anchors, browser pages for shell and tabs.

1. **Normalizer `Run`** — given Source Records, Events gain Patch-capable metadata (or sibling Patch records) for known edit tools. Highest existing seam for Changes data.
2. **Store / HTTP trace + events** — Conversation, Timeline, and Signal chips consume the same Event page the viewer already loads. Add fields only through this JSON.
3. **Collector `Collect`** — Anchor files appear in the Trace ZIP for Git and described Jujutsu history; nothing is written into the source repository.
4. **Browser shell** — unauthenticated `GET /` is the Svelte document; authenticated session list and `/traces/{id}` render the workspace tabs.

Do not add a second Event parser in the browser.

## Testing Decisions

- Test external behavior: HTML/JSON over HTTP, normalizer outputs, ZIP contents. Do not assert Svelte component internals.
- Extend `internal/normalize` tests for Patch extraction (same style as tool-call pairing tests).
- Extend collector tests with a temporary Git repo and a temporary colocated Jujutsu repo when `jj` is available; skip Jujutsu when the binary is missing.
- Extend `internal/server` page/API tests for workspace JSON (Patches, Signals, Anchors) and for Child Trace cards.
- Update e2e foundation: `GET /` is HTML for the Svelte shell (doctype / `data-sveltekit` or login route still reachable), not the Go template string `Open your archive`.
- Port `web/static/pi-transcript/*.test.mjs` adapter tests to the typed module; keep Turn grouping and cycle guards.
- Prior art: `transcript_pages_test.go`, `normalize_test.go`, `collector_test.go`, `foundation_server_test.go`.

## Out of Scope

- Export, publish, share links, scrubbing, MCP, HuggingFace, datasets, capsules.
- User-defined evaluations and AgentTrace-style score dashboards.
- Patch survival, blame, change bursts, Context tree of model input.
- Git notes, post-commit hooks that run for collaborators, pushing any Traicr ref.
- Insights page (deferred).
- Command palette, third theme, light mode, Linear-as-reference (issue #9), and a right-hand child-transcript inspector.
- Reconstructing file bytes from disk at collect time.
- Inlining Child Trace history that was not collected.

## Further Notes

Implementation order: (1) land SvelteKit on `main` and fix e2e, (2) Svelte Conversation, (3) Changes from Patches, (4) Timeline and Signal chips, (5) three-pane session list, (6) Anchors plus Repository page, (7) Insights.

Every UI ticket in that sequence must ship the Visual language for the surfaces it touches. Aesthetic is not a trailing cleanup ticket.

ADRs: 0006 nothing leaves the archive; 0007 embedded SvelteKit; 0008 archive-side Anchors. Glossary: Trace, Event, Turn, Branch, Child Trace, Patch, Anchor, Signal, Repository.

Issue #9 is superseded by this spec. Issue #8 is satisfied by Child Trace cards in Conversation, not by the paper-theme mockups in `docs/mockups/subagent-transcript-concepts.md`.

PR 21's GitHub base is `cursor/shadcn-ui-d7e0`, not `main`. Landing it means replaying those commits onto `main`, not using the GitHub merge button on that base.
