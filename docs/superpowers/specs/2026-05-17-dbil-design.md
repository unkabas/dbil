# DBil — Design Specification

- **Date:** 2026-05-17
- **Status:** Draft v1 (pending user review)
- **Spec scope:** MVP (v0.1) with explicit roadmap to v1.0
- **License:** Apache 2.0 with CLA from contributors

## 1. Summary

DBil is an open-source production-grade PostgreSQL tool, distributed as a single ~25MB Docker container. Its two value propositions are:

1. **Dev experience:** drop into an existing `docker-compose` project, auto-discover Postgres services, and immediately get a GUI/SQL editor that replaces TablePlus/pgAdmin without any manual configuration.
2. **Production experience:** point at a production Postgres database to get pgAnalyze-class observability (slow queries, locks, missing/unused indexes, bloat, replication lag) for free as open source.

A single binary supports both **solo** and **team** modes through one authentication code path. Security is treated as a first-class feature: envelope encryption (master key → data encryption keys → credentials), per-connection passphrase for production-tagged databases, tamper-evident audit log, and hardened container defaults.

## 2. Goals (MVP, v0.1, ~3 months)

- One static Go binary; Docker image ≤25MB; RAM ≤50MB at idle on linux/amd64 and linux/arm64.
- Default UI port **4242** (configurable via `DBIL_PORT`). Chosen because 8080/3000/5000 frequently collide with project backends.
- Hybrid solo/team mode in one binary, identical authentication middleware for both.
- PostgreSQL connectivity: SSL/TLS, SSH tunnels, IAM authentication (AWS RDS, Google Cloud SQL).
- Connection tagging: `local` / `dev` / `staging` / `production`, with default policies inherited per tag.
- Auto-discovery of Postgres services in the same Docker network via read-only Docker socket and `dbil.*` container labels. Optional fallback via `DBIL_AUTO_CONNECT` JSON environment variable when the Docker socket is not available.
- SQL editor with schema-aware autocompletion (CodeMirror 6 + Postgres grammar).
- ER diagram view (React Flow).
- **Wow feature: production-grade observability** with active sessions, lock waiting chains, slow query analysis, index advisor, bloat detector, replication lag, connection pool monitor.
- Encryption-at-rest for all stored credentials and session state via SQLCipher + envelope encryption.
- Tamper-evident audit log with hash chain.
- Hardened container: non-root UID 1000, read-only root filesystem, dropped capabilities, distroless base image.

## 3. Non-goals (MVP)

- Non-PostgreSQL databases (MySQL, MongoDB, Redis, etc.). The driver interface is in place from day one for testability, but a single real implementation ships.
- AI-native features (planned v0.2).
- Schema-as-code / migration UI (planned v0.3).
- Live multi-user collaboration on the same query (planned v1.0).
- Granular RBAC at the table/column level (MVP grants permissions per-connection only).
- SaaS hosting; DBil is self-hosted only.
- Formal SOC 2 / HIPAA certification (cost-driven, deferred indefinitely).

## 4. Tech stack

- **Language (backend):** Go. Static binary, `embed.FS` for SPA, `pgx` (best-in-class Postgres driver in any ecosystem), distroless base.
- **Language (frontend):** React 18 + Vite. Compiled SPA embedded in the binary. React chosen over Svelte for ecosystem breadth (SQL editors, data grids, charts are React-first) and contributor availability.
- **Editor component:** CodeMirror 6 with a Postgres grammar and a custom schema-aware completion source. (Monaco rejected as too heavy.)
- **Data grid:** TanStack Table v8 with virtualization for million-row results.
- **Charts:** uPlot (≈40KB, fast on large time series).
- **Diagram:** React Flow.
- **HTTP routing:** `chi` for REST; `coder/websocket` for live observability streams.
- **State store:** `modernc.org/sqlite` (pure Go, no CGO) with **application-layer envelope encryption** of sensitive columns (credentials, audit details). The DB file itself is not encrypted at filesystem level; field-level AES-256-GCM via the MK→DEK envelope provides the at-rest protection. This trades file-level encryption (SQLCipher would have required CGO and broken clean cross-compilation to linux/arm64) for a clean static binary, while keeping the same effective threat model — leaking the DB file without the MK yields ciphertext for every sensitive value. Migrations via `golang-migrate`. Generated query code via `sqlc`.
- **Build:** Multi-arch Docker images for linux/amd64 and linux/arm64. Optional cosign-signed images.

## 5. Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                         Browser (React SPA)                      │
└────────────────────────────┬─────────────────────────────────────┘
                             │ HTTPS / WSS
                ┌────────────▼─────────────┐
                │      HTTP Server (chi)   │
                │  middleware: requireAuth │  ← single path, solo + team
                │  middleware: audit       │
                │  middleware: rate-limit  │
                └─────┬────────┬───────────┘
                      │        │
       ┌──────────────▼─┐    ┌─▼──────────────┐
       │ Auth subsystem │    │ API handlers   │
       │  sessions      │    │  /connections  │
       │  users / RBAC  │    │  /query        │
       │  OIDC adapter  │    │  /schema       │
       └────────┬───────┘    │  /observ       │
                │            │  /audit        │
                │            └────┬───────────┘
                │                 │
                │       ┌─────────▼──────────────────────────┐
                │       │     Connection Manager             │
                │       │  pool per connection (pgx)         │
                │       │  TLS, SSH tunnel, IAM              │
                │       │  tag-based policy enforcement      │
                │       └────┬────────────────┬──────────────┘
                │            │                │
                │            │       ┌────────▼──────────────┐
                │            │       │ Postgres Driver iface │
                │            │       │ (single real impl,    │
                │            │       │  iface for tests +    │
                │            │       │  future engines)      │
                │            │       └───────────┬───────────┘
                │            │                   │
                │            │                   ▼ user's Postgres
                │            │
                │   ┌────────▼────────┐    ┌────────────────────┐
                │   │ Query Executor  │    │ Schema Introspector│
                │   │  safety guards  │    │  pg_catalog reads  │
                │   │  audit hooks    │    │  TTL cache         │
                │   └─────────────────┘    └────────────────────┘
                │
                │   ┌─────────────────────────────────────┐
                │   │  Observability Collector            │
                │   │ goroutine per connection            │
                │   │  pg_stat_statements                 │
                │   │  pg_stat_activity, pg_locks         │
                │   │  index / bloat advisors             │
                │   │  writes hot/warm/cold buckets       │
                │   └────────────┬────────────────────────┘
                │                │
       ┌────────▼────────────────▼──────────────────────┐
       │                Crypto Layer                    │
       │  MK loader chain (KMS → keychain → file → env  │
       │    → TTY)                                      │
       │  envelope encryption (MK → DEK → creds)        │
       │  per-connection passphrase (memory-only)       │
       │  master-key rotation                           │
       └────────────────┬───────────────────────────────┘
                        │
       ┌────────────────▼────────────────────────────────┐
       │           State Store (SQLCipher)               │
       │  users, sessions, RBAC                          │
       │  connections (encrypted creds)                  │
       │  observability metrics (hot / warm / cold)      │
       │  audit log (append-only, hash chain)            │
       │  saved queries, snapshots                       │
       └─────────────────────────────────────────────────┘

       ┌─────────────────────────────────────────────────┐
       │       Compose Discovery (optional)              │
       │  reads `dbil.*` labels via read-only Docker     │
       │   socket OR `DBIL_AUTO_CONNECT` env JSON        │
       └─────────────────────────────────────────────────┘
```

### Component boundaries

- **Postgres Driver interface:** one real implementation (`pgx`), interface from day one for unit-test mocks.
- **Connection Manager** does not know about users or audit.
- **API handlers** do not know about encryption.
- **Crypto Layer** does not know about Postgres.
- **Query Executor** is the single location where write operations to user databases happen; all safety guards (read-only flag, prod-tag policy, second approval) live here.
- **State Store** owns migrations from day one (`golang-migrate`).

### Data flow

- **Hot path (query execution):** Browser → middleware (auth + audit) → API handler → Query Executor → `pgx` pool → Postgres → result back. Audit log writes intent before execution and result after.
- **Cold path (observability):** independent goroutines per connection, polling at tag-dependent intervals, panic-isolated, writing into SQLite metric buckets.
- **Crypto path:** master key loaded once at startup via the loader chain. DEKs decrypted on demand, kept in-memory, zeroed on shutdown. No plaintext key ever written to disk.

## 6. Security model

### 6.1 Threat model

| Threat | Defense | Location |
|---|---|---|
| Leaked host env vars (`ps`, `/proc`, debugger) | MK not in env by default; env is last resort with warning log | Crypto Layer |
| Leaked `dbil-data` volume (backup/snapshot) | All sensitive columns (credentials, audit details) AES-256-GCM encrypted at the field level via the MK→DEK envelope; leaked file yields ciphertext for sensitive values | State Store |
| Leaked SQLite file plus leaked MK | Per-connection passphrase for `production`-tagged connections; credentials remain unreadable | Crypto Layer + Connection Manager |
| Host root compromised | Out of scope; KMS-backed MK reduces blast radius | KMS opt-in |
| Insider with container access | Per-connection passphrase, audit to external syslog, secure-erase on credential deletion | Crypto + Audit |
| Brute force of leaked encrypted DEK | argon2id (m=64MB, t=3, p=4); MK ≥256-bit; KDF salt stored in file | Crypto Layer |
| MITM DBil ↔ Postgres | TLS required for `production` connections; optional server cert pinning | Connection Manager |
| MITM Browser ↔ DBil | TLS required in team mode (recommended via Caddy/Traefik); self-signed warning in solo | HTTP Server |
| Stolen session cookie | Secure + HttpOnly + SameSite=Lax; short TTL for production operations; optional IP/UA binding | Auth |
| SQL injection in DBil's own SQLite | All queries via `sqlc`-generated prepared statements | State Store |
| Brute force of DBil user passwords | argon2id + rate limit per user per IP + lockout | Auth |
| Race condition during MK rotation | Exclusive lock; new DEK written atomically, then old removed | Crypto Layer |
| Audit log tampering | Hash chain: `entry_hash = sha256(prev_hash ‖ canonicalize(entry))`; checkpoint exported to syslog every N entries | Audit |
| Heavy query DoS | Per-tag statement timeout (5s dev, 30s prod-read, 5m admin); connection pool limits | Query Executor |
| Curious analyst exfiltration | RBAC per connection (MVP); table/column-level in post-MVP | Auth |

### 6.2 Crypto specifics

- **Master Key (MK):** 32 random bytes from `crypto/rand`, base64url for display.
- **DEK wrap key:** `HKDF-SHA256(MK, info="dbil:dek-wrap-v1")` → 32 bytes. (Previously also a SQLite-file key was derived; with field-level encryption replacing file-level, only the wrap key is needed.)
- **DEK storage:** `AES-256-GCM(wrap_key, DEK, nonce, aad="conn:"+conn_id)`.
- **Credentials encryption:** `AES-256-GCM(DEK, plaintext, nonce, aad="creds:"+conn_id+":"+version)`. Plaintext fields include hostname, port, database, username, password, ssh key, ssl cert.
- **Per-connection passphrase wrapping** (for `production`): user enters in UI; `argon2id(passphrase, conn_salt, m=64MB, t=3, p=4)` → 32-byte key; additional `AES-256-GCM` wrap applied on top of the DEK ciphertext. Passphrase-derived key lives only in process memory, bound to the session, dropped on logout or TTL expiry.
- **DBil user passwords:** argon2id (m=64MB, t=3, p=4), 16-byte salt, constant-time compare.
- **Session tokens:** 32 random bytes, base64url. Stored as `sha256(token)`. TTL: 7d solo default, 12h team default, 1h for production-operation sessions.

### 6.3 Master Key loader chain

In priority order, first available wins, fallback to next on miss:

1. **External KMS** (AWS KMS, GCP KMS, Azure Key Vault, HashiCorp Vault) via `DBIL_KMS_PROVIDER` + key reference. Key material never leaves the KMS; we receive only decrypted DEKs on demand. Recommended for production team deployments.
2. **OS keychain** (macOS Keychain, libsecret on Linux, Windows DPAPI). Used outside the container context.
3. **Mounted secret file** at `DBIL_MASTER_KEY_FILE` (default `/run/secrets/dbil_master_key`, mode 0400). Recommended for Docker Compose with `secrets:` blocks.
4. **Environment variable** `DBIL_MASTER_KEY`. Marked as dev-only in documentation; emits a startup warning.
5. **Interactive passphrase via TTY**, for air-gapped or paranoid setups; not usable for headless Docker.
6. **Auto-generated fallback** (solo zero-config first run only): MK is generated, written to `/data/master.key` mode 0400, a prominent warning is logged with a link to the production hardening guide.

### 6.4 Audit log format

```
entry = {
  id:        monotonic uint64,
  ts:        nanoseconds since unix epoch,
  user_id:   string,
  action:    "query.execute" | "conn.create" | "auth.login" | ...,
  resource:  "conn:42" | "user:7" | ...,
  details:   canonical JSON,
  prev_hash: 32 bytes,
}

entry_hash = sha256(
  uint64_be(id)
  ‖ uint64_be(ts)
  ‖ user_id
  ‖ action
  ‖ resource
  ‖ canonical_json(details)
  ‖ prev_hash
)
```

- Genesis: `prev_hash = sha256("dbil-audit-genesis-v1")`.
- Every 1000 entries, `(id, entry_hash)` is exported to external syslog when `DBIL_AUDIT_SYSLOG` is configured.
- Tampering with any past entry breaks every subsequent hash; integrity check on startup; UI banner on mismatch.

### 6.5 Container hardening

- Non-root UID 1000 created in Dockerfile.
- Read-only root filesystem (`--read-only`), writable mount only at `/data`.
- `--cap-drop=ALL --security-opt=no-new-privileges`.
- Distroless base (`gcr.io/distroless/static-debian12:nonroot`); no shell, no coreutils.
- Healthcheck endpoint `/healthz` (unauthenticated, minimal payload).
- Multi-architecture images: linux/amd64, linux/arm64. Cosign-signed images opt-in.

### 6.6 Network model

```
┌─ docker-compose project ────────────────────────┐
│                                                 │
│  ┌─────┐   ┌─────┐   ┌──────────┐   ┌──────┐    │
│  │ web │   │ api │   │ postgres │   │ dbil │    │
│  └──┬──┘   └──┬──┘   └────┬─────┘   └──┬───┘    │
│     │         │           │            │        │
│     └─────────┴── appnet ─┴────────────┘        │
│                                                 │
└────────────┬────────────────────────────────────┘
             │ only DBil UI port (:4242) exposed
             ▼
       host:4242 ─→ browser
```

- Postgres ports never need host exposure; DBil reaches it by service name inside the network.
- DBil exposes only its UI port to host.
- Team-mode deployments should sit behind Caddy or Traefik with auto-TLS (Let's Encrypt). DBil does not terminate TLS itself in MVP.
- CORS: strict same-origin. SPA served from the same origin as the API.
- CSP: strict; no `unsafe-inline`; nonces for inline styles.

## 7. Compose drop-in and auto-discovery

### 7.1 Three discovery levels

- **Level 0 — no discovery.** Add connections manually through the UI. This is the default; nothing to configure.
- **Level 1 — env-based, no Docker socket.** DBil reads `DBIL_AUTO_CONNECT` containing JSON connection descriptors; credentials reference target container env vars by name. Lower magic, lower attack surface.
- **Level 2 — Docker socket label scanning (opt-in).** Read-only mount of `/var/run/docker.sock`. DBil listens to Docker events, filters by `DBIL_NETWORK`, reads `dbil.*` labels, presents matches in the UI as **discovered, pending approval**.

### 7.2 Discovery behavior (Level 2)

1. Container with `dbil.enable=true` detected on the configured network.
2. Labels read: `dbil.alias`, `dbil.tag`, `dbil.port` (default 5432), `dbil.creds.username_env`, `dbil.creds.password_env`, `dbil.creds.database_env`.
3. Credentials resolved by inspecting the target container's own environment variables (which already exist for the application to consume).
4. Discovered connection enters `pending approval` state; user clicks **Approve** in the UI; for `production`-tagged connections, the per-connection passphrase prompt fires before first use.
5. Containers leaving the network mark connections `unreachable`; connections are not auto-deleted.

The approval step exists specifically to prevent a rogue container with `dbil.*` labels from being silently added to the connection list.

### 7.3 Reference compose snippet

```yaml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: app
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: appdb
    labels:
      dbil.enable: "true"
      dbil.alias: "App DB"
      dbil.tag: "dev"
      dbil.creds.username_env: "POSTGRES_USER"
      dbil.creds.password_env: "POSTGRES_PASSWORD"
      dbil.creds.database_env: "POSTGRES_DB"
    networks: [appnet]

  dbil:
    image: dbil/dbil:latest
    ports: ["4242:4242"]
    volumes:
      - ./dbil-data:/data
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      DBIL_DISCOVER: "docker"
      DBIL_NETWORK: "appnet"
      DBIL_MASTER_KEY_FILE: "/run/secrets/dbil_master_key"
    secrets: [dbil_master_key]
    networks: [appnet]
    depends_on: [postgres]

secrets:
  dbil_master_key:
    file: ./secrets/dbil_master_key

networks:
  appnet:
```

### 7.4 Connection tag policies

Tags drive default policies; users can override per-connection.

| Policy | `local` | `dev` | `staging` | `production` |
|---|---|---|---|---|
| TLS required | no | no | yes | yes (refuse without TLS) |
| UI highlight | gray | green | yellow | red persistent badge |
| Statement timeout | 5 min | 30 s | 30 s | 10 s read / 60 s admin |
| DML allowed | yes | yes | confirm | confirm + whitelist comparison |
| DDL allowed | yes | yes | confirm | second approval required |
| Per-connection passphrase | no | no | optional | required |
| Audit retention | 7 d | 30 d | 90 d | 365 d |
| Schema cache TTL | 30 s | 5 m | 30 m | 1 h |
| Observability polling | 60 s | 30 s | 10 s | 5 s |
| `DELETE`/`UPDATE` without `WHERE` | warning | warning | block + typed confirmation | block + second approval |

## 8. Observability scope

### 8.1 Data sources

- `pg_stat_activity` — active sessions, what they execute, for how long, waiting-chain construction.
- `pg_locks` joined with `pg_stat_activity` — blocking-graph visualization.
- `pg_stat_statements` — slow query rankings, p50/p95/p99, frequency, cache hits.
- `pg_stat_user_tables` / `pg_stat_user_indexes` — unused, duplicate, and likely-missing indexes.
- `pg_class` + `pgstattuple` if available, fallback formula otherwise — bloat estimation.
- `pg_stat_replication` — replication lag.
- `pg_stat_database` — cache hit ratio, deadlocks, temp file usage.

### 8.2 UI pages

1. **Overview** — TPS, connection count, cache hit ratio, replication lag, active lock chains.
2. **Slow queries** — table of `pg_stat_statements` entries with per-query trend chart.
3. **Locks** — directed graph of waiting chains; **kill backend** action (with confirmation and audit entry).
4. **Index advisor** — recommendations with copy-ready `CREATE INDEX` and reasoning.
5. **Bloat** — tables and indexes with `VACUUM`/`REINDEX` recommendations.
6. **Sessions** — `pg_stat_activity` with filters; kill action available.

### 8.3 Storage of metrics

- **Hot:** raw samples at 5–10s resolution for 24 h.
- **Warm:** 1-minute aggregates for 7 days.
- **Cold:** 1-hour aggregates for 90 days, compressed.
- Optional Prometheus/OpenTelemetry export via `/metrics` endpoint for long-term external storage.

### 8.4 Required Postgres permissions

Read-only role with `pg_monitor` (or `rds_superuser` on RDS, or equivalent on Cloud SQL). With fewer permissions, DBil degrades gracefully: features that require missing roles are marked **requires pg_monitor** in the UI rather than failing silently.

## 9. Solo / team mode behavior

- **First-run bootstrap** (both modes): admin user is created with a random password; password is written to `/data/initial-credentials.txt` (mode 0600) and logged once. Forced password change on first login.
- **Solo mode** = team of one. Same user table, same RBAC, same middleware. The only differences are:
  - "Remember me" defaults to on; sessions live for 7 days.
  - No OIDC; only local password login.
- **Team mode** adds OIDC connectors and additional users via `users.yaml` or the UI. No second authentication code path.
- **Static linter check in CI:** every HTTP handler must call `requireAuth()`. Mechanically enforced rather than relying on convention.

## 10. Roadmap

- **v0.1 (MVP, ~3 months):** everything in this document.
- **v0.2:** AI-native features — schema-aware SQL generation, EXPLAIN translation, optional local LLM via Ollama, MCP server so external agents reach Postgres through DBil's safety and audit layers.
- **v0.3:** Next-generation EXPLAIN visualizer (flame graphs, side-by-side plan comparison, "what if we added this index" simulation).
- **v0.4:** Schema-as-code — UI-driven migrations, schema diff across environments, branch-style schema previews.
- **v1.0:** Live collaboration (multi-cursor SQL editor, auto-refreshing dashboards, shareable RLS-aware snapshots). Compliance pack (PII detection, query approval workflows, session recording).
- **Beyond v1.0:** other database engines (MySQL, MongoDB, ClickHouse, Redis) as separate companion binaries rather than a single fused tool.

## 11. Testing strategy

- **Unit:** crypto layer, audit hash chain, RBAC matrix, query safety guards must hit ≥80% coverage and gate merges.
- **Integration:** real Postgres via `testcontainers-go`; real SQLite. No database mocks in integration tier.
- **E2E:** Playwright against a running binary with a real Postgres; smoke flow includes add connection → execute query → see audit entry → kill session.
- **Fuzz:** SQL safety guards, credentials parser, auth middleware.
- **Static lint:** CI rule that every handler calls `requireAuth()`.
- **Security scanning:** `govulncheck` in CI, dependency audit, `trivy` on Docker images.
- **Load:** one scenario at 100 concurrent users sharing one Postgres connection; observe latency and memory.

## 12. Error handling

- **User SQL errors** are surfaced verbatim with line highlighting; not wrapped or hidden.
- **Connection failures** retry with exponential backoff (max 30 s); connection status reflected in UI.
- **Crypto failures** (corrupt MK, corrupt DEK) fail fast; the system does not attempt automatic repair. Audit entry written; manual intervention required.
- **Audit chain integrity break** is a critical event: external syslog notification fires; UI displays a persistent banner; reads continue but writes that depend on chain integrity are blocked until acknowledged.
- **Out-of-memory protection:** query results larger than 100 MB are either streamed or truncated with a clear warning; never silently OOM.
- **Goroutine panic** (e.g., one observability worker) is recovered, logged, and the worker is restarted; the process stays alive.

## 13. Known tradeoffs

- **Dev drop-in versus production observability tension.** The compose drop-in shines for development; the observability wow feature shines for production. These are two complementary stories under one tool; the README leads with both rather than pretending they are the same magic. The "dev DBil predicts a query's behavior on production" feature deliberately bridges them.
- **Hybrid solo/team in one binary** has a higher auth-bug surface than a clean split. We mitigate with a single code path and a CI-enforced lint rule, but the risk is acknowledged.
- **Postgres-only** sacrifices reach for depth. Users with mixed-database environments will keep DBeaver-class tools around for non-Postgres engines; we must be obviously better at Postgres to be worth the second tool.
- **Docker socket access for auto-discovery** is a real security cost; it is opt-in, documented, and an env-based fallback exists.
- **`pg_stat_statements` requirement** for the most interesting slow-query views means some installations need a parameter change; the UI explains this clearly.
- **Long-term metric retention** is capped at 90 days inside SQLite; longer needs go through Prometheus/OTel export.

## 14. Open questions

- Final Docker Hub / GHCR namespace.
- Funding and sustainability model (donations, optional paid enterprise tier post-v1.0, sponsorships) is deferred until adoption is observable.
- Whether to support a "headless" mode for purely API-driven use (CI pipelines querying DBil for audit data, for example) is deferred to v0.3 unless demand surfaces sooner.
