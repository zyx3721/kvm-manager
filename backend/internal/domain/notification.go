package domain

import (
	"encoding/json"
	"time"
)

type NotificationChannel struct {
	ID                   string          `json:"id"`
	Enabled              bool            `json:"enabled"`
	PasswordResetEnabled bool            `json:"passwordResetEnabled"`
	Config               json.RawMessage `json:"config" swaggertype:"object"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

type AlertNotificationDelivery struct {
	ID            string          `json:"id"`
	Alert         Alert           `json:"alert"`
	EventType     string          `json:"eventType"`
	ChannelID     string          `json:"channelId"`
	Status        string          `json:"status"`
	Payload       json.RawMessage `json:"payload" swaggertype:"object"`
	Error         string          `json:"error"`
	RetryCount    int             `json:"retryCount"`
	NextRetryAt   time.Time       `json:"nextRetryAt"`
	LastAttemptAt *time.Time      `json:"lastAttemptAt,omitempty"`
	SentAt        *time.Time      `json:"sentAt,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}
