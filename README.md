# dbil

Lightweight, security-first PostgreSQL tool. Drops into your `docker-compose` for
dev and works as a pgAnalyze-class observability dashboard against production —
all from a single ~25 MB container, with the React UI embedded in the binary.

Status: **pre-alpha**. Plans 1 – 4 (backend) and Plan 5 (frontend) are landed.
Plans 6 – 9 (observability, schema introspection, compose discovery, release
pipeline) are next.

## Build, test, run

```bash
# First-time setup
make web-deps    # npm install in web/
make tidy        # go mod tidy

# Build the binary with the embedded SPA
make build       # produces ./bin/dbil (~25 MB)

# Run locally
DBIL_DATA_DIR=$(mktemp -d) ./bin/dbil init
./bin/dbil serve
# Open http://localhost:4242 in your browser. First-run credentials are in
# $DBIL_DATA_DIR/initial-credentials.txt (mode 0600).
```

```bash
# Tests + lints
make test        # race detector + coverage
make lint        # golangci-lint (Go side)
make lint-auth   # static check: every API handler is auth-gated
make cover       # HTML coverage report at coverage.html

# Docker (multi-stage: Node builds the SPA, Go embeds it, distroless runtime)
make docker      # docker build -t dbil:dev .
docker run --rm -p 4242:4242 -v $(pwd)/dbil-data:/data dbil:dev init
docker run --rm -p 4242:4242 -v $(pwd)/dbil-data:/data dbil:dev serve
```

Default UI port is **4242**. Data directory is `./dbil-data` outside containers,
`/data` inside.

## Frontend development

The SPA lives in [`web/`](web). For hot reload during UI work:

```bash
cd web && npm run dev
# Opens http://127.0.0.1:5173/ with /api requests proxied to localhost:4242
```

Run a separate `./bin/dbil serve` so the proxy has a backend to talk to.

## Docs

- Design spec: [`docs/superpowers/specs/2026-05-17-dbil-design.md`](docs/superpowers/specs/2026-05-17-dbil-design.md)
- Implementation plans: [`docs/superpowers/plans/`](docs/superpowers/plans)
