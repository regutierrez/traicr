# The server generates missing titles with an opt-in model API

About 1,900 traces have no name anywhere: every `cursor-agent` session and every Pi session that was never named. Renaming them by hand does not scale, and new untitled traces arrive with every upload, so a one-time client batch would leave the same backlog again. The server sees every upload from every machine and already has the transcripts, so it names them.

A background worker calls an OpenAI-compatible chat-completions API configured with `TRAICR_TITLE_*` variables. It is off unless configured, because it sends unredacted transcript text to that provider. The worker only names traces with no manual or collected title, waits until a trace has been idle for 30 minutes so live sessions are not named mid-conversation, and names a trace again only when a new revision arrives. The generated title is stored in `traces.generated_title`; display order is manual override, then collected title, then generated title, and all of them are searchable.

The model sees the user's own words, not the harness's injected context, from the start and end of the session plus the final assistant message. Deriving a title from the first user message alone was rejected because it often does not describe what the session was for.

Failures that would affect every trace (network, authentication, rate limits, server errors) pause the worker without recording anything. A request-specific rejection is recorded as an empty generated title only after the API has produced at least one title, so a wrong model name cannot mark the whole backlog as done.
