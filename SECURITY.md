# Security policy

## Reporting a vulnerability

If you find a security issue, please **do not** open a public GitHub
issue. Email <k3team.com@gmail.com> with:

- A description of the problem and the impact you observed.
- A minimal reproduction (proof-of-concept) when possible.
- Whether the issue is already public (e.g. discussed elsewhere).

I will acknowledge receipt within 72 hours and aim to ship a fix or
mitigation within 14 days for high-severity findings.

## Verifying releases

Every container image is signed with cosign keyless OIDC against the
public Rekor transparency log. Verify before running:

```bash
cosign verify ghcr.io/unkabas/dbil:v1.0.0 \
  --certificate-identity-regexp='https://github.com/unkabas/dbil/.*' \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

A SPDX SBOM is attached to each GitHub release.

## Threat model summary

DBil is designed assuming the host machine is **trusted** but the
state file on disk is not. The goals are:

1. **No plaintext credentials at rest.** Every connection password,
   username, and database name is AES-256-GCM-encrypted with a per-
   connection DEK that is itself wrapped under a 32-byte master key
   (MK). The MK comes from one of six loaders (KMS, OS keychain,
   mounted secret file, env var, TTY, auto-generated) in priority
   order; env-only and auto-generated keys emit a startup warning.
2. **No plaintext credentials in audit logs.** Audit detail blobs are
   encrypted with an audit-specific DEK derived deterministically from
   the MK via HKDF.
3. **Tamper-evident audit chain.** Every audit row's hash chains
   forward via SHA-256 over the canonicalised previous entry; a single
   altered row breaks `AuditRepo.VerifyChain`.
4. **Auth-gated by construction.** A static AST lint
   (`scripts/lint-auth/main.go`) enforces that every HTTP handler is
   either under `auth.RequireAuth` or on the explicit unauthed
   allowlist (`/healthz`, `/api/auth/login`). CI fails on any new
   handler that escapes the gate.
5. **Tag-driven safety.** `production`-tagged connections require a
   per-connection passphrase, refuse DDL outright, and block
   `DELETE`/`UPDATE` without `WHERE`. `staging` requires explicit
   `X-Confirm: yes` for dangerous statements.
6. **Read-only container surface.** The reference compose runs the
   binary in a distroless image with `read_only: true`, `cap_drop:
   [ALL]`, `cap_add: [CHOWN, SETGID, SETUID]`, and
   `no-new-privileges`. Those three capabilities are used only during
   startup so the process can fix `/data` ownership and drop to UID
   65532 before opening the state DB or serving HTTP.

## What is **not** in scope

- Memory-resident exfiltration on a compromised host (a process that
  can read /proc/<pid>/mem can read the unwrapped MK; mitigated by
  using a KMS-backed loader).
- Side-channel attacks against `crypto/aes` in pure Go.
- Bugs in `pgx/v5` or the PostgreSQL server itself.

## Production hardening checklist

- Provision the MK from KMS, the OS keychain, or a mounted secret
  file — never via `DBIL_MASTER_KEY`.
- Mount the Docker socket read-only when using Level-2 discovery.
- Run with `read_only: true`, `cap_drop: [ALL]`, `cap_add:
  [CHOWN, SETGID, SETUID]`,
  `security_opt: [no-new-privileges:true]`. See
  `examples/docker-compose.production.yml`.
- Rotate the admin password and all auto-generated MKs on first run.
- Set per-connection passphrases for every `production`-tagged
  connection.
