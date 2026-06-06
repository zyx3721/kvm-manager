package router

import (
	"errors"
	"net/http"
	"strings"

	"kvm-manager/backend/internal/repository"
)

func (r *router) handleAlertRoute(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "alerts.manage") {
		return
	}
	id, action, ok := parseAlertPath(req.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	if req.Method != http.MethodPost || action != "resolve" {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
		return
	}
	if err := r.store.ResolveAlert(req.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "alert_not_found", "告警不存在或已解决")
			return
		}
		writeError(w, http.StatusInternalServerError, "resolve_alert_failed", "解决告警失败")
		return
	}
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "alert.resolve", "alert", id, repository.ClientIP(req), map[string]any{})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func parseAlertPath(path string) (id string, action string, ok bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/api/alerts/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
