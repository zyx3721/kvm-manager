package repository

import (
	"testing"
	"time"
)

func TestAlertNotificationRetryDelayWithConfig(t *testing.T) {
	config := AlertNotificationRetryConfig{
		BaseDelaySeconds: 20,
		MaxDelayMinutes:  2,
	}
	if got := AlertNotificationRetryDelayWithConfig(1, config); got != 40*time.Second {
		t.Fatalf("retry delay = %v, want 40s", got)
	}
	if got := AlertNotificationRetryDelayWithConfig(6, config); got != 2*time.Minute {
		t.Fatalf("retry delay cap = %v, want 2m", got)
	}
}

func TestNormalizeAlertNotificationRetryConfigAllowsZeroRetries(t *testing.T) {
	config := normalizeAlertNotificationRetryConfig(AlertNotificationRetryConfig{
		MaxRetryCount:    0,
		BaseDelaySeconds: 30,
		MaxDelayMinutes:  15,
	})
	if config.MaxRetryCount != 0 {
		t.Fatalf("MaxRetryCount = %d, want 0", config.MaxRetryCount)
	}
}
