# DBil v0.1 — Plan 6 (Observability collectors + UI)

## Context

The product's wow feature per spec section 8: pgAnalyze-class observability,
shipped open source. Every other tool either nests this behind a paywall
(pgAnalyze, DataDog) or leaves it for the user to glue together
(pgHero, pgwatch).

After Plan 6 a user can:

1. Open `/observ` against a registered Postgres connection.
2. See live overview tiles (TPS, cache hit ratio, active sessions, replication
   lag) updating every 5 seconds with sparklines for the last 5 minutes.
3. Read the top slow queries from `pg_stat_statements` with mean / p95 / p99,
   call count, total time, and a per-query trend line that flags regressions
   over the trailing window.
4. Inspect lock-waiting chains in real time — see which session holds the
   lock, which sessions are blocked, how long each has been waiting.
5. Eventually (v0.6.1, separate slice): index advisor, bloat detector,
   active-sessions live list with wait events.

## Scope of Plan 6 (v0.6.0)

| In | Out (v0.6.1+) |
|---|---|
| Overview metrics (TPS, cache hit, sessions, replication lag) collector | Index advisor |
| Slow queries snapshot from `pg_stat_statements` | Bloat detector |
| Lock waiting chain live query | Anomaly detection banner |
| 24 h hot retention in SQLite at 10 s resolution | Hot/warm/cold aggregation buckets |
| Per-connection collector lifecycle (start on first registration, stop on delete) | Active-sessions live list (wait events) |
| API endpoints `/api/connections/{id}/observ/{overview,slow,locks}` | Prometheus exporter |
| UI: header + 4 tiles + slow queries table + lock chains card | UI: index advisor, bloat panel, sessions panel |
| Graceful degradation when `pg_stat_statements` is not available | Custom-metric API |

## Spec alignment

Section 8 of the spec defines the wow feature. Plan 6 covers the
'observability data sources' table, the 'UI pages' enumeration up to slow
queries + locks, and the per-tag polling frequency from section 7.4
(local=60 s, dev=30 s, staging=10 s, production=5 s).

## Tech additions

| Purpose | Module |
|---|---|
| Frontend charts | `uplot` (≈40 KB, fast time-series) — replaces hand-rolled SVG |
| Backend periodic worker | stdlib `time.Ticker` inside a goroutine per connection |

## File structure added

```
internal/store/migrations/
  0004_observability.up.sql          # overview_samples + slow_query_snaps tables
  0004_observability.down.sql
internal/store/
  observability.go                   # ObservabilityRepo
  observability_test.go
internal/observ/
  collector.go                       # CollectorManager + Collector type
  collector_test.go
  overview.go                        # pg_stat_database + pg_stat_replication
  slow_queries.go                    # pg_stat_statements
  locks.go                           # pg_stat_activity + pg_locks (live query, no storage)
internal/server/handlers/
  observ.go                          # GET overview/slow/locks endpoints
  observ_test.go
  handlers.go                        # MODIFIED — registers the new routes
cmd/dbil/serve_cmd.go                # MODIFIED — start/stop collectors per connection
web/src/api/
  observ.ts                          # TanStack Query hooks for overview/slow/locks
web/src/pages/
  ObservabilityPage.tsx              # MODIFIED — real page replacing the placeholder
web/src/components/observ/
  MetricTile.tsx                     # tile + sparkline
  SlowQueriesTable.tsx
  LockChainCard.tsx
web/package.json                     # MODIFIED — adds uplot dependency
tests/e2e/
  observ_test.go                     # testcontainers Postgres with pg_stat_statements
```

## Data model

```sql
-- One row per (conn, ts), 10 s resolution, ~24 h retention (≈8 640 rows/conn).
CREATE TABLE overview_samples (
  conn_id        INTEGER NOT NULL REFERENCES connections(id) ON DELETE CASCADE,
  ts_ns          INTEGER NOT NULL,
  tps            REAL    NOT NULL,
  cache_hit      REAL    NOT NULL,        -- 0..1
  active_conns   INTEGER NOT NULL,
  idle_conns     INTEGER NOT NULL,
  rep_lag_ms     INTEGER,                  -- nullable when no replica
  PRIMARY KEY (conn_id, ts_ns)
);
CREATE INDEX idx_overview_conn_ts ON overview_samples(conn_id, ts_ns);

-- Snapshot of pg_stat_statements (≤ 100 rows kept per snapshot,
-- only the latest snapshot per conn is queried by the UI).
CREATE TABLE slow_query_snaps (
  conn_id     INTEGER NOT NULL REFERENCES connections(id) ON DELETE CASCADE,
  taken_at_ns INTEGER NOT NULL,
  query_hash  TEXT NOT NULL,           -- normalised query text hash
  preview     TEXT NOT NULL,           -- truncated query text
  mean_ms     REAL NOT NULL,
  p95_ms     REAL NOT NULL,
  p99_ms     REAL NOT NULL,
  calls       INTEGER NOT NULL,
  total_ms    REAL NOT NULL,
  rows_avg    REAL NOT NULL,
  PRIMARY KEY (conn_id, taken_at_ns, query_hash)
);
CREATE INDEX idx_slow_conn_ts ON slow_query_snaps(conn_id, taken_at_ns);
```

Both tables get cleaned by a periodic janitor: rows older than 24 h for
overview, older than 1 h for slow_query_snaps (we only need the latest
snapshot for the UI). Janitor runs every 5 minutes.

## Tasks

### Phase A — Storage

**Task 1 — Migration 0004**: SQL above + down migration.
**Task 2 — `ObservabilityRepo`**: `RecordOverview`, `ListOverviewSince(connID, since)`,
`SnapshotSlow(rows)`, `LatestSlow(connID)`, `DeleteOlderThan(connID, ts)`.
Tests cover round-trip, time-window query, retention sweep, FK cascade on
connection delete.

### Phase B — Collector framework

**Task 3 — `internal/observ/collector.go`**: 
- `type Collector interface { Tick(ctx, pool, repo) error }`
- `type Manager struct{ ... }` with `Start(connID, pool, policy)`,
  `Stop(connID)`, `Shutdown()`. Owns one goroutine per active collector ×
  connection. Polling interval comes from `policy.PolicyFor(tag).PollInterval`
  (extended in this plan: local 60 s, dev 30 s, staging 10 s, production 5 s).
- Goroutine: `for { select { <-ticker; collector.Tick(); | <-ctx.Done(); return } }`
  with `defer recover()` so a panicking collector restarts itself after a 30 s
  back-off, logged via slog.

**Task 4 — Overview collector (`overview.go`)**: one `pg_stat_database`
snapshot per tick. Reads `xact_commit + xact_rollback` deltas vs previous
snapshot to compute TPS; `blks_hit / (blks_hit + blks_read)` for cache
ratio; `numbackends` for active connections. Replication lag from
`pg_stat_replication` (max(replay_lag) across replicas), `NULL` if no
replica row.

**Task 5 — Slow queries collector (`slow_queries.go`)**: `SELECT query,
mean_exec_time, calls, total_exec_time, rows FROM pg_stat_statements ORDER
BY total_exec_time DESC LIMIT 100`. Hashes query text via sha256 truncated
to 16 hex chars for `query_hash`. Skips collection when the extension is
not present (logged once per connection).

**Task 6 — Lock chains (`locks.go`)**: live query (no storage). Returns
`[]LockChainHead{ head: { pid, user, query, age_ms }, blocked: [...] }`.
Query joins `pg_stat_activity` with `pg_locks` and aggregates by waiting
chain.

### Phase C — HTTP endpoints

**Task 7 — `handlers/observ.go`** (RequireAuth-gated):
- `GET /api/connections/{id}/observ/overview?since=<unix_ms>` →
  `[]OverviewSample`
- `GET /api/connections/{id}/observ/slow` → `{ taken_at, rows: [...] }`
- `GET /api/connections/{id}/observ/locks` → `{ chains: [...] }`

**Task 8 — Mount + lint-auth** updated.

### Phase D — CLI wiring

**Task 9 — `serve_cmd.go`**: on startup, list existing connections and
`mgr.Start(connID, pool, policy)` each. On `ctx.Done()`, `mgr.Shutdown()`
before `srv.Shutdown()`. Connection create / delete hook into the existing
`ConnectionsRepo` writes via observation pattern (small callback registered
in `handlers/connections.go`).

### Phase E — Frontend

**Task 10 — `web/src/api/observ.ts`**: TanStack Query hooks
`useOverview`, `useSlowQueries`, `useLocks` with `refetchInterval` driven
by the connection tag (mirrored from policy).

**Task 11 — `MetricTile.tsx`**: title + big number + delta vs previous +
sparkline (uplot). Live-dot pulse when fresh.

**Task 12 — `SlowQueriesTable.tsx`**: header (taken_at, copy / export
buttons) + sortable table (preview / mean / p95 / p99 / calls / total).

**Task 13 — `LockChainCard.tsx`**: head row (holder) + children (blocked
sessions) with `age_ms` and a 'kill backend' affordance (no-op for now —
backend `pg_terminate_backend` endpoint lands in v0.6.1).

**Task 14 — `ObservabilityPage.tsx`**: composes the page per the design's
layout: 5-min header strip with pause / interval select, 4 tiles, slow
queries table, lock chains. Header live-dot when the most recent overview
sample is < 30 s old.

### Phase F — Test + tag

**Task 15 — `tests/e2e/observ_test.go`**: testcontainers Postgres with
`shared_preload_libraries=pg_stat_statements` + `CREATE EXTENSION
pg_stat_statements`. Login → register connection → wait for first sample →
GET overview returns ≥ 1 sample → run a few queries → GET slow returns
those queries.

**Task 16 — `make test cover lint-auth`** all green. Tag `v0.6.0-observ`.

## Risks acknowledged

- **`pg_stat_statements` is not enabled by default** on managed Postgres.
  The UI shows a single info banner with the exact `ALTER SYSTEM SET
  shared_preload_libraries` line plus a CREATE EXTENSION snippet when the
  extension is missing.
- **Polling overhead on heavily loaded primaries.** The default 5 s for
  production is conservative; users can override per-connection in v0.6.1.
- **`pg_locks` joined with `pg_stat_activity` is fast** but on databases
  with thousands of sessions can briefly hold a snapshot; we only query
  on demand (UI tab open), not on a poll loop.
- **No anomaly detection** in v0.6.0 — the design's "replication lag spike
  → click jumps to viewer PID" banner is deferred. The data is there; the
  trigger logic is the deferred piece.
- **Janitor runs in-process.** A long sleep is wasteful but acceptable for
  Plan 6; a proper background worker pool returns in v0.7+.

## End-to-end verification (after v0.6.0)

```bash
# fresh stack
make build
DATA=$(mktemp -d) && ./bin/dbil init
./bin/dbil serve &

# bring a real Postgres up with pg_stat_statements
docker run -d --rm --name pg-obs -p 5432:5432 \
  -e POSTGRES_PASSWORD=secret postgres:16-alpine \
  -c shared_preload_libraries=pg_stat_statements
sleep 3
docker exec pg-obs psql -U postgres -c \
  "CREATE EXTENSION pg_stat_statements"

# (login + register connection via the UI or curl)
# open http://localhost:4242/observ → tiles light up after ~10 s

# generate some load
docker exec pg-obs psql -U postgres -c \
  "CREATE TABLE t (id int); INSERT INTO t SELECT generate_series(1, 1e6); SELECT count(*) FROM t WHERE id > 0;"

# slow queries table populates after the next collector tick
```
