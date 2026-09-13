# Subagent transcript concepts (issue #8)

Visual concepts for "Open subagent transcripts from their parent thread"
([#8](https://github.com/regutierrez/traicr/issues/8)). All three use the
existing Traicr theme tokens (`--paper #f7f8f4`, `--surface #fff`, `--ink
#202b26`, `--muted #606d65`, `--line #dce2d9`, `--green #25583e`, `--soft
#eaf0e7`) and the transcript viewer's existing layout (session tree sidebar,
nav, header card, message stream). All transcript content is synthetic.

Shared vocabulary across the concepts, so whichever ships stays consistent:

- **Delegated-work row** — a quiet single-line entry in the message stream:
  `↳ <child title>` plus a right-aligned mono meta line (`harness · status ·
  duration`) and a `›` chevron. Same height rhythm as a tool row, no extra
  card chrome.
- **Status without clutter** — filled green dot = unread, hollow green ring =
  running, `✓` = finished. No badges, no counts on the row itself.
- **Child header** — `DELEGATED WORK` eyebrow, child title (with fallback),
  one mono meta line (`harness · model · cwd`), and an `Open standalone ↗`
  link that preserves the deep-linkable child URL.
- **Close/back always returns to the invoking entry** in the parent (focus
  and scroll position), stated once in the pane/sheet footer.
- **Nesting** — a child transcript can contain its own delegated-work rows;
  the same affordance recurses one level at a time.

## Concept A — inline chips + docked side pane

`desktop-a-inline-chips-side-pane.svg` (1440×1000)

Each child appears as a delegated-work row **at its invocation point** in the
parent stream (falling back to a parent-level group when only the parent link
is known). Clicking a row docks a child-transcript pane on the right; the
selected row gets a soft fill and green outline, with a thin connector to the
pane. The parent column stays live and scrollable — nothing is dimmed or
blocked. The session tree also marks `◆ agent` entries, so children are
discoverable from the sidebar too. `✕`/`esc` closes the pane and returns to
the invoking row.

- Strength: parent remains the primary context at all times; reading parent
  and child side by side is the natural review posture.
- Cost: needs ~1200px+ of width; on narrower desktop windows the pane should
  fall back to the mobile sheet.

## Concept B — delegated-work overlay with agent switcher

`desktop-b-delegated-work-overlay.svg` (1440×1000)

The closed state is even quieter: one summary row per delegation point
(`↳ Delegated work · 3 agents`). Opening it raises a centered sheet over the
dimmed parent: a slim **AGENTS rail** on the left lists every child with
title, status glyph, and one meta line; the selected child's transcript fills
the rest. A breadcrumb (`parent title ▸ Delegated work`) keeps the parent
named, and the footer documents the keyboard model (`↑↓` switch agent, `esc`
return to parent, `↵` open standalone).

- Strength: best when one session has many children — switching agents is one
  keystroke and the parent stream carries only one row per delegation point.
- Cost: the parent is dimmed while inspecting, so you cannot read parent and
  child simultaneously; slightly heavier interaction (open group → pick
  agent).

## Mobile — full-screen child sheet

`mobile-child-transcript-sheet.svg` (390×844)

Tapping a delegated-work row slides up a sheet that covers the parent but
leaves a dimmed sliver of it (and the grabber) visible at the top. The header
is the back control — `‹ <parent title>` — so the return target is always
named, and a quiet `‹ 2 / 3 ›` pager steps between sibling agents without
returning to the parent first. Swipe-down or the back control returns to the
invoking entry. This sheet serves both desktop concepts on narrow screens.

## Recommendation

**Ship Concept A**, with the mobile sheet for narrow screens.

- It satisfies the issue's core requirement most directly: inspect a child
  *without leaving the parent* — the parent is never dimmed or interrupted,
  which also makes "closing returns to the invoking entry" trivial (the entry
  never left the viewport).
- Per-invocation rows exploit the origin-event metadata the issue proposes
  collecting, while degrading gracefully to a parent-level group when only
  `parent_native_trace_id` is known.
- It layers naturally on the existing viewer: the pane is a sibling of
  `#content` in `transcript.html`, reusing the sidebar/resizer pattern that
  already exists.

If sessions with many children (5+) become common, adopt Concept B's rail as
the pane header inside A's docked pane — the two compose rather than
conflict.
