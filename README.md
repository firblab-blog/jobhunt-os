# JobHunt OS

JobHunt OS is a local-first, self-hosted job hunt command center for tracking
applications, documents, correspondence, interviews, follow-ups, and outcomes
without handing private career data to a SaaS platform.

## Install with Docker Compose

Docker Compose is the recommended install path for end users.

On Linux, if you want `./data` files owned by your host user from first boot,
create the optional `.env` file shown in [Data Directory](#data-directory) after
downloading `docker-compose.yml` and before running `docker compose up -d`.

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

The default Compose file stores JobHunt OS data in `./data` next to
`docker-compose.yml`. Back up `./data` before upgrades and before putting
important job hunt data in the app.

Upgrade a Compose install from the directory that contains `docker-compose.yml`:

```sh
docker compose pull
docker compose up -d
```

## Container Images

The public container image is published under:

```text
ghcr.io/firblab-blog/jobhunt-os
```

Public image tags use these shapes:

```text
ghcr.io/firblab-blog/jobhunt-os:latest
ghcr.io/firblab-blog/jobhunt-os:vX.Y.Z
ghcr.io/firblab-blog/jobhunt-os:sha-<shortsha>
```

The provided Compose file uses `latest`. When available, pin a versioned or
commit tag in `docker-compose.yml` when you want more explicit upgrades.

## Data Directory

The Compose file prepares this host-side layout:

```text
data/
  jobhunt-os.db
  documents/
  tmp/
```

On Linux, you can make new files in `./data` owned by your host user by creating
a `.env` file before first start:

```sh
printf "JOBHUNT_UID=%s\nJOBHUNT_GID=%s\n" "$(id -u)" "$(id -g)" > .env
```

If you skip this, the install still starts with the image's non-root UID/GID.
You can add `.env` later and run `docker compose up -d` again; the data-prep
helper will be recreated if the Compose configuration changes and will repair
ownership for the configured UID/GID.

## Self-Hosting Docs

- [Install](docs/INSTALL.md)
- [Configuration](docs/CONFIGURATION.md)
- [Backup and restore](docs/BACKUP_AND_RESTORE.md)
- [Upgrading](docs/UPGRADING.md)
- [Reverse proxy](docs/REVERSE_PROXY.md)
- [Releases](docs/RELEASES.md)
- [Operations](docs/OPERATIONS.md)

## Developer and Source Usage

Run from source:

```sh
go run ./cmd/jobhunt-os
```

Then open `http://127.0.0.1:8080`.

Set a different address with:

```sh
JOBHUNT_ADDR=127.0.0.1:9090 go run ./cmd/jobhunt-os
```

Build and run a local image for development testing:

```sh
docker build -t jobhunt-os:local .
mkdir -p tmp/jobhunt-os-data/tmp
docker run --rm --user "$(id -u):$(id -g)" -p 127.0.0.1:8080:8080 -v "$PWD/tmp/jobhunt-os-data:/data" jobhunt-os:local
```

Run tests:

```sh
go test ./...
```

## Current Status

This is a clean rebuild. The older `firblab-job-hunt` repository is treated as
historical data and product research, not as the codebase to carry forward.

The repository is starting intentionally small:

- Go backend and server-rendered UI
- Standard library only at scaffold time
- Local data storage planned around SQLite
- Manual-first workflows before any automation
- Synthetic fixtures only; no real personal data belongs in this repository

## Public Readiness

- License: MIT; see [LICENSE](LICENSE).
- Data hygiene: do not commit real resumes, cover letters, applications,
  recruiter messages, correspondence, or other personal job hunt data. Public
  fixtures should be synthetic and named `sample-*.yaml`.
- Module path: `github.com/firblab-blog/jobhunt-os`.

## Product Shape

The first durable product should make the manual job hunt workflow excellent:

- application tracker with status, priority, compensation, location, source,
  contacts, and next action
- document library for resumes, cover letters, work samples, and reusable
  snippets
- correspondence log for recruiter and hiring-team updates
- interview and follow-up timeline
- dashboard for stale applications, upcoming actions, active loops, and recent
  changes
- import/export so users can leave at any time

See [docs/ROADMAP.md](docs/ROADMAP.md) and
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Dependency Posture

The project defaults to a boring, reviewable dependency posture. Any dependency
must earn its place by reducing real risk or complexity. The first important
dependency decision is SQLite access; see
[docs/decisions/0001-sqlite-driver.md](docs/decisions/0001-sqlite-driver.md).

## License

JobHunt OS is available under the MIT License; see [LICENSE](LICENSE).
