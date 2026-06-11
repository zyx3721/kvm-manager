package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultSessionHours = 24
const defaultSessionIdleHours = 12

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
	Redis    RedisConfig
	Runtime  RuntimeConfig
}

type ServerConfig struct {
	Host string
	Port string
	Mode string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
	SSLMode  string
}

type JWTConfig struct {
	Secret          string
	ExpireHours     int
	IdleExpireHours int
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type RuntimeConfig struct {
	SyncInterval        time.Duration
	DeepSyncInterval    time.Duration
	SyncFastTimeout     time.Duration
	SyncFullTimeout     time.Duration
	SyncConcurrency     int
	MetricRetentionDays int
	LogRetentionDays    int
	MetricStreamMaxLen  int64
}

func Load(logger *slog.Logger) (Config, error) {
	expireHours := envInt("JWT_EXPIRE_HOURS", defaultSessionHours)
	idleExpireHours := envInt("SESSION_IDLE_TIMEOUT_HOURS", defaultSessionIdleHours)
	cfg := Config{
		Server: ServerConfig{
			Host: envOrDefault("SERVER_HOST", "localhost"),
			Port: envOrDefault("SERVER_PORT", "8080"),
			Mode: envOrDefault("SERVER_MODE", "release"),
		},
		Database: DatabaseConfig{
			Host:     envOrDefault("DB_HOST", "localhost"),
			Port:     envOrDefault("DB_PORT", "5432"),
			Name:     envOrDefault("DB_NAME", "kvm_manager"),
			User:     envOrDefault("DB_USER", "kvm_manager"),
			Password: envOrDefault("DB_PASSWORD", "kvm_manager_dev"),
			SSLMode:  envOrDefault("DB_SSLMODE", "disable"),
		},
		JWT: JWTConfig{
			Secret:          os.Getenv("JWT_SECRET"),
			ExpireHours:     expireHours,
			IdleExpireHours: idleExpireHours,
		},
		Redis: RedisConfig{
			Addr:     envOrDefault("REDIS_ADDR", "localhost:6379"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       envIntAllowZero("REDIS_DB", 0),
		},
		Runtime: RuntimeConfig{
			SyncInterval:        envDuration("RUNTIME_SYNC_INTERVAL", 30*time.Second),
			DeepSyncInterval:    envDuration("RUNTIME_DEEP_SYNC_INTERVAL", 10*time.Minute),
			SyncFastTimeout:     time.Duration(envInt("RUNTIME_SYNC_FAST_TIMEOUT_SECONDS", 12)) * time.Second,
			SyncFullTimeout:     time.Duration(envInt("RUNTIME_SYNC_FULL_TIMEOUT_SECONDS", 60)) * time.Second,
			SyncConcurrency:     envInt("RUNTIME_SYNC_CONCURRENCY", 3),
			MetricRetentionDays: envInt("METRIC_RETENTION_DAYS", 30),
			LogRetentionDays:    envInt("LOG_RETENTION_DAYS", 30),
			MetricStreamMaxLen:  int64(envInt("METRIC_STREAM_MAXLEN", 10000)),
		},
	}

	if err := cfg.Database.Validate(); err != nil {
		return Config{}, err
	}
	if cfg.JWT.ExpireHours <= 0 {
		cfg.JWT.ExpireHours = defaultSessionHours
	}
	if cfg.JWT.IdleExpireHours <= 0 {
		cfg.JWT.IdleExpireHours = defaultSessionIdleHours
	}
	if strings.TrimSpace(cfg.JWT.Secret) == "" {
		secret, err := randomSecret(32)
		if err != nil {
			return Config{}, fmt.Errorf("generate temporary jwt secret: %w", err)
		}
		cfg.JWT.Secret = secret
		logger.Warn("JWT_SECRET is not set; generated a temporary secret for this process")
	}

	return cfg, nil
}

func (s ServerConfig) Addr() string {
	host := strings.TrimSpace(s.Host)
	port := strings.TrimSpace(s.Port)
	if port == "" {
		port = "8080"
	}
	if host == "" || host == "0.0.0.0" {
		return ":" + port
	}
	return net.JoinHostPort(host, port)
}

func (d DatabaseConfig) Validate() error {
	if strings.TrimSpace(d.Host) == "" {
		return fmt.Errorf("DB_HOST cannot be empty")
	}
	if strings.TrimSpace(d.Port) == "" {
		return fmt.Errorf("DB_PORT cannot be empty")
	}
	if strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("DB_NAME cannot be empty")
	}
	if strings.TrimSpace(d.User) == "" {
		return fmt.Errorf("DB_USER cannot be empty")
	}
	return nil
}

func (d DatabaseConfig) DSN() string {
	values := url.Values{}
	values.Set("sslmode", envOrDefault("DB_SSLMODE", d.SSLMode))
	return (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(d.User, d.Password),
		Host:     net.JoinHostPort(d.Host, d.Port),
		Path:     d.Name,
		RawQuery: values.Encode(),
	}).String()
}

func (j JWTConfig) SessionTTL() time.Duration {
	return time.Duration(j.ExpireHours) * time.Hour
}

func (j JWTConfig) SessionIdleTTL() time.Duration {
	return time.Duration(j.IdleExpireHours) * time.Hour
}

func envOrDefault(key string, fallback string) string {
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

func envIntAllowZero(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	if value == "0" {
		return 0
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func randomSecret(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
