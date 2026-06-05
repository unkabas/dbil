<p align="center">
  <img src=".github/assets/logo.png" alt="dbil logo" width="180" />
</p>

<h1 align="center">dbil</h1>

<p align="center">
  A PostgreSQL workspace you can drop into a <code>docker-compose</code> project. It's
  one 20-megabyte container with the React UI baked in. Schema viewer,
  query editor, observability, all in there.
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License: Apache 2.0" /></a>
  <a href="https://github.com/unkabas/dbil/actions/workflows/ci.yml"><img src="https://github.com/unkabas/dbil/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://github.com/unkabas/dbil/releases"><img src="https://img.shields.io/github/v/release/unkabas/dbil?include_prereleases" alt="Release" /></a>
</p>

<p align="center">
  <video src="https://github.com/user-attachments/assets/689d40df-6fe7-4a13-87b9-110aba811b35"
         width="820"
         autoplay
         loop
         muted
         playsinline
         controls>
    <img src=".github/assets/demo.gif" alt="dbil demo" width="820" />
  </video>
</p>

---

## What you get

Point it at a Postgres instance and you can browse the schema as a real
ER diagram (built from `pg_catalog`, no manual layout), page through
table rows, run SQL with autocompletion, and watch live metrics —
TPS, cache hit ratio, replication lag, slow queries, lock chains,
unused indexes. Production-tagged connections refuse DDL outright and
block `DELETE`/`UPDATE` without a `WHERE`. If something is blocking
five other sessions, hit Kill and dbil sends `pg_terminate_backend`.

Connection passwords don't sit in plaintext anywhere. The master key
unlocks a per-connection DEK; that DEK encrypts the credential fields.
Every audit entry is hashed forward, so tampering with one row breaks
the chain.

## Run it next to your Postgres

The fastest path is the production compose example in this repo:

```bash
git clone https://github.com/unkabas/dbil
cd dbil/examples

# generate the master key (32 random bytes, mode 0400)
mkdir -p secrets
head -c 32 /dev/urandom > secrets/dbil_master_key
chmod 0400 secrets/dbil_master_key

# pick a postgres password — anything you like
echo "POSTGRES_PASSWORD=$(openssl rand -base64 24)" > .env

# first run creates the admin user and prints the password
docker compose -f docker-compose.production.yml up -d
```

Open <http://localhost:4242>. Log in as `admin@local` with the password
from the dbil startup logs. Go to **Discover** — dbil already saw your
postgres container (it reads `dbil.*` labels on the same network).
Approve it, enter a per-connection passphrase, and you're in.

Need the admin password later? It's stored inside the container volume:

```bash
docker compose exec dbil cat /data/initial-credentials.txt
```

## Want to add dbil to your own compose

Two things on your Postgres service — labels and the network — then a
dbil service in the same network with the Docker socket mounted:

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
    image: ghcr.io/unkabas/dbil:latest
    command: ["serve"]
    ports: ["4242:4242"]
    volumes:
      - dbil_data:/data
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      DBIL_DISCOVER: "docker"
      DBIL_NETWORK: "appnet"
    networks: [appnet]

volumes:
  dbil_data:

networks:
  appnet:
    name: appnet
```

Run `docker compose up -d`. On an empty `/data`, `dbil serve`
bootstraps the state DB, creates the first admin, writes
`/data/initial-credentials.txt`, then starts the UI. The container
starts as root only long enough to fix named-volume ownership and then
drops to UID 65532 / GID 0 before opening the state DB or serving HTTP;
GID 0 lets dbil read the Docker socket without running the server as
root. The explicit `name: appnet` on the network keeps compose from
prefixing it with the project name — otherwise `DBIL_NETWORK` won't
match what the engine reports.

If you don't want dbil touching the Docker socket at all, drop the
`DBIL_DISCOVER` env and the socket mount. You can still add
connections by hand from the UI.

## Verifying the image

Every release tag publishes a multi-arch image signed with cosign
keyless OIDC. Check it before you run anything in production:

```bash
cosign verify ghcr.io/unkabas/dbil:latest \
  --certificate-identity-regexp='https://github.com/unkabas/dbil/.*' \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

SPDX SBOMs come attached to each [GitHub
release](https://github.com/unkabas/dbil/releases).

## Native binaries

If you'd rather not run Docker, every release has prebuilt binaries
for `linux/{amd64,arm64}` and `darwin/{amd64,arm64}`. Grab one,
make it executable, then:

```bash
DBIL_DATA_DIR=./dbil-data ./dbil serve
```

## Tags and policies

A connection lives under one of four tags. They drive how aggressive
dbil is about protecting you:

- `local` — anything goes, 5-minute statement timeout.
- `dev` — same, 30-second timeout.
- `staging` — DML and DDL want an `X-Confirm: yes` header.
  `DELETE`/`UPDATE` without a `WHERE` is blocked.
- `production` — DDL is blocked outright. DML wants confirmation.
  Each production connection has its own passphrase, separate from
  the master key. Lose the passphrase, lose access — by design.

You set the tag when you create the connection. dbil's auto-discovery
reads it from the `dbil.tag` label.

## Security model

Short version. The longer one is in [`SECURITY.md`](SECURITY.md).

The state file (`/data/dbil.db`, SQLite) is application-encrypted: per
field, per row, with AES-256-GCM and AAD that binds the ciphertext to
the connection id. A leaked `.db` file is still ciphertext without the
master key. The master key comes from one of six loaders — KMS, OS
keychain, a mounted secret file, an env var, a TTY prompt, or
auto-generated as a last resort. Env-var and auto-generated keys
print a startup warning so you don't ship them by accident.

Audit rows carry encrypted detail blobs and a SHA-256 chain hash.
Mutate one row in the DB and `AuditRepo.VerifyChain` flags it. Every
HTTP handler sits behind `auth.RequireAuth` — a static AST check
(`scripts/lint-auth`) fails CI if anyone ever forgets.

## Build from source

```bash
git clone https://github.com/unkabas/dbil
cd dbil
make web-deps tidy
make test
make build       # ./bin/dbil with the SPA embedded
make docker      # docker build -t dbil:dev .
```

Frontend hot-reload for UI work:

```bash
cd web && npm run dev
# http://127.0.0.1:5173, /api proxied to localhost:4242
```

## Docs

- [Feature guide](FEATURES.md) — roles & users, inline data editing, SSH tunnels
- [Contributing](CONTRIBUTING.md)
- [Security disclosure](SECURITY.md)
- [Apache 2.0 license](LICENSE)
