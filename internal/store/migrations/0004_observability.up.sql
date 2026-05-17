CREATE TABLE overview_samples (
  conn_id      INTEGER NOT NULL REFERENCES connections(id) ON DELETE CASCADE,
  ts_ns        INTEGER NOT NULL,
  tps          REAL NOT NULL,
  cache_hit    REAL NOT NULL,
  active_conns INTEGER NOT NULL,
  idle_conns   INTEGER NOT NULL,
  rep_lag_ms   INTEGER,
  PRIMARY KEY (conn_id, ts_ns)
);

CREATE INDEX idx_overview_conn_ts ON overview_samples(conn_id, ts_ns);

CREATE TABLE slow_query_snaps (
  conn_id     INTEGER NOT NULL REFERENCES connections(id) ON DELETE CASCADE,
  taken_at_ns INTEGER NOT NULL,
  query_hash  TEXT NOT NULL,
  preview     TEXT NOT NULL,
  mean_ms     REAL NOT NULL,
  p95_ms      REAL NOT NULL,
  p99_ms      REAL NOT NULL,
  calls       INTEGER NOT NULL,
  total_ms    REAL NOT NULL,
  rows_avg    REAL NOT NULL,
  PRIMARY KEY (conn_id, taken_at_ns, query_hash)
);

CREATE INDEX idx_slow_conn_ts ON slow_query_snaps(conn_id, taken_at_ns);
