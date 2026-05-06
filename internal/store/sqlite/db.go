// Package sqlite provides the SQLite-backed store foundation.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const (
	DatabaseFilename = "jobhunt-os.db"

	driverName         = "sqlite"
	busyTimeoutMillis  = "5000"
	foreignKeysEnabled = "ON"
)

func DBPath(dataDir string) string {
	return filepath.Join(dataDir, DatabaseFilename)
}

// Open creates dataDir when needed, opens jobhunt-os.db inside it, and enables
// SQLite PRAGMAs through the DSN so every opened connection receives them.
func Open(ctx context.Context, dataDir string) (*sql.DB, string, error) {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return nil, "", fmt.Errorf("sqlite data dir is required")
	}

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, "", fmt.Errorf("create sqlite data dir %q: %w", dataDir, err)
	}

	dbPath := DBPath(dataDir)
	db, err := sql.Open(driverName, dsn(dbPath))
	if err != nil {
		return nil, "", fmt.Errorf("open sqlite database %q: %w", dbPath, err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, "", fmt.Errorf("ping sqlite database %q: %w", dbPath, err)
	}

	return db, dbPath, nil
}

func dsn(dbPath string) string {
	query := url.Values{}
	query.Add("_pragma", "foreign_keys="+foreignKeysEnabled)
	query.Add("_pragma", "busy_timeout="+busyTimeoutMillis)

	u := url.URL{
		Scheme:   "file",
		Path:     filepath.ToSlash(dbPath),
		RawQuery: query.Encode(),
	}
	return u.String()
}
