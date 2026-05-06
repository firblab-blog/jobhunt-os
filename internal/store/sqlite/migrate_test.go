package sqlite

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestOpenCreatesDataDirAndConfiguresPragmas(t *testing.T) {
	t.Parallel()

	dataDir := filepath.Join(t.TempDir(), "state")
	db, dbPath, err := Open(context.Background(), dataDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if dbPath != filepath.Join(dataDir, DatabaseFilename) {
		t.Fatalf("dbPath = %q, want %q", dbPath, filepath.Join(dataDir, DatabaseFilename))
	}
	if _, err := os.Stat(dataDir); err != nil {
		t.Fatalf("data dir was not created: %v", err)
	}

	var foreignKeys int
	if err := db.QueryRowContext(context.Background(), "PRAGMA foreign_keys;").Scan(&foreignKeys); err != nil {
		t.Fatalf("query foreign_keys pragma: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("PRAGMA foreign_keys = %d, want 1", foreignKeys)
	}

	var busyTimeout int
	if err := db.QueryRowContext(context.Background(), "PRAGMA busy_timeout;").Scan(&busyTimeout); err != nil {
		t.Fatalf("query busy_timeout pragma: %v", err)
	}
	if busyTimeout != 5000 {
		t.Fatalf("PRAGMA busy_timeout = %d, want 5000", busyTimeout)
	}
}

func TestMigrateFSAppliesMigrationsInLexicalOrderOnce(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMigration(t, root, "0002_second.sql", `
INSERT INTO order_log (value)
SELECT value || '-second'
FROM order_log
WHERE value = 'first';
`)
	writeMigration(t, root, "0001_first.sql", `
CREATE TABLE order_log (
  value TEXT NOT NULL
);

INSERT INTO order_log (value)
VALUES ('first');
`)

	db := openTestDB(t)

	if err := MigrateFS(context.Background(), db, os.DirFS(root)); err != nil {
		t.Fatalf("MigrateFS() error = %v", err)
	}
	if err := MigrateFS(context.Background(), db, os.DirFS(root)); err != nil {
		t.Fatalf("second MigrateFS() error = %v", err)
	}

	values := queryStrings(t, db, "SELECT value FROM order_log ORDER BY rowid;")
	wantValues := []string{"first", "first-second"}
	if !reflect.DeepEqual(values, wantValues) {
		t.Fatalf("order_log values = %v, want %v", values, wantValues)
	}

	versions := queryStrings(t, db, "SELECT version FROM schema_migrations ORDER BY version;")
	wantVersions := []string{"0001_first", "0002_second"}
	if !reflect.DeepEqual(versions, wantVersions) {
		t.Fatalf("schema_migrations versions = %v, want %v", versions, wantVersions)
	}
}

func TestMigrateAppliesEmbeddedMigrations(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	var count int
	err := db.QueryRowContext(context.Background(), `
SELECT count(*)
FROM sqlite_master
WHERE type = 'table'
  AND name IN ('applications', 'contacts', 'schema_migrations');
`).Scan(&count)
	if err != nil {
		t.Fatalf("query migrated tables: %v", err)
	}
	if count != 3 {
		t.Fatalf("migrated table count = %d, want 3", count)
	}
}

func TestMigrateFSRollsBackFailedMigration(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMigration(t, root, "0001_broken.sql", `
CREATE TABLE partial_migration (
  id TEXT PRIMARY KEY
);

INSERT INTO missing_table (id)
VALUES ('nope');
`)

	db := openTestDB(t)

	if err := MigrateFS(context.Background(), db, os.DirFS(root)); err == nil {
		t.Fatalf("MigrateFS() error = nil, want error")
	}

	var count int
	err := db.QueryRowContext(context.Background(), `
SELECT count(*)
FROM sqlite_master
WHERE type = 'table'
  AND name = 'partial_migration';
`).Scan(&count)
	if err != nil {
		t.Fatalf("query partial migration table: %v", err)
	}
	if count != 0 {
		t.Fatalf("partial_migration table count = %d, want 0", count)
	}
}

func writeMigration(t *testing.T, root string, name string, body string) {
	t.Helper()

	dir := filepath.Join(root, "migrations")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create migrations dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write migration %s: %v", name, err)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, _, err := Open(context.Background(), filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func queryStrings(t *testing.T, db *sql.DB, query string) []string {
	t.Helper()

	rows, err := db.QueryContext(context.Background(), query)
	if err != nil {
		t.Fatalf("query strings: %v", err)
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatalf("scan string: %v", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate strings: %v", err)
	}

	return values
}
