package repository

import (
	"context"
	"time"
)

const (
	deleteTasksBeforeSQL     = `DELETE FROM tasks WHERE created_at < $1`
	deleteAuditLogsBeforeSQL = `DELETE FROM audit_logs WHERE created_at < $1`
	deleteAlertsBeforeSQL    = `DELETE FROM alerts WHERE created_at < $1`
)

func (s *Store) DeleteLogRecordsBefore(ctx context.Context, before time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, deleteTasksBeforeSQL, before); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, deleteAuditLogsBeforeSQL, before); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, deleteAlertsBeforeSQL, before); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
