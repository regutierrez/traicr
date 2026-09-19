# Static files

The server embeds this directory. `traicr-theme.css` holds the shared theme tokens, imported by both the SvelteKit app in `web/app` and the transcript viewer in `pi-transcript/`. The built SvelteKit app lands in `ui/`, which is generated and not committed.
