# 0001: SQLite Driver Decision

Status: proposed

## Context

JobHunt OS should use SQLite for local storage because it is portable, durable, easy to back up, and requires no external service. Go's standard library provides `database/sql`, but it does not include a SQLite driver.

## Options

### `modernc.org/sqlite`

- Pure Go.
- Avoids CGO for easier cross-compilation.
- Brings a larger Go dependency graph.

### `github.com/mattn/go-sqlite3`

- Mature and widely used.
- Wraps the canonical SQLite C library.
- Requires CGO, which complicates cross-platform release builds.

### Embedded SQLite via Rust sidecar

- Keeps Go web code simple while using Rust storage tooling.
- Adds architecture complexity too early.

## Lean Recommendation

Start with explicit repository interfaces and migration files, but do not add a SQLite driver until the first persistent workflow is ready. When persistence starts, prefer `modernc.org/sqlite` for portability unless its dependency graph feels too large after review.

Whichever driver is selected, keep SQL explicit and avoid an ORM.
