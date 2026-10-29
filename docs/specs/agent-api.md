# Agent API

## Problem Statement

I want any agent to manage my Trace Archive: find and read Traces, rename and delete them, check that imports are healthy, and read what the server logged. Most of this already exists under `/api/v1`, but an agent cannot use it without reading Go handlers. Nothing documents the parameters, pagination, or error shapes. There is no single answer to "is the archive healthy?". Server logs go only to the container's stdout.

## Solution

`/api/v1` is the product (ADR 0011). The server publishes its own contract as an OpenAPI document, and gains two read endpoints: `status` and `logs`. A Traicr Executor app turns that document into agent tools with Executor's OpenAPI helper and asks for my approval before every delete. Auth stays the existing admin bearer token.

```
                 GET /api/v1/openapi.json  (public, describes shapes only)
┌──────────────────────────────┐ ─────────────────────────▶ ┌──────────────────────────┐
│ traicr-server                │                            │ Executor app "traicr"    │
│ /api/v1 + status + logs      │ ◀───────────────────────── │ liveOpenapiRouter        │
│                              │   Bearer TRAICR_ADMIN_TOKEN│ + approval on DELETE/POST│
└──────────────────────────────┘                            └────────────┬─────────────┘
                                                                         │ MCP
                                                                  any agent
```

The `traicr` CLI subcommands are a later consumer of the same contract and are not part of this spec.

## User Stories

1. As an agent, I fetch one document and know every endpoint, parameter, response, and error code without reading source code.
2. As an agent, I list Traces filtered by Harness, Repository, Source Machine, model, or date, newest first, and page with a cursor.
3. As an agent, I search Events with full-text, exact, or regex mode and open the matching Trace.
4. As an agent, I read a Trace's metadata, page through its Events, and inspect the Source Records behind an Event.
5. As an agent, I rename a Trace or clear its manual title.
6. As an agent, I delete a Trace, and Executor asks the owner to approve each delete first.
7. As an agent, I list recent imports and can tell a complete import from one that was interrupted.
8. As an agent, I call one endpoint and learn when each Source Machine last imported, whether its latest import had failures, how big the archive is, and whether titles are falling behind.
9. As an agent, I read recent server log records filtered by level and time, including import failures and title-worker pauses, and never see a token or API key.
10. As the owner, collector login and upload keep working exactly as before.

## Implementation Decisions

### Contract: OpenAPI document

- Hand-write one OpenAPI 3.1 JSON document, embed it in the server binary, and serve it at `GET /api/v1/openapi.json`.
- The route is **unauthenticated**. It describes request and response shapes, not archive data, and Executor's `liveOpenapiRouter` fetches a `source.url` without credentials.
- OpenAPI was chosen over a markdown reference because Executor generates the tools from it. One file is both the agent doc and the tool definitions, so a second description cannot drift.
- Describe every authenticated `/api/v1` operation. Leave out the two public routes: the browser-only `GET /api/v1/csrf` and `GET /api/v1/openapi.json` itself.
- Include request and response examples, including a paginated list and an upload stream that ends in `failed`.
- Give each operation a tag and an `operationId`. Executor names tools `<tag>.<operationId>`, for example `traces.listTraces`, `traces.deleteTrace`, `imports.listImports`, `status.getStatus`, `logs.listLogs`.
- Declare one security scheme, `adminToken` (`type: http`, `scheme: bearer`), on every operation.
- Document the shared conventions once, in the document's `info.description`:
  - **Auth:** `Authorization: Bearer <TRAICR_ADMIN_TOKEN>`. Without a valid token, the request gets `401`. The one exception: a `GET` or `HEAD` that carries a valid browser session.
  - **Pagination:** only `listTraces`, `searchEvents`, `listTraceEvents`, `listImports`, and `listLogs` page. They take `limit` (1–200, default 50) and an opaque `cursor`, and return `next_cursor` only when there is another page. `listMachines`, `listEventSources`, and `listRevisionSources` return plain arrays.
  - **Errors:** `{"api_version":1,"_tag":"…","message":"…","error":{"code":"…","message":"…"}}`. List the codes per operation: `invalid_id`, `invalid_limit`, `invalid_query`, `invalid_json`, `invalid_title`, `content_type`, `not_found`, `transcript_changed`, `import_busy`, `upload_failed`, `cancelled`, `internal_error`, `unauthorized`.
  - **Upload stream:** after the body is received, `POST /api/v1/imports` answers `200` with NDJSON progress records: `validating`, then `normalizing` records, then exactly one final record.
    - The final record is either `complete`, which carries the Import Report (that report can still list failed Traces), or `failed`, which carries an `error` string.
    - Validation and import failures arrive as this `failed` record, not as the error envelope.
  - **Dates:** `after` and `before` accept `YYYY-MM-DD` or RFC 3339.
- Keep the existing `error` object. The collector and the web UI already read it.
- Repeat the code as a top-level `_tag` and the message as a top-level `message`. Executor matches a documented error only when the response body carries a `_tag` equal to the documented code; otherwise it shows a generic "HTTP 404" failure. A description in the document alone does not help, because Executor checks the body against the documented schema.
- Document each code as its own schema whose `_tag` is a `const`. A status with several codes lists them with `anyOf`.

### Existing endpoints, now documented

| Tag | Operation | Route | Notes for the doc |
| --- | --- | --- | --- |
| traces | `listTraces` | `GET /api/v1/cards` | One row per Trace, newest `updated_at` first. Filters: `q`, `mode`, `harness`, `model`, `machine` (Source Machine ID), `repository` (remote or local path), `after`, `before`, `role`, `kind`, `tool`. A date filter matches Traces with any Event in the range. A Child Trace is listed under its top collected ancestor; a match in it sets `matched_child`. |
| traces | `getTrace` | `GET /api/v1/traces/{id}` | Includes revisions, parents, children. |
| traces | `listTraceEvents` | `GET /api/v1/traces/{id}/events` | `cursor` or `key` (start at an Event key), `limit`. |
| traces | `renameTrace` | `PATCH /api/v1/traces/{id}` | Body `{"title_override": "…"}`. An empty string clears it. |
| traces | `deleteTrace` | `DELETE /api/v1/traces/{id}` | Permanent. Returns `204`. |
| search | `searchEvents` | `GET /api/v1/search` | Same filters as `listTraces`. Returns Events with `trace_id`. |
| events | `listEventSources` | `GET /api/v1/events/{id}/sources` | |
| revisions | `listRevisionSources` | `GET /api/v1/revisions/{id}/sources` | |
| revisions | `getRevisionFile` | `GET /api/v1/revisions/{id}/file` | `path` is required. `preview=1` returns JSON (first 1 MiB). Otherwise returns the original bytes. |
| imports | `listImports` | `GET /api/v1/imports` | Counts only; per-Trace outcomes are in `getImport`. |
| imports | `getImport` | `GET /api/v1/imports/{id}` | |
| imports | `uploadTraceZip` | `POST /api/v1/imports` | `application/zip` in, NDJSON progress out. |
| machines | `listMachines` | `GET /api/v1/machines` | `last_seen` is the start time of the machine's latest import. |

- `listTraces` keeps the `/cards` path. A new `/traces` alias would add a second name for the same handler, and agents see the operation name, not the path.
- **Additive change:** import reports gain `"complete": bool`. Today an import that started but never finished (interrupted, or still running) has no `report_json`, so its report shows all zero counts and looks like an empty success.
  - The list and detail readers set `complete` from whether `report_json` is present, after decoding.
  - Readers never trust a stored value. Older reports were written without the field and must still read as `complete: true`.
  - The final `complete` record of the upload stream also carries `complete: true`.

### `GET /api/v1/status`

The response is one JSON object, built from cheap queries. It does not walk the filesystem.

```json
{
  "version": "…",
  "running_import_id": 0,
  "machines": [
    {
      "id": "…", "hostname": "…",
      "last_import_at": "…", "last_import_id": 42,
      "last_import_complete": true, "last_import_failed_traces": 0
    }
  ],
  "imports": { "total": 120, "with_failed_traces": 3, "incomplete": 1 },
  "traces": 2400,
  "events": 1800000,
  "titles": { "enabled": true, "pending": 12 },
  "storage": { "database_bytes": 0, "objects_bytes": 0 }
}
```

- `version` comes from `version.CurrentBuildInfo()`.
- `running_import_id` is the `imports` row the store is processing right now. It is omitted when no row is active.
  - The store sets this ID when `beginImport` commits the row and clears it when `Import` returns. One lock covers both steps and the status read.
  - The upload slot alone cannot be used, because it is also held while a ZIP is still being received and validated, before any row exists.
- `machines[].last_import_*` comes from each machine's newest `imports` row: group by `machine_id`, take `MAX(id)`, then join. Do it in one grouped query, not one query per machine.
- `imports.with_failed_traces` counts reports whose `failed` is greater than 0.
  - Read it with `json_extract(CAST(report_json AS TEXT), '$.failed')`. The column holds JSON text stored as a BLOB, and the cast makes that explicit, so SQLite cannot mistake it for JSONB.
- `imports.incomplete` counts rows without `report_json`, except the active row.
  - Read the active ID and count under the same lock that covers import start and finish. Reading the ID before and after the count is not enough: an import can start and finish entirely between the two reads.
  - An older interrupted row still counts while a new upload is running.
- Uploads rejected during validation never create an `imports` row. They show up only in logs (below).
- `titles.enabled` is true when `TRAICR_TITLE_API_URL` is set.
- `titles.pending` counts Traces with no manual or collected title whose latest revision has not been named yet. It uses the same condition as `TitleCandidates`, without the 30-minute idle cutoff.
  - Worker pauses are not stored as state. They appear in logs as `trace naming paused`.
- `storage.database_bytes` is the size of `traicr.db` plus `traicr.db-wal`.
- `storage.objects_bytes` is `COALESCE(SUM(size),0)` over `source_objects`. This is the recorded size of stored objects, not total disk usage.

### `GET /api/v1/logs`

- Change `cmd/traicr-server/main.go` to wrap the JSON handler in a small `slog.Handler`. That handler:
  1. Redacts secrets.
  2. Writes to stdout, as today.
  3. Appends the record to an in-memory ring buffer of the last 1,000 records.
- **Stop logging provider messages.** The main defense is not logging text that can contain a key.
  - Today the title worker logs the provider's error message verbatim (`trace naming paused`, `"error": err.Error()`). Some providers repeat a masked or partial key in that message, and `errorMessage` cuts it to 300 characters, which can split a key.
  - Exact-value replacement cannot catch either case.
  - Change that log line to safe fields only: `error_type`, the HTTP `status` when the error is a `RequestError`, and `retry_in`. This changes an existing line's attributes. It is not a new line.
- **Redaction, as a backup:** before a record reaches stdout or the buffer, replace every occurrence of `TRAICR_ADMIN_TOKEN` and `TRAICR_TITLE_API_KEY` with `[redacted]`.
  - Skip empty secrets. The title key is optional.
  - Cover the message and every attribute value after `Resolve`. Turn `error` and `fmt.Stringer` values into strings first; `routes.go` already logs `"error", err`.
  - Cover values inside groups.
  - The handler keeps working through `WithAttrs` and `WithGroup`. Pre-bound attributes are redacted when they are bound, and keep their group placement.
  - Derived handlers share one locked buffer and counter, but never change each other's bound attributes.
  - Redaction happens in the shared handler, so stdout gets it too.
- **Buffer:** under one lock, assign `seq` (it increases by one per record) and append the record. Store copied, already-redacted values, never references to the caller's data.
- Each record is returned as `{"seq", "time", "level", "message", "attrs"}`.
- Query parameters:
  - `level`: minimum level, one of `info`, `warn`, `error`. Default `info`. The JSON handler drops `debug` records before capture, so `debug` is not offered.
  - `since`: RFC 3339.
  - `limit`: 1–200, default 50.
  - `cursor`: opaque.
- Records come newest first. Filters apply before `limit`. `next_cursor` continues strictly below the last returned `seq`.
- The cursor includes a random ID created when the process starts. After a restart, an old cursor gets `400 invalid_query` instead of returning unrelated new records.
- The buffer is lost on restart. `docker compose logs` stays the long-term record.
- A persisted log table was rejected. Every log line would become a database write that needs its own retention and cleanup, which is too much for a one-person archive.
- **New log line:** `POST /api/v1/imports` logs `import rejected` at `warn` level, with the validation error, when `archive.Validate` fails. Today those failures reach only the uploading client.
- Do not add other new log lines in this spec.

### Executor app

- The app source lives at `integrations/executor/traicr/` in this repo, so a contract change and its consumer change ship in the same PR. It is deployed with the `executor apps` CLI, like the other homelab apps.
- `liveOpenapiRouter`:
  - `source: { url: "<TRAICR_URL>/api/v1/openapi.json" }`
  - `allowedOrigin` set to the Traicr origin
  - `securitySchemes: ["adminToken"]`
  - one `secrets` account method whose `token` field fills `adminToken`
- The admin token is entered through Executor's account connect flow. It is never stored in the app source.
- Approvals, using `withApprovals`:
  - `traces.deleteTrace` and `imports.uploadTraceZip`: `always()`
  - every other tool, including `traces.renameTrace`: no approval
- The app has no custom tools. If a generated tool turns out wrong, fix the OpenAPI document, not the app.

### Auth

No changes. All of this uses the existing `api()` middleware: the admin bearer token, or a browser session for `GET`. Scoped tokens are deferred (ADR 0011).

### Test seams

1. **HTTP handler (`NewHTTPHandler`)**: status, logs, `complete` on import reports, the public OpenAPI route.
2. **Logging handler**: records in, stdout bytes and buffer contents out.
3. **The OpenAPI document vs. the route table**: a drift check.

## Testing Decisions

- **Contract drift:** a server test fails when the route table and `openapi.json` disagree. The check runs in both directions:
  - every `/api/v1` method and path pattern registered on the mux has an operation in the document, except the public `GET /api/v1/csrf` and `GET /api/v1/openapi.json`;
  - every operation in the document is registered.
  - Expected seam: a package-private slice of `/api/v1` routes in `routes.go` that registration actually uses and the test reads. This checks operation coverage only, not response schemas.
- **Document validity:** the document parses as OpenAPI 3.1, and every operation has a tag, an `operationId`, the `adminToken` security requirement, and at least one documented error response.
- **Status:** seed two machines through `Store.Import`:
  - one whose latest import has a failed Trace;
  - one with an older failure but a clean latest import;
  - plus an `imports` row without `report_json`.
  - Assert per-machine `last_import_failed_traces` (the second machine must read 0, not its older failure), `imports.with_failed_traces`, and `imports.incomplete`.
  - Assert that the old interrupted row still counts as `incomplete` while another import is active, and that the active row does not count.
  - Assert `complete: false` for the interrupted row in `GET /api/v1/imports`. Assert `complete: true` for a stored report written before the field existed.
- **Logs:**
  - Log a record that contains both secrets in its message, in a nested group attribute, in an `error` value, and in a pre-bound `WithAttrs` attribute. Assert that neither value appears in `GET /api/v1/logs` or in the stdout bytes, and that `[redacted]` does.
  - With an empty title key, assert that ordinary text is unchanged.
  - Make the title worker pause on a provider error whose message contains a masked key fragment. Assert the fragment does not appear in the logs.
  - Assert `level=warn` drops `info` records.
  - Assert paging past the 1,000-record buffer does not repeat or invent records.
  - Assert a cursor from another process instance gets `400`.
- **Rejected upload:** an invalid ZIP produces an `import rejected` record in `GET /api/v1/logs`.
- **Auth:**
  - `GET /api/v1/status` and `GET /api/v1/logs` return `401` without a token.
  - `GET /api/v1/openapi.json` returns `200` without one.
- **Compatibility:** existing collector upload and `traicr traces rename` tests pass unchanged.
- **Executor app:** verified by hand once, with a live run of the issue #38 workflow:
  1. Search, then read the matching Trace's Events.
  2. Rename the Trace, with no approval prompt.
  3. Delete a Trace: the call stops at an approval, then runs after approval.
  4. List recent imports.
  5. Call `status` and `logs`.
  - No committed tests for the app.
- Prior art: `routes_test.go`, `auth_boundary_test.go`, `routes_boundary_test.go`.

## Out of Scope

- Scoped, revocable, or per-agent API tokens.
- A Traicr-native MCP server.
- New `traicr` CLI subcommands (follow-up issue).
- Tags, collections, pin or archive, bulk operations, soft delete, undo.
- Per-Trace processing logs or a persisted log table.
- Recording rejected uploads as `imports` rows.
- Title-worker pause state in `status` (use logs).
- Removing or renaming fields of the error envelope, or renaming `/cards`.

## Further Notes

Implementation order:

1. Route list slice, OpenAPI document, public route, and drift test.
2. `complete` on import reports, then `GET /api/v1/status`.
3. Safe fields on the title worker's pause log, redacting log handler, ring buffer, `GET /api/v1/logs`, and the `import rejected` line.
4. Executor app.

To confirm before step 4:

- Executor must be able to reach the Traicr origin, the same way it reaches the other homelab apps.
- Executor's OpenAPI error mapping needs a top-level `_tag` and `message` (settled above; checked with Executor's own matcher from `apps@0.0.1-beta.92`).

Issue: #38. ADR: 0011. Glossary: Trace, Event, Source Record, Source Machine, Import, Import Report, Harness, Repository.
