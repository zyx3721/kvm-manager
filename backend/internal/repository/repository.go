package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/internal/service/auth"
)

var ErrNotFound = errors.New("record not found")

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store  { return &Store{pool: pool} }
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

func searchPattern(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}
	return "%" + strings.ReplaceAll(query, "%", `\%`) + "%"
}

func (s *Store) FindUserByUsername(ctx context.Context, username string) (domain.User, string, error) {
	var user domain.User
	var passwordHash string
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, username, COALESCE(email, ''), password_hash, display_name, role, COALESCE(NULLIF(source, ''), 'local'), disabled, last_login_at, created_at, updated_at
		FROM users WHERE username=$1
	`, username).Scan(&user.ID, &user.Username, &user.Email, &passwordHash, &user.DisplayName, &user.Role, &user.Source, &user.Disabled, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, "", ErrNotFound
	}
	if err != nil {
		return domain.User{}, "", err
	}
	users, err := s.attachAccessToUsers(ctx, []domain.User{user})
	if err != nil {
		return domain.User{}, "", err
	}
	return users[0], passwordHash, nil
}

func (s *Store) UpsertUser(ctx context.Context, username, passwordHash, displayName, role string) (domain.User, error) {
	var user domain.User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users(username, email, password_hash, display_name, role)
		VALUES($1, '', $2, $3, $4)
		ON CONFLICT (username) DO UPDATE SET password_hash=EXCLUDED.password_hash, display_name=EXCLUDED.display_name, role=EXCLUDED.role, disabled=false, updated_at=now()
		RETURNING id::text, username, COALESCE(email, ''), display_name, role, COALESCE(NULLIF(source, ''), 'local'), disabled, last_login_at, created_at, updated_at
	`, username, passwordHash, displayName, role).Scan(&user.ID, &user.Username, &user.Email, &user.DisplayName, &user.Role, &user.Source, &user.Disabled, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return domain.User{}, err
	}
	if err := s.replaceUserRoles(ctx, user.ID, []string{role}); err != nil {
		return domain.User{}, err
	}
	return s.FindUserByID(ctx, user.ID)
}

func (s *Store) UpdateUserPassword(ctx context.Context, userID, passwordHash string) error {
	cmd, err := s.pool.Exec(ctx, `UPDATE users SET password_hash=$2, updated_at=now() WHERE id=$1`, userID, passwordHash)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) RecordUserLogin(ctx context.Context, userID string) error {
	cmd, err := s.pool.Exec(ctx, `UPDATE users SET last_login_at=now(), updated_at=now() WHERE id=$1`, userID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateTask(ctx context.Context, taskType, status, targetType, targetID string, payload any, createdBy string, errorMessage string) (domain.Task, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return domain.Task{}, err
	}
	var task domain.Task
	err = s.pool.QueryRow(ctx, `
		INSERT INTO tasks(type, status, target_type, target_id, payload, created_by, error_message, finished_at)
		VALUES($1, $2, $3, NULLIF($4, ''), $5, NULLIF($6, '')::uuid, NULLIF($7, ''), CASE WHEN $2 IN ('completed','failed') THEN now() ELSE NULL END)
		RETURNING id::text, type, status, target_type, COALESCE(target_id::text, ''), payload, COALESCE(error_message, ''), COALESCE(created_by::text, ''), created_at, updated_at, finished_at
	`, taskType, status, targetType, targetID, payloadBytes, createdBy, errorMessage).Scan(&task.ID, &task.Type, &task.Status, &task.TargetType, &task.TargetID, &task.Payload, &task.ErrorMessage, &task.CreatedBy, &task.CreatedAt, &task.UpdatedAt, &task.FinishedAt)
	return task, err
}

func (s *Store) ListTasks(ctx context.Context, status string, limit int, offset int, query string, jsonKey string, jsonValue string) ([]domain.Task, int, error) {
	if limit < 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	search := searchPattern(query)
	jsonKey = strings.TrimSpace(jsonKey)
	jsonSearch := searchPattern(jsonValue)
	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM tasks
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR concat_ws(' ', type, status, CASE status WHEN 'queued' THEN '排队中' WHEN 'running' THEN '运行中' WHEN 'completed' THEN '已完成' WHEN 'failed' THEN '失败' ELSE status END, target_type, COALESCE(target_id::text, ''), target_type || '/' || COALESCE(target_id::text, '-'), COALESCE(error_message, ''), CASE WHEN payload ? 'totalAgents' THEN concat_ws(' ', '异常 ' || COALESCE(payload->>'failedAgents', '0'), (payload->>'syncedAgents') || '/' || (payload->>'totalAgents') || '，异常 ' || COALESCE(payload->>'failedAgents', '0')) ELSE replace(COALESCE(payload->>'message', ''), '失败', '异常') END) ILIKE $2)
		  AND (($3 = '' AND $4 = '') OR ($3 = '' AND payload::text ILIKE $4) OR ($3 <> '' AND $4 = '' AND payload ? $3) OR ($3 <> '' AND $4 <> '' AND COALESCE(payload->>$3, '') ILIKE $4))
	`, status, search, jsonKey, jsonSearch).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, type, status, target_type, COALESCE(target_id::text, ''), payload, COALESCE(error_message, ''), COALESCE(created_by::text, ''), created_at, updated_at, finished_at
		FROM tasks
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR concat_ws(' ', type, status, CASE status WHEN 'queued' THEN '排队中' WHEN 'running' THEN '运行中' WHEN 'completed' THEN '已完成' WHEN 'failed' THEN '失败' ELSE status END, target_type, COALESCE(target_id::text, ''), target_type || '/' || COALESCE(target_id::text, '-'), COALESCE(error_message, ''), CASE WHEN payload ? 'totalAgents' THEN concat_ws(' ', '异常 ' || COALESCE(payload->>'failedAgents', '0'), (payload->>'syncedAgents') || '/' || (payload->>'totalAgents') || '，异常 ' || COALESCE(payload->>'failedAgents', '0')) ELSE replace(COALESCE(payload->>'message', ''), '失败', '异常') END) ILIKE $2)
		  AND (($3 = '' AND $4 = '') OR ($3 = '' AND payload::text ILIKE $4) OR ($3 <> '' AND $4 = '' AND payload ? $3) OR ($3 <> '' AND $4 <> '' AND COALESCE(payload->>$3, '') ILIKE $4))
		ORDER BY created_at DESC LIMIT NULLIF($5, 0) OFFSET $6
	`, status, search, jsonKey, jsonSearch, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	tasks := make([]domain.Task, 0)
	for rows.Next() {
		var task domain.Task
		if err := rows.Scan(&task.ID, &task.Type, &task.Status, &task.TargetType, &task.TargetID, &task.Payload, &task.ErrorMessage, &task.CreatedBy, &task.CreatedAt, &task.UpdatedAt, &task.FinishedAt); err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, task)
	}
	return tasks, total, rows.Err()
}

func (s *Store) GetTask(ctx context.Context, id string) (domain.Task, error) {
	var task domain.Task
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, type, status, target_type, COALESCE(target_id::text, ''), payload, COALESCE(error_message, ''), COALESCE(created_by::text, ''), created_at, updated_at, finished_at
		FROM tasks WHERE id=$1
	`, id).Scan(&task.ID, &task.Type, &task.Status, &task.TargetType, &task.TargetID, &task.Payload, &task.ErrorMessage, &task.CreatedBy, &task.CreatedAt, &task.UpdatedAt, &task.FinishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Task{}, ErrNotFound
	}
	return task, err
}

func (s *Store) FindActiveTaskByType(ctx context.Context, taskType string) (domain.Task, error) {
	var task domain.Task
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, type, status, target_type, COALESCE(target_id::text, ''), payload, COALESCE(error_message, ''), COALESCE(created_by::text, ''), created_at, updated_at, finished_at
		FROM tasks
		WHERE type=$1 AND status IN ('queued', 'running')
		ORDER BY created_at DESC
		LIMIT 1
	`, taskType).Scan(&task.ID, &task.Type, &task.Status, &task.TargetType, &task.TargetID, &task.Payload, &task.ErrorMessage, &task.CreatedBy, &task.CreatedAt, &task.UpdatedAt, &task.FinishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Task{}, ErrNotFound
	}
	return task, err
}

func (s *Store) ClaimTask(ctx context.Context, id string) (bool, error) {
	cmd, err := s.pool.Exec(ctx, `
		UPDATE tasks SET status='running', updated_at=now()
		WHERE id=$1 AND status='queued'
	`, id)
	if err != nil {
		return false, err
	}
	return cmd.RowsAffected() > 0, nil
}

func (s *Store) UpdateTaskProgress(ctx context.Context, id string, payload any, errorMessage string) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	cmd, err := s.pool.Exec(ctx, `
		UPDATE tasks SET payload=$2, error_message=NULLIF($3, ''), updated_at=now()
		WHERE id=$1
	`, id, payloadBytes, errorMessage)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) FinishTask(ctx context.Context, id, status string, payload any, errorMessage string) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	cmd, err := s.pool.Exec(ctx, `
		UPDATE tasks SET status=$2, payload=$3, error_message=NULLIF($4, ''), finished_at=now(), updated_at=now()
		WHERE id=$1
	`, id, status, payloadBytes, errorMessage)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListQueuedTasksByType(ctx context.Context, taskType string) ([]domain.Task, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, type, status, target_type, COALESCE(target_id::text, ''), payload, COALESCE(error_message, ''), COALESCE(created_by::text, ''), created_at, updated_at, finished_at
		FROM tasks WHERE type=$1 AND status='queued' ORDER BY created_at ASC
	`, taskType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := make([]domain.Task, 0)
	for rows.Next() {
		var task domain.Task
		if err := rows.Scan(&task.ID, &task.Type, &task.Status, &task.TargetType, &task.TargetID, &task.Payload, &task.ErrorMessage, &task.CreatedBy, &task.CreatedAt, &task.UpdatedAt, &task.FinishedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *Store) FailRunningTasksByType(ctx context.Context, taskType, message string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE tasks SET status='failed', error_message=$2, finished_at=now(), updated_at=now()
		WHERE type=$1 AND status='running'
	`, taskType, message)
	return err
}

func (s *Store) WriteAudit(ctx context.Context, userID, action, resourceType, resourceID, ipAddress string, metadata any) error {
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO audit_logs(user_id, action, resource_type, resource_id, ip_address, metadata)
		VALUES(NULLIF($1, '')::uuid, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6)
	`, userID, action, resourceType, resourceID, ipAddress, metadataBytes)
	return err
}

func (s *Store) ListAuditLogs(ctx context.Context, limit int, offset int, query string, jsonKey string, jsonValue string) ([]domain.AuditLog, int, error) {
	if limit < 0 || limit > 200 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	search := searchPattern(query)
	jsonKey = strings.TrimSpace(jsonKey)
	jsonSearch := searchPattern(jsonValue)
	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM audit_logs a LEFT JOIN users u ON u.id = a.user_id
		WHERE ($1 = '' OR concat_ws(' ', a.action, COALESCE(u.username, ''), a.resource_type, COALESCE(a.resource_id::text, ''), a.resource_type || '/' || COALESCE(a.resource_id::text, '-'), COALESCE(a.ip_address, ''), a.metadata::text) ILIKE $1)
		  AND (($2 = '' AND $3 = '') OR ($2 = '' AND a.metadata::text ILIKE $3) OR ($2 <> '' AND $3 = '' AND a.metadata ? $2) OR ($2 <> '' AND $3 <> '' AND COALESCE(a.metadata->>$2, '') ILIKE $3))
	`, search, jsonKey, jsonSearch).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT a.id::text, COALESCE(a.user_id::text, ''), COALESCE(u.username, ''), a.action, a.resource_type,
		       COALESCE(a.resource_id::text, ''), COALESCE(a.ip_address, ''), a.metadata, a.created_at
		FROM audit_logs a LEFT JOIN users u ON u.id = a.user_id
		WHERE ($1 = '' OR concat_ws(' ', a.action, COALESCE(u.username, ''), a.resource_type, COALESCE(a.resource_id::text, ''), a.resource_type || '/' || COALESCE(a.resource_id::text, '-'), COALESCE(a.ip_address, ''), a.metadata::text) ILIKE $1)
		  AND (($2 = '' AND $3 = '') OR ($2 = '' AND a.metadata::text ILIKE $3) OR ($2 <> '' AND $3 = '' AND a.metadata ? $2) OR ($2 <> '' AND $3 <> '' AND COALESCE(a.metadata->>$2, '') ILIKE $3))
		ORDER BY a.created_at DESC LIMIT NULLIF($4, 0) OFFSET $5
	`, search, jsonKey, jsonSearch, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	logs := make([]domain.AuditLog, 0)
	for rows.Next() {
		var log domain.AuditLog
		if err := rows.Scan(&log.ID, &log.UserID, &log.Username, &log.Action, &log.ResourceType, &log.ResourceID, &log.IPAddress, &log.Metadata, &log.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, log)
	}
	return logs, total, rows.Err()
}

func (s *Store) UpsertActiveAlert(ctx context.Context, level, sourceType, sourceID, title, message string, metadata any) error {
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO alerts(level, status, source_type, source_id, title, message, metadata)
		VALUES($1, 'active', $2, $3, $4, $5, $6)
		ON CONFLICT (source_type, source_id, title) WHERE status = 'active'
		DO UPDATE SET level=EXCLUDED.level, message=EXCLUDED.message, metadata=EXCLUDED.metadata, last_seen_at=now(), updated_at=now()
	`, level, sourceType, sourceID, title, message, metadataBytes)
	return err
}

func (s *Store) ResolveActiveAlert(ctx context.Context, sourceType, sourceID, title string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE alerts SET status='resolved', resolved_at=now(), updated_at=now()
		WHERE source_type=$1 AND source_id=$2 AND title=$3 AND status='active'
	`, sourceType, sourceID, title)
	return err
}

func (s *Store) ResolveActiveAlertsBySource(ctx context.Context, sourceType, sourceID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE alerts SET status='resolved', resolved_at=now(), updated_at=now()
		WHERE source_type=$1 AND source_id=$2 AND status='active'
	`, sourceType, sourceID)
	return err
}

func (s *Store) ResolveAlert(ctx context.Context, id string) error {
	cmd, err := s.pool.Exec(ctx, `
		UPDATE alerts SET status='resolved', resolved_at=now(), updated_at=now()
		WHERE id=$1 AND status='active'
	`, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListAlerts(ctx context.Context, status string, limit int, offset int, query string, jsonKey string, jsonValue string) ([]domain.Alert, int, error) {
	if limit < 0 || limit > 200 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	search := searchPattern(query)
	jsonKey = strings.TrimSpace(jsonKey)
	jsonSearch := searchPattern(jsonValue)
	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM alerts
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR concat_ws(' ', level, CASE level WHEN 'info' THEN '信息' WHEN 'warning' THEN '警告' WHEN 'critical' THEN '严重' ELSE level END, status, CASE status WHEN 'active' THEN '活跃' WHEN 'resolved' THEN '已解决' ELSE status END, source_type, CASE source_type WHEN 'agent' THEN 'Agent' WHEN 'host' THEN '宿主机' WHEN 'virtual_machine' THEN '虚拟机' WHEN 'system' THEN '系统' WHEN 'snapshot' THEN '快照' ELSE source_type END, source_id, (CASE source_type WHEN 'agent' THEN 'Agent' WHEN 'host' THEN '宿主机' WHEN 'virtual_machine' THEN '虚拟机' WHEN 'system' THEN '系统' WHEN 'snapshot' THEN '快照' ELSE source_type END) || '/' || COALESCE(source_id, '-'), source_type || '/' || COALESCE(source_id, '-'), title, message, metadata::text, CASE WHEN notification_sent_at IS NULL THEN '待发送' ELSE '已触达' END) ILIKE $2)
		  AND (($3 = '' AND $4 = '') OR ($3 = '' AND metadata::text ILIKE $4) OR ($3 <> '' AND $4 = '' AND metadata ? $3) OR ($3 <> '' AND $4 <> '' AND COALESCE(metadata->>$3, '') ILIKE $4))
	`, status, search, jsonKey, jsonSearch).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, level, status, source_type, source_id, title, message, metadata,
		       first_seen_at, last_seen_at, resolved_at, notification_sent_at, read_at, dismissed_at, created_at, updated_at
		FROM alerts
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR concat_ws(' ', level, CASE level WHEN 'info' THEN '信息' WHEN 'warning' THEN '警告' WHEN 'critical' THEN '严重' ELSE level END, status, CASE status WHEN 'active' THEN '活跃' WHEN 'resolved' THEN '已解决' ELSE status END, source_type, CASE source_type WHEN 'agent' THEN 'Agent' WHEN 'host' THEN '宿主机' WHEN 'virtual_machine' THEN '虚拟机' WHEN 'system' THEN '系统' WHEN 'snapshot' THEN '快照' ELSE source_type END, source_id, (CASE source_type WHEN 'agent' THEN 'Agent' WHEN 'host' THEN '宿主机' WHEN 'virtual_machine' THEN '虚拟机' WHEN 'system' THEN '系统' WHEN 'snapshot' THEN '快照' ELSE source_type END) || '/' || COALESCE(source_id, '-'), source_type || '/' || COALESCE(source_id, '-'), title, message, metadata::text, CASE WHEN notification_sent_at IS NULL THEN '待发送' ELSE '已触达' END) ILIKE $2)
		  AND (($3 = '' AND $4 = '') OR ($3 = '' AND metadata::text ILIKE $4) OR ($3 <> '' AND $4 = '' AND metadata ? $3) OR ($3 <> '' AND $4 <> '' AND COALESCE(metadata->>$3, '') ILIKE $4))
		ORDER BY last_seen_at DESC LIMIT NULLIF($5, 0) OFFSET $6
	`, status, search, jsonKey, jsonSearch, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	alerts := make([]domain.Alert, 0)
	for rows.Next() {
		var alert domain.Alert
		if err := rows.Scan(&alert.ID, &alert.Level, &alert.Status, &alert.SourceType, &alert.SourceID, &alert.Title, &alert.Message, &alert.Metadata, &alert.FirstSeenAt, &alert.LastSeenAt, &alert.ResolvedAt, &alert.NotificationSentAt, &alert.ReadAt, &alert.DismissedAt, &alert.CreatedAt, &alert.UpdatedAt); err != nil {
			return nil, 0, err
		}
		alerts = append(alerts, alert)
	}
	return alerts, total, rows.Err()
}
func (s *Store) CreateAgent(ctx context.Context, name, endpoint, token, tokenCiphertext string, tlsInsecure bool) (domain.Agent, error) {
	var agent domain.Agent
	err := s.pool.QueryRow(ctx, `
		INSERT INTO agents(name, endpoint, token_hash, token_ciphertext, tls_insecure)
		VALUES($1, $2, $3, $4, $5)
		RETURNING id::text, name, endpoint, tls_insecure, status, version, capabilities, last_heartbeat_at, last_error, token_ciphertext, failure_count, last_sync_started_at, last_sync_finished_at, created_at, updated_at
	`, name, endpoint, hashToken(token), tokenCiphertext, tlsInsecure).Scan(&agent.ID, &agent.Name, &agent.Endpoint, &agent.TLSInsecure, &agent.Status, &agent.Version, &agent.Capabilities, &agent.LastHeartbeatAt, &agent.LastError, &agent.TokenCiphertext, &agent.FailureCount, &agent.LastSyncStartedAt, &agent.LastSyncFinishedAt, &agent.CreatedAt, &agent.UpdatedAt)
	return agent, err
}

func (s *Store) ListAgents(ctx context.Context) ([]domain.Agent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, name, endpoint, tls_insecure, status, version, capabilities, last_heartbeat_at, last_error, token_ciphertext, failure_count, last_sync_started_at, last_sync_finished_at, created_at, updated_at
		FROM agents ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	agents := make([]domain.Agent, 0)
	for rows.Next() {
		var agent domain.Agent
		if err := rows.Scan(&agent.ID, &agent.Name, &agent.Endpoint, &agent.TLSInsecure, &agent.Status, &agent.Version, &agent.Capabilities, &agent.LastHeartbeatAt, &agent.LastError, &agent.TokenCiphertext, &agent.FailureCount, &agent.LastSyncStartedAt, &agent.LastSyncFinishedAt, &agent.CreatedAt, &agent.UpdatedAt); err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	return agents, rows.Err()
}

func (s *Store) GetAgentByName(ctx context.Context, name string) (domain.Agent, error) {
	var agent domain.Agent
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, name, endpoint, tls_insecure, status, version, capabilities, last_heartbeat_at, last_error, token_ciphertext, failure_count, last_sync_started_at, last_sync_finished_at, created_at, updated_at
		FROM agents WHERE name=$1
	`, name).Scan(&agent.ID, &agent.Name, &agent.Endpoint, &agent.TLSInsecure, &agent.Status, &agent.Version, &agent.Capabilities, &agent.LastHeartbeatAt, &agent.LastError, &agent.TokenCiphertext, &agent.FailureCount, &agent.LastSyncStartedAt, &agent.LastSyncFinishedAt, &agent.CreatedAt, &agent.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Agent{}, ErrNotFound
	}
	return agent, err
}
func (s *Store) GetAgentByEndpoint(ctx context.Context, endpoint string) (domain.Agent, error) {
	var agent domain.Agent
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, name, endpoint, tls_insecure, status, version, capabilities, last_heartbeat_at, last_error, token_ciphertext, failure_count, last_sync_started_at, last_sync_finished_at, created_at, updated_at
		FROM agents WHERE endpoint=$1
	`, endpoint).Scan(&agent.ID, &agent.Name, &agent.Endpoint, &agent.TLSInsecure, &agent.Status, &agent.Version, &agent.Capabilities, &agent.LastHeartbeatAt, &agent.LastError, &agent.TokenCiphertext, &agent.FailureCount, &agent.LastSyncStartedAt, &agent.LastSyncFinishedAt, &agent.CreatedAt, &agent.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Agent{}, ErrNotFound
	}
	return agent, err
}
func (s *Store) GetAgent(ctx context.Context, id string) (domain.Agent, error) {
	var agent domain.Agent
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, name, endpoint, tls_insecure, status, version, capabilities, last_heartbeat_at, last_error, token_ciphertext, failure_count, last_sync_started_at, last_sync_finished_at, created_at, updated_at
		FROM agents WHERE id=$1
	`, id).Scan(&agent.ID, &agent.Name, &agent.Endpoint, &agent.TLSInsecure, &agent.Status, &agent.Version, &agent.Capabilities, &agent.LastHeartbeatAt, &agent.LastError, &agent.TokenCiphertext, &agent.FailureCount, &agent.LastSyncStartedAt, &agent.LastSyncFinishedAt, &agent.CreatedAt, &agent.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Agent{}, ErrNotFound
	}
	return agent, err
}

func (s *Store) GetAgentTokenHash(ctx context.Context, id string) (string, error) {
	var tokenHash string
	err := s.pool.QueryRow(ctx, `SELECT token_hash FROM agents WHERE id=$1`, id).Scan(&tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return tokenHash, err
}

func (s *Store) VerifyAgentToken(ctx context.Context, id string, token string) error {
	tokenHash, err := s.GetAgentTokenHash(ctx, id)
	if err != nil {
		return err
	}
	if hashToken(token) != tokenHash {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteAgent(ctx context.Context, id string) error {
	cmd, err := s.pool.Exec(ctx, `DELETE FROM agents WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateAgentHealth(ctx context.Context, id, status, version string, capabilities any, lastError string) error {
	capabilityBytes, err := json.Marshal(capabilities)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE agents SET status=$2, version=$3, capabilities=$4, last_error=$5,
		last_heartbeat_at=CASE WHEN $2 = 'online' THEN now() ELSE last_heartbeat_at END,
		updated_at=now()
		WHERE id=$1
	`, id, status, version, capabilityBytes, lastError)
	return err
}

func (s *Store) UpdateAgentTokenCiphertext(ctx context.Context, id string, tokenCiphertext string) error {
	_, err := s.pool.Exec(ctx, `UPDATE agents SET token_ciphertext=$2, updated_at=now() WHERE id=$1`, id, tokenCiphertext)
	return err
}

func (s *Store) MarkAgentSyncStarted(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE agents SET last_sync_started_at=now(), updated_at=now() WHERE id=$1`, id)
	return err
}

func (s *Store) UpdateAgentSyncSuccess(ctx context.Context, id, version string, capabilities any) error {
	capabilityBytes, err := json.Marshal(capabilities)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE agents SET status='online', version=$2, capabilities=$3, last_error='', failure_count=0,
		last_heartbeat_at=now(), last_sync_finished_at=now(), updated_at=now()
		WHERE id=$1
	`, id, version, capabilityBytes)
	return err
}

func (s *Store) UpdateAgentSyncFailure(ctx context.Context, id string, lastError string, offlineFailureLimit int) error {
	if offlineFailureLimit <= 0 {
		offlineFailureLimit = 3
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE agents SET failure_count=failure_count+1,
		status=CASE WHEN failure_count + 1 >= $3 THEN 'offline' ELSE status END,
		last_error=$2, last_sync_finished_at=now(), updated_at=now()
		WHERE id=$1
	`, id, lastError, offlineFailureLimit)
	return err
}
func HashTokenForLookup(token string) string { return hashToken(token) }

func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func ClientIP(req *http.Request) string {
	for _, value := range []string{req.Header.Get("X-Forwarded-For"), req.Header.Get("X-Real-IP"), req.RemoteAddr} {
		if ip := normalizeClientIP(value); ip != "" {
			return ip
		}
	}
	return ""
}

func normalizeClientIP(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, ",") {
		value = strings.TrimSpace(strings.Split(value, ",")[0])
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		return host
	}
	value = strings.Trim(value, "[]")
	if net.ParseIP(value) != nil {
		return value
	}
	return value
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Store) EnsureDefaultAdmin(ctx context.Context) error {
	var userCount int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM users`).Scan(&userCount); err != nil {
		return err
	}
	if userCount > 0 {
		return nil
	}
	passwordHash, err := auth.HashPassword("123456")
	if err != nil {
		return err
	}
	_, err = s.UpsertUser(ctx, "admin", passwordHash, "admin", "admin")
	return err
}
