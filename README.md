# JobHunt OS

JobHunt OS is a local-first, self-hosted app for tracking applications,
documents, contacts, next actions, interviews, and outcomes without sending
private career data to a SaaS platform.

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
important job hunt data in the app. The backup guide keeps the default flow
simple and includes optional encrypted storage examples.

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

The provided Compose file uses `latest`, which tracks the newest versioned
release. Pin a versioned or commit tag in `docker-compose.yml` when you want
more explicit upgrades.

For the `v0.1.4` release line:

```text
ghcr.io/firblab-blog/jobhunt-os:v0.1.4
```

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
- [Security](docs/SECURITY.md)
- [Security review](docs/SECURITY_REVIEW.md)
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

## Current Capabilities

As of `v0.1.4`, JobHunt OS includes:

- Dashboard with an application pipeline Sankey and signal strip
- Applications page with next actions, search, status filtering, and a Sankey
  flow above the application list
- Application detail pages with timeline events, status updates, next actions,
  and posting PDF attachment support
- Documents page for PDF uploads and downloads
- Contacts page
- Settings page with theme selection and JSON export
- SQLite storage under the configured data directory

The repository must not contain real personal job hunt data. Public fixtures
and test data should be synthetic.

## Public Readiness

- License: MIT; see [LICENSE](LICENSE).
- Data hygiene: do not commit real resumes, cover letters, applications,
  recruiter messages, correspondence, or other personal job hunt data. Public
  fixtures should be synthetic and named `sample-*.yaml`.
- Agent safety: AI agents working in this repository must default to read-only
  behavior; see [AGENTS.md](AGENTS.md).
- Security posture: see the public
  [security review](docs/SECURITY_REVIEW.md).
- Module path: `github.com/firblab-blog/jobhunt-os`.

## Product Scope

The current product is centered on manual job hunt tracking:

- applications with status, priority, compensation, location, source, notes,
  contacts, and next action fields
- documents for resumes, cover letters, work samples, and job posting PDFs
- contacts connected to applications and timeline entries
- timeline events for interviews, follow-ups, decisions, and notes
- dashboard and application-level pipeline views
- JSON export for review and migration work; exports contain sensitive job-hunt
  data and should be stored carefully

See [docs/ROADMAP.md](docs/ROADMAP.md) and
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Dependency Posture

The project keeps dependencies limited and reviewable. Any dependency should
reduce implementation risk or remove meaningful complexity. The first recorded
dependency decision is SQLite access; see
[docs/decisions/0001-sqlite-driver.md](docs/decisions/0001-sqlite-driver.md).

## License

JobHunt OS is available under the MIT License; see [LICENSE](LICENSE).
