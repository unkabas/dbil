# DBil v0.1 — Plan 8 (Compose auto-discovery)

## Context

Spec section 7 says DBil drops into an existing `docker-compose` project
and surfaces Postgres services without manual configuration. v0.8.0
ships the two non-magic discovery levels:

- **Level 1 — env JSON** (`DBIL_AUTO_CONNECT`): zero socket access,
  declarative.
- **Level 2 — Docker socket label scan** (`DBIL_DISCOVER=docker`):
  reads `dbil.*` labels off containers on the configured network,
  resolves credentials by inspecting target-container env vars.

Both feed the same `discovered_connections` table; the UI lists pending
items and the user explicitly **approves** before any connection is
created. The approval gate is the security boundary that prevents a
rogue container from silently joining the connection list.

## Scope (v0.8.0)

| In | Out (v0.8.1+) |
|---|---|
| `internal/discover` package: env JSON parser + Docker socket scanner | Docker event-driven live updates (we poll on a 30 s tick) |
| New table `discovered_connections` + migration `0005_discovered.up.sql` | Auto-mark "unreachable" when containers leave (lazy: only updated on next scan) |
| `GET /api/discover` — list pending + approved + rejected entries | Network-filtered TLS / TLS-mode hints (always `disable` from auto-discover) |
| `POST /api/discover/{id}/approve` — promote one entry to a real connection | UI for editing discovered entries before approval |
| `POST /api/discover/{id}/reject` — mark rejected; never resurfaced | Per-container passphrase enforcement on `production` tag (deferred to v0.8.1) |
| Background scanner started by `dbil serve` when `DBIL_DISCOVER` is set | Hot-plug socket re-attach |
| TopNav "Discover" pill with pending count + new `/discover` page | |
| Audit entries: `discover.detected`, `discover.approved`, `discover.rejected` | |

## Tech additions

- **`github.com/docker/docker/client`** (stdlib-style API, no CGO) — for
  the Level-2 socket reader. The package supports Docker engine API
  through both unix:// and tcp:// hosts.

Add as a Go dep with `go get`; vendored module tree stays clean.

## File structure added

```
internal/discover/                  # new
  types.go                          # DiscoveredEntry + status enum
  env.go                            # DBIL_AUTO_CONNECT parser
  env_test.go
  docker.go                         # DockerScanner: list, label-parse, resolve creds
  docker_test.go                    # fakeClient implementing the dockerLister iface
  manager.go                        # Manager: tick loop, dedup by source-key, persists via Repo
  manager_test.go
internal/store/migrations/
  0005_discovered.up.sql            # discovered_connections table
  0005_discovered.down.sql
internal/store/
  discovered.go                     # DiscoveredRepo: Upsert, List, Approve, Reject
  discovered_test.go
internal/server/handlers/
  discover.go                       # GET /api/discover, approve, reject
  handlers.go                       # MODIFIED — Deps + routes
cmd/dbil/
  serve_cmd.go                      # MODIFIED — start the discover manager
web/src/api/
  discover.ts                       # types + hooks
web/src/pages/
  DiscoverPage.tsx                  # list + approve/reject
web/src/components/
  TopNav.tsx                        # MODIFIED — pending count pill
```

## Data model on the wire

```ts
// /api/discover
interface DiscoverResponse {
  entries: Array<{
    id: number
    source: 'env' | 'docker'
    source_key: string          // hash to deduplicate across scans
    alias: string
    host: string
    port: number
    database: string
    username: string
    tag: 'local' | 'dev' | 'staging' | 'production'
    has_password: boolean       // never send the password to the wire
    status: 'pending' | 'approved' | 'rejected' | 'unreachable'
    last_seen_ms: number
    created_at_ms: number
  }>
}

// /api/discover/{id}/approve
//   body: { passphrase?: string }   // required when target tag === "production"
//   200:  { connection_id: number }
//   412:  passphrase required
```

## Wire-up summary

`dbil serve` builds `discover.Manager` from a `discover.Config` parsed
from env vars (`DBIL_DISCOVER`, `DBIL_AUTO_CONNECT`, `DBIL_NETWORK`).
On startup the manager runs one immediate scan and starts a 30 s tick.
Detected entries upsert into `discovered_connections`; approving an
entry creates a real `connections` row through the existing repo (so
the existing pgx.Manager picks it up).

## Tasks

### Phase A — Storage

**Task 1 — Migration `0005_discovered.up.sql`:**

```sql
CREATE TABLE discovered_connections (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  source          TEXT NOT NULL CHECK (source IN ('env','docker')),
  source_key      TEXT NOT NULL,           -- e.g. "docker:<container-id>:<port>"
  alias           TEXT NOT NULL,
  host            TEXT NOT NULL,
  port            INTEGER NOT NULL,
  database        TEXT NOT NULL,
  username        TEXT NOT NULL,
  password_enc    BLOB,                    -- envelope-encrypted via store DEK
  password_nonce  BLOB,
  tag             TEXT NOT NULL,
  status          TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','approved','rejected','unreachable')),
  last_seen_ns    INTEGER NOT NULL,
  created_at_ns   INTEGER NOT NULL,
  approved_conn_id INTEGER REFERENCES connections(id) ON DELETE SET NULL,
  UNIQUE (source, source_key)
);
CREATE INDEX idx_discovered_status ON discovered_connections(status);
```

**Task 2 — `internal/store/discovered.go`:**

- `DiscoveredRepo` constructed with the same `MasterKey` as ConnectionsRepo.
- `Upsert(ctx, entry)`: if `(source, source_key)` exists in `pending` or
  `unreachable`, update `last_seen_ns` and refresh fields; in
  `rejected`, no-op; in `approved`, refresh `last_seen_ns` only.
- `List(ctx, statusFilter ...string)`: list with optional filter.
- `Approve(ctx, id) (decryptedPassword string, entry Entry, err error)`:
  decrypts password, sets `status='approved'`, returns the password to
  the caller so the existing ConnectionsRepo.Create runs normally.
- `Reject(ctx, id)`: status → `rejected`.
- `MarkUnreachable(ctx, source string, keepKeys []string)`: any
  `pending` entries for `source` whose `source_key` is not in
  `keepKeys` become `unreachable` (run after each scan).
- Password encryption: use the same audit-DEK trick as audit details
  (`HKDF(MK, "dbil:discover-dek-v1", 32)`) to keep state migrations
  simple.

**Task 3 — `internal/store/discovered_test.go`:** insert, upsert
idempotent on second tick, password round-trip, status transitions,
mark-unreachable.

### Phase B — Env JSON discoverer

**Task 4 — `internal/discover/types.go`:**

```go
type Source string
const (
    SourceEnv    Source = "env"
    SourceDocker Source = "docker"
)
type Status string
const (
    StatusPending     Status = "pending"
    StatusApproved    Status = "approved"
    StatusRejected    Status = "rejected"
    StatusUnreachable Status = "unreachable"
)
type Entry struct {
    Source   Source
    Key      string
    Alias    string
    Host     string
    Port     int
    Database string
    Username string
    Password string
    Tag      string
}
```

**Task 5 — `internal/discover/env.go`:**

`func ParseEnvJSON(raw string) ([]Entry, error)` — accepts:

```json
[
  {"alias":"app", "host":"postgres", "port":5432,
   "user":"app", "password":"…", "database":"appdb", "tag":"dev"}
]
```

Validations: alias non-empty, host non-empty, port 1-65535, tag in the
known set. Source key = `"env:" + sha256(alias|host|port|database)[:16]`.

**Task 6 — `internal/discover/env_test.go`:** valid JSON; missing
fields; invalid tag; deterministic key.

### Phase C — Docker socket discoverer

**Task 7 — `internal/discover/docker.go`:**

- `type dockerLister interface { ContainerList(ctx, ListOptions) ([]Container, error); ContainerInspect(ctx, id) (ContainerJSON, error) }` —
  the small shape we need from `client.Client`.
- `type DockerScanner struct { Client dockerLister; Network string }`.
- `Scan(ctx)` returns `[]Entry`:
  1. List all containers (only running).
  2. Filter by `dbil.enable == "true"`.
  3. If `Network != ""`, drop containers not attached to that network.
  4. For each, read labels:
     - `dbil.alias` — defaults to container name.
     - `dbil.tag` — defaults to `"dev"`.
     - `dbil.port` — defaults to `5432`.
     - `dbil.creds.username_env`, `dbil.creds.password_env`,
       `dbil.creds.database_env`.
  5. Inspect the target container to obtain its env vars; resolve the
     three `*_env` references to actual values. Missing/blank env
     entries skip the container with a `discover.skip` log line and a
     `reason`.
  6. Host = container's IP on the named network (or `<container-name>`
     when only the default bridge is in use).
- Source key: `"docker:" + container.ID[:16]`.

**Task 8 — `internal/discover/docker_test.go`:** fake client returning
canned `[]Container` + `ContainerJSON`. Cases: enable=false skipped;
missing username_env yields skip-with-reason; happy path resolves
host+port+creds.

### Phase D — Manager + persistence

**Task 9 — `internal/discover/manager.go`:**

- `Config { Mode string; AutoConnectJSON string; DockerHost string; Network string }`.
- `NewManager(cfg, repo, audit, log) *Manager`.
- `RunOnce(ctx)`: build entries from env + docker (per Mode), call
  `Repo.Upsert`, then `Repo.MarkUnreachable(source, seenKeys)`,
  appending `discover.detected` audit entries for **new** items only.
- `Start(ctx)`: launches a goroutine that ticks every 30 s; tick
  deadline 8 s like the observ collector.
- `Shutdown()`: cancels the goroutine; waits for it.

**Task 10 — `internal/discover/manager_test.go`:** Manager with
in-memory repo + fake docker + env injected JSON; first tick discovers,
second tick refreshes (no duplicate audit), removed container → marked
unreachable.

### Phase E — HTTP

**Task 11 — `handlers/discover.go`:**

- `GET /api/discover` — returns `entries` sorted by `created_at_ns` desc.
- `POST /api/discover/{id}/approve` body `{passphrase?: string}`:
  loads entry, decrypts password through DiscoveredRepo, calls
  `ConnectionsRepo.Create` with passphrase, sets `approved_conn_id`,
  writes `discover.approved` audit.
- `POST /api/discover/{id}/reject` — sets status; audit.

**Task 12 — Wire-up:**

- Extend `handlers.Deps` with `Discovered *store.DiscoveredRepo`,
  `DiscoverMgr *discover.Manager` (latter optional, nil-safe).
- Routes mounted under `RequireAuth`; lint-auth must stay green.

### Phase F — `dbil serve`

**Task 13 — `cmd/dbil/serve_cmd.go`:**

- Parse `DBIL_DISCOVER` (`""|env|docker|both`), `DBIL_AUTO_CONNECT`,
  `DBIL_NETWORK`.
- If non-empty, build the manager, wire docker client via
  `client.NewClientWithOpts(client.WithHost(dockerHost),
  client.WithAPIVersionNegotiation())`.
- Start the manager after the observ manager; shut down before it on
  signal.

### Phase G — Frontend

**Task 14 — `web/src/api/discover.ts`:** types + hooks
`useDiscovered`, `useApproveDiscover`, `useRejectDiscover`.

**Task 15 — `web/src/components/TopNav.tsx`:** existing menu gains a
"Discover" item; when pending count > 0, show a small amber pill with
the count.

**Task 16 — `web/src/pages/DiscoverPage.tsx`:** table of entries with
status badges, alias/host/port/tag columns, Approve/Reject actions.
Approve dialog asks for passphrase only when the entry's tag is
`production`.

### Phase H — Tests + tag

**Task 17 — `go test ./...` + `make lint-auth` + `npm run build`
green. Tag `v0.8.0-discover`.

## Risks acknowledged

- **Docker socket access** is broad; the spec demands read-only mount
  (`:ro`). We never call container-create / network-create / exec
  endpoints, so the read-only mount is sufficient. Document this in
  `examples/docker-compose.dev.yml`.
- **Credential lookup by env-var name** trusts target container env
  vars are present and named per labels. Missing vars surface as
  skipped entries with a clear reason in logs — not a runtime error.
- **Polling at 30 s** means container churn is observed lazily. Live
  Docker events go into v0.8.1.
- **Approval is the only security boundary;** a malicious container
  setting `dbil.*` labels can advertise itself but cannot create a
  real connection until a logged-in user approves. The UI surfaces the
  source (`env` vs `docker`) on every entry.
