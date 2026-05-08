# Install JobHunt OS

Docker Compose is the recommended self-hosted install path for JobHunt OS.
There is no management CLI, Helm chart, or binary release channel required for
the current self-hosted setup.

## Requirements

- Docker
- Docker Compose v2, usually available as `docker compose`
- A host directory where JobHunt OS can keep `docker-compose.yml` and `data/`

## Quick Install

Create an install directory, download the Compose file, and start the app:

On Linux, if you want `./data` files owned by your host user from first boot,
create the optional `.env` file in [Linux Data Ownership](#linux-data-ownership)
after downloading `docker-compose.yml` and before running
`docker compose up -d`.

```sh
mkdir jobhunt-os
cd jobhunt-os
curl -O https://raw.githubusercontent.com/firblab-blog/jobhunt-os/main/deploy/docker-compose.yml
docker compose up -d
```

Then open:

```text
http://127.0.0.1:8080
```

The default Compose file stores application data in `./data` next to
`docker-compose.yml`.

## Linux Data Ownership

The app container runs as a non-root user. The Compose file includes a one-shot
`jobhunt-os-init` helper that creates the expected `./data` directories and
sets ownership before the app starts.

For host-owned files on Linux, create a `.env` file before first boot:

```sh
printf "JOBHUNT_UID=%s\nJOBHUNT_GID=%s\n" "$(id -u)" "$(id -g)" > .env
docker compose up -d
```

The `.env` file is optional, but it is recommended for Linux bind mounts because
it makes the container write `./data` files as your host user. If you add or
change it later, run `docker compose up -d` again so the data-prep helper can
be recreated and adjust ownership.

The prepared data directory has this shape:

```text
data/
  jobhunt-os.db
  documents/
  tmp/
```

## Public Image

The Compose file uses the public container image:

```text
ghcr.io/firblab-blog/jobhunt-os:latest
```

The `latest` tag is the default install path for now. When versioned tags are
published, users who want slower, explicit upgrades should pin the image tag in
`docker-compose.yml` instead of tracking `latest`.

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

## Next Steps

- Review [CONFIGURATION.md](CONFIGURATION.md) before changing environment
  variables.
- Read [BACKUP_AND_RESTORE.md](BACKUP_AND_RESTORE.md) before putting important
  data in the app.
- Read [UPGRADING.md](UPGRADING.md) before pulling a new image.
