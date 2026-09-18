# Traicr UI concepts

SvelteKit + shadcn-svelte prototypes for issue #9. Three complete copies of the homepage and trace viewer, using synthetic archive data.

This is a frontend direction check, not a replacement for the Go search/import API. The existing HTMX templates stay until a concept is chosen.

## Concepts

| Copy | Routes | Reference |
| --- | --- | --- |
| Inbox | `/inbox`, `/inbox/[id]` | Linear + [traces.com](https://traces.com/) |
| Studio | `/studio`, `/studio/[id]` | Cursor + [AgentTrace](https://github.com/tensorstax/agenttrace) |
| Ledger | `/ledger`, `/ledger/[id]` | [OpenTraces](https://github.com/JayFarei/opentraces) + traces.com |

`/` is the comparison gallery.

## Run

```sh
cd web/app
npm install
npm run dev -- --host 127.0.0.1 --port 5173
```

```sh
npm run check
```
