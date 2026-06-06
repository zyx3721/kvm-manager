package router

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/pkg/agent"
	"kvm-manager/backend/pkg/tokencrypto"
)

func (r *router) handleGetVM(w http.ResponseWriter, req *http.Request, id string) {
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	writeJSON(w, http.StatusOK, vm)
}

func (r *router) handleRefreshVM(w http.ResponseWriter, req *http.Request, id string) {
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	agentRecord, err := r.store.GetAgent(req.Context(), vm.HostID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "agent_not_bound", "该虚拟机所属宿主机未绑定 Agent，无法刷新")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_agent_failed", "读取 Agent 失败")
		return
	}
	token, err := tokencrypto.Open(r.cfg.JWT.Secret, agentRecord.TokenCiphertext)
	if err != nil || strings.TrimSpace(token) == "" {
		r.logger.Error("open agent token for single vm refresh failed", "error", err, "agent", agentRecord.ID)
		writeError(w, http.StatusBadRequest, "agent_token_unavailable", "Agent 令牌不可用于刷新虚拟机，请重新保存 Agent")
		return
	}
	refreshed, err := r.runtime.SyncVMWithToken(req.Context(), agentRecord.ID, token, id, vm.Name)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "agent_vm_refresh_failed", agent.UserFacingErrorMessage(err))
		return
	}
	r.runtime.Broadcast("runtime.updated")
	session := currentSession(req)
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "vm.refresh", "virtual_machine", id, repository.ClientIP(req), map[string]any{"name": vm.Name, "agent": agentRecord.Name})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "vm": refreshed})
}

func (r *router) handleVMAction(w http.ResponseWriter, req *http.Request, id string, action string) {
	agentAction, ok := mapVMAction(action)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown_action", "不支持的虚拟机操作")
		return
	}
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	agentRecord, err := r.store.GetAgent(req.Context(), vm.HostID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "agent_not_bound", "该虚拟机所属宿主机未绑定 Agent，无法下发远程操作")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_agent_failed", "读取 Agent 失败")
		return
	}
	token, err := tokencrypto.Open(r.cfg.JWT.Secret, agentRecord.TokenCiphertext)
	if err != nil || strings.TrimSpace(token) == "" {
		r.logger.Error("open agent token for vm action failed", "error", err, "agent", agentRecord.ID)
		writeError(w, http.StatusBadRequest, "agent_token_unavailable", "Agent 令牌不可用于执行操作，请重新保存 Agent")
		return
	}
	client := agent.NewClient(agentRecord.TLSInsecure)
	if err := client.RunVMAction(req.Context(), agent.Config{Endpoint: agentRecord.Endpoint, Token: token, TLSInsecure: agentRecord.TLSInsecure}, vm.Name, agentAction); err != nil {
		r.logger.Error("agent vm action failed", "error", err, "action", action, "vm", id)
		writeError(w, http.StatusServiceUnavailable, "agent_vm_action_failed", agent.UserFacingErrorMessage(err))
		return
	}
	refreshedVM := vm
	if shouldRemoveVMAndDelayFullSyncAfterVMAction(action) {
		r.runtime.RemoveVM(id, agentRecord.ID)
		r.runtime.Broadcast("runtime.updated")
		r.runtime.SyncAgentWithTokenDelayedFull(context.Background(), agentRecord.ID, token)
	} else if shouldUpdateStatusAndDelayFullSyncAfterVMAction(action) {
		if updated, ok := r.runtime.UpdateVMStatus(id, statusAfterVMAction(action)); ok {
			refreshedVM = updated
		}
		r.runtime.Broadcast("runtime.updated")
		r.runtime.SyncAgentWithTokenDelayedFull(context.Background(), agentRecord.ID, token)
	} else {
		_ = r.runtime.SyncAgentWithToken(req.Context(), agentRecord.ID, token)
		r.runtime.Broadcast("runtime.updated")
	}
	session := currentSession(req)
	task, err := r.store.CreateTask(req.Context(), "vm."+action, "completed", "virtual_machine", id, map[string]any{"agentAction": agentAction, "agent": agentRecord.Name}, session.User.ID, "")
	if err != nil {
		r.logger.Warn("create vm task failed", "error", err)
	}
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "vm."+action, "virtual_machine", id, repository.ClientIP(req), map[string]any{"name": vm.Name, "agent": agentRecord.Name})
	writeJSON(w, http.StatusOK, map[string]any{"vm": refreshedVM, "task": task})
}

func shouldUpdateStatusAndDelayFullSyncAfterVMAction(action string) bool {
	switch action {
	case "start", "resume", "reboot", "force-reboot", "shutdown", "stop", "force-shutdown", "force-stop", "pause":
		return true
	default:
		return false
	}
}

func shouldRemoveVMAndDelayFullSyncAfterVMAction(action string) bool {
	switch action {
	case "delete", "force-delete":
		return true
	default:
		return false
	}
}

func statusAfterVMAction(action string) string {
	switch action {
	case "pause":
		return "paused"
	case "shutdown", "stop", "force-shutdown", "force-stop", "delete", "force-delete":
		return "stopped"
	default:
		return "running"
	}
}
