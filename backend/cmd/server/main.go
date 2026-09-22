// 后端服务入口：解析命令行参数、加载配置并启动 HTTP 服务
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"kvm-manager/backend/api/router"
	"kvm-manager/backend/config"
	_ "kvm-manager/backend/docs"
	"kvm-manager/backend/internal/buildinfo"
	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/internal/service/notification"
	"kvm-manager/backend/internal/service/realtime"
	"kvm-manager/backend/pkg/database"
)

// flagValues 汇总命令行参数的显式取值，零值表示未设置
type flagValues struct {
	envPath                    string
	serverHost                 string
	serverPort                 string
	serverMode                 string
	dbHost                     string
	dbPort                     string
	dbName                     string
	dbUser                     string
	dbPassword                 string
	dbSSLMode                  string
	jwtSecret                  string
	jwtExpireHours             int
	redisAddr                  string
	redisPassword              string
	redisDB                    int
	runtimeSyncInterval        time.Duration
	runtimeDeepSyncInterval    time.Duration
	runtimeSyncFastTimeoutSecs int
	runtimeSyncFullTimeoutSecs int
	runtimeSyncConcurrency     int
	metricRetentionDays        int
	logRetentionDays           int
	metricStreamMaxLen         int
}

// versionText 组装 -v/-version 输出的版本信息文本
func versionText() string {
	return fmt.Sprintf("kvm-manager %s\ncommit: %s\nbuild: %s\ngo: %s\n", buildinfo.Version, buildinfo.Commit, buildinfo.BuildDate, runtime.Version())
}

// setUsage 自定义 -h/--help 输出：-v 与 -version 合并一行，各参数描述统一换行缩进对齐
func setUsage() {
	flag.Usage = func() {
		w := flag.CommandLine.Output()
		fmt.Fprintf(w, "Usage of %s:\n", os.Args[0])
		flag.VisitAll(func(f *flag.Flag) {
			if f.Name == "version" {
				return
			}
			if f.Name == "v" {
				fmt.Fprintf(w, "  -v, -version\n    \t%s\n", f.Usage)
				return
			}
			name, usage := flag.UnquoteUsage(f)
			fmt.Fprintf(w, "  -%s %s\n    \t%s (default %q)\n", f.Name, name, usage, f.DefValue)
		})
	}
}

// overridesFromFlags 将显式传入的命令行参数映射为环境变量键值，零值表示未设置不覆盖
func overridesFromFlags(v flagValues) map[string]string {
	overrides := make(map[string]string)
	put := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			overrides[key] = value
		}
	}
	putInt := func(key string, value int) {
		if value != 0 {
			overrides[key] = strconv.Itoa(value)
		}
	}
	put("SERVER_HOST", v.serverHost)
	put("SERVER_PORT", v.serverPort)
	put("SERVER_MODE", v.serverMode)
	put("DB_HOST", v.dbHost)
	put("DB_PORT", v.dbPort)
	put("DB_NAME", v.dbName)
	put("DB_USER", v.dbUser)
	put("DB_PASSWORD", v.dbPassword)
	put("DB_SSLMODE", v.dbSSLMode)
	put("JWT_SECRET", v.jwtSecret)
	putInt("JWT_EXPIRE_HOURS", v.jwtExpireHours)
	put("REDIS_ADDR", v.redisAddr)
	put("REDIS_PASSWORD", v.redisPassword)
	putInt("REDIS_DB", v.redisDB)
	if v.runtimeSyncInterval > 0 {
		put("RUNTIME_SYNC_INTERVAL", v.runtimeSyncInterval.String())
	}
	if v.runtimeDeepSyncInterval > 0 {
		put("RUNTIME_DEEP_SYNC_INTERVAL", v.runtimeDeepSyncInterval.String())
	}
	putInt("RUNTIME_SYNC_FAST_TIMEOUT_SECONDS", v.runtimeSyncFastTimeoutSecs)
	putInt("RUNTIME_SYNC_FULL_TIMEOUT_SECONDS", v.runtimeSyncFullTimeoutSecs)
	putInt("RUNTIME_SYNC_CONCURRENCY", v.runtimeSyncConcurrency)
	putInt("METRIC_RETENTION_DAYS", v.metricRetentionDays)
	putInt("LOG_RETENTION_DAYS", v.logRetentionDays)
	putInt("METRIC_STREAM_MAXLEN", v.metricStreamMaxLen)
	return overrides
}

// applyFlagOverrides 将显式命令行参数写入进程环境变量，保证显式参数优先于环境变量与 .env
func applyFlagOverrides(overrides map[string]string) {
	for key, value := range overrides {
		_ = os.Setenv(key, value)
	}
}

// loadDotEnv 按自定义路径、工作目录 ./.env、backend/.env、可执行文件同目录顺序加载首个存在的 .env 文件，
// 已存在的环境变量不被覆盖；全部未命中时静默放行，交由配置默认值兜底
func loadDotEnv(customPath string) {
	seen := map[string]struct{}{}
	candidates := []string{}
	if strings.TrimSpace(customPath) != "" {
		candidates = append(candidates, customPath)
	}
	candidates = append(candidates, ".env", filepath.Join("backend", ".env"))
	if exePath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exePath), ".env"))
	}
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if err := godotenv.Load(candidate); err == nil {
			return
		}
	}
}

// parseFlags 声明并解析全部命令行参数，逐一对应 .env 环境变量
func parseFlags() (showVersion bool, flagVals flagValues) {
	flag.BoolVar(&showVersion, "v", false, "显示版本信息并退出")
	flag.BoolVar(&showVersion, "version", false, "显示版本信息并退出")
	flag.StringVar(&flagVals.envPath, "env", "", ".env 配置文件路径，默认按工作目录 ./.env、backend/.env、可执行文件同目录顺序查找")
	flag.StringVar(&flagVals.serverHost, "server-host", "", "HTTP 监听主机，等价环境变量 SERVER_HOST")
	flag.StringVar(&flagVals.serverPort, "server-port", "", "HTTP 监听端口，等价环境变量 SERVER_PORT")
	flag.StringVar(&flagVals.serverMode, "server-mode", "", "服务运行模式标记，等价环境变量 SERVER_MODE")
	flag.StringVar(&flagVals.dbHost, "db-host", "", "PostgreSQL 主机，等价环境变量 DB_HOST")
	flag.StringVar(&flagVals.dbPort, "db-port", "", "PostgreSQL 端口，等价环境变量 DB_PORT")
	flag.StringVar(&flagVals.dbName, "db-name", "", "PostgreSQL 数据库名，等价环境变量 DB_NAME")
	flag.StringVar(&flagVals.dbUser, "db-user", "", "PostgreSQL 用户名，等价环境变量 DB_USER")
	flag.StringVar(&flagVals.dbPassword, "db-password", "", "PostgreSQL 密码，等价环境变量 DB_PASSWORD")
	flag.StringVar(&flagVals.dbSSLMode, "db-sslmode", "", "PostgreSQL SSL 模式，等价环境变量 DB_SSLMODE")
	flag.StringVar(&flagVals.jwtSecret, "jwt-secret", "", "JWT/Session 签名密钥，等价环境变量 JWT_SECRET")
	flag.IntVar(&flagVals.jwtExpireHours, "jwt-expire-hours", 0, "登录会话有效期（小时），等价环境变量 JWT_EXPIRE_HOURS")
	flag.StringVar(&flagVals.redisAddr, "redis-addr", "", "Redis 地址，等价环境变量 REDIS_ADDR")
	flag.StringVar(&flagVals.redisPassword, "redis-password", "", "Redis 密码，等价环境变量 REDIS_PASSWORD")
	flag.IntVar(&flagVals.redisDB, "redis-db", 0, "Redis 数据库编号，等价环境变量 REDIS_DB")
	flag.DurationVar(&flagVals.runtimeSyncInterval, "runtime-sync-interval", 0, "常规刷新间隔，等价环境变量 RUNTIME_SYNC_INTERVAL")
	flag.DurationVar(&flagVals.runtimeDeepSyncInterval, "runtime-deep-sync-interval", 0, "深度刷新间隔，等价环境变量 RUNTIME_DEEP_SYNC_INTERVAL")
	flag.IntVar(&flagVals.runtimeSyncFastTimeoutSecs, "runtime-sync-fast-timeout-seconds", 0, "快速同步超时秒数，等价环境变量 RUNTIME_SYNC_FAST_TIMEOUT_SECONDS")
	flag.IntVar(&flagVals.runtimeSyncFullTimeoutSecs, "runtime-sync-full-timeout-seconds", 0, "全量同步超时秒数，等价环境变量 RUNTIME_SYNC_FULL_TIMEOUT_SECONDS")
	flag.IntVar(&flagVals.runtimeSyncConcurrency, "runtime-sync-concurrency", 0, "同步并发数，等价环境变量 RUNTIME_SYNC_CONCURRENCY")
	flag.IntVar(&flagVals.metricRetentionDays, "metric-retention-days", 0, "指标保留天数，等价环境变量 METRIC_RETENTION_DAYS")
	flag.IntVar(&flagVals.logRetentionDays, "log-retention-days", 0, "日志保留天数，等价环境变量 LOG_RETENTION_DAYS")
	flag.IntVar(&flagVals.metricStreamMaxLen, "metric-stream-maxlen", 0, "指标 Stream 最大长度，等价环境变量 METRIC_STREAM_MAXLEN")
	setUsage()
	flag.Parse()
	return showVersion, flagVals
}

// @title KVM Manager API
// @version 1.0
// @description KVM 虚拟化资源管理控制台后端 API，提供认证、Agent 管理、运行态资源、刷新事件、任务、审计和告警接口。
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	showVersion, flagVals := parseFlags()
	if showVersion {
		fmt.Print(versionText())
		return
	}
	loadDotEnv(flagVals.envPath)
	applyFlagOverrides(overridesFromFlags(flagVals))

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
	runtimeService.StartLogRetention(ctx, cfg.Runtime.LogRetentionDays)
	runtimeService.StartMetricRollups(ctx)

	addr := cfg.Server.Addr()
	server := &http.Server{Addr: addr, Handler: router.NewRouter(cfg, store, runtimeService, notificationService, logger, redisClient), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		logger.Info("backend server listening", "addr", addr, "version", buildinfo.Version)
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
