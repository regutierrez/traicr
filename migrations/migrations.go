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
