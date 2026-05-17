CREATE TABLE discovered_connections (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    source           TEXT NOT NULL CHECK (source IN ('env','docker')),
    source_key       TEXT NOT NULL,
    alias            TEXT NOT NULL,
    host             TEXT NOT NULL,
    port             INTEGER NOT NULL,
    database         TEXT NOT NULL,
    username         TEXT NOT NULL,
    password_nonce   BLOB,
    password_ct      BLOB,
    tag              TEXT NOT NULL CHECK (tag IN ('local','dev','staging','production')),
    status           TEXT NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending','approved','rejected','unreachable')),
    last_seen_ns     INTEGER NOT NULL,
    created_at_ns    INTEGER NOT NULL,
    approved_conn_id INTEGER REFERENCES connections(id) ON DELETE SET NULL,
    UNIQUE (source, source_key)
);

CREATE INDEX idx_discovered_status ON discovered_connections(status);
