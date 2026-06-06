package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"kvm-manager/backend/internal/domain"
)

func (s *Store) CreateSession(ctx context.Context, token string, userID string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO sessions(token_hash, user_id, expires_at, last_seen_at) VALUES($1, $2, $3, now())`, hashToken(token), userID, expiresAt)
	return err
}

func (s *Store) FindSession(ctx context.Context, token string) (domain.Session, error) {
	var session domain.Session
	err := s.pool.QueryRow(ctx, `
		SELECT u.id::text, u.username, u.display_name, u.role, COALESCE(NULLIF(u.source, ''), 'local'), u.disabled, u.last_login_at, u.created_at, u.updated_at, s.expires_at, s.last_seen_at
		FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.token_hash=$1
	`, hashToken(token)).Scan(&session.User.ID, &session.User.Username, &session.User.DisplayName, &session.User.Role, &session.User.Source, &session.User.Disabled, &session.User.LastLoginAt, &session.User.CreatedAt, &session.User.UpdatedAt, &session.ExpiresAt, &session.LastSeenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, ErrNotFound
	}
	if err != nil {
		return domain.Session{}, err
	}
	users, err := s.attachAccessToUsers(ctx, []domain.User{session.User})
	if err != nil {
		return domain.Session{}, err
	}
	session.User = users[0]
	session.Token = token
	return session, nil
}

func (s *Store) TouchSession(ctx context.Context, token string, seenAt time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE sessions SET last_seen_at=$2 WHERE token_hash=$1 AND last_seen_at < $2`, hashToken(token), seenAt)
	return err
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash=$1`, hashToken(token))
	return err
}

func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	return err
}
