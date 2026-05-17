CREATE TABLE connections (
  id                  INTEGER PRIMARY KEY AUTOINCREMENT,
  alias               TEXT UNIQUE NOT NULL,
  host                TEXT NOT NULL,
  port                INTEGER NOT NULL,
  tag                 TEXT NOT NULL CHECK (tag IN ('local','dev','staging','production')),
  tls_mode            TEXT NOT NULL DEFAULT 'disable',
  requires_passphrase INTEGER NOT NULL DEFAULT 0,
  salt                BLOB,
  dek_nonce           BLOB NOT NULL,
  dek_ciphertext      BLOB NOT NULL,
  username_nonce      BLOB NOT NULL,
  username_ct         BLOB NOT NULL,
  password_nonce      BLOB NOT NULL,
  password_ct         BLOB NOT NULL,
  database_nonce      BLOB NOT NULL,
  database_ct         BLOB NOT NULL,
  created_at          INTEGER NOT NULL,
  updated_at          INTEGER NOT NULL
);

CREATE INDEX idx_connections_tag ON connections(tag);
