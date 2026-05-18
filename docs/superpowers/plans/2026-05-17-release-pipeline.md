# DBil v1.0 — Plan 9 (Packaging + release pipeline)

## Context

Plans 1-8 ship every piece of behavior the v1.0 MVP needs: foundation,
auth, connections, query safety, frontend, observability, schema
introspector, and compose auto-discovery. What's missing is the
**release surface** — the artifacts a stranger downloading dbil for the
first time expects to find: signed multi-arch images, native binaries,
a checked-in SBOM, a real README, a license, and a security disclosure
contact.

After Plan 9 we cut **v1.0.0**, the first public drop.

## Scope

| In | Out |
|---|---|
| Multi-arch (`linux/amd64`, `linux/arm64`) Docker image pushed to ghcr.io on every `v*` tag | Homebrew, deb, rpm packages |
| Native binaries for `linux/{amd64,arm64}`, `darwin/{amd64,arm64}` attached to the GitHub release | Windows builds (no MVP user asked for them) |
| cosign keyless signing of the published image | KMS-backed signing |
| SPDX SBOM attached to release | License-scan automation |
| Image size guard (≤30 MB hard ceiling) | Continuous size tracking |
| Production-grade `docker-compose.production.yml` example | Helm chart |
| README rewrite + CONTRIBUTING + SECURITY + LICENSE | Marketing landing site |

## File structure added / changed

```
.github/workflows/release.yml     # new
Makefile                          # MODIFIED — add image-size target
examples/
  docker-compose.dev.yml          # MODIFIED — minimal solo-dev posture
  docker-compose.production.yml   # NEW — read-only, no-new-privileges, dbil.* labels
README.md                         # rewritten
CONTRIBUTING.md                   # NEW
SECURITY.md                       # NEW
LICENSE                           # NEW (Apache 2.0)
```

## Tasks

### Task 1 — `.github/workflows/release.yml`

Single workflow, triggered on `push` of any `v*` tag. Jobs:

- **build_binaries**: cross-compile for `linux/{amd64,arm64}` and
  `darwin/{amd64,arm64}`. Build the SPA first with
  `npm ci && npm run build`. ldflags must match the Dockerfile's
  `-X main.version=… -X main.commit=… -X main.date=…`. Upload binaries
  as workflow artifacts.

- **build_image**: docker buildx, multi-arch (`linux/amd64,linux/arm64`),
  using the existing `Dockerfile`. Tags: `ghcr.io/unkabas/dbil:<tag>` and
  `ghcr.io/unkabas/dbil:latest`. Push to ghcr.io via `GITHUB_TOKEN`.

- **sign**: cosign keyless via GitHub OIDC. Run
  `cosign sign --yes ghcr.io/unkabas/dbil@<digest>`. Image goes onto the
  public Rekor transparency log.

- **sbom**: `anchore/sbom-action` produces SPDX JSON. Upload as release
  asset.

- **release**: `softprops/action-gh-release` (or
  `gh release create $TAG --generate-notes`) creates the GitHub release
  and attaches the four binaries + SBOM.

- **size_guard**: after the image build, query the manifest digest's
  size and fail when it exceeds 30 MB.

### Task 2 — `make image-size`

Adds a target that runs:

```bash
docker image inspect dbil:latest --format '{{.Size}}'
```

…fails the make recipe when size > 30 MB. Used locally before tagging
and in the release workflow.

### Task 3 — `examples/docker-compose.production.yml`

Production-grade posture:

- `read_only: true`, `cap_drop: [ALL]`,
  `security_opt: [no-new-privileges:true]`, `tmpfs: /tmp`.
- `secrets:` block with `dbil_master_key` (referenced as
  `DBIL_MASTER_KEY_FILE=/run/secrets/dbil_master_key`, mode 0400).
- A sample postgres companion with the full `dbil.*` label set so the
  compose acts as a working Level-2 discovery demo.
- `restart: unless-stopped`.

`examples/docker-compose.dev.yml` becomes the minimal solo dev case:
single dbil service + auto-MK + a bind-mounted `./dbil-data`.

### Task 4 — README rewrite

Replace the existing pre-alpha README with the v1.0 launch version
covering the killer-feature checklist, drop-in compose quickstart,
security model summary, image install instructions, badges (license,
release, CI), and a pointer into `docs/superpowers/`. Keep it scannable
— no walls of text.

### Task 5 — CONTRIBUTING.md

Short. How to clone, `make test`, conventional commits, no
Co-Authored-By trailer (project preference). Link to the design spec.

### Task 6 — SECURITY.md

Disclosure email + short threat model summary (encrypted at rest, no
plaintext creds in audit, hash-chained audit, RequireAuth enforced by
AST lint, sandboxed UI port 4242). Mention `cosign verify` against the
released image.

### Task 7 — LICENSE

Apache 2.0 — full standard text + copyright line for the user.

### Task 8 — Cut v1.0.0

After the branch merges, tag `v1.0.0` on `main` and push. Verify the
release workflow runs and produces:

- `ghcr.io/unkabas/dbil:v1.0.0` (multi-arch) signed by cosign.
- 4 binaries attached to the GitHub release.
- SBOM SPDX JSON attached.
- Image size under 30 MB.

## Risks acknowledged

- **ghcr.io login** needs `packages: write` permission on the workflow.
  Will set explicitly at job level.
- **cosign keyless** requires the workflow to have `id-token: write`.
  Documented in the workflow file.
- **Image size drift** is gated by the new make target + release step;
  the v0.8 multi-arch build already lands well under 25 MB on amd64.
- **First push to ghcr.io** may need the package set to public via the
  GitHub UI after the first publish — documented in README.
