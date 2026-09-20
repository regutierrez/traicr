# Sanitized harness fixtures

These files contain invented prompts, paths, IDs, and tool output. They are minimal structural examples, not copied user traces.

| Directory | Collector adapter | Represented format and source |
| --- | --- | --- |
| `cursor-agent` | `cursor-agent-jsonl` | Current defensive transcript shape documented by [`cursor-history`](https://github.com/S2thend/cursor-history/blob/main/src/core/store-stack/transcript.ts); Cursor does not publish a stable schema |
| `cursor` | `cursor-sqlite-rows` | `cursorDiskKV` composer and bubble rows described by the [`cursor-history` data report](https://github.com/S2thend/cursor-history/blob/main/docs/cursor_history_message_data_structure_analysis_report.md); Cursor does not publish a stable schema |
| `amp` | `amp-thread-export` | Export `v:353`, sanitized from field names and block types emitted by the installed `amp threads export` command on 2026-09-06; [Amp CLI docs](https://ampcode.com/manual#cli) do not currently specify a stable export schema |
| `opencode-v2` | `opencode-export` | Current `{info,messages}` export and message/part schemas, [`export.ts`](https://github.com/anomalyco/opencode/blob/dev/packages/opencode/src/cli/cmd/export.ts) and [`session.ts`](https://github.com/anomalyco/opencode/blob/dev/packages/schema/src/session.ts) |
| `codex-app-server` | `codex-app-server` | Current app-server v2 `thread/read` result, [`README.md`](https://github.com/openai/codex/blob/main/codex-rs/app-server/README.md) |
| `codex-rollout` | `codex-rollout-jsonl` | Current compatibility rollout records, [`rollout_payload.rs`](https://github.com/openai/codex/blob/main/codex-rs/history/src/rollout_payload.rs) |
| `grok-build` | `grok-native-session` | Native chat format 1 summary and ACP update log, [`persistence.rs`](https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-shell/src/session/persistence.rs) and [`storage/mod.rs`](https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-shell/src/session/storage/mod.rs) |

Unknown-version and malformed-record cases are assembled in tests so their deliberately invalid bytes cannot be mistaken for native fixture examples.

Pi, Claude Code, and invented Amp viewer shapes live in Go tests, not as importable source files.
