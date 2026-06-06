package router

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	"kvm-manager/backend/config"
	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/internal/service/auth"
	"kvm-manager/backend/internal/service/notification"
	"kvm-manager/backend/internal/service/realtime"
	"kvm-manager/backend/pkg/tokencrypto"
)

type router struct {
	cfg     config.Config
	logger  *slog.Logger
	store   *repository.Store
	runtime *realtime.Service
	notify  *notification.Service
	redis   redis.Cmdable
	auth    *auth.Service
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Provider string `json:"provider"`
}

type changePasswordRequest struct {
	OldPassword     string `json:"old_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

type createAgentRequest struct {
	Name        string `json:"name"`
	Endpoint    string `json:"endpoint"`
	Token       string `json:"token"`
	TLSInsecure bool   `json:"tlsInsecure"`
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func NewRouter(cfg config.Config, store *repository.Store, runtime *realtime.Service, notify *notification.Service, logger *slog.Logger, redisClient redis.Cmdable) http.Handler {
	r := &router{cfg: cfg, logger: logger, store: store, runtime: runtime, notify: notify, redis: redisClient, auth: auth.NewServiceWithIdleTTL(store, cfg.JWT.SessionTTL(), cfg.JWT.SessionIdleTTL())}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", r.handleHealth)
	mux.HandleFunc("GET /swagger/", httpSwagger.WrapHandler)
	mux.HandleFunc("GET /api/auth/providers", r.handlePublicAuthProviders)
	mux.HandleFunc("GET /api/public/base-config", r.handlePublicSystemBaseConfig)
	mux.HandleFunc("POST /api/auth/login", r.handleLogin)
	mux.HandleFunc("GET /api/auth/password-reset/captcha", r.handlePasswordResetCaptcha)
	mux.HandleFunc("POST /api/auth/password-reset/verify", r.handlePasswordResetVerify)
	mux.HandleFunc("POST /api/auth/password-reset/send-code", r.handlePasswordResetSendCode)
	mux.HandleFunc("POST /api/auth/password-reset/confirm", r.handlePasswordResetConfirm)
	mux.Handle("GET /api/auth/me", r.requireAuth(http.HandlerFunc(r.handleMe)))
	mux.Handle("POST /api/auth/logout", r.requireAuth(http.HandlerFunc(r.handleLogout)))
	mux.Handle("PUT /api/auth/password", r.requireAuth(http.HandlerFunc(r.handleChangePassword)))
	mux.Handle("GET /api/dashboard/summary", r.requireAuth(http.HandlerFunc(r.handleDashboardSummary)))
	mux.Handle("GET /api/events", r.requireAuth(http.HandlerFunc(r.handleEvents)))
	mux.Handle("GET /api/metrics/hosts/", r.requireAuth(http.HandlerFunc(r.handleHostMetrics)))
	mux.Handle("GET /api/metrics/vms/", r.requireAuth(http.HandlerFunc(r.handleVMMetrics)))
	mux.Handle("POST /api/refresh", r.requireAuth(http.HandlerFunc(r.handleRefresh)))
	mux.Handle("GET /api/tasks/", r.requireAuth(http.HandlerFunc(r.handleTaskRoute)))
	mux.Handle("GET /api/hosts", r.requireAuth(http.HandlerFunc(r.handleListHosts)))
	mux.Handle("GET /api/storage-pools/", r.requireAuth(http.HandlerFunc(r.handleStoragePoolRoute)))
	mux.Handle("POST /api/storage-pools/", r.requireAuth(http.HandlerFunc(r.handleStoragePoolRoute)))
	mux.Handle("PUT /api/storage-pools/", r.requireAuth(http.HandlerFunc(r.handleStoragePoolRoute)))
	mux.Handle("DELETE /api/storage-pools/", r.requireAuth(http.HandlerFunc(r.handleStoragePoolRoute)))
	mux.Handle("GET /api/network-pools/", r.requireAuth(http.HandlerFunc(r.handleNetworkPoolRoute)))
	mux.Handle("POST /api/network-pools/", r.requireAuth(http.HandlerFunc(r.handleNetworkPoolRoute)))
	mux.Handle("PUT /api/network-pools/", r.requireAuth(http.HandlerFunc(r.handleNetworkPoolRoute)))
	mux.Handle("DELETE /api/network-pools/", r.requireAuth(http.HandlerFunc(r.handleNetworkPoolRoute)))
	mux.Handle("GET /api/host-interfaces/", r.requireAuth(http.HandlerFunc(r.handleHostInterfaceRoute)))
	mux.Handle("POST /api/host-interfaces/", r.requireAuth(http.HandlerFunc(r.handleHostInterfaceRoute)))
	mux.Handle("PUT /api/host-interfaces/", r.requireAuth(http.HandlerFunc(r.handleHostInterfaceRoute)))
	mux.Handle("DELETE /api/host-interfaces/", r.requireAuth(http.HandlerFunc(r.handleHostInterfaceRoute)))
	mux.Handle("GET /api/agents", r.requireAuth(http.HandlerFunc(r.handleListAgents)))
	mux.Handle("POST /api/agents/test-connection", r.requireAuth(http.HandlerFunc(r.handleAgentProbe)))
	mux.Handle("POST /api/agents", r.requireAuth(http.HandlerFunc(r.handleCreateAgent)))
	mux.Handle("POST /api/agents/", r.requireAuth(http.HandlerFunc(r.handleAgentRoute)))
	mux.Handle("DELETE /api/agents/", r.requireAuth(http.HandlerFunc(r.handleAgentRoute)))
	mux.Handle("GET /api/vms", r.requireAuth(http.HandlerFunc(r.handleListVMs)))
	mux.Handle("POST /api/vms", r.requireAuth(http.HandlerFunc(r.handleCreateVM)))
	mux.Handle("GET /api/vms/", r.requireAuth(http.HandlerFunc(r.handleVMRoute)))
	mux.Handle("PUT /api/vms/", r.requireAuth(http.HandlerFunc(r.handleVMRoute)))
	mux.Handle("POST /api/vms/", r.requireAuth(http.HandlerFunc(r.handleVMRoute)))
	mux.Handle("DELETE /api/vms/", r.requireAuth(http.HandlerFunc(r.handleVMRoute)))
	mux.Handle("GET /api/snapshots", r.requireAuth(http.HandlerFunc(r.handleListSnapshots)))
	mux.Handle("POST /api/snapshots", r.requireAuth(http.HandlerFunc(r.handleCreateSnapshot)))
	mux.Handle("POST /api/snapshots/refresh", r.requireAuth(http.HandlerFunc(r.handleRefreshSnapshots)))
	mux.Handle("PUT /api/snapshots/", r.requireAuth(http.HandlerFunc(r.handleSnapshotRoute)))
	mux.Handle("POST /api/snapshots/", r.requireAuth(http.HandlerFunc(r.handleSnapshotRoute)))
	mux.Handle("GET /api/tasks", r.requireAuth(http.HandlerFunc(r.handleListTasks)))
	mux.Handle("GET /api/audit-logs", r.requireAuth(http.HandlerFunc(r.handleListAuditLogs)))
	mux.Handle("GET /api/alerts", r.requireAuth(http.HandlerFunc(r.handleListAlerts)))
	mux.Handle("GET /api/alerts/", r.requireAuth(http.HandlerFunc(r.handleAlertRoute)))
	mux.Handle("POST /api/alerts/", r.requireAuth(http.HandlerFunc(r.handleAlertRoute)))
	mux.Handle("GET /api/notifications", r.requireAuth(http.HandlerFunc(r.handleListNotifications)))
	mux.Handle("GET /api/notifications/unread-count", r.requireAuth(http.HandlerFunc(r.handleUnreadNotificationCount)))
	mux.Handle("POST /api/notifications/read-all", r.requireAuth(http.HandlerFunc(r.handleMarkAllNotificationsRead)))
	mux.Handle("POST /api/notifications/clear", r.requireAuth(http.HandlerFunc(r.handleClearNotifications)))
	mux.Handle("POST /api/notifications/", r.requireAuth(http.HandlerFunc(r.handleNotificationAction)))
	mux.Handle("GET /api/settings/base-config", r.requirePermission("settings.base.read", http.HandlerFunc(r.handleGetSystemBaseConfig)))
	mux.Handle("PUT /api/settings/base-config", r.requirePermission("settings.base.manage", http.HandlerFunc(r.handleUpdateSystemBaseConfig)))
	mux.Handle("GET /api/settings/notifications", r.requirePermission("settings.notifications.read", http.HandlerFunc(r.handleListNotificationChannels)))
	mux.Handle("PUT /api/settings/notifications/", r.requirePermission("settings.notifications.manage", http.HandlerFunc(r.handleNotificationRoute)))
	mux.Handle("POST /api/settings/notifications/", r.requirePermission("settings.notifications.manage", http.HandlerFunc(r.handleNotificationRoute)))
	mux.Handle("GET /api/settings/auth-providers", r.requirePermission("settings.auth.read", http.HandlerFunc(r.handleListAuthProviders)))
	mux.Handle("PUT /api/settings/auth-providers/", r.requirePermission("settings.auth.manage", http.HandlerFunc(r.handleAuthProviderRoute)))
	mux.Handle("POST /api/settings/auth-providers/", r.requirePermission("settings.auth.manage", http.HandlerFunc(r.handleAuthProviderRoute)))
	mux.Handle("GET /api/settings/users", r.requirePermission("settings.users.read", http.HandlerFunc(r.handleListManagedUsers)))
	mux.Handle("POST /api/settings/users", r.requirePermission("settings.users.manage", http.HandlerFunc(r.handleCreateManagedUser)))
	mux.Handle("PUT /api/settings/users/", r.requirePermission("settings.users.manage", http.HandlerFunc(r.handleManagedUserRoute)))
	mux.Handle("DELETE /api/settings/users/", r.requirePermission("settings.users.manage", http.HandlerFunc(r.handleManagedUserRoute)))
	mux.Handle("POST /api/settings/users/", r.requirePermission("settings.users.manage", http.HandlerFunc(r.handleManagedUserRoute)))
	mux.Handle("GET /api/settings/user-groups", r.requirePermission("settings.users.read", http.HandlerFunc(r.handleListUserGroups)))
	mux.Handle("POST /api/settings/user-groups", r.requirePermission("settings.users.manage", http.HandlerFunc(r.handleCreateUserGroup)))
	mux.Handle("PUT /api/settings/user-groups/", r.requirePermission("settings.users.manage", http.HandlerFunc(r.handleUserGroupRoute)))
	mux.Handle("DELETE /api/settings/user-groups/", r.requirePermission("settings.users.manage", http.HandlerFunc(r.handleUserGroupRoute)))
	mux.Handle("GET /api/settings/roles", r.requirePermission("settings.users.read", http.HandlerFunc(r.handleListRoles)))
	mux.Handle("POST /api/settings/roles", r.requirePermission("settings.users.manage", http.HandlerFunc(r.handleCreateRole)))
	mux.Handle("PUT /api/settings/roles/", r.requirePermission("settings.users.manage", http.HandlerFunc(r.handleRoleRoute)))
	mux.Handle("DELETE /api/settings/roles/", r.requirePermission("settings.users.manage", http.HandlerFunc(r.handleRoleRoute)))
	mux.Handle("GET /api/settings/permissions", r.requirePermission("settings.users.read", http.HandlerFunc(r.handleListPermissions)))
	return r.withCORS(r.withRequestLog(mux))
}

func (r *router) handleHealth(w http.ResponseWriter, req *http.Request) {
	redisStatus := "unreachable"
	if r.redis != nil {
		redisStatus = "ok"
		if err := r.redis.Ping(req.Context()).Err(); err != nil {
			redisStatus = "unreachable"
		}
	}
	if err := r.store.Pool().Ping(req.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "degraded", "database": "unreachable", "redis": redisStatus, "time": time.Now().UTC().Format(time.RFC3339)})
		return
	}
	status := "ok"
	if redisStatus != "ok" {
		status = "degraded"
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": status, "database": "ok", "redis": redisStatus, "time": time.Now().UTC().Format(time.RFC3339)})
}

func (r *router) handleLogin(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	var body loginRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "请求体必须是包含 username 和 password 的 JSON")
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	if body.Username == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "missing_credentials", "请输入用户名和密码")
		return
	}
	provider := strings.TrimSpace(body.Provider)
	if provider == "" {
		provider = "local"
	}
	session, err := r.auth.LoginWithProvider(req.Context(), provider, body.Username, body.Password)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotProvisioned) {
			writeError(w, http.StatusUnauthorized, "user_not_provisioned", "用户未在平台中启用")
			return
		}
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "用户名或密码不正确")
			return
		}
		r.logger.Error("login failed", "error", err)
		writeError(w, http.StatusInternalServerError, "login_failed", "登录失败，请稍后重试")
		return
	}
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "auth.login", "user", session.User.ID, repository.ClientIP(req), map[string]any{"username": session.User.Username})
	writeJSON(w, http.StatusOK, session)
}

func (r *router) handleMe(w http.ResponseWriter, req *http.Request) {
	session := currentSession(req)
	writeJSON(w, http.StatusOK, map[string]any{"user": session.User, "expires_at": session.ExpiresAt, "last_seen_at": session.LastSeenAt})
}

func (r *router) handleLogout(w http.ResponseWriter, req *http.Request) {
	session := currentSession(req)
	_ = r.auth.Logout(req.Context(), bearerToken(req))
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "auth.logout", "user", session.User.ID, repository.ClientIP(req), map[string]any{"username": session.User.Username})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *router) handleChangePassword(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	var body changePasswordRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "请求参数格式不正确")
		return
	}
	if len(body.OldPassword) < 6 || len(body.NewPassword) < 6 || len(body.ConfirmPassword) < 6 {
		writeError(w, http.StatusBadRequest, "invalid_password", "密码至少 6 个字符")
		return
	}
	if body.NewPassword != body.ConfirmPassword {
		writeError(w, http.StatusBadRequest, "password_mismatch", "新密码与确认密码不一致")
		return
	}
	if body.NewPassword == body.OldPassword {
		writeError(w, http.StatusBadRequest, "same_password", "新密码不能与旧密码相同")
		return
	}

	session := currentSession(req)
	_, passwordHash, err := r.store.FindUserByUsername(req.Context(), session.User.Username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user_not_found", "用户不存在")
			return
		}
		r.logger.Error("find user for password change failed", "error", err)
		writeError(w, http.StatusInternalServerError, "password_change_failed", "密码修改失败，请稍后重试")
		return
	}
	if err := auth.VerifyPassword(passwordHash, body.OldPassword); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_old_password", "旧密码不正确")
		return
	}
	newHash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		r.logger.Error("hash new password failed", "error", err)
		writeError(w, http.StatusInternalServerError, "password_change_failed", "密码加密失败")
		return
	}
	if err := r.store.UpdateUserPassword(req.Context(), session.User.ID, newHash); err != nil {
		r.logger.Error("update password failed", "error", err)
		writeError(w, http.StatusInternalServerError, "password_change_failed", "密码修改失败")
		return
	}
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "auth.password.change", "user", session.User.ID, repository.ClientIP(req), map[string]any{"username": session.User.Username})
	writeJSON(w, http.StatusOK, map[string]string{"message": "密码已修改"})
}

func (r *router) handleDashboardSummary(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "dashboard.read") {
		return
	}
	logs, _, err := r.store.ListAuditLogs(req.Context(), 8, 0, "", "", "")
	if err != nil {
		r.logger.Error("dashboard audit summary failed", "error", err)
		writeError(w, http.StatusInternalServerError, "dashboard_failed", "读取仪表盘数据失败")
		return
	}
	alerts, _, err := r.store.ListAlerts(req.Context(), "active", 6, 0, "", "", "")
	if err != nil {
		r.logger.Error("dashboard alerts failed", "error", err)
		writeError(w, http.StatusInternalServerError, "dashboard_failed", "读取仪表盘告警失败")
		return
	}
	writeJSON(w, http.StatusOK, r.runtime.DashboardSummary(logs, alerts))
}

func (r *router) handleListVMs(w http.ResponseWriter, req *http.Request) {
	user := currentSession(req).User
	if !hasAssociatedVMReadPermission(user) {
		writeError(w, http.StatusForbidden, "permission_denied", "当前用户无权执行此操作")
		return
	}
	vms := r.runtime.ListVMs(
		req.URL.Query().Get("status"),
		strings.TrimSpace(req.URL.Query().Get("q")),
		strings.TrimSpace(req.URL.Query().Get("hostId")),
	)
	writeJSON(w, http.StatusOK, map[string]any{"items": vms, "total": len(vms)})
}

func (r *router) handleListHosts(w http.ResponseWriter, req *http.Request) {
	user := currentSession(req).User
	if !hasAssociatedHostReadPermission(user) {
		writeError(w, http.StatusForbidden, "permission_denied", "当前用户无权执行此操作")
		return
	}
	hosts := r.runtime.ListHosts()
	writeJSON(w, http.StatusOK, map[string]any{"items": hosts, "total": len(hosts)})
}

func (r *router) agentTokenForVM(w http.ResponseWriter, req *http.Request, vm domain.VirtualMachine) (domain.Agent, string, bool) {
	agentRecord, err := r.store.GetAgent(req.Context(), vm.HostID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "agent_not_bound", "虚拟机所属宿主机未绑定 Agent，无法执行操作")
			return domain.Agent{}, "", false
		}
		writeError(w, http.StatusInternalServerError, "get_agent_failed", "读取 Agent 失败")
		return domain.Agent{}, "", false
	}
	token, err := tokencrypto.Open(r.cfg.JWT.Secret, agentRecord.TokenCiphertext)
	if err != nil || strings.TrimSpace(token) == "" {
		r.logger.Error("open agent token failed", "error", err, "agent", agentRecord.ID)
		writeError(w, http.StatusBadRequest, "agent_token_unavailable", "Agent 令牌不可用于执行操作，请重新保存 Agent")
		return domain.Agent{}, "", false
	}
	return agentRecord, token, true
}

func (r *router) handleVMRoute(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet {
		if id, ok := parseVMConsolePath(req.URL.Path); ok {
			if !r.ensurePermission(w, req, "vms.console") {
				return
			}
			r.handleVMConsoleWS(w, req, id)
			return
		}
	}
	id, action, ok := parseVMPath(req.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	if permission := vmRoutePermission(req.Method, action); permission != "" && !r.ensurePermission(w, req, permission) {
		return
	}
	if req.Method == http.MethodGet && action == "" {
		r.handleGetVM(w, req, id)
		return
	}
	if req.Method == http.MethodGet && action == "config" {
		r.handleGetVMConfig(w, req, id)
		return
	}
	if req.Method == http.MethodGet && action == "console" {
		r.handleGetVMConsole(w, req, id)
		return
	}
	if req.Method == http.MethodPut && action == "config" {
		r.handleUpdateVMConfig(w, req, id)
		return
	}
	if req.Method == http.MethodPut && action == "console" {
		r.handleUpdateVMConsole(w, req, id)
		return
	}
	if req.Method == http.MethodPut && action == "rename" {
		r.handleRenameVM(w, req, id)
		return
	}
	if req.Method == http.MethodPut && action == "autostart" {
		r.handleUpdateVMAutostart(w, req, id)
		return
	}
	if req.Method == http.MethodPut && action == "media" {
		r.handleConnectVMMedia(w, req, id)
		return
	}
	if req.Method == http.MethodDelete && action == "media" {
		r.handleDisconnectVMMedia(w, req, id)
		return
	}
	if req.Method == http.MethodPut && action == "xml" {
		r.handleUpdateVMXML(w, req, id)
		return
	}
	if req.Method == http.MethodPut && action == "devices" {
		r.handleUpdateVMDevices(w, req, id)
		return
	}
	if req.Method == http.MethodPost && action == "template-mark" {
		r.handleMarkVMTemplate(w, req, id)
		return
	}
	if req.Method == http.MethodDelete && action == "template-mark" {
		r.handleUnmarkVMTemplate(w, req, id)
		return
	}
	if req.Method == http.MethodPost && action == "template-create" {
		r.handleCreateVMFromTemplate(w, req, id)
		return
	}
	if req.Method == http.MethodPost && action == "clone" {
		r.handleCloneVM(w, req, id)
		return
	}
	if req.Method == http.MethodPost && action == "migrate" {
		r.handleMigrateVM(w, req, id)
		return
	}
	if req.Method == http.MethodPost && action == "migrate-precheck" {
		r.handlePrecheckVMMigration(w, req, id)
		return
	}
	if req.Method == http.MethodPost && action == "migrate-ssh-key" {
		r.handleSetupVMMigrationSSHKey(w, req, id)
		return
	}
	if req.Method == http.MethodPost && action == "migrate-hostname" {
		r.handleSetupVMMigrationHostname(w, req, id)
		return
	}
	if req.Method == http.MethodPost && action == "refresh" {
		r.handleRefreshVM(w, req, id)
		return
	}
	if req.Method == http.MethodPost && action != "" {
		r.handleVMAction(w, req, id, action)
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
}

func (r *router) handleListTasks(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "operations.read") {
		return
	}
	limit := parseLimit(req, 50)
	offset := parseOffset(req, limit)
	query := req.URL.Query()
	tasks, total, err := r.store.ListTasks(req.Context(), strings.TrimSpace(query.Get("status")), limit, offset, strings.TrimSpace(query.Get("q")), strings.TrimSpace(query.Get("payloadKey")), strings.TrimSpace(query.Get("payloadValue")))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_tasks_failed", "读取任务列表失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": tasks, "total": total})
}

func (r *router) handleTaskRoute(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "operations.read") {
		return
	}
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
		return
	}
	id := strings.Trim(strings.TrimPrefix(req.URL.Path, "/api/tasks/"), "/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	task, err := r.store.GetTask(req.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "task_not_found", "任务不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_task_failed", "读取任务失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": task})
}

func (r *router) handleListAuditLogs(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "operations.read") {
		return
	}
	limit := parseLimit(req, 20)
	offset := parseOffset(req, limit)
	query := req.URL.Query()
	logs, total, err := r.store.ListAuditLogs(req.Context(), limit, offset, strings.TrimSpace(query.Get("q")), strings.TrimSpace(query.Get("metadataKey")), strings.TrimSpace(query.Get("metadataValue")))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_audit_failed", "读取审计日志失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": logs, "total": total})
}

func (r *router) handleListAlerts(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "operations.read") {
		return
	}
	limit := parseLimit(req, 20)
	offset := parseOffset(req, limit)
	query := req.URL.Query()
	alerts, total, err := r.store.ListAlerts(req.Context(), strings.TrimSpace(query.Get("status")), limit, offset, strings.TrimSpace(query.Get("q")), strings.TrimSpace(query.Get("metadataKey")), strings.TrimSpace(query.Get("metadataValue")))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_alerts_failed", "读取告警列表失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": alerts, "total": total})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: code, Message: message})
}

func currentSession(req *http.Request) domain.Session {
	return req.Context().Value(sessionContextKey{}).(domain.Session)
}
