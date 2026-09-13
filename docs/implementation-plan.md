# Traicr Implementation Plan

This plan delivers the accepted [system design](./design.md) as reviewable vertical slices. Each milestone must leave the repository buildable and tested. Harness adapters are added only after the archive, import, deduplication, and search contracts work end to end.

Milestones 1 through 9 deliver the first-release MVP. Mutation testing begins only after that MVP is complete so feature delivery establishes the behavior and test suite before mutation analysis hardens it.

## Repository layout

This plan records historical milestones. The layout below matches the current tree, not the paths proposed when the plan was written.

```text
cmd/
  traicr/
  traicr-server/
internal/
  archive/
  collector/
    adapters/
  config/
  domain/
  normalize/
  search/
  server/
  store/
  version/
migrations/
web/
  static/
  templates/
testdata/
  collector/
  harnesses/
test/
  e2e/
  scale/
scripts/
```

`internal/domain` owns Trace ZIP and normalized Event types. Harness packages depend on those types; the domain does not depend on harness packages.

The collector and server use one Go module at `github.com/regutierrez/traicr`. Collector builds must not require CGo so GitHub Actions can cross-compile standalone macOS and Linux binaries. The SQLite dependency must be verified for read-only database access, backup-safe snapshots, FTS5, and the trigram tokenizer before it is selected.

## Milestone 1: Go foundation

### Work

- Create the Go module and both command entry points.
- Add deterministic version metadata for development and release builds.
- Establish package boundaries shown above.
- Add formatting, static analysis, unit-test, and race-test commands.
- Add GitHub Actions for Linux tests and the macOS/Linux amd64/arm64 build matrix.
- Add a multi-stage Docker build for `traicr-server` running as a non-root user.
- Add a development Docker Compose file with one `/data` volume and required admin token.
- Keep server templates, migrations, and static files explicit in the image layout.

### Acceptance

- `go test ./...` passes.
- Both applications build on Linux.
- Collector cross-builds for all four target platform combinations.
- The server image starts, requires an admin token, writes only under `/data`, and answers `/healthz`.
- No harness behavior is implemented yet.

## Milestone 2: Domain and Trace ZIP contract

### Work

- Define versioned manifest, descriptor, source-machine, file, warning, and Import Report types.
- Define normalized Trace, Trace Revision, Event, observation, repository, branch, child-trace, model, tool, and attachment types.
- Implement canonical SHA-256 revision digest calculation.
- Implement streaming ZIP creation with a soft 2 GB split target and ZIP64 support.
- Implement strict archive validation and bounded extraction.
- Implement source-machine configuration and hostname observation.
- Implement Collection State without harness-specific fields.
- Produce synthetic Trace ZIP fixtures with text, attachments, branches, child traces, unknown records, and malformed variants.

### Tests

- Manifest and descriptor JSON round trips
- Stable digest despite ZIP timestamp, order, and compression differences
- Archive splitting without splitting a trace
- Empty, corrupt, truncated, oversized, and unsupported archives
- Path traversal, absolute path, duplicate path, link, digest mismatch, and expansion-limit rejection
- Collection State advances only from successful Import Report outcomes
- Lost Collection State causes recollection without changing trace identity

### Acceptance

- A synthetic collector can create independently importable numbered Trace ZIPs.
- Unsafe containers are rejected before any trace is committed.
- Unknown source formats remain valid Source Records.

## Milestone 3: Storage and import vertical slice

### Work

- Add ordered SQLite migrations for the tables in the design.
- Enable WAL, foreign keys, busy timeout, and explicit transaction boundaries.
- Implement content-addressed object writes using temporary files and atomic rename.
- Implement per-trace import transactions and Import Reports.
- Implement trace, revision, Source Record, Event, observation, and repository deduplication.
- Implement branch and Child Trace relationships.
- Implement streamed `POST /api/v1/imports` responses using newline-delimited JSON.
- Implement one bounded import writer while allowing concurrent reads.
- Implement deletion and mark-and-sweep object cleanup.
- Add one synthetic Harness Normalizer to prove the server boundary.

### Tests

- Exact ZIP retry produces `unchanged` outcomes and no duplicate rows or objects.
- The same trace from two machine IDs remains one trace.
- A stale revision imported later cannot remove Events.
- Conflicting observations remain attached to one logical Event.
- One malformed trace does not roll back valid peers.
- Interrupted object writes and database commits leave recoverable state.
- Hard deletion preserves shared objects and removes unreferenced objects.
- HTTP disconnect and retry remain idempotent.

### Acceptance

- A synthetic Trace ZIP can be uploaded, stored, normalized, retried, inspected, and deleted through the API.
- The response streams phase and per-trace progress on the original upload request.
- Memory use is bounded by buffers and one active trace rather than archive size.

## Milestone 4: Search and minimal Web UI

### Work

- Split human-readable Event text into bounded overlapping chunks.
- Add FTS5 full-text and trigram indexes.
- Implement full-text, exact, and Go regular-expression search modes.
- Add indexed filters for harness, model, source machine, repository, date, role, Event kind, and tool.
- Add stable cursor pagination, result snippets, and cancellation.
- Implement bearer-token API authentication.
- Implement token login, signed browser sessions, CSRF protection, and logout.
- Add initial Go templates and HTMX interactions for search and trace viewing.
- Add Imports, Source Machines, Source Record inspection, and hard-deletion pages.

### Tests

- Terms, phrases, code identifiers, punctuation, paths, and multiline exact matches
- Regular expressions with and without extractable literal candidates
- Matches crossing text-chunk boundaries
- Binary and base64 payload exclusion with metadata inclusion
- Filter combinations and stable pagination during concurrent imports
- HTML escaping, unauthorized requests, invalid sessions, CSRF rejection, and deletion confirmation
- Duplicate observations collapse into one logical search result

### Acceptance

- A user can find an Event using each search mode, filter it, open its trace, inspect Source Records, and delete the trace.
- Searches remain usable while an import writes through WAL.
- The UI requires no Node.js toolchain or separate frontend service.

## Milestone 5: First real harness vertical slice

Implement Pi first because its session graph, model changes, tool records, compactions, and JSONL format are publicly documented. It exercises most normalized concepts without beginning with a private database format.

### Collector

- Discover default and configured Pi session directories.
- Prefer documented machine-readable export behavior.
- Preserve the session header, entry IDs, parent IDs, branches, messages, model changes, tool calls, results, compactions, images, and unknown extension records.
- Detect incomplete tails and emit warnings.

### Normalizer

- Reconstruct logical order and branches from IDs rather than file order.
- Link tool calls and results.
- Emit message, reasoning, model, tool, compaction, attachment, and Child Trace data.
- Produce stable native Event keys.

### Acceptance

- A sanitized Pi fixture passes collect, archive, upload, normalize, search, view, retry, revision update, and deletion tests.
- Unknown Pi records survive import and appear in Source Record inspection.

## Milestone 6: Supported-export adapters

Add one adapter at a time. Each adapter includes discovery, collection, Source Record fixtures, a Harness Normalizer, warning behavior, golden normalized output, and end-to-end import tests before the next starts.

### OpenCode

- Discover sessions with `opencode session list --format json`.
- Export each changed session with `opencode export`.
- Preserve session parents, messages, typed parts, reasoning, tools, attachments, costs, tokens, diffs, todos, and unknown part types.
- Treat CLI and desktop as the same source family.

### Amp

- List authenticated threads with `amp threads list --json`.
- Export full thread payloads with `amp threads export`.
- Preserve thread identity, messages, tool blocks, attachments, model metadata, subagents, compaction summaries, and unknown content blocks.
- Report authentication and remote-access errors without inspecting Amp cache logs.

### Codex

- Use the installed Codex app-server protocol and version-matched schemas.
- List and page thread, turn, and item projections.
- Preserve forks, parent threads, subagents, compactions, models, reasoning settings, tools, usage, Git context, and unknown items.
- Keep raw rollout-file parsing isolated as a tested fallback.

### Grok Build

- Snapshot the complete native session directory (`grok-native-session`). Do not run `grok export`.
- Preserve the official session ID, summaries, updates, branches, worktree context, tools, attachments, compactions, and subagent relationships.
- Never conflate Grok Build with another harness using a Grok model.

### Acceptance

- Every adapter has at least one complete fixture and malformed or newer-version fixture.
- Missing executables, unauthenticated commands, non-zero exits, timeouts, and changing sessions produce actionable warnings.
- No adapter invokes a shell with trace-derived arguments.

## Milestone 7: Local-storage adapters

### Claude Code

- Discover project session JSONL under supported Claude locations.
- Snapshot complete newline-terminated records while tolerating asynchronous lag.
- Preserve UUID and parent graphs, branches, rewinds, child agents, API content blocks, model metadata, tool IDs, attachments, auxiliary references, and unknown records.
- Avoid treating global prompt history as a complete transcript.

### Cursor agent

- Discover current JSONL transcripts and companion session metadata.
- Treat transcript and metadata as one revision.
- Tolerate partial tails and metadata/transcript update races.
- Preserve original structured content rather than reducing it to text.

### Cursor editor

- Discover global and workspace storage on macOS and Linux.
- Open SQLite read-only or use the backup API so WAL data is handled correctly.
- Extract only relevant keys and rows with original table, key, and value.
- Implement separately tested normalizers for each supported Chat and Composer generation.
- Report unknown keys and schema generations rather than guessing roles or ordering.

### Acceptance

- Live database tests prove that collection does not mutate Cursor storage.
- Ordering, role, model, tool, diff, and context reconstruction are fixture-tested for each supported generation.
- Schema changes produce partial or unsupported outcomes without Source Record loss.

## Milestone 8: Renormalization and operational hardening

### Work

- Persist normalizer names and versions with every run.
- Detect outdated revisions after deployment.
- Rebuild one revision at a time in a background worker.
- Atomically replace normalized observations only after successful completion.
- Keep previous Events when a rebuild fails.
- Add structured logs that exclude tokens and Source Record bodies.
- Add import, normalizer, search, deletion, and garbage-collection diagnostics to the UI.
- Add configurable archive safety limits and graceful shutdown.
- Exercise direct Tailscale HTTP deployment and later Caddy proxy headers without adding TLS to Traicr.

### Tests

- Version increase queues only affected harness revisions.
- Search remains available and unchanged until an atomic replacement succeeds.
- Failed rebuild preserves previous Events.
- Restart safely resumes pending renormalization without duplicating work.
- Security headers, proxy mode, cookie mode, and trusted forwarded headers behave as configured.

### Acceptance

- A normalizer bug can be fixed and every retained revision rebuilt without recollection.
- Deployment remains one container and one volume.

## Milestone 9: Scale and release

### Scale verification

- Generate synthetic corpora with millions of Events and representative text sizes.
- Exercise multi-gigabyte ZIP creation, upload, validation, normalization, and deletion.
- Confirm process memory does not grow linearly with archive size.
- Measure database and FTS growth, import throughput, full-text latency, exact-search latency, and uncached regex behavior.
- Verify WAL checkpoints do not cause long search outages.
- Document expected disk overhead and operational limits from measured results.

### Release

- Build standalone collector binaries for macOS amd64, macOS arm64, Linux amd64, and Linux arm64.
- Publish SHA-256 checksums with every release.
- Publish the server image and an example Docker Compose file.
- Document Tailscale-only HTTP binding, Caddy proxying, token rotation, data-volume ownership, and upgrades.
- Keep collector upgrades manual; do not add an auto-updater.

### Acceptance

- A fresh macOS or Linux machine can download one collector binary, collect a supported trace, and upload it without Go or another runtime.
- A fresh Debian server can start Traicr from Docker Compose with one token and one volume.
- The complete adapter, import, deduplication, search, UI, deletion, and renormalization test suite passes.

## Milestone 10: Post-MVP mutation testing

### Work

- Select and pin a Go mutation-testing tool, then commit its configuration and direct developer commands.
- Mutate production Go code while continuing to express permanent checks as ordinary `*_test.go` behavior tests.
- Establish and review a baseline across the unit-test packages without rebuilding the Docker end-to-end environment for every mutant.
- Classify surviving mutants as missing behavior checks, equivalent mutations, unnecessary production logic, or tool limitations.
- Add or strengthen ordinary tests for meaningful survivors and simplify production code when a survivor exposes unnecessary logic.
- Run diff-scoped mutation analysis on pull requests and periodic full analysis separately from the ordinary unit and end-to-end suites.
- Introduce mutation-score enforcement only after the baseline is reviewed, using a threshold that prevents regression rather than an arbitrary target.
- Keep generated mutants, temporary worktrees, and reports out of version control.

### Acceptance

- A documented, reproducible command runs mutation analysis from a clean checkout with the pinned tool version.
- Every surviving mutant in security, archive validation, import, deduplication, authentication, and search decision logic is either killed by a behavior test or explicitly classified.
- CI reports mutation results for changed Go code within an acceptable runtime.
- The enforced mutation threshold is derived from a reviewed baseline and does not require Docker end-to-end execution for each mutant.

## Fixture and compatibility policy

- Never commit personal, proprietary, or secret-bearing real traces.
- Reduce observed formats into the smallest sanitized fixture that preserves structure and edge cases.
- Record the harness version and source format represented by every fixture.
- Preserve unknown fields in Source Records and ignore them only in normalized projections.
- Add a fixture before fixing any format regression.
- Treat private storage layouts as versioned compatibility adapters, not permanent contracts.
- Keep the `0xSero/ai-data-extraction` project as research material only; do not copy its inconsistent output schema or silent-error behavior.

## Definition of done

The first release is complete when:

- All requested harness families collect on macOS and Linux.
- Every collector writes valid, independently importable Trace ZIPs.
- Repeated and cross-machine imports never create duplicate logical traces or Events.
- Branches and Child Traces remain navigable.
- Source Records survive unsupported and failed normalization.
- Full-text, exact, and regex search work with all accepted filters.
- The Web UI exposes search, traces, Source Records, imports, machines, and deletion.
- The API is authenticated and versioned.
- The Docker deployment uses one process and one persistent volume.
- Standalone collector releases require no Go installation.
- Tests cover malformed input, security boundaries, format evolution, live-file races, retries, and scale behavior.
