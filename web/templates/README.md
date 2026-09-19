# Templates

Go's `html/template` escapes every source value. Layout, search, transcript, session details, source inspection, imports, and machines are rendered by the server, with no client-side router. Native attachment URLs are displayed as text, never fetched or executed.

Templates use daisyUI component classes (`btn`, `badge`, `table`, `modal`, `kbd`, …) plus Tailwind utilities. When you add or change classes, run `make ui` so `web/static/app.css` includes them; Tailwind only emits the classes it finds in `web/templates` and the scripts listed in `web/ui/app.css`.

Keyboard shortcuts are declared on elements with `data-shortcut="keys"`, `data-shortcut-label`, and `data-shortcut-group`. Lists that support j/k navigation mark their container with `data-keynav="<item selector>"`; the item's primary link may carry `data-details` for the `o` shortcut.

Templates and static files are embedded in the server binary.
