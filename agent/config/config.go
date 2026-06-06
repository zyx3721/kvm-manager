package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Host           string
	Port           string
	Token          string
	TLSCert        string
	TLSKey         string
	LibvirtURI     string
	CommandTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Host:           envOrDefault("AGENT_HOST", "0.0.0.0"),
		Port:           envOrDefault("AGENT_PORT", "9443"),
		Token:          strings.TrimSpace(os.Getenv("AGENT_TOKEN")),
		TLSCert:        strings.TrimSpace(os.Getenv("AGENT_TLS_CERT")),
		TLSKey:         strings.TrimSpace(os.Getenv("AGENT_TLS_KEY")),
		LibvirtURI:     envOrDefault("LIBVIRT_URI", "qemu:///system"),
		CommandTimeout: time.Duration(envInt("COMMAND_TIMEOUT_SECONDS", 8)) * time.Second,
	}
	if cfg.Token == "" {
		return Config{}, fmt.Errorf("AGENT_TOKEN cannot be empty")
	}
	if (cfg.TLSCert == "") != (cfg.TLSKey == "") {
		return Config{}, fmt.Errorf("AGENT_TLS_CERT and AGENT_TLS_KEY must be configured together")
	}
	return cfg, nil
}

func (c Config) Addr() string {
	return c.Host + ":" + c.Port
}

func (c Config) TLSConfigured() bool {
	return c.TLSCert != "" && c.TLSKey != ""
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
