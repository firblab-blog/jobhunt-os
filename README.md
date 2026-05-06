# JobHunt OS

JobHunt OS is a local-first job hunt command center for tracking applications, documents, correspondence, interviews, follow-ups, and outcomes without handing private career data to a SaaS platform.

This repository is starting intentionally small:

- Go backend and server-rendered UI
- Standard library only at scaffold time
- Local data storage planned around SQLite
- Manual-first workflows before any automation
- Synthetic fixtures only; real job hunt history stays private

## Current Status

This is a clean rebuild. The older `firblab-job-hunt` repository is treated as historical data and product research, not as the codebase to carry forward.

## Run

```sh
go run ./cmd/jobhunt-os
```

Then open `http://127.0.0.1:8080`.

Set a different address with:

```sh
JOBHUNT_ADDR=127.0.0.1:9090 go run ./cmd/jobhunt-os
```

## Test

```sh
go test ./...
```

## Product Shape

The first durable product should make the manual job hunt workflow excellent:

- application tracker with status, priority, compensation, location, source, contacts, and next action
- document library for resumes, cover letters, work samples, and reusable snippets
- correspondence log for recruiter and hiring-team updates
- interview and follow-up timeline
- dashboard for stale applications, upcoming actions, active loops, and recent changes
- import/export so users can leave at any time

See [docs/ROADMAP.md](docs/ROADMAP.md) and [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Dependency Posture

The project defaults to a boring, reviewable dependency posture. Any dependency must earn its place by reducing real risk or complexity. The first important dependency decision is SQLite access; see [docs/decisions/0001-sqlite-driver.md](docs/decisions/0001-sqlite-driver.md).

## License

License is not selected yet. Pick one before publishing a public mirror.
