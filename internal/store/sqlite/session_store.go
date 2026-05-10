package sqlite

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/firblab-blog/jobhunt-os/internal/session"
)

type SessionStore struct {
	db     *sql.DB
	policy session.Policy
	now    func() time.Time
}

func NewSessionStore(db *sql.DB, policy session.Policy) *SessionStore {
	return &SessionStore{
		db:     db,
		policy: policy,
		now:    time.Now,
	}
}

func (s *SessionStore) CreateSession(ctx context.Context, metadata session.Metadata) (session.Session, string, error) {
	if err := s.policy.Validate(); err != nil {
		return session.Session{}, "", err
	}

	token, tokenHash, err := newSessionToken()
	if err != nil {
		return session.Session{}, "", err
	}
	id, err := newID("ses")
	if err != nil {
		return session.Session{}, "", err
	}

	now := s.currentTime()
	expiresAt := now.Add(s.policy.AbsoluteTimeout).UTC().Truncate(time.Millisecond)
	_, err = s.db.ExecContext(ctx, insertSessionSQL,
		id,
		tokenHash,
		formatTime(now),
		formatTime(now),
		formatTime(expiresAt),
		nullableString(userAgentHash(metadata.UserAgent)),
		nullableString(strings.TrimSpace(metadata.ClientIPPrefix)),
	)
	if err != nil {
		return session.Session{}, "", fmt.Errorf("create session: %w", err)
	}

	created, err := querySessionByID(ctx, s.db, id)
	if err != nil {
		return session.Session{}, "", err
	}
	return created, token, nil
}

func (s *SessionStore) LookupSession(ctx context.Context, rawToken string) (session.Session, error) {
	return s.lookupSession(ctx, rawToken, s.currentTime())
}

func (s *SessionStore) TouchSession(ctx context.Context, rawToken string) (session.Session, error) {
	now := s.currentTime()
	found, err := s.lookupSession(ctx, rawToken, now)
	if err != nil {
		return session.Session{}, err
	}

	result, err := s.db.ExecContext(ctx, touchSessionSQL, formatTime(now), found.ID)
	if err != nil {
		return session.Session{}, fmt.Errorf("touch session: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return session.Session{}, fmt.Errorf("check session touch: %w", err)
	}
	if updated == 0 {
		return session.Session{}, session.ErrNotFound
	}

	touched, err := querySessionByID(ctx, s.db, found.ID)
	if err != nil {
		return session.Session{}, err
	}
	if !s.sessionActive(touched, now) {
		return session.Session{}, session.ErrNotFound
	}
	return touched, nil
}

func (s *SessionStore) RevokeSession(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("session id is required")
	}

	result, err := s.db.ExecContext(ctx, revokeSessionSQL, formatTime(s.currentTime()), id)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check session revoke: %w", err)
	}
	if updated == 0 {
		return session.ErrNotFound
	}
	return nil
}

func (s *SessionStore) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	if err := s.policy.Validate(); err != nil {
		return 0, err
	}

	now := s.currentTime()
	idleCutoff := now.Add(-s.policy.IdleTimeout).UTC().Truncate(time.Millisecond)
	result, err := s.db.ExecContext(ctx, deleteExpiredSessionsSQL, formatTime(now), formatTime(idleCutoff))
	if err != nil {
		return 0, fmt.Errorf("cleanup expired sessions: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("check expired session cleanup: %w", err)
	}
	return deleted, nil
}

func (s *SessionStore) lookupSession(ctx context.Context, rawToken string, now time.Time) (session.Session, error) {
	if err := s.policy.Validate(); err != nil {
		return session.Session{}, err
	}

	tokenHash, err := hashRawSessionToken(rawToken)
	if err != nil {
		return session.Session{}, session.ErrNotFound
	}

	found, storedHash, err := querySessionByTokenHash(ctx, s.db, tokenHash)
	if err != nil {
		return session.Session{}, err
	}
	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(tokenHash)) != 1 {
		return session.Session{}, session.ErrNotFound
	}
	if !s.sessionActive(found, now) {
		return session.Session{}, session.ErrNotFound
	}
	return found, nil
}

func (s *SessionStore) sessionActive(found session.Session, now time.Time) bool {
	if found.RevokedAt != nil {
		return false
	}

	now = now.UTC()
	if !now.Before(found.ExpiresAt) {
		return false
	}

	idleExpiresAt := found.LastSeenAt.Add(s.policy.IdleTimeout)
	return now.Before(idleExpiresAt)
}

func (s *SessionStore) currentTime() time.Time {
	return s.now().UTC().Truncate(time.Millisecond)
}

type sessionQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type sessionScanner interface {
	Scan(dest ...any) error
}

func querySessionByID(ctx context.Context, q sessionQueryer, id string) (session.Session, error) {
	found, _, err := scanSession(q.QueryRowContext(ctx, selectSessionByIDSQL, id))
	if err == nil {
		return found, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return session.Session{}, session.ErrNotFound
	}
	return session.Session{}, fmt.Errorf("get session: %w", err)
}

func querySessionByTokenHash(ctx context.Context, q sessionQueryer, tokenHash string) (session.Session, string, error) {
	found, storedHash, err := scanSession(q.QueryRowContext(ctx, selectSessionByTokenHashSQL, tokenHash))
	if err == nil {
		return found, storedHash, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return session.Session{}, "", session.ErrNotFound
	}
	return session.Session{}, "", fmt.Errorf("get session: %w", err)
}

func scanSession(row sessionScanner) (session.Session, string, error) {
	var found session.Session
	var tokenHash string
	var createdAt string
	var lastSeenAt string
	var expiresAt string
	var revokedAt sql.NullString
	var userAgentHash sql.NullString
	var clientIPPrefix sql.NullString

	err := row.Scan(
		&found.ID,
		&tokenHash,
		&createdAt,
		&lastSeenAt,
		&expiresAt,
		&revokedAt,
		&userAgentHash,
		&clientIPPrefix,
	)
	if err != nil {
		return session.Session{}, "", err
	}

	var parseErr error
	found.CreatedAt, parseErr = parseSQLiteTime(createdAt)
	if parseErr != nil {
		return session.Session{}, "", fmt.Errorf("parse session created_at: %w", parseErr)
	}
	found.LastSeenAt, parseErr = parseSQLiteTime(lastSeenAt)
	if parseErr != nil {
		return session.Session{}, "", fmt.Errorf("parse session last_seen_at: %w", parseErr)
	}
	found.ExpiresAt, parseErr = parseSQLiteTime(expiresAt)
	if parseErr != nil {
		return session.Session{}, "", fmt.Errorf("parse session expires_at: %w", parseErr)
	}
	if revokedAt.Valid {
		parsed, err := parseSQLiteTime(revokedAt.String)
		if err != nil {
			return session.Session{}, "", fmt.Errorf("parse session revoked_at: %w", err)
		}
		found.RevokedAt = &parsed
	}
	if userAgentHash.Valid {
		found.UserAgentHash = userAgentHash.String
	}
	if clientIPPrefix.Valid {
		found.ClientIPPrefix = clientIPPrefix.String
	}

	return found, tokenHash, nil
}

func newSessionToken() (string, string, error) {
	raw := make([]byte, session.TokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate session token: %w", err)
	}

	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, hashSessionTokenBytes(raw), nil
}

func hashRawSessionToken(rawToken string) (string, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return "", fmt.Errorf("session token is required")
	}

	raw, err := base64.RawURLEncoding.DecodeString(rawToken)
	if err != nil || len(raw) < session.TokenBytes {
		return "", fmt.Errorf("session token is invalid")
	}
	return hashSessionTokenBytes(raw), nil
}

func hashSessionTokenBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func userAgentHash(userAgent string) string {
	userAgent = strings.TrimSpace(userAgent)
	if userAgent == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(userAgent))
	return hex.EncodeToString(sum[:])
}

const sessionColumnsSQL = `
id,
token_hash,
created_at,
last_seen_at,
expires_at,
revoked_at,
user_agent_hash,
client_ip_prefix
`

const insertSessionSQL = `
INSERT INTO sessions (
  id,
  token_hash,
  created_at,
  last_seen_at,
  expires_at,
  user_agent_hash,
  client_ip_prefix
)
VALUES (?, ?, ?, ?, ?, ?, ?);
`

const selectSessionByIDSQL = `
SELECT ` + sessionColumnsSQL + `
FROM sessions
WHERE id = ?;
`

const selectSessionByTokenHashSQL = `
SELECT ` + sessionColumnsSQL + `
FROM sessions
WHERE token_hash = ?;
`

const touchSessionSQL = `
UPDATE sessions
SET last_seen_at = ?
WHERE id = ?
  AND revoked_at IS NULL;
`

const revokeSessionSQL = `
UPDATE sessions
SET revoked_at = ?
WHERE id = ?
  AND revoked_at IS NULL;
`

const deleteExpiredSessionsSQL = `
DELETE FROM sessions
WHERE expires_at <= ?
   OR last_seen_at <= ?;
`
