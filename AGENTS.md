# Traicr agent guide

Use this guide to find what is wrong when a transcript renders badly or loads slowly. Read `CONTEXT.md` for the domain words (Trace, Event, Revision, Harness).

Every `/api/v1` call needs `Authorization: Bearer $TRAICR_TOKEN`. `traicr login` saves the server URL and admin token in `~/.config/traicr/collector.json` (`server_url`, `token`). If neither that file nor `TRAICR_TOKEN` exists, ask the user for the token. Never print the token.

## Run a local server

```sh
(cd web/app && npm ci && npm run build)   # the Go binary embeds web/static/ui
TRAICR_ADMIN_TOKEN=local-dev-token TRAICR_DATA_DIR=/tmp/traicr-dev \
TRAICR_LISTEN_ADDRESS=127.0.0.1:8080 go run ./cmd/traicr-server
```

Load traces into it with `traicr collect --output /tmp/zips` and `curl -H "Authorization: Bearer local-dev-token" -H 'Content-Type: application/zip' --data-binary @<zip> http://127.0.0.1:8080/api/v1/imports`. On the live server, only read: do not call `DELETE /api/v1/traces/{id}` or `POST /api/v1/imports` unless the user asks.

## Read status and logs

- `GET /api/v1/status`: imports per machine, trace and event counts, storage.
- `GET /api/v1/logs?level=warn`: the newest 1,000 server log records, kept in memory until restart. Useful records:
  - `request`: every HTTP request with `route`, `path`, `status`, `duration_ms`, `bytes`, and `trace_id`. It is a warning when it took 1 second or more, or failed with a 5xx.
  - `transcript load`: what a browser measured loading a trace: `fetch_ms`, `adapter_ms`, `pages`, `events`. It is a warning at 1 second or more.
  - `transcript anomaly`: one record per anomaly kind the transcript adapter found, with `kind`, `count`, `message`, and sample `event_ids`. `off_path` is info, because sessions branch normally.
- `GET /api/v1/openapi.json` describes every operation.

## Inspect a trace as the UI renders it

```sh
cd web/app && npm run --silent inspect-trace -- <trace-id> [--revision <id>] [--json] [--full]
```

It reads the same pages the transcript page reads (`/api/v1/traces/{id}/transcript` for Amp, `/api/v1/traces/{id}/events` for other harnesses), runs the same adapter in `src/lib/transcript/`, and prints load timing, anomalies, and the shown path turn by turn. Each line starts with the event IDs behind it; open one with `GET /api/v1/events/{id}/sources` to see the original Source Record. Set `TRAICR_URL` and `TRAICR_TOKEN` to inspect a server other than the saved one.

Anomaly kinds are defined in `AnomalyKind` in `web/app/src/lib/transcript/types.ts`. In the browser, add `?debug=1` to a trace URL to show the same timing and anomalies above the transcript.

## Find broken traces across the archive

```sh
cd web/app && npm run --silent audit-traces -- [--harness <name>] [--limit <n>] [<trace-id>...] > /tmp/audit.jsonl
```

It runs the same adapter over every trace (or the ones you name) and prints one JSON line per trace: title and working directory present, normalization status, event and branch counts, path length, empty or repeated rows, tool results the page hides, anomalies, and timings. The last line is a summary per harness. Run it against two servers, one with the old build and one with the new, to compare a parser or adapter change across real traces.

## Take a screenshot of the UI

With `agent-browser` and a named session:

```sh
export AGENT_BROWSER_SESSION=traicr-ui
agent-browser open http://127.0.0.1:8080/login
agent-browser snapshot -i                      # find the Admin token box and the Unlock button
agent-browser fill @e2 "$TRAICR_TOKEN" && agent-browser click @e3
agent-browser open "http://127.0.0.1:8080/traces/<trace-id>?debug=1"
agent-browser wait '.stream .row' --timeout 120000
agent-browser screenshot /tmp/trace.png
```

`agent-browser eval "Math.round(performance.now())"` after the wait gives the time from navigation to the first painted rows. Big transcripts paint their first 150 rows at once and add the rest in chunks on later frames, so wait until `document.querySelectorAll('.stream .row').length` stops growing before you search the page or count rows.

## Measure transcript loading at scale

```sh
go test -tags scale -count=1 -run TestAmpTranscriptLoadAtScale -v ./test/scale
```

It imports one Amp trace of about 10,000 events (`TRAICR_SCALE_TRANSCRIPT_EVENTS`) and logs the time to page through its transcript.
