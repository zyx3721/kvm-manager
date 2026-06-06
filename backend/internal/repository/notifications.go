package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"kvm-manager/backend/internal/domain"
)

func (s *Store) ListNotificationChannels(ctx context.Context) ([]domain.NotificationChannel, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, enabled, password_reset_enabled, config, created_at, updated_at
		FROM notification_channels
		ORDER BY CASE id
			WHEN 'webhook' THEN 1
			WHEN 'email' THEN 2
			WHEN 'lark' THEN 3
			WHEN 'wechat' THEN 4
			WHEN 'dingtalk' THEN 5
			ELSE 9
		END, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.NotificationChannel, 0)
	for rows.Next() {
		var item domain.NotificationChannel
		if err := rows.Scan(&item.ID, &item.Enabled, &item.PasswordResetEnabled, &item.Config, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetNotificationChannel(ctx context.Context, id string) (domain.NotificationChannel, error) {
	var item domain.NotificationChannel
	err := s.pool.QueryRow(ctx, `
		SELECT id, enabled, password_reset_enabled, config, created_at, updated_at
		FROM notification_channels WHERE id=$1
	`, id).Scan(&item.ID, &item.Enabled, &item.PasswordResetEnabled, &item.Config, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NotificationChannel{}, ErrNotFound
	}
	return item, err
}

func (s *Store) UpsertNotificationChannel(ctx context.Context, id string, enabled bool, passwordResetEnabled bool, config any) (domain.NotificationChannel, error) {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return domain.NotificationChannel{}, err
	}
	var item domain.NotificationChannel
	err = s.pool.QueryRow(ctx, `
		INSERT INTO notification_channels(id, enabled, password_reset_enabled, config)
		VALUES($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET enabled=EXCLUDED.enabled, password_reset_enabled=EXCLUDED.password_reset_enabled, config=EXCLUDED.config, updated_at=now()
		RETURNING id, enabled, password_reset_enabled, config, created_at, updated_at
	`, id, enabled, passwordResetEnabled, configBytes).Scan(&item.ID, &item.Enabled, &item.PasswordResetEnabled, &item.Config, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}

func (s *Store) MarkAlertNotificationSent(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE alerts SET notification_sent_at=now(), updated_at=now() WHERE id=$1`, id)
	return err
}

func (s *Store) ListPendingAlertNotifications(ctx context.Context, limit int) ([]domain.Alert, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, level, status, source_type, source_id, title, message, metadata,
		       first_seen_at, last_seen_at, resolved_at, notification_sent_at, read_at, dismissed_at, created_at, updated_at
		FROM alerts
		WHERE status='active' AND notification_sent_at IS NULL
		ORDER BY last_seen_at DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Alert, 0)
	for rows.Next() {
		var alert domain.Alert
		if err := rows.Scan(&alert.ID, &alert.Level, &alert.Status, &alert.SourceType, &alert.SourceID, &alert.Title, &alert.Message, &alert.Metadata, &alert.FirstSeenAt, &alert.LastSeenAt, &alert.ResolvedAt, &alert.NotificationSentAt, &alert.ReadAt, &alert.DismissedAt, &alert.CreatedAt, &alert.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, alert)
	}
	return items, rows.Err()
}
