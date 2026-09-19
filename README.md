# Traicr

Traicr collects, preserves, and searches one person's AI coding traces across multiple macOS and Linux machines.

The client collects and uploads traces. The server imports, stores, and searches them.

## Planned sources

- Amp
- Cursor editor and `cursor-agent`
- Claude Code
- Codex
- Pi
- Grok Build
- OpenCode

## Shape

Traicr has two Go applications:

- A manually run collector installed on each source machine
- A Docker-hosted server with a search API and Web UI

Collectors create lossless Trace ZIPs. The server retains their native Source Records, derives common searchable Events, deduplicates sessions across machines and revisions, and indexes text in SQLite.

## Installation

### Client with Go

Install Go 1.27 or newer, then install the collector on each source machine:

```sh
go install github.com/regutierrez/traicr/cmd/traicr@latest
```

Go installs `traicr` into `$GOBIN`, or `$GOPATH/bin` when `GOBIN` is unset (normally `$HOME/go/bin`). Add that directory to your `PATH`.

To install from a local checkout into `$HOME/.local/bin` instead:

```sh
GOBIN="$HOME/.local/bin" go install ./cmd/traicr
```

Run this command from the repository root and make sure `$HOME/.local/bin` is on your `PATH`.

### Server with Docker

With Docker installed, clone the repository and build the server image:

```sh
git clone https://github.com/regutierrez/traicr.git
cd traicr
docker build -t traicr-server .
```

Set a strong admin token without putting its value in shell history (Bash):

```bash
read -rsp 'Traicr admin token: ' TRAICR_ADMIN_TOKEN; echo
export TRAICR_ADMIN_TOKEN
```

Start the server with a persistent data volume:

```sh
docker run -d --name traicr-server \
  --restart unless-stopped \
  --read-only \
  --security-opt no-new-privileges:true \
  -p 127.0.0.1:8080:8080 \
  -e TRAICR_ADMIN_TOKEN \
  -e TRAICR_DATA_DIR=/data \
  -e TRAICR_LISTEN_ADDRESS=:8080 \
  -e TMPDIR=/data/tmp \
  -v traicr-data:/data \
  traicr-server
curl http://127.0.0.1:8080/healthz
```

Open <http://127.0.0.1:8080> in your browser. The port is available only on this machine. For collectors on other machines, bind to a private network address or use an HTTPS reverse proxy. Do not expose plain HTTP to the public internet. The `traicr-data` volume contains unredacted transcripts; keep it when replacing the container.

### Connect and upload

With the same `TRAICR_ADMIN_TOKEN` set in the client shell:

```sh
traicr login http://127.0.0.1:8080
traicr collect --output ./traces
traicr upload ./traces/*.zip
```

Replace the URL with your server's address when it runs on another machine. Run `upload` only when collection produces ZIP files. Collection and upload are manual; `go install` does not set up a background service or schedule. Use cron or another scheduler for automatic uploads.

### Saved session metadata

The collector uses the latest saved Pi session name (from `/name` or extensions such as `pi-rename`) as the transcript title. UUIDs remain the session IDs. Traicr does not generate names or change Pi session files.

To repair missing titles from an older collector, first update both the client and server, then recollect Pi sessions, including acknowledged revisions:

```sh
traicr collect --harness pi --all --output ./pi-title-backfill
traicr upload ./pi-title-backfill/*.zip
```

Use a new output directory and run `upload` only if ZIP files were created. The server fills missing titles without creating duplicate revisions. It preserves existing titles and does not restore names from stale revisions. Sessions without a saved name still use their ID as the display fallback.

Claude Code collection uses the last non-empty `ai-title` record as the title and the first absolute `cwd` in the session records as the working directory. Recollecting acknowledged Claude sessions backfills either missing field without replacing existing or newer metadata:

```sh
traicr collect --harness claude-code --all --output ./claude-metadata-backfill
traicr upload ./claude-metadata-backfill/*.zip
```

Collection reads but does not modify the native Claude JSONL files.

## Development

The browser UI is a SvelteKit app in `web/app`, using shadcn-svelte. Go serves that client for every page, including the transcript viewer, and the JSON API under `/api/v1`. Build the UI before `go run` if you want the real pages instead of the fallback shell:

```sh
cd web/app
npm ci
npm run build
```

`npm run dev` serves the UI on port 5173 and proxies `/api`, login, and the transcript viewer to `http://127.0.0.1:8080`. The Docker image builds the UI itself.

### Git hooks

This repository uses `core.hooksPath=.githooks` (no husky or lefthook). Enable the hooks once per clone:

```sh
git config core.hooksPath .githooks
```

`commit-msg` strips Cursor attribution trailers. `prepare-commit-msg` exports Rafael as `GIT_AUTHOR_*` / `GIT_COMMITTER_*` when the ident is Cursor Agent (and sets local `user.name` / `user.email`). Git resolves author before that hook, so `post-commit` amends the commit to `regutierrez <rpegutierrez@gmail.com>` when HEAD still has a Cursor author or committer. Hosted Cursor cloud agents may still force Cursor Agent as author; squash-merge to Rafael (`regutierrez` / `rpegutierrez@gmail.com`) if that happens.

Run the complete local checks and builds:

```sh
make all
make test-race
```

Run the Docker-backed end-to-end tests explicitly:

```sh
go test -count=1 -tags=e2e -timeout=5m ./test/e2e
```

The end-to-end build tag keeps Docker-dependent tests out of ordinary `go test ./...` runs.

Release metadata is supplied explicitly, so repeated builds with the same inputs are deterministic:

```sh
make build VERSION=v0.1.0 COMMIT="$(git rev-parse HEAD)" BUILD_DATE=2026-08-22T10:00:00Z
```

Start the foundation server with Docker Compose:

```sh
export TRAICR_ADMIN_TOKEN='replace-with-a-long-random-token'
docker compose up --build
curl http://localhost:8080/healthz
```

The Compose service uses one persistent `/data` volume and a read-only container filesystem. The server refuses to start when `TRAICR_ADMIN_TOKEN` is absent.

## Documentation

- [Domain language](./CONTEXT.md)
- [System design](./docs/design.md)
- [Transcript browsing and Pi viewer](./docs/transcript-viewer.md)
- [Implementation plan](./docs/implementation-plan.md)
- [Architecture decisions](./docs/adr/)
