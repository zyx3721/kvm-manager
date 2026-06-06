package router

import (
	"errors"
	"net/http"
	"strings"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/internal/repository"
)

func (r *router) handleAlertRoute(w http.ResponseWriter, req *http.Request) {
	id, action, ok := parseAlertPath(req.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	if req.Method == http.MethodGet && action == "deliveries" {
		r.handleAlertDeliveries(w, req, id)
		return
	}
	if req.Method != http.MethodPost || action != "resolve" {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
		return
	}
	if !r.ensurePermission(w, req, "alerts.manage") {
		return
	}
	alert, err := r.store.ResolveAlertReturning(req.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "alert_not_found", "告警不存在或已解决")
			return
		}
		writeError(w, http.StatusInternalServerError, "resolve_alert_failed", "解决告警失败")
		return
	}
	r.runtime.QueueResolvedAlertNotifications(req.Context(), []domain.Alert{alert})
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "alert.resolve", "alert", id, repository.ClientIP(req), map[string]any{})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *router) handleAlertDeliveries(w http.ResponseWriter, req *http.Request, id string) {
	if !r.ensurePermission(w, req, "operations.read") {
		return
	}
	deliveries, err := r.store.ListAlertNotificationDeliveries(req.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_alert_deliveries_failed", "读取告警通知投递记录失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": deliveries, "total": len(deliveries)})
}

func parseAlertPath(path string) (id string, action string, ok bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/api/alerts/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
