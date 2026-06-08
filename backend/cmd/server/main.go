package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"kvm-manager/backend/api/router"
	"kvm-manager/backend/config"
	_ "kvm-manager/backend/docs"
	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/internal/service/notification"
	"kvm-manager/backend/internal/service/realtime"
	"kvm-manager/backend/pkg/database"
)

// @title KVM Manager API
// @version 1.0
// @description KVM 虚拟化资源管理控制台后端 API，提供认证、Agent 管理、运行态资源、刷新事件、任务、审计和告警接口。
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Warn("load .env failed", "error", err)
	}
	cfg, err := config.Load(logger)
	if err != nil {
		logger.Error("load config failed", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg)
	if err != nil {
		logger.Error("connect database failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool, logger); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}

	store := repository.New(pool)
	if err := store.EnsureDefaultAdmin(ctx); err != nil {
		logger.Error("initialize admin failed", "error", err)
		os.Exit(1)
	}

	redisClient, err := realtime.NewRedisClient(ctx, cfg.Redis)
	if err != nil {
		logger.Error("connect redis failed", "addr", cfg.Redis.Addr, "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()
	logger.Info("redis runtime cache enabled", "addr", cfg.Redis.Addr)

	runtimeService := realtime.NewWithOptions(store, logger, cfg.JWT.Secret, realtime.NewRedisRuntimeStore(redisClient), redisClient, realtime.Options{
		SyncFastTimeout:    cfg.Runtime.SyncFastTimeout,
		SyncFullTimeout:    cfg.Runtime.SyncFullTimeout,
		SyncConcurrency:    cfg.Runtime.SyncConcurrency,
		MetricStreamMaxLen: cfg.Runtime.MetricStreamMaxLen,
	})
	notificationService := notification.NewService(store, logger)
	runtimeService.SetNotifier(notificationService)
	runtimeService.StartRefreshWorker(ctx)
	runtimeService.StartMetricWriter(ctx, redisClient)
	runtimeService.StartScheduledRefresh(ctx, cfg.Runtime.SyncInterval)
	runtimeService.StartScheduledDeepRefresh(ctx, cfg.Runtime.DeepSyncInterval)
	runtimeService.StartMetricRetention(ctx, cfg.Runtime.MetricRetentionDays)
	runtimeService.StartMetricRollups(ctx)

	addr := cfg.Server.Addr()
	server := &http.Server{Addr: addr, Handler: router.NewRouter(cfg, store, runtimeService, notificationService, logger, redisClient), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		logger.Info("backend server listening", "addr", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("backend server stopped")
}
