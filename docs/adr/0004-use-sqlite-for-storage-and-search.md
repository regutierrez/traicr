# Use SQLite for storage and search

Traicr will store normalized data and FTS5 search indexes in SQLite using WAL mode, while Source Records live as content-addressed files beside the database. Its single-user, single-server workload does not justify a separate PostgreSQL service; this decision should be revisited only if Traicr gains multiple users, multiple application instances, or sustained concurrent writes.
