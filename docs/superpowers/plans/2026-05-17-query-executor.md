# DBil v0.1 — Plan 4 (Query executor + tag-driven safety guards)

## Context

After Plan 3 you can register Postgres connections, but you can't run a single
query against them yet. Plan 4 closes that gap and is the last piece needed
before the SPA can be built on top (Plan 5).

It adds:

- A `Pool.Execute(ctx, sql)` extension to the Driver / Pool abstraction so
  every driver returns DBil's own `Result` shape rather than leaking pgx
  types up the stack.
- An `internal/sqlcheck` package that classifies a SQL statement as
  Read / DML / DDL / Other and flags `DELETE`/`UPDATE` without a `WHERE`
  clause as dangerous. Best-effort lexical detection — documented as such;
  full AST analysis is a v0.2+ concern.
- An `internal/policy` package that maps a connection's tag to safety
  policy: statement timeout, DML / DDL gating, dangerous-statement gating,
  confirmation requirement.
- `Manager.Execute(ctx, id, passphrase, sql, confirm)` which applies the
  policy *before* dialing through to the driver and writes a
  `query.execute` (or `query.blocked`) audit entry either way.
- `POST /api/connections/{id}/query` taking `{sql}`, headers
  `X-Connection-Passphrase` and `X-Confirm`, returning a JSON `Result`.

After Plan 4 you can drive the whole product through curl: log in, add a
Postgres, run `SELECT 1`, audit log captures every statement.

## Spec alignment

Implements spec section 5 (Query Executor) and the relevant rows of the
section 7.4 tag-policy table — except "second approval" workflows, which
need durable state and the SPA, and so are deferred. Until then,
production-tagged DDL is **blocked outright**; the policy table calls
this out explicitly.

## Tech additions

None. pgx is already in. SQL classification is stdlib + `unicode`.

## File structure added

```
internal/sqlcheck/
  classify.go                # Class type + Classify(sql) + IsDangerous(sql)
  classify_test.go
internal/policy/
  policy.go                  # Policy struct + PolicyFor(tag)
  policy_test.go
internal/postgres/
  driver.go                  # MODIFIED: Pool.Execute(ctx, sql) added
  pgx.go                     # MODIFIED: implements Execute
  manager.go                 # MODIFIED: Execute(ctx, id, passphrase,
                             #          sql, confirm) with policy +
                             #          audit hook
  manager_test.go            # MODIFIED: new tests for Execute
internal/server/handlers/
  query.go                   # POST /api/connections/{id}/query
  query_test.go
  handlers.go                # MODIFIED: Mount registers the new route
tests/e2e/
  query_test.go              # testcontainers Postgres SELECT 1 + INSERT
```

## Result + Policy shapes

```go
// internal/postgres
type Result struct {
    Columns      []ColumnDef // ordered as in the query
    Rows         [][]any     // each row is len(Columns)
    RowsAffected int64       // for DML/DDL
    CommandTag   string      // pgx CommandTag string, e.g. "SELECT 5"
    Duration     time.Duration
    Truncated    bool        // true when row cap was hit
}

type ColumnDef struct {
    Name     string
    TypeName string // pgx DataTypeName, e.g. "int4", "text"
}
```

```go
// internal/policy
type Policy struct {
    Timeout                  time.Duration
    DMLAllowed               bool
    DMLRequiresConfirm       bool
    DDLAllowed               bool
    DDLRequiresConfirm       bool
    DangerousAllowed         bool // DELETE/UPDATE without WHERE
    DangerousRequiresConfirm bool
}

// PolicyFor returns the default Policy for a connection tag.
// local  : 5m  timeout, DML+DDL allowed, dangerous OK
// dev    : 30s timeout, DML+DDL allowed, dangerous OK
// staging: 30s timeout, DML+DDL allowed with confirmation, dangerous w/ confirm
// production: 10s timeout, DML w/ confirm, DDL blocked outright,
//             dangerous blocked outright
```

## Manager.Execute shape

```go
type ExecuteResult struct {
    Result *postgres.Result
    Class  sqlcheck.Class
    Policy policy.Policy
}

func (m *Manager) Execute(
    ctx context.Context,
    id int64,
    passphrase string,
    sql string,
    confirm bool,
) (*ExecuteResult, error)
```

Returns one of:

- `(result, nil)` on success — audit `query.execute` already written
- `(nil, ErrBlockedByPolicy{Reason})` when the statement class is not
  allowed for this tag — audit `query.blocked` written
- `(nil, ErrConfirmationRequired{Reason})` when the policy demands a
  confirmation header — no audit (the caller hasn't taken an action yet)
- driver / probe errors wrapped unchanged

## Tasks

### Phase A — SQL classification

**Task 1 — `internal/sqlcheck`**
- `type Class int; const ( ClassRead, ClassDML, ClassDDL, ClassOther )`
- `func Classify(sql string) Class` — strips line + block comments,
  takes the first keyword, switches:
    `SELECT WITH EXPLAIN SHOW VALUES TABLE` → Read
    `INSERT UPDATE DELETE MERGE TRUNCATE COPY CALL` → DML
    `CREATE ALTER DROP GRANT REVOKE COMMENT REINDEX VACUUM ANALYZE` → DDL
    otherwise Other
- `func IsDangerous(sql string) bool` — best-effort: upper-cased,
  comments stripped, returns true when a statement matches
  `DELETE\s+FROM` (no `WHERE`) or `^UPDATE\s+\S+\s+SET` (no `WHERE`).
- Tests across every keyword, comments stripped, semicolons OK,
  CRLF OK, dangerous detection true / false cases.

### Phase B — Policy

**Task 2 — `internal/policy`**
- Policy struct (fields above), `PolicyFor(tag string) Policy` plus
  `ErrInvalidTag` for unknown values. Tests verify each of the four tags
  return the documented numbers.

### Phase C — Postgres driver + Manager Execute

**Task 3 — Extend `Pool` interface**
- Add `Execute(ctx, sql) (*Result, error)`.
- `pgxPool.Execute`: runs `pool.Query(ctx, sql)`, collects field
  descriptions (name + DataTypeName), iterates rows up to a hard cap
  (default 10 000 rows; later tied to a cfg). Returns Result.
- For DML/DDL pgx's `CommandTag().RowsAffected()` is the count;
  use it. (CommandTag also surfaces "INSERT 0 5" etc.)
- Update `manager_test.go`'s `fakePool` to implement Execute returning
  canned Result.

**Task 4 — `Manager.Execute`**
- Pre-flight: classify sql, ask `PolicyFor(tag)`, optionally call
  `IsDangerous`. Build `(allowed, requiresConfirm, reason)`.
- If `!allowed`: write `query.blocked` audit, return `ErrBlockedByPolicy`.
- If `requiresConfirm && !confirm`: return `ErrConfirmationRequired`
  (no audit — caller hasn't acted yet).
- Resolve pool via `OpenByID` (passphrase handling unchanged).
- `ctx, cancel = context.WithTimeout(ctx, policy.Timeout); defer cancel()`.
- Call `pool.Execute(ctx, sql)`.
- Audit `query.execute` with sql length, class, duration, row count.
  Successful or driver-error, an audit entry is written; the *attempt*
  is the auditable event.
- Tests: each branch — allowed read, allowed DML, blocked DDL on
  production, confirmation required and supplied, timeout enforcement
  (use a fake pool that sleeps to verify ctx deadline).

### Phase D — HTTP handler

**Task 5 — `handlers.QueryHandler`**
- `POST /api/connections/{id}/query` body `{"sql": "..."}`,
  headers `X-Connection-Passphrase` (optional) and
  `X-Confirm: yes` (optional).
- Maps Manager errors to HTTP:
    `store.ErrConnectionNotFound` → 404
    `store.ErrPassphraseRequired` → 428
    `store.ErrInvalidPassphrase` → 401
    `ErrConfirmationRequired` → 428 (`{"error":"confirmation required","reason":...}`)
    `ErrBlockedByPolicy` → 403 (`{"error":"blocked by policy","reason":...}`)
    `context.DeadlineExceeded` → 504 (`{"error":"statement timeout"}`)
    other → 502 (driver error).
- On success returns 200 with the Result encoded as JSON.

**Task 6 — Mount + lint-auth**
- Register inside the existing `RequireAuth` group. Run `make lint-auth`.

### Phase E — E2E + tag

**Task 7 — `tests/e2e/query_test.go`**
- Skips if Docker isn't available.
- Spins up `postgres:16-alpine`, runs init/serve/login/POST connection
  (tag=`local`), then:
    a) `SELECT 1 AS n` returns Result with one column and one row
       holding the value 1.
    b) `CREATE TABLE t (id INT); INSERT INTO t VALUES (1)` — sent as
       separate statements (multi-statement support is Plan 5+); we
       use a single `INSERT INTO t (id) VALUES (1)` after a separate
       `CREATE TABLE`. Both succeed.
    c) `SELECT id FROM t` returns 1 row.
    d) Re-tag the connection? Not possible — Plan 3 has no update API.
       Instead create a *second* connection at tag=`production` pointing
       at the same container and verify a `DROP TABLE t` is blocked with
       403, then re-runs with `X-Confirm: yes` is still 403 (production
       DDL is blocked outright).

**Task 8 — Coverage + tag**
- `go test ./...` green. `make lint-auth` green.
- Tag `v0.4.0-query`.

## Known limitations

- **Multi-statement queries** are not supported in this plan — one statement
  per request. The SPA's editor (Plan 5) will split by `;` client-side
  before posting.
- **Result row cap** is a hard 10 000; size-based cap (truncate at ~100 MB
  per spec section 12) is Plan 6.
- **"Second approval" workflows** are not implemented. Until they ship in
  a later plan, production-tagged DDL and dangerous statements are blocked
  outright by policy. The `X-Confirm` header upgrades staging-tagged
  blocks to allowed but never overrides a production-tagged hard block.
- **SQL classification is lexical.** `WITH ... DELETE` constructs may be
  classified as Read; a malicious user with write privileges on production
  could route around this. The audit chain still captures the statement,
  and the underlying Postgres role can be `SELECT`-only to provide the
  real defence.

## End-to-end verification

After Plan 4 ships (Docker available):

```bash
# init + serve as before, $TOKEN as before
docker run -d --rm --name pg -p 5432:5432 -e POSTGRES_PASSWORD=secret postgres:16-alpine
sleep 2
ID=$(curl -s -X POST http://localhost:4242/api/connections \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"alias":"local","host":"127.0.0.1","port":5432,"username":"postgres",
       "password":"secret","database":"postgres","tag":"local","tls_mode":"disable"}' \
  | jq -r .id)

# read
curl -s -X POST http://localhost:4242/api/connections/$ID/query \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"sql":"SELECT 1 AS n"}' | jq

# blocked production DDL
PID=$(curl -s -X POST http://localhost:4242/api/connections \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"alias":"prod","host":"127.0.0.1","port":5432,"username":"postgres",
       "password":"secret","database":"postgres","tag":"production","tls_mode":"disable"}' \
  | jq -r .id)
curl -s -X POST http://localhost:4242/api/connections/$PID/query \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"sql":"DROP TABLE x"}' -w '%{http_code}\n'   # -> 403
```
