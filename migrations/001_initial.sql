PRAGMA foreign_keys = ON;

CREATE TABLE source_machines (
    id TEXT PRIMARY KEY,
    hostname TEXT NOT NULL,
    os TEXT NOT NULL,
    arch TEXT NOT NULL,
    first_seen TEXT NOT NULL,
    last_seen TEXT NOT NULL
);

CREATE TABLE imports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TEXT NOT NULL,
    machine_id TEXT NOT NULL REFERENCES source_machines(id),
    report_json BLOB
);

CREATE TABLE repositories (
    id INTEGER PRIMARY KEY,
    identity TEXT NOT NULL UNIQUE,
    remote TEXT NOT NULL,
    root TEXT NOT NULL
);

CREATE TABLE repository_paths (
    repository_id INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    PRIMARY KEY(repository_id, path)
);

CREATE TABLE traces (
    id INTEGER PRIMARY KEY,
    harness TEXT NOT NULL,
    native_trace_id TEXT NOT NULL,
    title TEXT NOT NULL,
    working_directory TEXT NOT NULL,
    repository_id INTEGER REFERENCES repositories(id),
    parent_native_trace_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(harness, native_trace_id)
);

CREATE TABLE trace_revisions (
    id INTEGER PRIMARY KEY,
    trace_id INTEGER NOT NULL REFERENCES traces(id) ON DELETE CASCADE,
    digest TEXT NOT NULL,
    native_updated_at TEXT NOT NULL,
    collected_at TEXT NOT NULL,
    adapter TEXT NOT NULL,
    status TEXT NOT NULL,
    normalizer_version INTEGER NOT NULL,
    UNIQUE(trace_id, digest)
);

CREATE TABLE revision_machines (
    revision_id INTEGER NOT NULL REFERENCES trace_revisions(id) ON DELETE CASCADE,
    machine_id TEXT NOT NULL REFERENCES source_machines(id),
    PRIMARY KEY(revision_id, machine_id)
);

CREATE TABLE source_objects (
    digest TEXT PRIMARY KEY,
    size INTEGER NOT NULL,
    path TEXT NOT NULL
);

CREATE TABLE revision_objects (
    revision_id INTEGER NOT NULL REFERENCES trace_revisions(id) ON DELETE CASCADE,
    relative_path TEXT NOT NULL,
    object_digest TEXT NOT NULL REFERENCES source_objects(digest),
    PRIMARY KEY(revision_id, relative_path)
);

CREATE TABLE events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    trace_id INTEGER NOT NULL REFERENCES traces(id) ON DELETE CASCADE,
    event_key TEXT NOT NULL,
    sort_time TEXT NOT NULL,
    sort_key TEXT NOT NULL,
    preferred_observation_id INTEGER,
    UNIQUE(trace_id, event_key)
);

CREATE INDEX events_chronology ON events(trace_id, sort_time, sort_key, id);

CREATE TABLE event_observations (
    id INTEGER PRIMARY KEY,
    event_id INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    digest TEXT NOT NULL,
    parent_key TEXT NOT NULL,
    branch TEXT NOT NULL,
    kind TEXT NOT NULL,
    role TEXT NOT NULL,
    model TEXT NOT NULL,
    provider TEXT NOT NULL,
    tool TEXT NOT NULL,
    call_id TEXT NOT NULL,
    event_time TEXT NOT NULL,
    text TEXT NOT NULL,
    event_json BLOB NOT NULL,
    UNIQUE(event_id, digest)
);

CREATE TABLE observation_revisions (
    observation_id INTEGER NOT NULL REFERENCES event_observations(id) ON DELETE CASCADE,
    revision_id INTEGER NOT NULL REFERENCES trace_revisions(id) ON DELETE CASCADE,
    searchable_text TEXT NOT NULL,
    PRIMARY KEY(observation_id, revision_id)
);

CREATE TABLE observation_sources (
    observation_id INTEGER NOT NULL,
    revision_id INTEGER NOT NULL,
    ordinal INTEGER NOT NULL,
    path TEXT NOT NULL,
    line INTEGER NOT NULL,
    PRIMARY KEY(observation_id, revision_id, ordinal),
    FOREIGN KEY(observation_id, revision_id) REFERENCES observation_revisions(observation_id, revision_id) ON DELETE CASCADE
);

CREATE TABLE search_chunks (
    id INTEGER PRIMARY KEY,
    observation_id INTEGER NOT NULL,
    revision_id INTEGER NOT NULL,
    event_id INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    FOREIGN KEY(observation_id, revision_id) REFERENCES observation_revisions(observation_id, revision_id) ON DELETE CASCADE
);

CREATE VIRTUAL TABLE search_words USING fts5(content, content='search_chunks', content_rowid='id');
CREATE VIRTUAL TABLE search_trigrams USING fts5(content, content='search_chunks', content_rowid='id', tokenize='trigram');

CREATE TRIGGER search_chunks_insert AFTER INSERT ON search_chunks BEGIN
    INSERT INTO search_words(rowid, content) VALUES (new.id, new.content);
    INSERT INTO search_trigrams(rowid, content) VALUES (new.id, new.content);
END;

CREATE TRIGGER search_chunks_delete AFTER DELETE ON search_chunks BEGIN
    INSERT INTO search_words(search_words, rowid, content) VALUES ('delete', old.id, old.content);
    INSERT INTO search_trigrams(search_trigrams, rowid, content) VALUES ('delete', old.id, old.content);
END;

CREATE TABLE normalizer_runs (
    id INTEGER PRIMARY KEY,
    revision_id INTEGER NOT NULL REFERENCES trace_revisions(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    status TEXT NOT NULL,
    diagnostics_json BLOB NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX trace_revisions_trace ON trace_revisions(trace_id);
CREATE INDEX revision_machines_machine ON revision_machines(machine_id);
CREATE INDEX event_observations_event ON event_observations(event_id);
CREATE INDEX observation_revisions_revision ON observation_revisions(revision_id);
CREATE INDEX event_observations_filters ON event_observations(role, kind, model, tool, event_time);
CREATE INDEX search_chunks_event ON search_chunks(event_id);
CREATE INDEX search_chunks_revision ON search_chunks(revision_id);

PRAGMA user_version = 1;
