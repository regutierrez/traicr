# Manual titles override collected titles without replacing them

Most traces arrive without a useful title. `cursor-agent` has no saved session name, and many Pi sessions were never named, so the archive shows their native IDs. Deriving a title from the first user message does not describe what the session was, so traces are named deliberately: by hand here, or by a model ([ADR 0010](./0010-generate-missing-titles-on-the-server.md)).

A manual title is stored in `traces.title_override`, separate from the collected `traces.title`. Imports only ever write the collected title, so a later upload never undoes a rename. Every reader shows the override when it is set, then the collected title, then a generated title. Clearing the override shows the collected title again.

The title is part of each Event's search text. Setting or clearing an override rebuilds the search text of every revision of that trace from its stored observations and the trace's current metadata, and new revisions include the override in their search labels. Both the override and the collected title stay searchable. Search is built from Events, so a trace with no Events (empty, failed, or unsupported normalization) has nothing to index and its title is not searchable until it gains Events.

Renaming is bearer-token only (`PATCH /api/v1/traces/{id}` and `traicr traces rename`). A browser session cannot rename a trace, which matches the rule that browser pages cannot drive mutations through the API.
