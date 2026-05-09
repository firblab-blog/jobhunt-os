# 0001: SQLite Driver Decision

Status: accepted

## Context

JobHunt OS should use SQLite for local storage because it is portable, durable,
file-backed, and requires no external service. Go's standard library provides
`database/sql`, but it does not include a SQLite driver.

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

- Keeps the Go web code limited to `database/sql`.
- Adds architecture complexity too early.

## Decision

Use explicit repository interfaces, SQL migrations, and `modernc.org/sqlite`.

The portability tradeoff is intentional: pure Go and no CGO reduce build
requirements, especially for machines without native compiler toolchains. The
cost is a larger Go dependency graph, which must be reviewed before the
dependency is added.

Keep SQL explicit and avoid an ORM. The store enables foreign key enforcement
through the SQLite DSN before running migrations or application queries.
