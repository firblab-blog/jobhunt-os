# Install JobHunt OS

Docker Compose is the recommended self-hosted install path for JobHunt OS. The
repository includes the deployment bundle under `deploy/`: Compose file, example
configuration, local secret location, and the default data directory.

## Requirements

- Git
- Docker
- Docker Compose v2, usually available as `docker compose`

## Quick Install

Clone the repository, copy the example deployment configuration, create a local
admin password file, and start the app from `deploy/`:

```sh
git clone https://github.com/firblab-blog/jobhunt-os.git
cd jobhunt-os/deploy
cp .env.example .env
mkdir -p .secrets
$EDITOR .env
$EDITOR .secrets/admin-password
chmod 600 .env .secrets/admin-password
docker compose up -d
```

Set `JOBHUNT_AUTH_USERNAME` in `.env`. Put only the password or passphrase in
`.secrets/admin-password`; use at least 15 printable characters. The app reads
that file as a Docker secret, hashes the password in memory with Argon2id at
startup, and does not require users to generate or paste password hashes.

Then open:

```text
http://127.0.0.1:8080
```

The default Compose file stores application data in `deploy/data` relative to
the cloned repository. Before putting important job-hunt data in the app, review
[BACKUP_AND_RESTORE.md](BACKUP_AND_RESTORE.md). The recommended default is a
simple data directory backup, with optional encryption for copies kept off the
machine.

## Configuration Files

The normal deployment files are:

```text
deploy/
  .env.example
  .env
  .secrets/
    admin-password
  data/
```

Only `.env.example` is committed. The local `.env`, `.secrets/`, and `data/`
paths are ignored by Git and Docker build context rules.

Common `.env` values:

```text
JOBHUNT_PUBLIC_BIND=127.0.0.1
JOBHUNT_PUBLIC_PORT=8080
JOBHUNT_AUTH_MODE=login
JOBHUNT_AUTH_USERNAME=<username>
JOBHUNT_AUTH_PASSWORD_SECRET_FILE=./.secrets/admin-password
JOBHUNT_SECURE_COOKIES=false
JOBHUNT_AUTH_TRUST_PROXY_HEADERS=false
JOBHUNT_SESSION_IDLE_TIMEOUT=12h
JOBHUNT_SESSION_ABSOLUTE_TIMEOUT=30d
JOBHUNT_UID=65532
JOBHUNT_GID=65532
```

See [CONFIGURATION.md](CONFIGURATION.md) for the full runtime reference.

## Linux Data Ownership

The app container runs as a non-root user. The Compose file includes a one-shot
`jobhunt-os-init` helper that creates the expected `./data` directories and sets
ownership before the app starts.

For host-owned files on Linux, set these values in `deploy/.env` before first
boot:

```text
JOBHUNT_UID=<host-uid>
JOBHUNT_GID=<host-gid>
```

Those ownership values are optional, but recommended for Linux bind mounts
because they make the container write `./data` files as your host user. If you
add or change them later, run `docker compose up -d` again from `deploy/` so the
data-prep helper can be recreated and adjust ownership.

The prepared data directory has this shape:

```text
data/
  jobhunt-os.db
  documents/
  tmp/
```

## Built-In Authentication

The Compose install requires the built-in login flow because the process listens
on the container network interface. The default path uses:

```text
JOBHUNT_AUTH_MODE=login
JOBHUNT_AUTH_USERNAME=<username>
JOBHUNT_AUTH_PASSWORD_FILE=/run/secrets/jobhunt_admin_password
```

`JOBHUNT_AUTH_PASSWORD_FILE` is set by `docker-compose.yml`; users normally edit
`JOBHUNT_AUTH_PASSWORD_SECRET_FILE` in `.env` only if they want the host-side
secret file to live somewhere other than `./.secrets/admin-password`.

When configured, `/healthz` remains unauthenticated for health checks. App
routes, JSON export, and document downloads require credentials.

Built-in authentication is only appropriate over `localhost`, HTTPS, VPN, or
another encrypted/trusted channel. Do not publish the app over plain HTTP on an
untrusted network.

For HTTPS reverse-proxy deployments, set `JOBHUNT_SECURE_COOKIES=true` in
`.env` so browsers only send app cookies over HTTPS. Set
`JOBHUNT_AUTH_TRUST_PROXY_HEADERS=true` only when the app is behind a trusted
reverse proxy that sanitizes forwarded headers. See
[REVERSE_PROXY.md](REVERSE_PROXY.md) for the recommended exposure pattern.

## Public Image

The Compose file uses the public container image:

```text
ghcr.io/firblab-blog/jobhunt-os:latest
```

The `latest` tag is the default install path and tracks the newest versioned
release. Pin the image tag in `docker-compose.yml` when upgrades should be
explicit.

Current versioned release example:

```text
ghcr.io/firblab-blog/jobhunt-os:v0.1.9
```

## Common Commands

Run these from `deploy/`.

Show running containers:

```sh
docker compose ps
```

Follow logs:

```sh
docker compose logs -f
```

Stop JobHunt OS:

```sh
docker compose down
```

Start it again or apply configuration changes:

```sh
docker compose up -d
```

## Network Binding

The default Compose file publishes JobHunt OS on the host loopback interface:

```yaml
ports:
  - "${JOBHUNT_PUBLIC_BIND:-127.0.0.1}:${JOBHUNT_PUBLIC_PORT:-8080}:8080"
```

With the default `.env`, the app is reachable from the host at
`http://127.0.0.1:8080`, but is not directly exposed on every network
interface.

Inside the container, `JOBHUNT_ADDR` is set to `0.0.0.0:8080`. That is expected:
the process must listen on the container network interface so Docker can forward
traffic to it. The host-side `JOBHUNT_PUBLIC_BIND` value is what controls
whether the install is local-only from outside the machine.

For HTTPS or a public hostname, put a reverse proxy in front of the local port.
See [REVERSE_PROXY.md](REVERSE_PROXY.md).

Local desktop use may run the Go binary directly without auth on loopback.
Non-loopback no-auth is refused unless both network binding is enabled and
`JOBHUNT_ALLOW_INSECURE_NO_AUTH=true` is set for a deployment protected
elsewhere.

## Next Steps

- Read [BACKUP_AND_RESTORE.md](BACKUP_AND_RESTORE.md) before putting important
  data in the app, and treat JSON exports as sensitive job-hunt records.
- Read [UPGRADING.md](UPGRADING.md) before pulling a new image.
- Use [CONFIGURATION.md](CONFIGURATION.md) as a reference when changing runtime
  settings.
