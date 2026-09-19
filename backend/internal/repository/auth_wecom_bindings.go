package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/internal/service/auth"
)

// FindUserByWecomAccount 按企微账号查绑定用户：仅绑定表匹配，无兜底。
func (s *Store) FindUserByWecomAccount(ctx context.Context, userid string) (domain.User, error) {
	var user domain.User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id::text, u.username, COALESCE(u.email, ''), u.display_name, u.role,
		       COALESCE(NULLIF(u.source, ''), 'local'), u.disabled, u.last_login_at, u.created_at, u.updated_at
		FROM auth_wecom_bindings b
		JOIN users u ON u.id = b.user_id
		WHERE b.wecom_userid = $1
	`, userid).Scan(&user.ID, &user.Username, &user.Email, &user.DisplayName, &user.Role,
		&user.Source, &user.Disabled, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	return user, err
}

// BindWecomAccount 为用户绑定企微账号：同一用户重复绑定视为换绑；
// 目标企微账号已被其他用户绑定时原子返回 auth.ErrWecomAlreadyBound。
func (s *Store) BindWecomAccount(ctx context.Context, userID, userid string) error {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO auth_wecom_bindings(user_id, wecom_userid)
		SELECT $1, $2
		WHERE NOT EXISTS (
			SELECT 1 FROM auth_wecom_bindings WHERE wecom_userid = $2 AND user_id <> $1
		)
		ON CONFLICT (user_id) DO UPDATE SET wecom_userid = EXCLUDED.wecom_userid, bound_at = now()
	`, userID, userid)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrWecomAlreadyBound
	}
	return nil
}

// UnbindWecomAccount 解除用户绑定，返回被解绑的企微账号（未绑定为空）。
func (s *Store) UnbindWecomAccount(ctx context.Context, userID string) (string, error) {
	var userid string
	err := s.pool.QueryRow(ctx, `
		DELETE FROM auth_wecom_bindings WHERE user_id = $1 RETURNING wecom_userid
	`, userID).Scan(&userid)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return userid, err
}

// UserHasWecomBinding 查询用户是否已绑定企微账号。
func (s *Store) UserHasWecomBinding(ctx context.Context, userID string) (bool, error) {
	var bound int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM auth_wecom_bindings WHERE user_id = $1`, userID).Scan(&bound)
	return bound > 0, err
}
