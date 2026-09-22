# Scrubbed session fixtures

These are real Pi, Amp, and Claude Code sessions used to exercise the transcript viewer. Tokens, personal emails, home directories, and host names were replaced. The original pack is not in git.

| Directory | Harness | Adapter | Native source |
| --- | --- | --- | --- |
| `pi-herdr` | `pi` | `pi-jsonl` | `source/records.jsonl` |
| `amp-transcript-support` | `amp` | `amp-thread-export` | `source/export.json` |
| `claude-cleanup` | `claude-code` | `claude-code-jsonl` | `source/records.jsonl` |

`testdata/harnesses` stays the small invented structural fixtures. Do not copy an unsanitized export here. Re-run `scrub.py` against a new pack, then `go test ./test/sessions`.
