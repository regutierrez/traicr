# Agent notes

Use this file when testing Traicr or spinning up a disposable archive. Product language lives in [`CONTEXT.md`](./CONTEXT.md). Install and Docker runbooks live in [`README.md`](./README.md).

## Test environment

A Cloud Agent or local preview needs a running server **and** the scrubbed viewer sessions. `.cursor/start.sh` only starts Docker. It does not start `traicr-server` and it does not import fixtures.

1. Build the embedded UI when `web/static/ui` is missing or the Svelte app changed:

```sh
cd web/app && npm ci && npm run build
```

2. Start the server on port 8080 with a disposable data directory. Do not point `TRAICR_DATA_DIR` at a personal archive:

```sh
export TRAICR_ADMIN_TOKEN=traicr-demo
export TRAICR_DATA_DIR=/tmp/traicr-data
export TRAICR_LISTEN_ADDRESS=:8080
mkdir -p "$TRAICR_DATA_DIR"
go run ./cmd/traicr-server
```

3. When `curl -sf http://127.0.0.1:8080/healthz` succeeds, import the scrubbed sessions from the repo root:

```sh
TRAICR_URL=http://127.0.0.1:8080 TRAICR_ADMIN_TOKEN=traicr-demo go run testdata/sessions/import.go
```

Open <http://127.0.0.1:8080> and sign in with `traicr-demo`. The archive should include:

| Title | Harness | Use it to check |
| --- | --- | --- |
| Check and plan vendoring sesh for herdr | Pi | Long JSONL transcript, tools, thinking |
| Amp transcript support | Amp | Large export, tool rows, hosted-image warnings |
| Codebase cleanup with implementing-pragmatic-code | Claude Code | JSONL cleanup session, `/home/user` paths |

Re-import is safe. The same native IDs update those traces instead of creating a second set. Delete the three traces first only when you need to drop a previous unscrubbed revision from the data dir.

## Which fixtures to use

- [`testdata/sessions`](./testdata/sessions/README.md) — real Pi, Amp, and Claude Code sessions with tokens, personal emails, home paths, and host names replaced. Use these for viewer, search, and layout work.
- [`testdata/harnesses`](./testdata/harnesses/README.md) — small invented records for normalizer and adapter unit tests. Do not replace them with live exports.

Do not import `destination.zip`, collector output from a personal machine, or any file that still contains secrets. To add another real session, scrub it with `testdata/sessions/scrub.py`, then `go test ./test/sessions`.

## Testing

```sh
make test
go test ./test/sessions
```

`go test ./...` covers packages without the `e2e` or `scale` tags. After transcript or session-list UI changes, open the three seeded sessions in the browser and confirm search, filters, and the contents index still work. Do not invent a large synthetic transcript when `testdata/sessions` already has one.

## Cursor Cloud specific instructions

Port 8080 is the published server port. After the environment is up, start `traicr-server` as above if it is not already listening, then run `testdata/sessions/import.go` before judging the viewer. The login token for this disposable flow is `traicr-demo`.
