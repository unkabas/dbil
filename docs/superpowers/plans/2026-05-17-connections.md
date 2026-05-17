# DBil v0.1 — Plan 3 (Connection Manager + first Postgres connect)

## Context

Plan 1 built the foundation. Plan 2 added the HTTP server + session auth.
DBil now boots and authenticates users — but it does not talk to PostgreSQL
yet, which is the whole point of the product.

Plan 3 changes that. It introduces:

- A `connections` table whose `password`, `username`, `database`, and TLS
  material are application-layer-encrypted via `crypto.EncryptField` under a
  per-connection DEK wrapped by the master key.
- A `Driver` interface in `internal/postgres` with one concrete
  implementation (pgx) — the seam for future engines / mocks.
- A `Manager` that owns a `pgx` pool per registered connection and exposes
  Open / Close / Ping / Probe.
- REST endpoints under `/api/connections`: list, get, create, delete, test.
- Optional per-connection passphrase: when set, the password column is wrapped
  with `WrapWithPassphrase` and the caller must supply `X-Connection-Passphrase`
  to test or use the connection. (Passphrase prompt UX in the SPA comes in Plan 5;
  the API surface ships now so the contract is stable.)

After Plan 3 ships you can `curl POST /api/connections` to register a
Postgres URL, `POST /api/connections/{id}/test` to verify reachability, and
the encrypted record survives an `init`/`serve` restart cycle.

## Spec alignment

This plan covers spec section 5 (Connection Manager) and the encrypted
credentials half of section 6 (Crypto + threat model rows 2, 3, and 5). No
SQL execution yet — that's Plan 4. No SSH tunnels or IAM auth either; they
land as Plan 3.5 once the basic flow is on disk and we know the request
shape.

## Tech additions

| Purpose | Module |
|---|---|
| Postgres driver | `github.com/jackc/pgx/v5` |
| Postgres connection pool | `github.com/jackc/pgx/v5/pgxpool` |
| Real-Postgres integration tests | `github.com/testcontainers/testcontainers-go` + `.../modules/postgres` |

`testcontainers-go` needs a Docker daemon at test time. Tests `t.Skip()` when
Docker is not available so contributors without Docker can still run unit
tests.

## File structure added

```
internal/postgres/
  driver.go                            # Driver interface + Conn record
  pgx.go                               # PGX Driver implementation
  manager.go                           # Manager: pool per registered conn
  manager_test.go
internal/store/
  connections.go                       # ConnectionsRepo (envelope-encrypted)
  connections_test.go
  migrations/
    0003_connections.up.sql
    0003_connections.down.sql
internal/server/handlers/
  connections.go                       # CRUD + /test endpoints
  connections_test.go
tests/e2e/
  connections_test.go                  # testcontainers-go Postgres
```

## Connection record shape

| Column | Encrypted | Notes |
|---|---|---|
| `id` | no | `INTEGER PRIMARY KEY AUTOINCREMENT` |
| `alias` | no | display name, also used as conn ID for AAD |
| `host`, `port` | no | hostname + port |
| `tag` | no | `local` / `dev` / `staging` / `production` |
| `tls_mode` | no | `disable`, `require`, `verify-ca`, `verify-full` |
| `requires_passphrase` | no | bool; true means a per-conn passphrase wraps the password |
| `salt` | no | 16-byte salt for argon2id when requires_passphrase = true |
| `dek_nonce` / `dek_ciphertext` | yes (under MK) | `WrapDEK` output; ties this conn's DEK to its alias via AAD |
| `username_nonce` / `username_ct` | yes (under DEK) | `EncryptField` of the username |
| `password_nonce` / `password_ct` | yes (under DEK *and* optional passphrase) | wraps the password |
| `database_nonce` / `database_ct` | yes (under DEK) | wraps the database name |
| `created_at`, `updated_at` | no | unix ns |

The DEK is generated via `crypto.GenerateDEK()` at create time, wrapped via
`crypto.WrapDEK(mk, alias, dek)`, and the wrapped form is what lives in the row.
Loading the connection requires the MK; *using* a `requires_passphrase=true`
connection additionally requires the per-session passphrase.

## Tasks

### Phase A — Connections storage

**Task 1 — Migration `0003_connections`**
- Up creates the table with the columns above and indexes on `alias` (unique)
  and `tag`.
- Down drops it.

**Task 2 — `ConnectionsRepo`**
- `Create(ctx, params)` — generates DEK, wraps with MK, encrypts every
  sensitive field, inserts.
- `Get(ctx, id)` — returns the row's decryptable shell (no plaintext until
  `Reveal` is called).
- `List(ctx)` — returns metadata only (no decryption).
- `Delete(ctx, id)`.
- `Reveal(ctx, id, passphrase string)` — unwraps DEK, decrypts every field;
  if `requires_passphrase` is true and `passphrase == ""` returns
  `ErrPassphraseRequired`. Returns the connection material that the postgres
  Manager needs to build a DSN.
- Tests round-trip every field, reject wrong passphrase, allow empty
  passphrase when `requires_passphrase=false`, refuse to overwrite alias.

### Phase B — Postgres driver + Manager

**Task 3 — `internal/postgres/driver.go`**
- `type Conn struct { Host, Port, Username, Password, Database, TLSMode string }`
  — what Manager needs to dial. Constructed from `ConnectionsRepo.Reveal`.
- `type Probe struct { Server string; Version string; SuperuserOK bool;
  HasPgStatStatements bool }` — what `/test` returns.
- `type Driver interface { Open(ctx, Conn) (Pool, error); Probe(ctx, Pool)
  (Probe, error); Close(Pool) }`.
- `type Pool interface { Ping(ctx) error; Close() }` — minimal surface
  Plan 3 needs; Plan 4 extends with `Query/Exec`.

**Task 4 — `internal/postgres/pgx.go`**
- `func New() Driver` returns the pgx-backed implementation.
- `Open` builds a DSN (`host=... port=... user=... password=... dbname=...
  sslmode=...`), opens a `pgxpool.Pool` with sensible limits
  (MaxConns=4, MinConns=0, MaxConnIdleTime=5m).
- `Probe` runs `SELECT version()`, checks `current_user`'s `pg_monitor`
  membership, checks pg_stat_statements via `SELECT 1 FROM
  pg_extension WHERE extname='pg_stat_statements'`.
- `Close` calls `pool.Close()`.

**Task 5 — `internal/postgres/manager.go`**
- `type Manager struct { Driver; ConnsRepo; map[int64]Pool; sync.Mutex }`.
- `OpenByID(ctx, id, passphrase)` — Reveal -> Driver.Open -> cache the pool.
- `Close(id)` — closes the cached pool.
- `Ping(ctx, id, passphrase)` — opens if needed, returns ping error.
- `Probe(ctx, id, passphrase)` — opens if needed, returns full Probe.
- Tests with `*Driver` mock (table-driven open errors, cached pool returns
  the same instance).

### Phase C — HTTP handlers

**Task 6 — `internal/server/handlers/connections.go` (all under RequireAuth)**
- `GET /api/connections` — list.
- `POST /api/connections` — create. Body:
  `{alias, host, port, tag, tls_mode, username, password, database,
    passphrase?}`. If `passphrase` non-empty, sets `requires_passphrase=true`.
  Audit: `connection.create`.
- `GET /api/connections/{id}` — metadata (no creds).
- `DELETE /api/connections/{id}` — also closes any cached pool. Audit:
  `connection.delete`.
- `POST /api/connections/{id}/test` — runs Manager.Probe, returns JSON.
  Header `X-Connection-Passphrase` carries the optional passphrase. Audit:
  `connection.test`.

**Task 7 — Mount + lint-auth allowlist unchanged**
- Update `Mount` to register these under the existing RequireAuth group.
- Re-run `lint-auth` to confirm no new unauthed routes.

### Phase D — E2E with real Postgres

**Task 8 — `tests/e2e/connections_test.go`**
- Skips when Docker is unavailable: `if _, err := exec.LookPath("docker");
  err != nil { t.Skip("docker not available") }`. Also catches the
  testcontainers `daemon: not running` error.
- Starts `postgres:16-alpine` via testcontainers-go.
- Builds dbil binary, runs init.
- Starts serve.
- Logs in.
- POSTs a connection pointing at the testcontainer.
- POSTs `/test` -> 200, response includes `version` containing "PostgreSQL".
- DELETEs the connection -> 204.
- Re-GETs -> 404.

### Phase E — Tag + finalize

**Task 9**
- `go test ./...` (testcontainers test will run if Docker is up).
- `make lint-auth`.
- Coverage check.
- Tag `v0.3.0-connections`.

## Risks acknowledged

- **No SSH tunnel yet.** Connections that need an SSH bastion will need to
  expose Postgres directly to DBil — that's actually correct for the compose
  drop-in use case. Tunnel support lands in Plan 3.5.
- **No IAM auth (RDS / Cloud SQL).** Same reasoning — opt-in feature, not
  on the critical path for the first Postgres connection.
- **Per-connection passphrase contract exists but no UX.** The API requires
  `X-Connection-Passphrase` for production-tagged connections, but until
  Plan 5's SPA ships there's no friendly way to enter it. curl callers
  must send the header manually.
- **No "test connection during create" yet.** Create only inserts; if the
  credentials are wrong the user sees it on the first `/test` call.

## End-to-end verification

After Plan 3 ships (with Docker available):

```bash
DATA=$(mktemp -d) && ./bin/dbil init
./bin/dbil serve &
TOKEN=$(curl -s -X POST http://localhost:4242/api/auth/login \
  -d '{"email":"admin@local","password":"'$(grep ^password $DATA/initial-credentials.txt|cut -d= -f2)'"}' \
  -H 'Content-Type: application/json' | jq -r .token)

# Start a local Postgres for demo:
docker run -d --rm --name pg -p 5432:5432 -e POSTGRES_PASSWORD=secret postgres:16-alpine
sleep 2

CONN=$(curl -s -X POST http://localhost:4242/api/connections \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"alias":"local","host":"127.0.0.1","port":5432,"username":"postgres",
       "password":"secret","database":"postgres","tag":"local","tls_mode":"disable"}')
ID=$(echo "$CONN" | jq -r .id)

curl -s -X POST http://localhost:4242/api/connections/$ID/test \
  -H "Authorization: Bearer $TOKEN"   # -> {"server":"...","version":"PostgreSQL 16..."}

curl -s -X DELETE http://localhost:4242/api/connections/$ID \
  -H "Authorization: Bearer $TOKEN" -w 'status=%{http_code}\n'
```

The e2e test in `tests/e2e/connections_test.go` automates these steps and
skips cleanly when Docker is not running.
