# Operations

JobHunt OS is intended to be boring to run with Docker Compose. The durable
state for the provided Compose install lives in `./data` next to
`docker-compose.yml`.

## Common Commands

Run these from the directory that contains `docker-compose.yml`.

Start or recreate the container:

```sh
docker compose up -d
```

Follow logs:

```sh
docker compose logs -f
```

Show container status:

```sh
docker compose ps
```

Pull the configured image tag:

```sh
docker compose pull
```

Stop and remove the container:

```sh
docker compose down
```

Back up `./data` before upgrades or other risky maintenance. The JSON export is
useful for inspection and migration work, but it is not a full restore mechanism
for the running app.

## Health Check

The health check endpoint is:

```text
/healthz
```

For the default local install:

```sh
curl http://127.0.0.1:8080/healthz
```

## Data Location

The provided Compose file mounts:

```yaml
volumes:
  - ./data:/data
```

That means application data, including the SQLite database and local files,
lives in the host `./data` directory. Keep backups of that directory outside the
install directory, preferably on another disk or backup service.

Expected layout:

```text
data/
  jobhunt-os.db
  documents/
  tmp/
```

The `jobhunt-os-init` Compose helper creates `documents/` and `tmp/` before the
app starts and sets ownership for `JOBHUNT_UID`/`JOBHUNT_GID` from `.env` when
those values are present.

## Asking For Help

When asking for help, collect:

- JobHunt OS image tag, such as `latest`, `v0.1.0`, or `sha-<shortsha>`
- Docker and Docker Compose versions
- Output from `docker compose ps`
- Relevant `docker compose logs -f` lines from startup or the failure
- Any recent changes to image tag, configuration, proxy, host, or storage
- Whether `http://127.0.0.1:8080/healthz` responds on the host

Do not share private resumes, cover letters, applications, recruiter messages,
uploaded documents, or raw database files. Redact names, email addresses, phone
numbers, company-specific notes, tokens, hostnames, and paths that reveal private
information before posting logs or screenshots.
