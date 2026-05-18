# dbil

> A security-first, drop-in PostgreSQL workspace. Single ~20 MB container.
> ER schema view, real-time observability, and locked-down production
> safety — embedded in one static binary.

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![CI](https://github.com/unkabas/dbil/actions/workflows/ci.yml/badge.svg)](https://github.com/unkabas/dbil/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/unkabas/dbil?include_prereleases)](https://github.com/unkabas/dbil/releases)

---

## Why dbil

- **Drops into any `docker-compose`.** Auto-discovers Postgres services via Docker socket labels (Level 2) or env-JSON (Level 1). You approve from the UI before any connection is created.
- **pgAnalyze-class observability** out of the box: TPS, cache hit, sessions, replication lag with sparklines, live `pg_stat_statements` slow-query rankings, and lock-chain graphs from `pg_blocking_pids`.
- **Real schema view** powered by `pg_catalog`: ER diagram with auto-laid-out tables, PK/FK/unique markers, type pills, and a paginated data browser bound to real rows.
- **Tag-driven safety**: every connection is `local`/`dev`/`staging`/`production`. Production blocks DDL outright, requires `X-Confirm: yes` for DML, and refuses `DELETE`/`UPDATE` without a `WHERE`.
- **Security-first by construction**: envelope encryption (master key → per-connection DEK → AEAD field), tamper-evident SHA-256 audit chain, six master-key loader chain (KMS / keychain / mounted secret / env / TTY / auto-generated), and an AST lint that enforces `RequireAuth` on every handler.
- **One static binary, distroless image, no CGO.** Multi-arch (`linux/amd64`, `linux/arm64`), cosign-signed, SBOM-attested.

## Quickstart — drop into your compose

```yaml
# docker-compose.yml
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

  dbil:
    image: ghcr.io/unkabas/dbil:latest
    ports: ["4242:4242"]
    volumes:
      - ./dbil-data:/data
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      DBIL_DISCOVER: "docker"
      DBIL_NETWORK: "default"
```

```bash
docker compose up -d
open http://localhost:4242
```

The first run generates an admin password into
`./dbil-data/initial-credentials.txt`. Log in, head to **Discover**,
approve the postgres service, and you're in.

A production-grade hardened example lives in
[`examples/docker-compose.production.yml`](examples/docker-compose.production.yml).

## Install

### Docker

```bash
docker pull ghcr.io/unkabas/dbil:latest
docker run --rm -p 4242:4242 -v $(pwd)/dbil-data:/data ghcr.io/unkabas/dbil:latest serve
```

Verify the image signature with cosign:

```bash
cosign verify ghcr.io/unkabas/dbil:latest \
  --certificate-identity-regexp='https://github.com/unkabas/dbil/.*' \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

### Native binary

Pre-built binaries for `linux/{amd64,arm64}` and `darwin/{amd64,arm64}`
ship on every [release](https://github.com/unkabas/dbil/releases).

## Features

| Feature | Status |
|---|---|
| ER schema view (real `pg_catalog`) | ✅ |
| Server-paginated data browser | ✅ |
| SQL editor (CodeMirror 6, Postgres grammar) | ✅ |
| Observability: TPS / cache hit / sessions / replication lag | ✅ |
| Slow-query rankings (`pg_stat_statements`) | ✅ |
| Live lock chains (`pg_blocking_pids`) | ✅ |
| Tag-driven DML/DDL safety gates | ✅ |
| Compose auto-discovery (env + Docker socket) | ✅ |
| Envelope-encrypted state at rest | ✅ |
| Tamper-evident audit chain | ✅ |
| `cosign`-signed multi-arch image + SBOM | ✅ |
| `pg_terminate_backend` from the UI | 🛣️ v1.1 |
| Index advisor + bloat detector | 🛣️ v1.1 |
| OS-native `pg_terminate_backend` kill, OTel /metrics export | 🛣️ v1.2 |

## Security model

See [`SECURITY.md`](SECURITY.md). TL;DR:

- Connection credentials never live on disk in plaintext (envelope-encrypted).
- Audit entries are encrypted *and* hash-chained.
- Every API handler is auth-gated (enforced by an AST check in CI).
- Production-tagged connections require a per-connection passphrase
  separate from the master key.
- Container ships as distroless, non-root, ready for `read_only: true`,
  `cap_drop: [ALL]`, and `no-new-privileges`.

## Build from source

```bash
git clone https://github.com/unkabas/dbil
cd dbil
make web-deps tidy
make test
make build       # ./bin/dbil
make docker      # docker build -t dbil:dev .
```

Run locally without Docker:

```bash
DBIL_DATA_DIR=$(mktemp -d) ./bin/dbil init
./bin/dbil serve
# http://localhost:4242
```

## Docs

- High-level design: [`docs/superpowers/specs/2026-05-17-dbil-design.md`](docs/superpowers/specs/2026-05-17-dbil-design.md)
- Per-plan implementation history: [`docs/superpowers/plans/`](docs/superpowers/plans)
- Contributing: [`CONTRIBUTING.md`](CONTRIBUTING.md)
- Security disclosure: [`SECURITY.md`](SECURITY.md)

## License

Apache 2.0 — see [`LICENSE`](LICENSE).
