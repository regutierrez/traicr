# Static files

The server embeds this directory. The SvelteKit shell in `web/app` owns its own dark-only tokens and does not import `--paper` / `--ink` / `--green`. `traicr-theme.css` remains for the vendored Pi transcript viewer in `pi-transcript/`. The built SvelteKit app lands in `ui/`, which is generated and not committed.
