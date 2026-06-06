package repository

import (
	"context"

	"kvm-manager/backend/internal/domain"
)

func (s *Store) ListAlertNotifications(ctx context.Context, status string, limit int) ([]domain.Alert, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	filter := "active"
	if status == "all" {
		filter = ""
	}
	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM alerts
		WHERE dismissed_at IS NULL AND ($1 = '' OR status = $1)
	`, filter).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, level, status, source_type, source_id, title, message, metadata,
		       first_seen_at, last_seen_at, resolved_at, notification_sent_at, read_at, dismissed_at, created_at, updated_at
		FROM alerts
		WHERE dismissed_at IS NULL AND ($1 = '' OR status = $1)
		ORDER BY last_seen_at DESC LIMIT $2
	`, filter, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]domain.Alert, 0)
	for rows.Next() {
		var alert domain.Alert
		if err := rows.Scan(&alert.ID, &alert.Level, &alert.Status, &alert.SourceType, &alert.SourceID, &alert.Title, &alert.Message, &alert.Metadata, &alert.FirstSeenAt, &alert.LastSeenAt, &alert.ResolvedAt, &alert.NotificationSentAt, &alert.ReadAt, &alert.DismissedAt, &alert.CreatedAt, &alert.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, alert)
	}
	return items, total, rows.Err()
}

func (s *Store) CountUnreadAlertNotifications(ctx context.Context) (int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM alerts
		WHERE status='active' AND read_at IS NULL AND dismissed_at IS NULL
	`).Scan(&total)
	return total, err
}

func (s *Store) MarkAlertNotificationRead(ctx context.Context, id string) error {
	cmd, err := s.pool.Exec(ctx, `
		UPDATE alerts SET read_at=COALESCE(read_at, now()), updated_at=now()
		WHERE id=$1 AND dismissed_at IS NULL
	`, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) MarkAllAlertNotificationsRead(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE alerts SET read_at=COALESCE(read_at, now()), updated_at=now()
		WHERE status='active' AND dismissed_at IS NULL
	`)
	return err
}

func (s *Store) DismissAlertNotifications(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE alerts SET dismissed_at=COALESCE(dismissed_at, now()), read_at=COALESCE(read_at, now()), updated_at=now()
		WHERE dismissed_at IS NULL
	`)
	return err
}
