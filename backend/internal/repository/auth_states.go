package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"kvm-manager/backend/internal/domain"
)

// CreateAuthState 写入一次性 OAuth state，并顺带清理过期记录，避免表无限增长。
func (s *Store) CreateAuthState(ctx context.Context, item domain.AuthState) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM auth_login_states WHERE expires_at < now()`); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO auth_login_states(state, provider, purpose, user_id, redirect, remote_ip, expires_at)
		VALUES($1, $2, $3, $4, $5, $6, $7)
	`, item.State, item.Provider, item.Purpose, item.UserID, item.Redirect, item.RemoteIP, item.ExpiresAt)
	return err
}

// TakeAuthState 原子取出并删除 state：取出即删，天然防重放。
func (s *Store) TakeAuthState(ctx context.Context, state string) (domain.AuthState, error) {
	var item domain.AuthState
	err := s.pool.QueryRow(ctx, `
		DELETE FROM auth_login_states
		WHERE state = $1
		RETURNING state, provider, purpose, user_id, redirect, remote_ip, expires_at
	`, state).Scan(&item.State, &item.Provider, &item.Purpose, &item.UserID, &item.Redirect, &item.RemoteIP, &item.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AuthState{}, ErrNotFound
	}
	return item, err
}
