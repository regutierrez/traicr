# Traicr

Traicr collects, preserves, and searches one person's AI coding traces across multiple macOS and Linux machines.

The project is currently in the design phase. Implementation has not started.

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

## Documentation

- [Domain language](./CONTEXT.md)
- [System design](./docs/design.md)
- [Implementation plan](./docs/implementation-plan.md)
- [Architecture decisions](./docs/adr/)
