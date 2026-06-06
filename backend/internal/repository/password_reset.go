package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"kvm-manager/backend/internal/domain"
)

func (s *Store) CreatePasswordResetToken(ctx context.Context, userID, channelID, contact, code, requestIP string, expiresAt time.Time) (domain.PasswordResetToken, error) {
	codeHash := hashResetCode(code)
	var token domain.PasswordResetToken
	err := s.pool.QueryRow(ctx, `
		INSERT INTO password_reset_tokens(user_id, channel_id, contact, code_hash, request_ip, expires_at)
		VALUES($1, $2, $3, $4, $5, $6)
		RETURNING id::text, user_id::text, channel_id, contact, code_hash, request_ip, expires_at, used_at, created_at
	`, userID, channelID, contact, codeHash, requestIP, expiresAt).Scan(&token.ID, &token.UserID, &token.ChannelID, &token.Contact, &token.CodeHash, &token.RequestIP, &token.ExpiresAt, &token.UsedAt, &token.CreatedAt)
	return token, err
}

func (s *Store) CountRecentPasswordResetTokens(ctx context.Context, userID string, since time.Time) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM password_reset_tokens
		WHERE created_at >= $1 AND user_id=$2
	`, since, userID).Scan(&count)
	return count, err
}

func (s *Store) LatestRecentPasswordResetTokenCreatedAt(ctx context.Context, userID string, since time.Time) (time.Time, bool, error) {
	var createdAt time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT created_at FROM password_reset_tokens
		WHERE created_at >= $1 AND user_id=$2
		ORDER BY created_at DESC
		LIMIT 1
	`, since, userID).Scan(&createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return createdAt, true, nil
}

func (s *Store) FindUsablePasswordResetToken(ctx context.Context, username, code string) (domain.PasswordResetToken, domain.User, error) {
	codeHash := hashResetCode(code)
	var token domain.PasswordResetToken
	var user domain.User
	err := s.pool.QueryRow(ctx, `
		SELECT t.id::text, t.user_id::text, t.channel_id, t.contact, t.code_hash, t.request_ip, t.expires_at, t.used_at, t.created_at,
		       u.id::text, u.username, u.display_name, u.role, COALESCE(NULLIF(u.source, ''), 'local'), u.disabled, u.last_login_at, u.created_at, u.updated_at
		FROM password_reset_tokens t
		JOIN users u ON u.id = t.user_id
		WHERE u.username=$1 AND t.code_hash=$2 AND t.used_at IS NULL AND t.expires_at > now()
		ORDER BY t.created_at DESC
		LIMIT 1
	`, username, codeHash).Scan(&token.ID, &token.UserID, &token.ChannelID, &token.Contact, &token.CodeHash, &token.RequestIP, &token.ExpiresAt, &token.UsedAt, &token.CreatedAt, &user.ID, &user.Username, &user.DisplayName, &user.Role, &user.Source, &user.Disabled, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PasswordResetToken{}, domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.PasswordResetToken{}, domain.User{}, err
	}
	users, err := s.attachAccessToUsers(ctx, []domain.User{user})
	if err != nil {
		return domain.PasswordResetToken{}, domain.User{}, err
	}
	return token, users[0], nil
}

func (s *Store) MarkPasswordResetTokenUsed(ctx context.Context, id string) error {
	cmd, err := s.pool.Exec(ctx, `UPDATE password_reset_tokens SET used_at=now() WHERE id=$1 AND used_at IS NULL`, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteUserSessions(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1`, userID)
	return err
}

func hashResetCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}
