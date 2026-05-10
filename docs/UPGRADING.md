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
from `deploy/.env`, or the image's default non-root UID/GID if `.env` is not
present.

Check the container and logs:

```sh
docker compose ps
docker compose logs -f
```

The application runs database migrations at startup. If the new container does
not start cleanly, keep the backup and inspect the logs before making further
changes.

## Authentication Upgrade Notes

New self-hosted and deployed installs should prefer:

```text
JOBHUNT_AUTH_MODE=login
JOBHUNT_AUTH_USERNAME=<username>
JOBHUNT_AUTH_PASSWORD_FILE=/run/secrets/jobhunt_admin_password
```

Docker Compose installs mount the password file from the local ignored secret
configured by `JOBHUNT_AUTH_PASSWORD_SECRET_FILE`; the default host path is
`deploy/.secrets/admin-password`.

Existing Argon2id and PBKDF2-SHA256 password hashes remain supported through
`JOBHUNT_AUTH_PASSWORD_HASH` for compatibility, so you do not need to rotate
immediately. Move to a password file when convenient, when a password may have
been exposed, or when access should be removed.

Loopback no-auth remains allowed for desktop/local use. Non-loopback no-auth is
refused unless `JOBHUNT_ALLOW_INSECURE_NO_AUTH=true` is set alongside network
binding. Deployed non-loopback instances must use login auth. Use HTTPS and
`JOBHUNT_SECURE_COOKIES=true` when access goes through a trusted reverse proxy;
keep secure cookies off for direct plain-HTTP LAN deployments.

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
    image: ghcr.io/firblab-blog/jobhunt-os:v0.1.9
```

Pinned tags make upgrades more explicit: edit the tag, back up, pull, and run
`docker compose up -d`.

JobHunt OS does not require a management CLI, Helm chart, or binary release to
upgrade a Docker Compose install.
