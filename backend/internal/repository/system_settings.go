package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"kvm-manager/backend/internal/domain"
)

const systemBaseConfigKey = "base_config"

var defaultSystemBaseConfig = map[string]any{
	"siteName":                         "KVM Manager",
	"loginName":                        "KVM Manager",
	"appName":                          "KVM Manager",
	"appSubtitle":                      "VIRTUALIZATION OPS",
	"iconData":                         "/favicon.svg",
	"passwordResetCodeTtlMinutes":      10,
	"passwordResetCaptchaTtlMinutes":   1,
	"passwordResetSendCooldownMinutes": 0.5,
	"passwordResetRateLimitMinutes":    5,
	"resourceWarningThreshold":         70,
	"resourceCriticalThreshold":        85,
	"resourceAlertConsecutiveCount":    3,
	"agentOfflineFailureCount":         3,
}

func (s *Store) GetSystemBaseConfig(ctx context.Context) (domain.SystemBaseConfig, error) {
	var (
		config domain.SystemBaseConfig
		value  []byte
	)
	err := s.pool.QueryRow(ctx, `
		SELECT value, created_at, updated_at
		FROM system_settings WHERE key=$1
	`, systemBaseConfigKey).Scan(&value, &config.CreatedAt, &config.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.UpsertSystemBaseConfig(ctx, defaultSystemBaseConfig)
	}
	if err != nil {
		return domain.SystemBaseConfig{}, err
	}
	return decodeSystemBaseConfig(value, config.CreatedAt, config.UpdatedAt)
}

func (s *Store) UpsertSystemBaseConfig(ctx context.Context, value map[string]any) (domain.SystemBaseConfig, error) {
	configBytes, err := json.Marshal(value)
	if err != nil {
		return domain.SystemBaseConfig{}, err
	}
	var (
		config    domain.SystemBaseConfig
		storedRaw []byte
	)
	err = s.pool.QueryRow(ctx, `
		INSERT INTO system_settings(key, value)
		VALUES($1, $2)
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=now()
		RETURNING value, created_at, updated_at
	`, systemBaseConfigKey, configBytes).Scan(&storedRaw, &config.CreatedAt, &config.UpdatedAt)
	if err != nil {
		return domain.SystemBaseConfig{}, err
	}
	return decodeSystemBaseConfig(storedRaw, config.CreatedAt, config.UpdatedAt)
}

func decodeSystemBaseConfig(raw []byte, createdAt, updatedAt time.Time) (domain.SystemBaseConfig, error) {
	var payload struct {
		SiteName                         string  `json:"siteName"`
		LoginName                        string  `json:"loginName"`
		AppName                          string  `json:"appName"`
		AppSubtitle                      string  `json:"appSubtitle"`
		IconData                         string  `json:"iconData"`
		PasswordResetCodeTTLMinutes      int     `json:"passwordResetCodeTtlMinutes"`
		PasswordResetCaptchaTTLMinutes   int     `json:"passwordResetCaptchaTtlMinutes"`
		PasswordResetSendCooldownMinutes float64 `json:"passwordResetSendCooldownMinutes"`
		PasswordResetRateLimitMinutes    int     `json:"passwordResetRateLimitMinutes"`
		ResourceWarningThreshold         int     `json:"resourceWarningThreshold"`
		ResourceCriticalThreshold        int     `json:"resourceCriticalThreshold"`
		ResourceAlertConsecutiveCount    int     `json:"resourceAlertConsecutiveCount"`
		AgentOfflineFailureCount         int     `json:"agentOfflineFailureCount"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return domain.SystemBaseConfig{}, err
	}
	config := domain.SystemBaseConfig{
		SiteName:                         fallbackText(payload.SiteName, "KVM Manager"),
		LoginName:                        fallbackText(payload.LoginName, "KVM Manager"),
		AppName:                          fallbackText(payload.AppName, "KVM Manager"),
		AppSubtitle:                      fallbackText(payload.AppSubtitle, "VIRTUALIZATION OPS"),
		IconData:                         fallbackText(payload.IconData, "/favicon.svg"),
		PasswordResetCodeTTLMinutes:      fallbackInt(payload.PasswordResetCodeTTLMinutes, 10),
		PasswordResetCaptchaTTLMinutes:   fallbackInt(payload.PasswordResetCaptchaTTLMinutes, 1),
		PasswordResetSendCooldownMinutes: fallbackFloat(payload.PasswordResetSendCooldownMinutes, 0.5),
		PasswordResetRateLimitMinutes:    fallbackInt(payload.PasswordResetRateLimitMinutes, 5),
		ResourceWarningThreshold:         fallbackInt(payload.ResourceWarningThreshold, 70),
		ResourceCriticalThreshold:        fallbackInt(payload.ResourceCriticalThreshold, 85),
		ResourceAlertConsecutiveCount:    fallbackInt(payload.ResourceAlertConsecutiveCount, 3),
		AgentOfflineFailureCount:         fallbackInt(payload.AgentOfflineFailureCount, 3),
		CreatedAt:                        createdAt,
		UpdatedAt:                        updatedAt,
	}
	return config, nil
}

func fallbackText(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func fallbackInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func fallbackFloat(value, fallback float64) float64 {
	if value <= 0 {
		return fallback
	}
	return value
}
