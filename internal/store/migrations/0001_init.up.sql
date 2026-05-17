CREATE TABLE users (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  email           TEXT UNIQUE NOT NULL,
  password_hash   TEXT NOT NULL,
  password_salt   BLOB NOT NULL,
  role            TEXT NOT NULL CHECK (role IN ('admin','member','viewer')),
  must_rotate     INTEGER NOT NULL DEFAULT 0,
  created_at      INTEGER NOT NULL,
  updated_at      INTEGER NOT NULL
);

CREATE TABLE audit_log (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  ts_ns           INTEGER NOT NULL,
  user_id         TEXT NOT NULL,
  action          TEXT NOT NULL,
  resource        TEXT NOT NULL,
  details_enc     BLOB NOT NULL,
  details_nonce   BLOB NOT NULL,
  prev_hash       BLOB NOT NULL,
  entry_hash      BLOB NOT NULL UNIQUE
);

CREATE INDEX idx_audit_ts ON audit_log(ts_ns);
