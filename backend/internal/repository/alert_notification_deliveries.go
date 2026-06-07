package repository

import (
	"context"
	"encoding/json"
	"time"

	"kvm-manager/backend/internal/domain"
)

const maxAlertNotificationRetryCount = 6

type AlertNotificationRetryConfig struct {
	MaxRetryCount    int
	BaseDelaySeconds int
	MaxDelayMinutes  int
}

func (s *Store) EnsureAlertNotificationDeliveries(ctx context.Context, alert domain.Alert, eventType string) error {
	channels, err := s.ListNotificationChannels(ctx)
	if err != nil {
		return err
	}
	enabledChannels := 0
	for _, channel := range channels {
		if !channel.Enabled {
			continue
		}
		if eventType == "recovery" && !notificationConfigSendRecovery(channel.Config) {
			continue
		}
		enabledChannels++
		payload, err := json.Marshal(map[string]any{
			"alertId":   alert.ID,
			"eventType": eventType,
		})
		if err != nil {
			return err
		}
		if _, err := s.pool.Exec(ctx, `
			INSERT INTO alert_notification_deliveries(alert_id, event_type, channel_id, payload)
			VALUES($1, $2, $3, $4)
			ON CONFLICT (alert_id, event_type, channel_id)
			DO UPDATE SET status='pending', error='', payload=EXCLUDED.payload, next_retry_at=now(), updated_at=now()
			WHERE alert_notification_deliveries.status='failed'
		`, alert.ID, eventType, channel.ID, payload); err != nil {
			return err
		}
	}
	if enabledChannels == 0 && eventType == "problem" {
		return s.MarkAlertNotificationSent(ctx, alert.ID)
	}
	return nil
}

func (s *Store) ListPendingAlertNotificationDeliveries(ctx context.Context, limit int) ([]domain.AlertNotificationDelivery, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT d.id::text, d.event_type, d.channel_id, d.status, d.payload, d.error,
		       d.retry_count, d.next_retry_at, d.last_attempt_at, d.sent_at, d.created_at, d.updated_at,
		       a.id::text, a.level, a.status, a.source_type, a.source_id, a.title, a.message, a.metadata,
		       a.first_seen_at, a.last_seen_at, a.resolved_at, a.notification_sent_at, a.read_at, a.dismissed_at, a.created_at, a.updated_at
		FROM alert_notification_deliveries d
		JOIN alerts a ON a.id = d.alert_id
		WHERE d.status = 'pending' AND d.next_retry_at <= now()
		ORDER BY d.next_retry_at ASC, d.created_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	return scanAlertNotificationDeliveryRows(rows)
}

func (s *Store) ListAlertNotificationDeliveries(ctx context.Context, alertID string) ([]domain.AlertNotificationDelivery, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT d.id::text, d.event_type, d.channel_id, d.status, d.payload, d.error,
		       d.retry_count, d.next_retry_at, d.last_attempt_at, d.sent_at, d.created_at, d.updated_at,
		       a.id::text, a.level, a.status, a.source_type, a.source_id, a.title, a.message, a.metadata,
		       a.first_seen_at, a.last_seen_at, a.resolved_at, a.notification_sent_at, a.read_at, a.dismissed_at, a.created_at, a.updated_at
		FROM alert_notification_deliveries d
		JOIN alerts a ON a.id = d.alert_id
		WHERE d.alert_id = $1
		ORDER BY d.created_at ASC, d.channel_id ASC
	`, alertID)
	if err != nil {
		return nil, err
	}
	return scanAlertNotificationDeliveryRows(rows)
}

func scanAlertNotificationDeliveryRows(rows interface {
	Close()
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]domain.AlertNotificationDelivery, error) {
	defer rows.Close()
	items := make([]domain.AlertNotificationDelivery, 0)
	for rows.Next() {
		var item domain.AlertNotificationDelivery
		if err := rows.Scan(
			&item.ID,
			&item.EventType,
			&item.ChannelID,
			&item.Status,
			&item.Payload,
			&item.Error,
			&item.RetryCount,
			&item.NextRetryAt,
			&item.LastAttemptAt,
			&item.SentAt,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.Alert.ID,
			&item.Alert.Level,
			&item.Alert.Status,
			&item.Alert.SourceType,
			&item.Alert.SourceID,
			&item.Alert.Title,
			&item.Alert.Message,
			&item.Alert.Metadata,
			&item.Alert.FirstSeenAt,
			&item.Alert.LastSeenAt,
			&item.Alert.ResolvedAt,
			&item.Alert.NotificationSentAt,
			&item.Alert.ReadAt,
			&item.Alert.DismissedAt,
			&item.Alert.CreatedAt,
			&item.Alert.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) MarkAlertNotificationDeliverySent(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE alert_notification_deliveries
		SET status='sent', error='', last_attempt_at=now(), sent_at=now(), updated_at=now()
		WHERE id=$1
	`, id)
	return err
}

func (s *Store) MarkAlertNotificationDeliveryFailed(ctx context.Context, id string, message string) error {
	return s.MarkAlertNotificationDeliveryFailedWithConfig(ctx, id, message, AlertNotificationRetryConfig{
		MaxRetryCount:    maxAlertNotificationRetryCount,
		BaseDelaySeconds: 30,
		MaxDelayMinutes:  15,
	})
}

func (s *Store) MarkAlertNotificationDeliveryFailedWithConfig(ctx context.Context, id string, message string, config AlertNotificationRetryConfig) error {
	config = normalizeAlertNotificationRetryConfig(config)
	_, err := s.pool.Exec(ctx, `
		UPDATE alert_notification_deliveries
		SET status=CASE WHEN retry_count + 1 >= $3 THEN 'failed' ELSE 'pending' END,
		    error=$2,
		    retry_count=retry_count + 1,
		    last_attempt_at=now(),
		    next_retry_at=now() + ((LEAST($5, (1 << LEAST(retry_count + 1, 8)) * $4))::text || ' seconds')::interval,
		    updated_at=now()
		WHERE id=$1
	`, id, message, config.MaxRetryCount, config.BaseDelaySeconds, config.MaxDelayMinutes*60)
	return err
}

func notificationConfigSendRecovery(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	var cfg struct {
		SendRecovery bool `json:"sendRecovery"`
	}
	_ = json.Unmarshal(data, &cfg)
	return cfg.SendRecovery
}

func AlertNotificationRetryLimit() int {
	return maxAlertNotificationRetryCount
}

func AlertNotificationRetryDelay(retryCount int) time.Duration {
	return AlertNotificationRetryDelayWithConfig(retryCount, AlertNotificationRetryConfig{
		BaseDelaySeconds: 30,
		MaxDelayMinutes:  15,
	})
}

func AlertNotificationRetryDelayWithConfig(retryCount int, config AlertNotificationRetryConfig) time.Duration {
	config = normalizeAlertNotificationRetryConfig(config)
	if retryCount < 0 {
		retryCount = 0
	}
	if retryCount > 8 {
		retryCount = 8
	}
	delay := time.Duration(1<<retryCount) * time.Duration(config.BaseDelaySeconds) * time.Second
	maxDelay := time.Duration(config.MaxDelayMinutes) * time.Minute
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

func normalizeAlertNotificationRetryConfig(config AlertNotificationRetryConfig) AlertNotificationRetryConfig {
	if config.MaxRetryCount < 0 || config.MaxRetryCount > 10 {
		config.MaxRetryCount = maxAlertNotificationRetryCount
	}
	if config.BaseDelaySeconds < 10 || config.BaseDelaySeconds > 300 {
		config.BaseDelaySeconds = 30
	}
	if config.MaxDelayMinutes < 1 || config.MaxDelayMinutes > 120 {
		config.MaxDelayMinutes = 15
	}
	if config.MaxDelayMinutes*60 < config.BaseDelaySeconds {
		config.MaxDelayMinutes = 15
	}
	return config
}
