package discord

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RateLimitStore persists the Discord rate-limit backoff deadline so it survives process
// restarts (e.g. Render free tier spinning the service down/up between requests).
type RateLimitStore interface {
	GetBlockedUntil(ctx context.Context) (time.Time, error)
	SetBlockedUntil(ctx context.Context, until time.Time) error
}

type PgRateLimitStore struct {
	conn *pgxpool.Pool
}

func NewPgRateLimitStore(conn *pgxpool.Pool) *PgRateLimitStore {
	return &PgRateLimitStore{conn: conn}
}

func (s *PgRateLimitStore) GetBlockedUntil(ctx context.Context) (time.Time, error) {
	var until time.Time
	err := s.conn.QueryRow(ctx, `SELECT "blockedUntil" FROM "game-table-booking".discord_rate_limit WHERE id = 1`).Scan(&until)

	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}

	if err != nil {
		return time.Time{}, fmt.Errorf("failed to load discord rate limit state: %w", err)
	}

	return until, nil
}

func (s *PgRateLimitStore) SetBlockedUntil(ctx context.Context, until time.Time) error {
	_, err := s.conn.Exec(ctx, `
		INSERT INTO "game-table-booking".discord_rate_limit (id, "blockedUntil")
		VALUES (1, $1)
		ON CONFLICT (id) DO UPDATE SET "blockedUntil" = EXCLUDED."blockedUntil"
	`, until)

	if err != nil {
		return fmt.Errorf("failed to persist discord rate limit state: %w", err)
	}

	return nil
}
