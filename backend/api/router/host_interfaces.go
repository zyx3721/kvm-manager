package router

import (
	"net/http"
	"strings"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/pkg/agent"
)

type hostInterfaceCreateRequest struct {
	Name              string   `json:"name"`
	StartMode         string   `json:"startMode"`
	Device            string   `json:"device"`
	Type              string   `json:"type"`
	STP               string   `json:"stp"`
	Delay             string   `json:"delay"`
	IPv4Mode          string   `json:"ipv4Mode"`
	IPv4Address       string   `json:"ipv4Address"`
	IPv4Gateway       string   `json:"ipv4Gateway"`
	IPv6Mode          string   `json:"ipv6Mode"`
	IPv6Address       string   `json:"ipv6Address"`
	IPv6Gateway       string   `json:"ipv6Gateway"`
	ApplySystemConfig bool     `json:"applySystemConfig"`
	DNSServers        []string `json:"dnsServers"`
}

type hostInterfaceStateUpdateRequest struct {
	Active bool `json:"active"`
}

func (r *router) handleHostInterfaceRoute(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet {
		if !r.ensurePermission(w, req, "host.interfaces.read") {
			return
		}
	} else if !r.ensurePermission(w, req, "host.interfaces.manage") {
		return
	}
	agentID, action, ok := parseHostResourcePath(req.URL.Path, "/api/host-interfaces/")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	agentRecord, token, ok := r.agentTokenForHost(w, req, agentID)
	if !ok {
		return
	}
	client := agent.NewClient(agentRecord.TLSInsecure)
	cfg := agent.Config{Endpoint: agentRecord.Endpoint, Token: token, TLSInsecure: agentRecord.TLSInsecure}
	if req.Method == http.MethodGet && action == "" {
		items, err := client.ListHostInterfaces(req.Context(), cfg)
		if err != nil {
			r.logger.Error("agent list host interfaces failed", "error", err, "agent", agentID)
			writeError(w, http.StatusServiceUnavailable, "agent_host_interfaces_failed", agent.UserFacingErrorMessage(err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
		return
	}
	if req.Method == http.MethodGet && action == "devices/list" {
		items, err := client.ListHostInterfaceDevices(req.Context(), cfg)
		if err != nil {
			r.logger.Error("agent list host interface devices failed", "error", err, "agent", agentID)
			writeError(w, http.StatusServiceUnavailable, "agent_host_interface_devices_failed", agent.UserFacingErrorMessage(err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
		return
	}
	if req.Method == http.MethodPost && action == "" {
		r.handleCreateHostInterface(w, req, agentRecord, cfg, client)
		return
	}
	if req.Method == http.MethodPut && strings.HasPrefix(action, "state/") {
		r.handleUpdateHostInterfaceState(w, req, agentRecord, cfg, client, strings.TrimPrefix(action, "state/"))
		return
	}
	if req.Method == http.MethodDelete && strings.HasPrefix(action, "delete/") {
		r.handleDeleteHostInterface(w, req, agentRecord, cfg, client, strings.TrimPrefix(action, "delete/"))
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
}

func (r *router) handleCreateHostInterface(w http.ResponseWriter, req *http.Request, agentRecord domain.Agent, cfg agent.Config, client *agent.Client) {
	defer req.Body.Close()
	var body hostInterfaceCreateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "接口参数格式不正确")
		return
	}
	item, err := client.CreateHostInterface(req.Context(), cfg, agent.HostInterfaceCreateRequest{
		Name:              strings.TrimSpace(body.Name),
		StartMode:         strings.TrimSpace(body.StartMode),
		Device:            strings.TrimSpace(body.Device),
		Type:              strings.TrimSpace(body.Type),
		STP:               strings.TrimSpace(body.STP),
		Delay:             strings.TrimSpace(body.Delay),
		IPv4Mode:          strings.TrimSpace(body.IPv4Mode),
		IPv4Address:       strings.TrimSpace(body.IPv4Address),
		IPv4Gateway:       strings.TrimSpace(body.IPv4Gateway),
		IPv6Mode:          strings.TrimSpace(body.IPv6Mode),
		IPv6Address:       strings.TrimSpace(body.IPv6Address),
		IPv6Gateway:       strings.TrimSpace(body.IPv6Gateway),
		ApplySystemConfig: body.ApplySystemConfig,
		DNSServers:        trimStringSlice(body.DNSServers),
	})
	if err != nil {
		r.logger.Error("agent create host interface failed", "error", err, "agent", agentRecord.ID)
		writeError(w, http.StatusServiceUnavailable, "agent_host_interface_create_failed", agent.UserFacingErrorMessage(err))
		return
	}
	session := currentSession(req)
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "host_interface.create", "agent", agentRecord.ID, repository.ClientIP(req), map[string]any{"agent": agentRecord.Name, "name": item.Name, "type": item.Type})
	r.broadcastHostInterfaceUpdated(agentRecord.ID, item.Name)
	writeJSON(w, http.StatusOK, item)
}

func trimStringSlice(values []string) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func (r *router) handleUpdateHostInterfaceState(w http.ResponseWriter, req *http.Request, agentRecord domain.Agent, cfg agent.Config, client *agent.Client, name string) {
	defer req.Body.Close()
	if strings.TrimSpace(name) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "接口名称不能为空")
		return
	}
	var body hostInterfaceStateUpdateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "接口状态参数格式不正确")
		return
	}
	if err := client.UpdateHostInterfaceState(req.Context(), cfg, name, agent.PoolStateUpdateRequest{Active: body.Active}); err != nil {
		r.logger.Error("agent update host interface state failed", "error", err, "agent", agentRecord.ID, "interface", name)
		writeError(w, http.StatusServiceUnavailable, "agent_host_interface_state_failed", agent.UserFacingErrorMessage(err))
		return
	}
	session := currentSession(req)
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "host_interface.state.update", "agent", agentRecord.ID, repository.ClientIP(req), map[string]any{"agent": agentRecord.Name, "name": name, "active": body.Active})
	r.broadcastHostInterfaceUpdated(agentRecord.ID, name)
	writeJSON(w, http.StatusOK, map[string]bool{"active": body.Active})
}

func (r *router) handleDeleteHostInterface(w http.ResponseWriter, req *http.Request, agentRecord domain.Agent, cfg agent.Config, client *agent.Client, name string) {
	if strings.TrimSpace(name) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "接口名称不能为空")
		return
	}
	if err := client.DeleteHostInterface(req.Context(), cfg, name); err != nil {
		r.logger.Error("agent delete host interface failed", "error", err, "agent", agentRecord.ID, "interface", name)
		writeError(w, http.StatusServiceUnavailable, "agent_host_interface_delete_failed", agent.UserFacingErrorMessage(err))
		return
	}
	session := currentSession(req)
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "host_interface.delete", "agent", agentRecord.ID, repository.ClientIP(req), map[string]any{"agent": agentRecord.Name, "name": name})
	r.broadcastHostInterfaceUpdated(agentRecord.ID, name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *router) broadcastHostInterfaceUpdated(agentID string, name string) {
	r.runtime.BroadcastPayload("host.interface.updated", map[string]any{"agentId": agentID, "name": name})
	r.runtime.Broadcast("runtime.updated")
}
