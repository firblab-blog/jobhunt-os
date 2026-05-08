# Upgrading

The recommended Docker Compose upgrade path is to back up first, pull the new
image, and recreate the container.

## Back Up First

Before upgrading, follow [BACKUP_AND_RESTORE.md](BACKUP_AND_RESTORE.md) and
keep a copy of the current `data/` directory.

## Upgrade a Docker Compose Install

From the directory that contains `docker-compose.yml`:

```sh
docker compose pull
docker compose up -d
```

The Compose data-prep helper runs before the app container on first start and
when Compose recreates it. It keeps `data/documents` and `data/tmp` present and
sets `./data` ownership for the configured `JOBHUNT_UID`/`JOBHUNT_GID` values
from `.env`, or the image's default non-root UID/GID if `.env` is not present.

Check the container and logs:

```sh
docker compose ps
docker compose logs -f
```

The application runs database migrations at startup. If the new container does
not start cleanly, keep the backup and inspect the logs before making further
changes.

## `latest` vs Pinned Tags

The default Compose file uses:

```text
ghcr.io/firblab-blog/jobhunt-os:latest
```

Using `latest` means `docker compose pull` will move you to the newest
versioned release image for the default install path.

When versioned image tags are available, you can pin the image in
`docker-compose.yml`:

```yaml
services:
  jobhunt-os:
    image: ghcr.io/firblab-blog/jobhunt-os:v0.1.0
```

Pinned tags make upgrades more explicit: edit the tag, back up, pull, and run
`docker compose up -d`.

JobHunt OS does not require a management CLI, Helm chart, or binary release to
upgrade a Docker Compose install.
