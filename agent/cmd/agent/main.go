package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"kvm-manager/agent/api/router"
	"kvm-manager/agent/config"
	"kvm-manager/agent/internal/kvm"
	"kvm-manager/agent/internal/security"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Warn("load .env failed", "error", err)
	}
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config failed", "error", err)
		os.Exit(1)
	}

	provider := kvm.NewVirshProvider(cfg.LibvirtURI, cfg.CommandTimeout).WithLogger(logger)
	auth := security.NewAuthenticator(cfg.Token)
	server := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           router.New(provider, logger, auth),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("kvm agent listening", "addr", cfg.Addr(), "tls", cfg.TLSConfigured(), "libvirt", cfg.LibvirtURI)
	if cfg.TLSConfigured() {
		if err := server.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("agent stopped", "error", err)
			os.Exit(1)
		}
		return
	}
	logger.Warn("TLS is not configured; use this mode only on trusted networks or behind a TLS proxy")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("agent stopped", "error", err)
		os.Exit(1)
	}
}
