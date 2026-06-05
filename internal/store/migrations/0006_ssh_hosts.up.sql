CREATE TABLE ssh_hosts (
  id                   INTEGER PRIMARY KEY AUTOINCREMENT,
  alias                TEXT UNIQUE NOT NULL,
  host                 TEXT NOT NULL,
  port                 INTEGER NOT NULL,
  username             TEXT NOT NULL,
  auth_method          TEXT NOT NULL CHECK (auth_method IN ('key','password')),
  host_key_fingerprint TEXT NOT NULL DEFAULT '',
  requires_passphrase  INTEGER NOT NULL DEFAULT 0,
  salt                 BLOB,
  dek_nonce            BLOB NOT NULL,
  dek_ciphertext       BLOB NOT NULL,
  secret_nonce         BLOB NOT NULL,
  secret_ct            BLOB NOT NULL,
  key_passphrase_nonce BLOB,
  key_passphrase_ct    BLOB,
  created_at           INTEGER NOT NULL,
  updated_at           INTEGER NOT NULL
);
