# Contributing to dbil

Thanks for considering a contribution.

## Quick start

```bash
git clone https://github.com/unkabas/dbil
cd dbil
make web-deps   # npm install in web/
make tidy       # go mod tidy
make test       # race + coverage
make build      # ./bin/dbil (with the SPA embedded)
```

Run the binary locally:

```bash
DBIL_DATA_DIR=$(mktemp -d) ./bin/dbil serve   # http://localhost:4242
```

## Code style

- **Go**: `gofmt`, `goimports`, `golangci-lint run` clean.
- **TS/React**: `tsc -b --noEmit && vite build` must pass.
- **Auth gating**: `make lint-auth` confirms every HTTP handler sits behind
  `auth.RequireAuth` or is on the small `unauthedRoutes` allowlist
  (`/healthz`, `/api/auth/login`). New routes must keep this lint green.

## Commits

- Conventional Commits: `feat:`, `fix:`, `chore:`, `docs:`, `test:`,
  `refactor:`, `ci:`, `build:`. Scope optional but appreciated
  (`feat(observ): …`).
- One logical change per commit. Phase-by-phase commits work well for
  larger features.
- **Do not** add a `Co-Authored-By` trailer. Commits should carry only
  the contributor's own identity.

## Pull requests

- Open against `main`.
- Include a short description, the change rationale, and a test plan.
- CI must pass (lint + lint-auth + race tests + govulncheck + multi-arch
  build).

## Architecture pointers

- Security model: [`SECURITY.md`](SECURITY.md)
- Threat-modeled secrets handling lives in `internal/crypto` (envelope
  encryption, MK loader chain) and `internal/audit` (tamper-evident hash
  chain).
