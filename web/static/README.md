# Static files

The server embeds these files. There is no frontend framework and no runtime asset request leaves the server.

- `app.css` is compiled from `web/ui/app.css` (Tailwind CSS v4 + daisyUI 5) with `make ui`. It carries the three themes (Ledger, Console, Blueprint), the page styles, and the transcript viewer chrome. Commit the output; the Go build and the Docker image never run Node.
- `fonts/` holds the Latin subsets of the theme typefaces (IBM Plex Sans and Mono, Schibsted Grotesk, JetBrains Mono, Source Serif 4, Source Code Pro), copied from their npm packages by `make ui`. All are licensed under the SIL Open Font License.
- `theme.js` applies the saved theme before the first paint. It is a plain script because the Content Security Policy forbids inline scripts.
- `shortcuts.js` provides keyboard shortcuts, the command palette, the help dialog, theme switching, and list navigation for every page. Bindings are declared with `data-shortcut` attributes in templates or registered by page scripts, so the palette and help dialog always describe the current page.
- `htmx.min.js` is the unmodified HTMX 2.0.4 distribution from `https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js`, under the adjacent Zero-Clause BSD license. HTMX only replaces search results; every form and navigation link also works without JavaScript. Evaluation, script execution in responses, and local history caching are disabled.
- `pi-transcript/` is the transcript viewer adapted from Pi's HTML export; see `docs/transcript-viewer.md`.

daisyUI effects that rely on `data:` URLs (noise, the loading spinner, tooltip arrows) are disabled or replaced because `img-src 'self'` does not allow them.
