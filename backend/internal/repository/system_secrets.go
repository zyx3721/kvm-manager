package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ResolveJWTSecret 读取持久化的 JWT 密钥；不存在时写入 fallback 并返回，
// 用于未显式配置 JWT_SECRET 时保持密钥跨重启稳定
func (s *Store) ResolveJWTSecret(ctx context.Context, fallback string) (string, error) {
	var value string
	err := s.pool.QueryRow(ctx, `SELECT value FROM system_secrets WHERE name='jwt_secret'`).Scan(&value)
	if err == nil && strings.TrimSpace(value) != "" {
		return value, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO system_secrets(name,value) VALUES('jwt_secret',$1)
		 ON CONFLICT (name) DO UPDATE SET value=EXCLUDED.value, updated_at=now()`, fallback); err != nil {
		return "", err
	}
	return fallback, nil
}
