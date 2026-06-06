package domain

import "time"

type PublicPasswordResetChannel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	RequiresTo  bool   `json:"requiresTo"`
}

type PasswordResetToken struct {
	ID        string     `json:"id"`
	UserID    string     `json:"userId"`
	ChannelID string     `json:"channelId"`
	Contact   string     `json:"contact"`
	CodeHash  string     `json:"-"`
	RequestIP string     `json:"requestIp"`
	ExpiresAt time.Time  `json:"expiresAt"`
	UsedAt    *time.Time `json:"usedAt,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}
