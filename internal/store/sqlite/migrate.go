package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	jobhuntos "github.com/firblab-blog/jobhunt-os"
)

const migrationsGlob = "migrations/*.sql"

func Migrate(ctx context.Context, db *sql.DB) error {
	return MigrateFS(ctx, db, jobhuntos.Migrations)
}

func MigrateFS(ctx context.Context, db *sql.DB, migrations fs.FS) error {
	names, err := fs.Glob(migrations, migrationsGlob)
	if err != nil {
		return fmt.Errorf("find sqlite migrations: %w", err)
	}
	sort.Strings(names)

	if _, err := db.ExecContext(ctx, createSchemaMigrationsSQL); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	for _, name := range names {
		applied, err := migrationApplied(ctx, db, migrationVersion(name))
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		body, err := fs.ReadFile(migrations, name)
		if err != nil {
			return fmt.Errorf("read sqlite migration %q: %w", name, err)
		}
		if err := applyMigration(ctx, db, migrationVersion(name), string(body)); err != nil {
			return fmt.Errorf("apply sqlite migration %q: %w", name, err)
		}
	}

	return nil
}

func migrationApplied(ctx context.Context, db *sql.DB, version string) (bool, error) {
	var applied int
	err := db.QueryRowContext(ctx, selectSchemaMigrationSQL, version).Scan(&applied)
	if err == nil {
		return applied == 1, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, fmt.Errorf("check sqlite migration %q: %w", version, err)
}

func applyMigration(ctx context.Context, db *sql.DB, version string, migrationSQL string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, migrationSQL); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, insertSchemaMigrationSQL, version); err != nil {
		return err
	}

	return tx.Commit()
}

func migrationVersion(name string) string {
	return strings.TrimSuffix(path.Base(name), ".sql")
}

const createSchemaMigrationsSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TEXT NOT NULL
);
`

const selectSchemaMigrationSQL = `
SELECT 1
FROM schema_migrations
WHERE version = ?;
`

const insertSchemaMigrationSQL = `
INSERT INTO schema_migrations (version, applied_at)
VALUES (?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
`
