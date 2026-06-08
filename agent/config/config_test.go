package config

import (
	"testing"
	"time"
)

func TestLoadUsesDefaultCommandTimeout(t *testing.T) {
	t.Setenv("AGENT_TOKEN", "test-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.CommandTimeout != 30*time.Second {
		t.Fatalf("expected default command timeout 30s, got %s", cfg.CommandTimeout)
	}
}

func TestLoadUsesConfiguredCommandTimeout(t *testing.T) {
	t.Setenv("AGENT_TOKEN", "test-token")
	t.Setenv("COMMAND_TIMEOUT_SECONDS", "45")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.CommandTimeout != 45*time.Second {
		t.Fatalf("expected command timeout 45s, got %s", cfg.CommandTimeout)
	}
}
