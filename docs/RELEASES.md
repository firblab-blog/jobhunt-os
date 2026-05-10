# Releases

JobHunt OS uses container images and GitHub Releases as the release artifacts
for the self-hosted Docker Compose install. There is no management CLI, Helm
chart, or public binary release channel yet.

## Image Tags

Public images are published under:

```text
ghcr.io/firblab-blog/jobhunt-os
```

Tags have these meanings:

- `latest`: the newest versioned release image. This is the default in the
  provided Compose file so first-time installs can stay simple. It tracks the
  most recent `vX.Y.Z` release.
- `vX.Y.Z`: a named release tag, for example `v0.1.9`. Use this when you want
  explicit upgrades.
- `sha-<shortsha>`: an image for a specific commit. Use this when you need an
  exact build for testing, rollback, or support.

Versioned releases use the Git tag shape:

```text
vX.Y.Z
```

## Release Process

The lightweight release process is:

1. Merge the intended release state to `main`.
2. Let CI publish a `sha-<shortsha>` image for that commit.
3. Create and push an annotated Git tag such as `v0.1.9`.
4. Let CI publish the matching `vX.Y.Z`, `latest`, and `sha-<shortsha>` images.
5. Let CI generate the image SBOM and container scan report.
6. Let CI mirror the Git tag to GitHub and create the matching GitHub Release.
7. Sanity-check the published image with Docker Compose before announcing it.

If a release has known upgrade notes, document them before or alongside the tag.

## Release Gate Checklist

Do not announce a release until the intended commit has passed these checks, or
until any exception is documented in the release notes.

- Go tests: run `go test ./...`, or confirm the CI test job passed for the
  release commit. For CI release evidence, prefer the race/coverage job.
- Go vet: run `go vet ./...`, or confirm the CI validation job passed.
- Go vulnerability check: run `govulncheck ./...`, or confirm the pinned
  `govulncheck` CI job passed.
- Image scan and SBOM: confirm the release image was scanned and that the
  CycloneDX SBOM artifact was generated. Fixed critical image vulnerabilities
  should block the release unless there is a written exception.
- Secret scan: scan the release diff and repository for credentials, local
  `.env` data, private job-search records, database files, uploaded documents,
  and non-sample fixtures. Use the project's CI scanner when available, or a
  local tool such as Gitleaks or TruffleHog with redacted output.
- Docs review: check `README.md`, install, upgrade, backup/restore, reverse
  proxy, security, and release notes for changes users need before upgrading or
  exposing the app on a network.
- Auth review: for deployed non-loopback release candidates, confirm the runtime
  uses `JOBHUNT_AUTH_MODE=login` and secret storage for plaintext passwords or
  real password hashes. For trusted HTTPS reverse-proxy access, also confirm
  `JOBHUNT_SECURE_COOKIES=true`; for direct plain-HTTP LAN access, keep secure
  cookies off so login sessions work. Existing PBKDF2-SHA256 hashes are
  legacy-compatible, but password-file configuration is preferred for new
  Compose installs.

## What To Pin

The user-facing Compose file intentionally keeps `latest` for the easiest
personal install path:

```yaml
services:
  jobhunt-os:
    image: ghcr.io/firblab-blog/jobhunt-os:latest
```

For production-ish self-hosted installs, pin a versioned image tag instead:

```yaml
services:
  jobhunt-os:
    image: ghcr.io/firblab-blog/jobhunt-os:v0.1.9
```

This makes upgrades deliberate: back up `./data`, edit the tag, pull the image,
and restart with Docker Compose.

Using `latest` is acceptable for casual personal installs where automatic
movement to the newest released image is welcome. Use `sha-<shortsha>` when you
need an exact commit rather than a release line.

## Supply Chain Checks

Release builds use digest-pinned base images where the project controls the
reference:

- `Dockerfile` pins the `golang:1.26.3` build image by digest.
- `deploy/docker-compose.yml` pins the `busybox:1.37.0` init helper by digest.
- CI uses a digest-pinned Go image for validation and a digest-pinned Trivy
  image for SBOM and vulnerability scanning.
- CI Docker publish jobs use digest-pinned `docker:28.0.1` and
  `docker:28.0.1-dind` images.

CI runs `govulncheck` with the pinned `golang.org/x/vuln` version recorded in
`.gitlab-ci.yml`. Publish pipelines generate these release artifacts:

- `gl-sbom.cdx.json`: CycloneDX SBOM for the container image.
- `gl-container-scanning-report.json`: GitLab container scanning report.

The Trivy scan records all severities in the report and fails the pipeline when
the image has a fixed critical vulnerability.

To scan a pulled release image locally:

```sh
trivy image ghcr.io/firblab-blog/jobhunt-os:v0.1.9
```

To generate a local CycloneDX SBOM:

```sh
trivy image --format cyclonedx --output jobhunt-os.cdx.json ghcr.io/firblab-blog/jobhunt-os:v0.1.9
```
