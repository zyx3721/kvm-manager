package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestLoadRuntimeDeepSyncInterval(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("RUNTIME_DEEP_SYNC_INTERVAL", "15m")

	cfg, err := Load(slog.Default())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Runtime.DeepSyncInterval != 15*time.Minute {
		t.Fatalf("expected deep sync interval 15m, got %s", cfg.Runtime.DeepSyncInterval)
	}
}

func TestLoadRuntimeDeepSyncIntervalCanDisable(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("RUNTIME_DEEP_SYNC_INTERVAL", "0")

	cfg, err := Load(slog.Default())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Runtime.DeepSyncInterval != 0 {
		t.Fatalf("expected disabled deep sync interval, got %s", cfg.Runtime.DeepSyncInterval)
	}
}

func TestLoadGeneratesTemporaryJWTSecretWhenMissing(t *testing.T) {
	t.Setenv("JWT_SECRET", "")

	cfg, err := Load(slog.Default())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if strings.TrimSpace(cfg.JWT.Secret) == "" {
		t.Fatal("expected generated jwt secret")
	}
	if cfg.JWT.Secret == "test-secret" {
		t.Fatal("expected generated jwt secret to differ from test placeholder")
	}
}
