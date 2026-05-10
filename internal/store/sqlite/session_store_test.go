package sqlite

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/firblab-blog/jobhunt-os/internal/session"
)

func TestSessionStoreCreateAndLookupValidSession(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, _, _ := newMigratedSessionStore(t, session.Policy{
		IdleTimeout:     30 * time.Minute,
		AbsoluteTimeout: 24 * time.Hour,
	})

	created, token, err := st.CreateSession(ctx, session.Metadata{
		UserAgent:      "Mozilla/5.0",
		ClientIPPrefix: "203.0.113.0/24",
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if created.ID == "" {
		t.Fatalf("session ID was empty")
	}
	rawToken, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("token was not base64url: %v", err)
	}
	if len(rawToken) < session.TokenBytes {
		t.Fatalf("token bytes = %d, want at least %d", len(rawToken), session.TokenBytes)
	}
	if created.UserAgentHash == "" {
		t.Fatalf("user agent hash was empty")
	}
	if created.ClientIPPrefix != "203.0.113.0/24" {
		t.Fatalf("client IP prefix = %q, want 203.0.113.0/24", created.ClientIPPrefix)
	}

	lookedUp, err := st.LookupSession(ctx, token)
	if err != nil {
		t.Fatalf("LookupSession() error = %v", err)
	}
	if lookedUp.ID != created.ID {
		t.Fatalf("LookupSession() ID = %q, want %q", lookedUp.ID, created.ID)
	}
	if !lookedUp.ExpiresAt.Equal(created.ExpiresAt) {
		t.Fatalf("LookupSession() ExpiresAt = %v, want %v", lookedUp.ExpiresAt, created.ExpiresAt)
	}
}

func TestSessionStoreDoesNotStoreRawToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, _, db := newMigratedSessionStore(t, session.Policy{
		IdleTimeout:     30 * time.Minute,
		AbsoluteTimeout: 24 * time.Hour,
	})

	created, token, err := st.CreateSession(ctx, session.Metadata{})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	var tokenHash string
	err = db.QueryRowContext(ctx, "SELECT token_hash FROM sessions WHERE id = ?;", created.ID).Scan(&tokenHash)
	if err != nil {
		t.Fatalf("query token_hash: %v", err)
	}
	if tokenHash == token {
		t.Fatalf("stored token_hash matched raw token")
	}
	if len(tokenHash) != 64 {
		t.Fatalf("token_hash length = %d, want 64 hex characters", len(tokenHash))
	}

	var rawTokenOccurrences int
	err = db.QueryRowContext(ctx, `
SELECT count(*)
FROM sessions
WHERE id = ?
  AND (
    token_hash = ?
    OR user_agent_hash = ?
    OR client_ip_prefix = ?
  );
`, created.ID, token, token, token).Scan(&rawTokenOccurrences)
	if err != nil {
		t.Fatalf("query raw token occurrences: %v", err)
	}
	if rawTokenOccurrences != 0 {
		t.Fatalf("raw token was stored in session row")
	}
}

func TestSessionStoreRejectsInvalidToken(t *testing.T) {
	t.Parallel()

	st, _, _ := newMigratedSessionStore(t, session.Policy{
		IdleTimeout:     30 * time.Minute,
		AbsoluteTimeout: 24 * time.Hour,
	})

	_, err := st.LookupSession(context.Background(), "not-a-valid-session-token")
	if !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("LookupSession(invalid) error = %v, want ErrNotFound", err)
	}
}

func TestSessionStoreRejectsExpiredSession(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, setNow, db := newMigratedSessionStore(t, session.Policy{
		IdleTimeout:     30 * time.Minute,
		AbsoluteTimeout: 24 * time.Hour,
	})
	created, token, err := st.CreateSession(ctx, session.Metadata{})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	expiresAt := created.CreatedAt.Add(time.Minute)
	if _, err := db.ExecContext(ctx, "UPDATE sessions SET expires_at = ? WHERE id = ?;", formatTime(expiresAt), created.ID); err != nil {
		t.Fatalf("force expire session: %v", err)
	}
	setNow(expiresAt)

	_, err = st.LookupSession(ctx, token)
	if !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("LookupSession(expired) error = %v, want ErrNotFound", err)
	}
}

func TestSessionStoreRejectsRevokedSession(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, _, _ := newMigratedSessionStore(t, session.Policy{
		IdleTimeout:     30 * time.Minute,
		AbsoluteTimeout: 24 * time.Hour,
	})
	created, token, err := st.CreateSession(ctx, session.Metadata{})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := st.RevokeSession(ctx, created.ID); err != nil {
		t.Fatalf("RevokeSession() error = %v", err)
	}

	_, err = st.LookupSession(ctx, token)
	if !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("LookupSession(revoked) error = %v, want ErrNotFound", err)
	}
}

func TestSessionStoreIdleTimeout(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, setNow, _ := newMigratedSessionStore(t, session.Policy{
		IdleTimeout:     15 * time.Minute,
		AbsoluteTimeout: 24 * time.Hour,
	})
	start := fixedSessionTestTime()
	setNow(start)
	_, token, err := st.CreateSession(ctx, session.Metadata{})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	setNow(start.Add(14 * time.Minute))
	touched, err := st.TouchSession(ctx, token)
	if err != nil {
		t.Fatalf("TouchSession(before idle timeout) error = %v", err)
	}
	if !touched.LastSeenAt.Equal(start.Add(14 * time.Minute)) {
		t.Fatalf("LastSeenAt = %v, want %v", touched.LastSeenAt, start.Add(14*time.Minute))
	}

	setNow(start.Add(29*time.Minute - time.Millisecond))
	if _, err := st.LookupSession(ctx, token); err != nil {
		t.Fatalf("LookupSession(before refreshed idle timeout) error = %v", err)
	}

	setNow(start.Add(29 * time.Minute))
	_, err = st.LookupSession(ctx, token)
	if !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("LookupSession(after idle timeout) error = %v, want ErrNotFound", err)
	}
}

func TestSessionStoreAbsoluteTimeout(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, setNow, _ := newMigratedSessionStore(t, session.Policy{
		IdleTimeout:     24 * time.Hour,
		AbsoluteTimeout: 30 * time.Minute,
	})
	start := fixedSessionTestTime()
	setNow(start)
	_, token, err := st.CreateSession(ctx, session.Metadata{})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	setNow(start.Add(29*time.Minute + 59*time.Second))
	if _, err := st.TouchSession(ctx, token); err != nil {
		t.Fatalf("TouchSession(before absolute timeout) error = %v", err)
	}

	setNow(start.Add(30 * time.Minute))
	_, err = st.LookupSession(ctx, token)
	if !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("LookupSession(after absolute timeout) error = %v, want ErrNotFound", err)
	}
}

func TestSessionStoreCleanupExpiredSessions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, setNow, db := newMigratedSessionStore(t, session.Policy{
		IdleTimeout:     30 * time.Minute,
		AbsoluteTimeout: 24 * time.Hour,
	})
	start := fixedSessionTestTime()
	setNow(start)
	expired, _, err := st.CreateSession(ctx, session.Metadata{})
	if err != nil {
		t.Fatalf("CreateSession(expired) error = %v", err)
	}

	setNow(start.Add(2 * time.Hour))
	active, activeToken, err := st.CreateSession(ctx, session.Metadata{})
	if err != nil {
		t.Fatalf("CreateSession(active) error = %v", err)
	}

	deleted, err := st.CleanupExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("CleanupExpiredSessions() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("CleanupExpiredSessions() deleted = %d, want 1", deleted)
	}
	assertSessionRowCount(t, db, expired.ID, 0)
	assertSessionRowCount(t, db, active.ID, 1)

	if _, err := st.LookupSession(ctx, activeToken); err != nil {
		t.Fatalf("LookupSession(active after cleanup) error = %v", err)
	}
}

func newMigratedSessionStore(t *testing.T, policy session.Policy) (*SessionStore, func(time.Time), *sql.DB) {
	t.Helper()

	db := openTestDB(t)
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	st := NewSessionStore(db, policy)
	now := fixedSessionTestTime()
	st.now = func() time.Time {
		return now
	}
	return st, func(next time.Time) {
		now = next
	}, db
}

func fixedSessionTestTime() time.Time {
	return time.Date(2026, 5, 9, 12, 30, 0, 123000000, time.UTC)
}

func assertSessionRowCount(t *testing.T, db *sql.DB, id string, want int) {
	t.Helper()

	var count int
	err := db.QueryRowContext(context.Background(), "SELECT count(*) FROM sessions WHERE id = ?;", id).Scan(&count)
	if err != nil {
		t.Fatalf("query session count: %v", err)
	}
	if count != want {
		t.Fatalf("session row count for %q = %d, want %d", id, count, want)
	}
}
