package router

import (
	"errors"
	"net/http"
	"strings"

	"kvm-manager/backend/internal/repository"
)

func (r *router) handleListNotifications(w http.ResponseWriter, req *http.Request) {
	limit := parseLimit(req, 20)
	status := strings.TrimSpace(req.URL.Query().Get("status"))
	if status == "" {
		status = "active"
	}
	items, total, err := r.store.ListAlertNotifications(req.Context(), status, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_notifications_failed", "读取通知消息失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (r *router) handleUnreadNotificationCount(w http.ResponseWriter, req *http.Request) {
	total, err := r.store.CountUnreadAlertNotifications(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "count_notifications_failed", "读取未读通知失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": total})
}

func (r *router) handleNotificationAction(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "alerts.manage") {
		return
	}
	id, action, ok := parseNotificationActionPath(req.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
		return
	}
	switch action {
	case "read":
		if err := r.store.MarkAlertNotificationRead(req.Context(), id); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				writeError(w, http.StatusNotFound, "notification_not_found", "通知消息不存在")
				return
			}
			writeError(w, http.StatusInternalServerError, "read_notification_failed", "标记通知失败")
			return
		}
		_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "notification.read", "alert", id, repository.ClientIP(req), map[string]any{})
	default:
		writeError(w, http.StatusNotFound, "unknown_action", "不支持的通知操作")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *router) handleMarkAllNotificationsRead(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "alerts.manage") {
		return
	}
	if err := r.store.MarkAllAlertNotificationsRead(req.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "read_notifications_failed", "标记通知失败")
		return
	}
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "notification.read_all", "notification", "", repository.ClientIP(req), map[string]any{})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *router) handleClearNotifications(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "alerts.manage") {
		return
	}
	if err := r.store.DismissAlertNotifications(req.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "clear_notifications_failed", "清空通知失败")
		return
	}
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "notification.clear", "notification", "", repository.ClientIP(req), map[string]any{})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func parseNotificationActionPath(path string) (id string, action string, ok bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/api/notifications/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
