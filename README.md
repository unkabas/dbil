# dbil

Lightweight, security-first PostgreSQL tool. Drops into your `docker-compose` for dev and works as a pgAnalyze-class observability dashboard against production — all from a single ~25 MB container.

Status: **pre-alpha** (Plan 1 / Foundation in progress).

## Build & test

```bash
make tidy     # download deps
make build    # build ./bin/dbil
make test     # race detector + coverage
make lint     # golangci-lint
make cover    # generates coverage.html
make docker   # docker build -t dbil:dev .
```

Default UI port is **4242**. Data directory is `./dbil-data` outside containers, `/data` inside.

See `docs/superpowers/specs/2026-05-17-dbil-design.md` for the full design and `docs/superpowers/plans/2026-05-17-foundation.md` for the current implementation plan.
