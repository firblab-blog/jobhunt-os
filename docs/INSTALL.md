# Install JobHunt OS

Docker Compose is the recommended self-hosted install path for JobHunt OS.
There is no management CLI, Helm chart, or binary release channel required for
the current self-hosted setup.

## Requirements

- Docker
- Docker Compose v2, usually available as `docker compose`
- A host directory where JobHunt OS can keep `docker-compose.yml` and `data/`

## Quick Install

Create an install directory, download the Compose file, create a local `.env`
with login auth, and start the app. Generate the password hash locally; see
[CONFIGURATION.md](CONFIGURATION.md) for the Argon2id hash command.

```sh
mkdir jobhunt-os
cd jobhunt-os
curl -O https://raw.githubusercontent.com/firblab-blog/jobhunt-os/main/deploy/docker-compose.yml
$EDITOR .env
docker compose up -d
```

The `.env` file must include:

```text
JOBHUNT_AUTH_MODE=login
JOBHUNT_AUTH_USERNAME=<username>
JOBHUNT_AUTH_PASSWORD_HASH='argon2id$v=19$m=19456,t=2,p=1$<salt-base64url>$<digest-base64url>'
```

Then open:

```text
http://127.0.0.1:8080
```

The default Compose file stores application data in `./data` next to
`docker-compose.yml`.

Before putting important job-hunt data in the app, review
[BACKUP_AND_RESTORE.md](BACKUP_AND_RESTORE.md). The recommended default is a
simple `./data` directory backup, with optional encryption for copies kept off
the machine.

## Linux Data Ownership

The app container runs as a non-root user. The Compose file includes a one-shot
`jobhunt-os-init` helper that creates the expected `./data` directories and
sets ownership before the app starts.

For host-owned files on Linux, add these values to `.env` before first boot:

```text
JOBHUNT_UID=<host-uid>
JOBHUNT_GID=<host-gid>
```

Those ownership values are optional, but recommended for Linux bind mounts
because they make the container write `./data` files as your host user. If you
add or change them later, run `docker compose up -d` again so the data-prep
helper can be recreated and adjust ownership.

The prepared data directory has this shape:

```text
data/
  jobhunt-os.db
  documents/
  tmp/
```

## Built-In Authentication

The Compose install requires the built-in login flow because the process listens
on the container network interface. Set the auth mode, username, and password
hash in your local `.env` file before starting Compose:

```text
JOBHUNT_AUTH_MODE=login
JOBHUNT_AUTH_USERNAME=<username>
JOBHUNT_AUTH_PASSWORD_HASH='argon2id$v=19$m=19456,t=2,p=1$<salt-base64url>$<digest-base64url>'
```

Generate the password hash locally. See [CONFIGURATION.md](CONFIGURATION.md)
for the Argon2id hash format and a command that prompts for the password
without writing it to shell history. Existing PBKDF2-SHA256 hashes remain
legacy-compatible if your install already uses one.

Use single quotes around the hash in Docker Compose `.env` files so the `$`
separators are passed literally. If you keep an unquoted example, escape each
`$` as `$$`.

Use a password or passphrase with at least 15 characters. JobHunt OS does not
require symbol/number/uppercase composition rules and does not force periodic
rotation; change the password when it may be exposed or access should be
removed. If you generate a supported hash with a different tool, choose a
password that still meets this policy. Do not commit plaintext passwords or real
password hashes to public repositories.

When configured, `/healthz` remains unauthenticated for health checks. App
routes, JSON export, and document downloads require credentials.

Built-in authentication is only appropriate over `localhost`, HTTPS, VPN, or
another encrypted/trusted channel. Do not publish the app over plain HTTP on an
untrusted network.

For HTTPS reverse-proxy deployments, also set
`JOBHUNT_SECURE_COOKIES=true` in `.env` so browsers only send app cookies over
HTTPS. Set `JOBHUNT_AUTH_TRUST_PROXY_HEADERS=true` only when the app is behind a
trusted reverse proxy that sanitizes forwarded headers. See
[REVERSE_PROXY.md](REVERSE_PROXY.md) for the recommended exposure pattern.
Deployed non-loopback instances, including firblab-v2/GitLab CI deployments,
must use login auth. Keep secure cookies off for direct plain-HTTP LAN access;
turn them on when users access the app through a trusted HTTPS reverse proxy.

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
ghcr.io/firblab-blog/jobhunt-os:v0.1.4
```

## Common Commands

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

Start it again:

```sh
docker compose up -d
```

Restart after changing configuration:

```sh
docker compose up -d
```

## Network Binding

The default Compose file publishes JobHunt OS on the host loopback interface:

```yaml
ports:
  - "127.0.0.1:8080:8080"
```

That means the app is reachable from the host at `http://127.0.0.1:8080`, but
is not directly exposed on every network interface.

Inside the container, `JOBHUNT_ADDR` is set to `0.0.0.0:8080`. That is expected:
the process must listen on the container network interface so Docker can forward
traffic to it. The host-side `127.0.0.1:8080:8080` port mapping is what keeps
the install local-only from outside the machine.

For HTTPS or a public hostname, put a reverse proxy in front of the local port.
See [REVERSE_PROXY.md](REVERSE_PROXY.md).

Local desktop use may run without auth on loopback. Non-loopback no-auth is
refused unless both network binding is enabled and
`JOBHUNT_ALLOW_INSECURE_NO_AUTH=true` is set for a deployment protected
elsewhere.

## Next Steps

- Review [CONFIGURATION.md](CONFIGURATION.md) before changing environment
  variables.
- Read [BACKUP_AND_RESTORE.md](BACKUP_AND_RESTORE.md) before putting important
  data in the app, and treat JSON exports as sensitive job-hunt records.
- Read [UPGRADING.md](UPGRADING.md) before pulling a new image.
