package repository

import "context"

// CountLoginFailures 统计用户名的密码失败记录总数与最近一次失败时间（Unix 秒）
func (s *Store) CountLoginFailures(ctx context.Context, username string) (int64, int64, error) {
	var count, lastFailedAt int64
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(MAX(failed_at),0) FROM login_failure_log WHERE username=$1`, username,
	).Scan(&count, &lastFailedAt)
	return count, lastFailedAt, err
}

// CreateLoginFailure 记录一次密码校验失败，并顺带清理超过一天的旧记录防止表无限增长
func (s *Store) CreateLoginFailure(ctx context.Context, username string, failedAt int64) error {
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO login_failure_log(username,failed_at) VALUES($1,$2)`, username, failedAt); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM login_failure_log WHERE failed_at<$1`, failedAt-24*3600)
	return err
}

// ClearLoginFailures 清除用户名的全部失败记录，返回实际删除行数
func (s *Store) ClearLoginFailures(ctx context.Context, username string) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM login_failure_log WHERE username=$1`, username)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
