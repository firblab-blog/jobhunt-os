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
  provided Compose file and tracks the most recent `vX.Y.Z` release.
- `vX.Y.Z`: a named release tag, for example `v0.1.0`. Use this when you want
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
3. Create and push an annotated Git tag such as `v0.1.0`.
4. Let CI publish the matching `vX.Y.Z`, `latest`, and `sha-<shortsha>` images.
5. Let CI mirror the Git tag to GitHub and create the matching GitHub Release.
6. Sanity-check the published image with Docker Compose before announcing it.

If a release has known upgrade notes, document them before or alongside the tag.

## What To Pin

For production-ish self-hosted installs, pin a versioned image tag:

```yaml
services:
  jobhunt-os:
    image: ghcr.io/firblab-blog/jobhunt-os:v0.1.0
```

This makes upgrades deliberate: back up `./data`, edit the tag, pull the image,
and restart with Docker Compose.

Using `latest` is acceptable for casual personal installs where automatic
movement to the newest released image is welcome. Use `sha-<shortsha>` when you
need an exact commit rather than a release line.
