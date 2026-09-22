package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"kvm-manager/backend/internal/domain"
)

// CreateSession 写入一条会话记录，登录签发 JWT 时调用。
func (s *Store) CreateSession(ctx context.Context, session domain.UserSession) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO user_sessions(jti, user_id, username, source, created_at, expires_at)
		VALUES($1, $2, $3, $4, $5, $6)
	`, session.JTI, session.UserID, session.Username, session.Source, session.CreatedAt, session.ExpiresAt)
	return err
}

// FindSession 按 JTI 查询会话并联出用户信息，认证中间件校验令牌是否仍有效时调用。
func (s *Store) FindSession(ctx context.Context, jti string) (domain.Session, error) {
	var session domain.Session
	var user domain.User
	err := s.pool.QueryRow(ctx, `
		SELECT s.expires_at,
		       u.id::text, u.username, u.display_name, u.role, COALESCE(NULLIF(u.source, ''), 'local'), u.disabled, u.last_login_at, u.created_at, u.updated_at
		FROM user_sessions s JOIN users u ON u.id = s.user_id
		WHERE s.jti=$1
	`, jti).Scan(&session.ExpiresAt, &user.ID, &user.Username, &user.DisplayName, &user.Role, &user.Source, &user.Disabled, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, ErrNotFound
	}
	if err != nil {
		return domain.Session{}, err
	}
	users, err := s.attachAccessToUsers(ctx, []domain.User{user})
	if err != nil {
		return domain.Session{}, err
	}
	session.User = users[0]
	return session, nil
}

// DeleteSession 按 JTI 删除会话记录，用户注销时调用使令牌立即失效。
func (s *Store) DeleteSession(ctx context.Context, jti string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM user_sessions WHERE jti=$1`, jti)
	return err
}

// DeleteExpiredSessions 删除已过期的会话记录，登录签发时惰性调用。
func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM user_sessions WHERE expires_at <= now()`)
	return err
}

// DeleteUserSessions 删除指定用户的全部会话记录，找回密码重置成功后强制重新登录时调用。
func (s *Store) DeleteUserSessions(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM user_sessions WHERE user_id=$1`, userID)
	return err
}
