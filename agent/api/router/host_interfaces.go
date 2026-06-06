package router

import (
	"net/http"
	"strings"

	"kvm-manager/agent/internal/kvm"
)

func (r *Router) handleHostInterfaces(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		items, err := r.provider.ListHostInterfaces()
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "list_host_interfaces_failed", "读取宿主机接口失败")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
	case http.MethodPost:
		defer req.Body.Close()
		var body kvm.HostInterfaceCreateRequest
		if err := decodeJSONBody(w, req, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "接口参数格式不正确")
			return
		}
		item, err := r.provider.CreateHostInterface(body)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "create_host_interface_failed", operationErrorMessage("创建宿主机接口失败", err))
			return
		}
		writeJSON(w, http.StatusOK, item)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
	}
}

func (r *Router) handleHostInterfaceRoute(w http.ResponseWriter, req *http.Request) {
	name, action, ok := parseHostInterfacePath(req.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	if req.Method == http.MethodGet && name == "devices" && action == "list" {
		r.handleHostInterfaceDevices(w, req)
		return
	}
	if req.Method == http.MethodPut && action == "state" {
		r.handleUpdateHostInterfaceState(w, req, name)
		return
	}
	if req.Method == http.MethodDelete && action == "delete" {
		r.handleDeleteHostInterface(w, req, name)
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
}

func (r *Router) handleHostInterfaceDevices(w http.ResponseWriter, req *http.Request) {
	items, err := r.provider.ListHostInterfaceDevices()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "list_host_interface_devices_failed", "读取宿主机接口设备失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (r *Router) handleUpdateHostInterfaceState(w http.ResponseWriter, req *http.Request, name string) {
	defer req.Body.Close()
	var body struct {
		Active bool `json:"active"`
	}
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "接口状态参数格式不正确")
		return
	}
	if err := r.provider.UpdateHostInterfaceState(name, body.Active); err != nil {
		r.logger.Error("host interface state update failed", "interface", name, "error", err)
		writeError(w, http.StatusServiceUnavailable, "host_interface_state_failed", operationErrorMessage("修改宿主机接口状态失败", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"active": body.Active})
}

func (r *Router) handleDeleteHostInterface(w http.ResponseWriter, req *http.Request, name string) {
	if err := r.provider.DeleteHostInterface(name); err != nil {
		r.logger.Error("host interface delete failed", "interface", name, "error", err)
		writeError(w, http.StatusServiceUnavailable, "host_interface_delete_failed", operationErrorMessage("删除宿主机接口失败", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func parseHostInterfacePath(path string) (name string, action string, ok bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/v1/host/interfaces/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 2 && parts[0] == "devices" && parts[1] == "list" {
		return parts[0], parts[1], true
	}
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
