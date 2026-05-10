// Package session defines server-side login session contracts.
package session

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const TokenBytes = 32

var ErrNotFound = errors.New("session: not found")

type Policy struct {
	IdleTimeout     time.Duration
	AbsoluteTimeout time.Duration
}

func (p Policy) Validate() error {
	if p.IdleTimeout <= 0 {
		return fmt.Errorf("idle timeout must be positive")
	}
	if p.AbsoluteTimeout <= 0 {
		return fmt.Errorf("absolute timeout must be positive")
	}
	return nil
}

type Metadata struct {
	UserAgent      string
	ClientIPPrefix string
}

type Session struct {
	ID             string
	CreatedAt      time.Time
	LastSeenAt     time.Time
	ExpiresAt      time.Time
	RevokedAt      *time.Time
	UserAgentHash  string
	ClientIPPrefix string
}

type Store interface {
	CreateSession(ctx context.Context, metadata Metadata) (Session, string, error)
	LookupSession(ctx context.Context, rawToken string) (Session, error)
	TouchSession(ctx context.Context, rawToken string) (Session, error)
	RevokeSession(ctx context.Context, id string) error
	CleanupExpiredSessions(ctx context.Context) (int64, error)
}
