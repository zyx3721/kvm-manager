package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"kvm-manager/backend/internal/domain"
)

const alertReturningColumns = `
	RETURNING id::text, level, status, source_type, source_id, title, message, metadata,
	       first_seen_at, last_seen_at, resolved_at, notification_sent_at, read_at, dismissed_at, created_at, updated_at
`

func (s *Store) ResolveActiveAlertReturning(ctx context.Context, sourceType, sourceID, title string) ([]domain.Alert, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE alerts SET status='resolved', resolved_at=now(), updated_at=now()
		WHERE source_type=$1 AND source_id=$2 AND title=$3 AND status='active'
		`+alertReturningColumns, sourceType, sourceID, title)
	if err != nil {
		return nil, err
	}
	return scanAlertRows(rows)
}

func (s *Store) ResolveActiveAlertsBySourceReturning(ctx context.Context, sourceType, sourceID string) ([]domain.Alert, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE alerts SET status='resolved', resolved_at=now(), updated_at=now()
		WHERE source_type=$1 AND source_id=$2 AND status='active'
		`+alertReturningColumns, sourceType, sourceID)
	if err != nil {
		return nil, err
	}
	return scanAlertRows(rows)
}

func (s *Store) ResolveAlertReturning(ctx context.Context, id string) (domain.Alert, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE alerts SET status='resolved', resolved_at=now(), updated_at=now()
		WHERE id=$1 AND status='active'
		`+alertReturningColumns, id)
	if err != nil {
		return domain.Alert{}, err
	}
	alerts, err := scanAlertRows(rows)
	if err != nil {
		return domain.Alert{}, err
	}
	if len(alerts) == 0 {
		return domain.Alert{}, ErrNotFound
	}
	return alerts[0], nil
}

func scanAlertRows(rows pgx.Rows) ([]domain.Alert, error) {
	defer rows.Close()
	items := make([]domain.Alert, 0)
	for rows.Next() {
		var alert domain.Alert
		if err := rows.Scan(
			&alert.ID,
			&alert.Level,
			&alert.Status,
			&alert.SourceType,
			&alert.SourceID,
			&alert.Title,
			&alert.Message,
			&alert.Metadata,
			&alert.FirstSeenAt,
			&alert.LastSeenAt,
			&alert.ResolvedAt,
			&alert.NotificationSentAt,
			&alert.ReadAt,
			&alert.DismissedAt,
			&alert.CreatedAt,
			&alert.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, alert)
	}
	return items, rows.Err()
}
