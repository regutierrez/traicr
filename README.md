# Traicr

Traicr collects, preserves, and searches one person's AI coding traces across multiple macOS and Linux machines.

The Go foundation is implemented. Trace collection, archive import, and search behavior will arrive in later milestones.

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

## Development

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
