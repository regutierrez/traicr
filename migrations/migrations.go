package migrations

import _ "embed"

//go:embed 001_initial.sql
var Initial string

const EventAliases = `
CREATE TABLE event_aliases (
    trace_id INTEGER NOT NULL REFERENCES traces(id) ON DELETE CASCADE,
    legacy_key TEXT NOT NULL,
    legacy_id INTEGER NOT NULL,
    event_key TEXT NOT NULL,
    PRIMARY KEY(trace_id, legacy_key)
);
CREATE INDEX event_aliases_id ON event_aliases(legacy_id);
CREATE INDEX event_aliases_target ON event_aliases(trace_id,event_key);
PRAGMA user_version = 2;
`

// TraceTitles adds the titles imports never replace: a manual override and a
// model-generated title with the latest revision it was generated from.
const TraceTitles = `
ALTER TABLE traces ADD COLUMN title_override TEXT NOT NULL DEFAULT '';
ALTER TABLE traces ADD COLUMN generated_title TEXT NOT NULL DEFAULT '';
ALTER TABLE traces ADD COLUMN generated_title_revision_id INTEGER NOT NULL DEFAULT 0;
PRAGMA user_version = 3;
`
