# DBil v0.1 — Plan 1 (Foundation)

## Context

DBil is a new open-source PostgreSQL tool: a single ~25MB Docker container that drops into existing `docker-compose` projects for dev and works as a pgAnalyze-class observability tool against production. The full design is in `docs/superpowers/specs/2026-05-17-dbil-design.md` (Apache 2.0 + CLA, hybrid solo/team in one binary, security-first envelope encryption).

The MVP spec covers many subsystems (crypto, auth, connection manager, query executor, observability collectors, frontend, compose discovery, packaging). It is **not** a single plan. This document is **Plan 1 of ~9** and produces the foundation that every later plan builds on: the project scaffold, the crypto layer, the state store, and a working `dbil init` bootstrap command.

After Plan 1 ships, a fresh container will:
1. Run the MK loader chain (KMS → mounted secret → env → auto-generate) and obtain a master key.
2. Open the SQLite state store (`modernc.org/sqlite`, pure Go, no CGO).
3. Run schema migrations.
4. Generate an admin user, write the random initial password to `/data/initial-credentials.txt` (mode 0600), and emit a genesis audit entry on the tamper-evident hash chain.
5. Exit cleanly. `dbil version` and `dbil init` are the only subcommands in this plan.

No HTTP server, no Postgres connectivity, no UI yet — those land in Plans 2–9.

### Spec deviation (already applied)

The spec originally specified **SQLCipher** for the state store. Plan 1 swaps it for **`modernc.org/sqlite` + application-layer envelope encryption of sensitive columns**. SQLCipher requires CGO, which breaks clean cross-compilation to `linux/arm64` and complicates the single-static-binary constraint. Threat model is preserved: every sensitive value (credentials, audit details, future session tokens) is AES-256-GCM-encrypted via the MK→DEK envelope before insertion, so a leaked DB file without the MK still yields ciphertext for every sensitive field. The spec has been edited in-place to match (uncommitted at the time this plan was written).

## Subsequent plans (for reference, not part of this work)

| # | Plan | Deliverable |
|---|---|---|
| **1** | **Foundation** (this plan) | scaffold + crypto + state store + `dbil init` |
| 2 | Auth + HTTP server | login, sessions, `requireAuth` middleware, audit-logged auth events, `dbil serve` |
| 3 | Connection manager + first Postgres connect | pgx pools, encrypted creds, TLS, SSH tunnels |
| 4 | Query executor + safety guards + audit hook | safe SQL exec, statement timeouts, tag-driven DML/DDL gates |
| 5 | Frontend skeleton (React + Vite + CodeMirror 6) | login → connections list → SQL editor |
| 6 | Observability collectors + UI | pg_stat workers, hot/warm/cold storage, charts |
| 7 | Schema introspector + ER view + tag policies UI | |
| 8 | Compose auto-discovery (Docker socket reader) | |
| 9 | Packaging + multi-arch + release pipeline | cosign-signed images, release notes generator |

## Tech stack used in Plan 1

| Purpose | Module |
|---|---|
| Go version | `go1.23` |
| CLI framework | `github.com/spf13/cobra` |
| Config | stdlib `os` + small wrapper (`internal/config`) |
| Logging | stdlib `log/slog` |
| KDF + AEAD | stdlib `crypto/aes`, `crypto/cipher`, `crypto/rand`, `crypto/hkdf` (Go 1.24+) or `golang.org/x/crypto/hkdf` (pin one), `golang.org/x/crypto/argon2` |
| SQLite | `modernc.org/sqlite` (pure Go) |
| Migrations | `github.com/golang-migrate/migrate/v4` with `file://` source and the sqlite driver |
| Generated queries | `sqlc` v1.27+ (build-time tool) |
| TTY input | `golang.org/x/term` |
| Tests | `github.com/stretchr/testify` (assert/require) |
| Tooling | `golangci-lint`, `gosec`, `govulncheck` (CI) |

## File structure created by Plan 1

```
open-source-dbil/
├── .editorconfig
├── .gitignore
├── .golangci.yml
├── .dockerignore
├── Dockerfile
├── Makefile
├── go.mod
├── go.sum
├── sqlc.yaml
├── cmd/
│   └── dbil/
│       ├── main.go                # cobra root, slog setup, version/init wiring
│       ├── init_cmd.go            # `dbil init`
│       └── version_cmd.go         # `dbil version` with ldflags-injected version
├── internal/
│   ├── audit/
│   │   ├── hashchain.go           # genesis, append, verify helpers (pure)
│   │   └── hashchain_test.go
│   ├── bootstrap/
│   │   ├── init.go                # orchestrates `dbil init` business logic
│   │   └── init_test.go
│   ├── config/
│   │   ├── config.go              # env-var loader; struct DBilConfig
│   │   └── config_test.go
│   ├── crypto/
│   │   ├── aead.go                # AES-256-GCM Encrypt/Decrypt with AAD
│   │   ├── aead_test.go
│   │   ├── envelope.go            # WrapDEK/UnwrapDEK, EncryptField/DecryptField
│   │   ├── envelope_test.go
│   │   ├── kdf.go                 # HKDF-SHA256, argon2id wrappers
│   │   ├── kdf_test.go
│   │   ├── masterkey.go           # MasterKey type + Loader interface
│   │   ├── loader_chain.go        # ordered fallback runner
│   │   ├── loader_chain_test.go
│   │   ├── loader_file.go
│   │   ├── loader_file_test.go
│   │   ├── loader_env.go
│   │   ├── loader_env_test.go
│   │   ├── loader_auto.go         # auto-generate + persist + warn
│   │   ├── loader_auto_test.go
│   │   ├── loader_stubs.go        # KMS/Keychain/TTY return ErrLoaderUnavailable
│   │   ├── passphrase.go          # WrapWithPassphrase/UnwrapWithPassphrase
│   │   ├── passphrase_test.go
│   │   ├── random.go              # RandomBytes
│   │   ├── random_test.go
│   │   ├── wipe.go                # Zero(b []byte)
│   │   └── wipe_test.go
│   ├── log/
│   │   └── log.go                 # slog setup, structured fields, redaction helpers
│   ├── migrations/
│   │   ├── embed.go               # //go:embed *.sql FS
│   │   └── 0001_init.up.sql       # users, audit_log
│   └── store/
│       ├── db.go                  # OpenDB(path) → *sql.DB, ping, busy_timeout PRAGMA
│       ├── db_test.go
│       ├── migrate.go             # ApplyMigrations(db, fs)
│       ├── migrate_test.go
│       ├── queries/
│       │   ├── users.sql          # sqlc query annotations
│       │   └── audit.sql
│       ├── gen/                   # sqlc-generated, .gitignore-d? no, committed
│       │   ├── db.go
│       │   ├── models.go
│       │   ├── users.sql.go
│       │   └── audit.sql.go
│       ├── users.go               # repository wrapper over gen
│       ├── users_test.go
│       ├── audit.go               # repository wrapper, ties hashchain to gen
│       └── audit_test.go
├── examples/
│   └── docker-compose.dev.yml     # solo dev usage (no postgres yet)
├── tests/
│   └── e2e/
│       └── init_test.go           # spawns built binary against temp dir
└── .github/
    └── workflows/
        └── ci.yml                 # lint + unit + integration + build matrix
```

## Tasks

Each task should be implemented red→green→commit. Per-task verification commands appear under **Verify**. Always run `go test ./...` after each task — the suite must remain green end-to-end.

### Task 1 — Project scaffold

- Initialize Go module: `go mod init github.com/unkabas/dbil`.
- Create `.gitignore` (ignore `bin/`, `dist/`, `*.test`, `coverage.out`, `.idea/`, `.vscode/`, `dbil-data/`).
- Create `.editorconfig` (LF, 4-space indent for Go via gofmt-default tab, 2 for yaml/json).
- Create `.golangci.yml` enabling `errcheck`, `gosec`, `govet`, `staticcheck`, `revive`, `gocritic`, `gofmt`, `goimports`, `bodyclose`, `noctx`, `sqlclosecheck`, `nilerr`.
- Create `Makefile` with targets: `tidy`, `build`, `test`, `lint`, `cover`, `docker`, `run-init`.
- Add `Made by unkabas` paragraph + build/test commands to `README.md`.

**Verify:** `make tidy && go build ./... && go test ./...` succeeds with empty placeholder tests.

### Task 2 — CI workflow

- `.github/workflows/ci.yml`: matrix on `linux/amd64`, runs `golangci-lint`, `go test -race -cover ./...`, `go build`. Cache modules. Run on PR and pushes to `main`.
- Add `govulncheck` step.
- Do NOT push images in this workflow yet (that comes in Plan 9).

**Verify:** Push a branch and confirm CI is green.

### Task 3 — `internal/crypto/random.go`

- `func Random(n int) ([]byte, error)` reads from `crypto/rand`. Returns error on short read.
- Test: 0 returns empty slice + nil error; 32 returns 32 bytes; two calls produce different bytes (statistical sanity).

### Task 4 — `internal/crypto/wipe.go`

- `func Zero(b []byte)` overwrites with zeroes. Use `runtime.KeepAlive` so the compiler does not elide.
- Test: filled slice becomes all-zero after call.

### Task 5 — `internal/crypto/kdf.go`

- `HKDF(secret []byte, info string, outLen int) ([]byte, error)` using HKDF-SHA256 with `salt=nil` (matches spec; salt is stored separately for passphrase derivation).
- `Argon2id(password, salt []byte, outLen uint32) []byte`. Hard-coded parameters: `time=3`, `memory=64*1024 KiB`, `threads=4`. Reject `len(salt) < 16`.
- `func NewSalt() ([]byte, error)` returns 16 random bytes.
- Tests: HKDF determinism, argon2 determinism for same input, argon2 short-salt rejection, NewSalt length.

### Task 6 — `internal/crypto/aead.go`

- `const NonceSize = 12`.
- `Encrypt(key, plaintext, aad []byte) (nonce, ciphertext []byte, err error)` builds AES-256-GCM, generates a fresh 12-byte nonce, seals. Reject `len(key) != 32`.
- `Decrypt(key, nonce, ciphertext, aad []byte) ([]byte, error)`. Reject malformed nonce length.
- Tests: round-trip with AAD, AAD mismatch yields error, nonce length validation, key length validation.

### Task 7 — `internal/crypto/masterkey.go` (types + loader interface)

- `type MasterKey []byte` (always exactly 32 bytes). Constructor `NewMasterKey(b []byte) (MasterKey, error)` validates length.
- `func (m MasterKey) Wipe()` calls `Zero`.
- `type Loader interface { Load(ctx context.Context) (MasterKey, Source, error) }`.
- `type Source` enum (string-typed): `SourceKMS`, `SourceKeychain`, `SourceFile`, `SourceEnv`, `SourceTTY`, `SourceAuto`.
- `var ErrLoaderUnavailable = errors.New("loader unavailable")`.

### Task 8 — `internal/crypto/loader_file.go`

- `func NewFileLoader(path string) Loader`. `Load` reads exactly 32 bytes; rejects shorter, ignores longer (read at most 64 then validate). Returns `ErrLoaderUnavailable` if path does not exist.
- Tests with `t.TempDir()`: missing file → `ErrLoaderUnavailable`; correct-length file → key bytes match; short file → distinct error wrapping (not unavailable, but real config error).

### Task 9 — `internal/crypto/loader_env.go`

- `func NewEnvLoader(varName string) Loader`. Reads env, expects base64url (no padding) → 32 bytes. Logs **warning** at slog level WARN that env-var MK is dev-only.
- Tests use `t.Setenv`.

### Task 10 — `internal/crypto/loader_auto.go`

- `func NewAutoLoader(persistPath string) Loader`. If `persistPath` exists, reads it (32 bytes). Else generates with `Random(32)`, writes to `persistPath` with mode 0400 atomically (`os.CreateTemp` + `os.Rename`), logs WARN with the message specified by spec section 5/6.3 (auto-generated MK warning + doc link).
- Tests with `t.TempDir()`: first call creates and returns; second call returns same bytes; created file is mode 0400.

### Task 11 — `internal/crypto/loader_stubs.go`

- Three loaders (`NewKMSLoader`, `NewKeychainLoader`, `NewTTYLoader`) that return `ErrLoaderUnavailable` with an explanatory wrapped error: e.g., `fmt.Errorf("kms loader not yet implemented (planned for v0.2): %w", ErrLoaderUnavailable)`. The interface placeholder unblocks the loader-chain test surface.

### Task 12 — `internal/crypto/loader_chain.go`

- `type Chain struct{ Loaders []Loader }`. `Load` iterates in order, returns the first successful result. On `ErrLoaderUnavailable` it continues; on any other error it fails fast. Returns `(MasterKey, Source, error)`.
- Tests: all unavailable → wrapped error; one returns success → that one wins; fail-fast on real error.

### Task 13 — `internal/crypto/envelope.go`

- Derive `wrapKey = HKDF(mk, "dbil:dek-wrap-v1", 32)`.
- `func GenerateDEK() ([]byte, error)` returns 32 random bytes.
- `type WrappedDEK struct{ Nonce, Ciphertext []byte }`.
- `func WrapDEK(mk MasterKey, connID string, dek []byte) (WrappedDEK, error)` with AAD `"conn:"+connID`.
- `func UnwrapDEK(mk MasterKey, connID string, w WrappedDEK) ([]byte, error)`.
- `type EncryptedField struct{ Nonce, Ciphertext []byte; Version uint32 }`. Version starts at 1; allows future format evolution.
- `func EncryptField(dek []byte, connID string, version uint32, plaintext []byte) (EncryptedField, error)` with AAD `"creds:"+connID+":"+itoa(version)`.
- `func DecryptField(dek []byte, connID string, ef EncryptedField) ([]byte, error)`.
- Tests: round-trip; tampered AAD fails; wrong DEK fails; nonce reuse across separate calls is statistically impossible.

### Task 14 — `internal/crypto/passphrase.go`

- `type PassphraseSalt []byte` (must be 16 bytes).
- `func DerivePassphraseKey(passphrase string, salt PassphraseSalt) ([]byte, error)` → 32-byte key via Argon2id (params from Task 5).
- `func WrapWithPassphrase(plaintext []byte, passKey []byte, connID string) (EncryptedField, error)` AAD `"pass:"+connID`.
- `func UnwrapWithPassphrase(ef EncryptedField, passKey []byte, connID string) ([]byte, error)`.
- Test: round-trip; wrong passphrase yields decrypt error; empty passphrase rejected (constraint: min length 8 — enforced at this layer).

### Task 15 — `internal/audit/hashchain.go`

- `type Entry struct{ ID uint64; TS int64; UserID, Action, Resource string; Details json.RawMessage; PrevHash [32]byte }`.
- `var GenesisHash = sha256.Sum256([]byte("dbil-audit-genesis-v1"))`.
- `func Canonicalize(details json.RawMessage) ([]byte, error)` — re-marshals through `map[string]any` → sorted keys for determinism (or use `github.com/gibson042/canonicaljson-go`; prefer stdlib + manual sort to keep deps minimal).
- `func Hash(e Entry) ([32]byte, error)` per the spec section 6.4 formula. Big-endian uint64s.
- `func Verify(prev, current Entry, currentHash [32]byte) error`.
- Tests: deterministic hash for identical input regardless of field-write order; genesis chain verification; mutated detail fails verification.

### Task 16 — `internal/config/config.go`

- `type DBilConfig struct{ DataDir, MasterKeyFile, MasterKeyEnvVar, AuditSyslogAddr string; Port int }`.
- `func Load() (DBilConfig, error)` reads:
  - `DBIL_DATA_DIR` (default `/data` in container, `./dbil-data` otherwise — detect via existence of `/.dockerenv`)
  - `DBIL_PORT` (default `4242`, validates 1–65535)
  - `DBIL_MASTER_KEY_FILE` (default empty → file loader skipped if empty)
  - `DBIL_MASTER_KEY_ENV` (name of env var to read, default `DBIL_MASTER_KEY`)
  - `DBIL_AUDIT_SYSLOG` (default empty)
- Tests: defaults are sensible; explicit overrides win; invalid port → error.

### Task 17 — `internal/log/log.go`

- `func Setup(level slog.Level, jsonOutput bool) *slog.Logger`. JSON in container (detected by `/.dockerenv`), human-readable otherwise.
- `func Redact(s string) string` — basic helper that masks credentials-looking substrings for log lines; mainly for future use, but provide and unit-test now.
- Tests: configure, log a line, assert format.

### Task 18 — `internal/store/db.go`

- `func Open(path string) (*sql.DB, error)`. Uses `modernc.org/sqlite` driver, registers DSN with `_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)`.
- `func Close(db *sql.DB) error`.
- Test: open temp DB, run `SELECT 1`, close cleanly.

### Task 19 — Migrations: `0001_init.up.sql` and `internal/migrations/embed.go`

Schema for Plan 1 only:

```sql
CREATE TABLE schema_migrations (
  version INTEGER PRIMARY KEY,
  dirty   INTEGER NOT NULL
);
CREATE TABLE users (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  email           TEXT UNIQUE NOT NULL,
  password_hash   TEXT NOT NULL,        -- argon2id encoded
  password_salt   BLOB NOT NULL,
  role            TEXT NOT NULL CHECK (role IN ('admin','member','viewer')),
  must_rotate     INTEGER NOT NULL DEFAULT 0,
  created_at      INTEGER NOT NULL,     -- unix nanoseconds
  updated_at      INTEGER NOT NULL
);
CREATE TABLE audit_log (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  ts_ns       INTEGER NOT NULL,
  user_id     TEXT NOT NULL,
  action      TEXT NOT NULL,
  resource    TEXT NOT NULL,
  details_enc BLOB NOT NULL,            -- EncryptField(dek, ..., canonical JSON)
  details_nonce BLOB NOT NULL,
  prev_hash   BLOB NOT NULL,
  entry_hash  BLOB NOT NULL UNIQUE
);
CREATE INDEX idx_audit_ts ON audit_log(ts_ns);
```

- `embed.go`: `//go:embed *.up.sql *.down.sql`. Provide a `Files embed.FS`.
- Provide also a `0001_init.down.sql` that drops the three tables.

### Task 20 — `internal/store/migrate.go`

- `func Apply(db *sql.DB, fs embed.FS) error` using `golang-migrate/v4` with `iofs` source and `sqlite` driver.
- Test: apply twice (idempotent); apply on fresh DB creates expected tables (query `sqlite_master`).

### Task 21 — sqlc setup and generated queries

- `sqlc.yaml` configures sqlc v2 with engine `sqlite`, queries dir `internal/store/queries/`, output to `internal/store/gen/`.
- `queries/users.sql` defines:
  - `InsertUser :one`
  - `GetUserByEmail :one`
  - `UpdateUserPassword :exec`
- `queries/audit.sql` defines:
  - `AppendAudit :one`
  - `GetLatestAudit :one`
- Commit the generated code. Add a `make generate` target that re-runs sqlc.
- Test: regenerate, build, run.

### Task 22 — `internal/store/users.go`

- Wraps the sqlc `Queries` for users. Holds reference to a DEK so the layer can encrypt nothing for users (passwords are hashed, not encrypted — but the audit details encryption uses a different store DEK that we also store).
- For Plan 1 we add a **single "store DEK"** kept in a `system_keys` table introduced via a tiny migration `0002_store_dek.up.sql` if we want audit details encrypted. KEEP IT SIMPLE: encrypt audit details with the wrap key directly (no DEK rotation needed for the audit table in Plan 1). Update spec deviation accordingly.

  *Decision for this plan:* audit `details_enc` is encrypted with `EncryptField` using a fixed system DEK derived as `HKDF(MK, "dbil:audit-dek-v1", 32)` — no row in DB needed; deterministic from MK. Document this in `internal/audit/hashchain.go` comments.

- `func (r *UsersRepo) Create(ctx, email string, password string, role string) (User, error)`: argon2id-hashes the password (generating a new salt), inserts, returns. Sets `must_rotate=1` if password was auto-generated.
- `func (r *UsersRepo) AdminExists(ctx) (bool, error)`.
- Tests against a real temp SQLite.

### Task 23 — `internal/store/audit.go`

- `func (r *AuditRepo) Append(ctx, userID, action, resource string, details map[string]any) (Entry, error)`:
  1. Marshal canonical JSON via `audit.Canonicalize`.
  2. Encrypt details with audit-DEK.
  3. Load latest entry to obtain `prev_hash` (or `GenesisHash` if first).
  4. Compute `entry_hash`.
  5. Insert atomically inside a tx.
- `func (r *AuditRepo) VerifyChain(ctx) error`: walks all entries, recomputes hashes, errors on mismatch.
- Tests: append two entries; verify chain; tamper detail bytes in DB and confirm verify fails.

### Task 24 — `internal/bootstrap/init.go`

`func RunInit(ctx, cfg config.DBilConfig, deps Deps) (InitResult, error)`:

1. Ensure `cfg.DataDir` exists (mode 0700).
2. Build loader chain in order:
   - KMS (stub) → returns unavailable
   - Keychain (stub) → unavailable
   - File loader on `cfg.MasterKeyFile` (skipped if empty)
   - Env loader on `cfg.MasterKeyEnvVar`
   - TTY (stub) → unavailable
   - Auto loader on `<DataDir>/master.key`
3. Load master key. Log the source at INFO. If source is `SourceAuto` or `SourceEnv`, log a prominent WARN with hardening doc link.
4. Open SQLite at `<DataDir>/dbil.db`.
5. Apply migrations.
6. If no admin exists: generate a random password (24 base32 chars), hash, insert admin user, write `<DataDir>/initial-credentials.txt` mode 0600 with `email=admin@local\npassword=<plaintext>\n`. Also log the password ONCE to stdout with a clear "rotate immediately" warning.
7. If audit_log is empty: append a genesis entry with `action="bootstrap.init"`, `resource="dbil"`, details `{"version":"<build-version>","master_key_source":"<src>"}`.
8. Return `InitResult{AdminEmail, MasterKeySource, AuditGenesisID, CreatedAdmin bool}`.

Tests using `t.TempDir()` and a mocked env:
- First call: admin created, file present mode 0600, audit length 1, MK auto-generated.
- Second call: idempotent — admin not recreated, audit length still 1.
- Env-provided MK: source reported correctly.
- File-provided MK with wrong size: error reported clearly.

### Task 25 — `cmd/dbil/main.go`, `version_cmd.go`, `init_cmd.go`

- `main.go`: cobra root command, version flag, sets up slog, dispatches to subcommands.
- `version_cmd.go`: prints `dbil v<version> (<commit>) built <date>`. Version, commit, date come from `-ldflags "-X main.version=... -X main.commit=... -X main.date=..."` set in Makefile.
- `init_cmd.go`: calls `bootstrap.RunInit`. Exits non-zero on error. Prints a short success summary.
- Test: a unit test calling the cobra root and asserting that `--help` lists `init` and `version`.

### Task 26 — End-to-end test `tests/e2e/init_test.go`

- Build the binary into `t.TempDir()` via `go build`.
- Run `dbil init` with `DBIL_DATA_DIR=<tempdir>` and no other config.
- Assertions:
  - Exit code 0.
  - `<tempdir>/master.key` exists, mode 0400, 32 bytes.
  - `<tempdir>/dbil.db` exists, non-empty.
  - `<tempdir>/initial-credentials.txt` exists, mode 0600, contains an `admin@local` line and a password line.
  - Open the DB directly with `modernc.org/sqlite` and verify exactly one row in `users` (admin) and one row in `audit_log` (genesis entry referencing `bootstrap.init`).
  - Re-run `dbil init` against the same dir — exit 0, idempotent, no second admin row, no second genesis row.

### Task 27 — `Dockerfile`, `.dockerignore`, `examples/docker-compose.dev.yml`

- **Dockerfile** multi-stage:
  - Stage 1 `golang:1.23-alpine`: copy, `go build -trimpath -ldflags='-s -w -X ...' -o /out/dbil ./cmd/dbil`.
  - Stage 2 `gcr.io/distroless/static-debian12:nonroot`: copy `/out/dbil` → `/dbil`. `USER nonroot:nonroot`. `ENTRYPOINT ["/dbil"]`. `EXPOSE 4242` (annotation only; we still don't serve yet).
- `.dockerignore`: ignore `.git`, `docs/`, `examples/`, `tests/`, `*.md`, `bin/`, `dist/`.
- `examples/docker-compose.dev.yml`: only the `dbil` service with `command: ["init"]`, volume `./dbil-data:/data`, no Postgres yet — the dev compose for Plan 1 demonstrates init only.
- Verify locally: `docker build -t dbil:plan1 .` then `docker run --rm -v $(pwd)/dbil-data:/data dbil:plan1 init` produces the same artifacts the e2e test verified, plus image size under 30MB (`docker image inspect dbil:plan1 --format '{{.Size}}'`).

### Task 28 — Final pass + commit topology

After all preceding tasks merge:

1. Run `make tidy lint test cover` — coverage report should show ≥80% on `internal/crypto`, `internal/audit`, `internal/store`, `internal/bootstrap`.
2. Run `govulncheck ./...`.
3. Ensure CI matrix green.
4. Tag the repo `v0.1.0-foundation` (NOT a release; just a milestone for Plan 1).
5. Commit message conventions: Conventional Commits (`feat:`, `chore:`, `test:`, `docs:`, `ci:`, `build:`). Author identity should be the user's global git config (see project memory).

## End-to-end verification

After Plan 1 is done, the following commands should all succeed from a clean checkout on `linux/amd64` or `linux/arm64`:

```bash
make tidy
make test           # unit + integration
make cover          # coverage HTML at coverage.html
make lint           # golangci-lint
make build          # ./bin/dbil
./bin/dbil version  # prints version string
DBIL_DATA_DIR=$(mktemp -d) ./bin/dbil init
ls -la $DBIL_DATA_DIR
#   -r-------- master.key
#   -rw------- initial-credentials.txt
#   -rw-r--r-- dbil.db
docker build -t dbil:plan1 .
docker image inspect dbil:plan1 --format '{{.Size}}'  # < 30000000 bytes
```

The e2e test in `tests/e2e/init_test.go` encodes the success criteria mechanically.

## Out of scope for Plan 1 (explicit)

- No HTTP server.
- No Postgres connectivity, no `pgx`.
- No frontend at all.
- No Docker socket reader.
- No KMS / OS keychain / TTY MK loaders (only stubs that return `ErrLoaderUnavailable`).
- No `serve` command — only `init` and `version`.
- No observability collectors.
- No tag-driven policies (data model added incrementally in later plans).
