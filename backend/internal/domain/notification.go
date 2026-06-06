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
