# Templates

Go's `html/template` escapes every source value. Layout, search, trace, source inspection, imports, and machines are rendered by the server, with no client-side router. Native attachment URLs are displayed as text, never fetched or executed.

Templates and static files are embedded in the server binary and also copied explicitly into the image under `/usr/share/traicr/web`.
