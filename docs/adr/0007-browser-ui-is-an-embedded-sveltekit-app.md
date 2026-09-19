# Browser UI is an embedded SvelteKit app

ADR 0003 put the Web UI in Go so the Docker image stayed Node-free. That constraint cannot carry a Cursor-like three-pane workspace, a Svelte conversation renderer, Insights charts, and a Changes view. The browser UI is therefore a SvelteKit 2 / Svelte 5 / shadcn-svelte app in `web/app`, built to static files and embedded in the Go server. Go keeps JSON APIs, auth, import, and storage. The image build adds a Node stage; the running container still has no Node.

Open pull request 21 is the baseline. The htmx templates and the daisyUI / shadcn-on-templates drafts are superseded. The vendored Pi transcript DOM is replaced by Svelte components; its adapter logic is kept as a typed module.
