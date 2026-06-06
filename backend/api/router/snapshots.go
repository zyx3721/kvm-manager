package router

import (
	"net/http"
	"regexp"
	"strings"

	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/pkg/agent"
	"kvm-manager/backend/pkg/tokencrypto"
)

type createSnapshotRequest struct {
	VMID        string   `json:"vmId"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type updateSnapshotAnnotationRequest struct {
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

var snapshotNamePattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,80}$`)

func (r *router) handleListSnapshots(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "snapshots.read") {
		return
	}
	snapshots := r.runtime.ListSnapshots()
	var err error
	snapshots, err = r.store.ApplySnapshotAnnotations(req.Context(), snapshots)
	if err != nil {
		r.logger.Error("apply snapshot annotations failed", "error", err)
		writeError(w, http.StatusInternalServerError, "list_snapshots_failed", "读取快照列表失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": snapshots, "total": len(snapshots)})
}

func (r *router) handleRefreshSnapshots(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "snapshots.read") {
		return
	}
	agents, err := r.store.ListAgents(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_agents_failed", "读取 Agent 列表失败")
		return
	}
	for _, item := range agents {
		token, err := tokencrypto.Open(r.cfg.JWT.Secret, item.TokenCiphertext)
		if err != nil {
			r.logger.Warn("decrypt agent token failed", "agent", item.ID, "error", err)
			continue
		}
		if err := r.runtime.SyncSnapshotsWithToken(req.Context(), item.ID, token); err != nil {
			r.logger.Warn("sync snapshots failed", "agent", item.ID, "error", err)
		}
	}
	r.runtime.Broadcast("runtime.updated")
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "snapshot.refresh", "snapshot", "", repository.ClientIP(req), map[string]any{})
	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

func (r *router) handleCreateSnapshot(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "snapshots.create") {
		return
	}
	defer req.Body.Close()
	var body createSnapshotRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "快照参数格式不正确")
		return
	}
	body.VMID = strings.TrimSpace(body.VMID)
	body.Name = strings.TrimSpace(body.Name)
	body.Description = strings.TrimSpace(body.Description)
	if body.VMID == "" || !snapshotNamePattern.MatchString(body.Name) {
		writeError(w, http.StatusBadRequest, "invalid_snapshot", "请选择虚拟机，并填写 1-80 位快照名称，名称仅支持字母、数字、点、下划线、冒号和连字符")
		return
	}
	if len(body.Tags) > 12 || len(body.Description) > 500 || hasOversizedSnapshotTag(body.Tags) {
		writeError(w, http.StatusBadRequest, "invalid_snapshot_annotation", "快照描述或标签数量超出限制")
		return
	}
	vm, ok := r.runtime.GetVM(body.VMID)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	if strings.ToLower(strings.TrimSpace(vm.Status)) != "stopped" {
		writeError(w, http.StatusBadRequest, "vm_must_be_stopped", "只有已关机的虚拟机可以创建快照")
		return
	}
	agentRecord, token, ok := r.agentTokenForVM(w, req, vm)
	if !ok {
		return
	}
	client := agent.NewClient(agentRecord.TLSInsecure)
	if err := client.CreateSnapshot(req.Context(), agent.Config{Endpoint: agentRecord.Endpoint, Token: token, TLSInsecure: agentRecord.TLSInsecure}, vm.Name, agent.SnapshotCreateRequest{Name: body.Name, Description: body.Description}); err != nil {
		r.logger.Error("agent snapshot create failed", "error", err, "vm", body.VMID)
		writeError(w, http.StatusServiceUnavailable, "agent_snapshot_create_failed", agent.UserFacingErrorMessage(err))
		return
	}
	_ = r.runtime.SyncSnapshotsWithToken(req.Context(), agentRecord.ID, token)
	r.runtime.Broadcast("runtime.updated")
	session := currentSession(req)
	annotationInput := repository.SnapshotAnnotationInput{Description: body.Description, Tags: body.Tags}
	createdSnapshot, exists := r.runtime.GetSnapshot(body.VMID + ":snapshot:" + body.Name)
	if exists && (body.Description != "" || len(body.Tags) > 0) {
		createdSnapshot, _ = r.store.UpsertSnapshotAnnotation(req.Context(), createdSnapshot, annotationInput, session.User.ID)
	}
	task, err := r.store.CreateTask(req.Context(), "snapshot.create", "completed", "virtual_machine", body.VMID, map[string]any{"snapshot": body.Name, "vm": vm.Name, "agent": agentRecord.Name, "snapshotType": "internal"}, session.User.ID, "")
	if err != nil {
		r.logger.Warn("create snapshot task failed", "error", err)
	}
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "snapshot.create", "virtual_machine", body.VMID, repository.ClientIP(req), map[string]any{"snapshot": body.Name, "vm": vm.Name, "agent": agentRecord.Name, "snapshotType": "internal"})
	writeJSON(w, http.StatusOK, map[string]any{"snapshot": createdSnapshot, "task": task})
}

func (r *router) handleSnapshotRoute(w http.ResponseWriter, req *http.Request) {
	id, action, ok := parseSnapshotPath(req.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	if req.Method == http.MethodPut && action == "annotation" {
		if !r.ensurePermission(w, req, "snapshots.update") {
			return
		}
		r.handleUpdateSnapshotAnnotation(w, req, id)
		return
	}
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
		return
	}
	if action != "revert" && action != "delete" {
		writeError(w, http.StatusNotFound, "unknown_action", "不支持的快照操作")
		return
	}
	if !r.ensurePermission(w, req, snapshotActionPermission(action)) {
		return
	}
	snapshot, ok := r.runtime.GetSnapshot(id)
	if !ok {
		writeError(w, http.StatusNotFound, "snapshot_not_found", "快照不存在")
		return
	}
	vm, ok := r.runtime.GetVM(snapshot.VMID)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "快照所属虚拟机不存在")
		return
	}
	agentRecord, token, ok := r.agentTokenForVM(w, req, vm)
	if !ok {
		return
	}
	client := agent.NewClient(agentRecord.TLSInsecure)
	if err := client.RunSnapshotAction(req.Context(), agent.Config{Endpoint: agentRecord.Endpoint, Token: token, TLSInsecure: agentRecord.TLSInsecure}, vm.Name, snapshot.Name, action); err != nil {
		r.logger.Error("agent snapshot action failed", "error", err, "action", action, "snapshot", id)
		writeError(w, http.StatusServiceUnavailable, "agent_snapshot_action_failed", agent.UserFacingErrorMessage(err))
		return
	}
	if action == "revert" {
		if _, err := r.runtime.SyncVMWithToken(req.Context(), agentRecord.ID, token, vm.ID, vm.Name); err != nil {
			r.logger.Warn("sync vm after snapshot revert failed", "error", err, "agent", agentRecord.ID, "vm", vm.ID)
		}
	}
	_ = r.runtime.SyncSnapshotsWithToken(req.Context(), agentRecord.ID, token)
	r.runtime.Broadcast("runtime.updated")
	session := currentSession(req)
	task, err := r.store.CreateTask(req.Context(), "snapshot."+action, "completed", "snapshot", id, map[string]any{"snapshot": snapshot.Name, "vm": vm.Name, "agent": agentRecord.Name}, session.User.ID, "")
	if err != nil {
		r.logger.Warn("create snapshot task failed", "error", err)
	}
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "snapshot."+action, "snapshot", id, repository.ClientIP(req), map[string]any{"snapshot": snapshot.Name, "vm": vm.Name, "agent": agentRecord.Name})
	writeJSON(w, http.StatusOK, map[string]any{"snapshot": snapshot, "task": task})
}

func snapshotActionPermission(action string) string {
	if action == "delete" {
		return "snapshots.delete"
	}
	return "snapshots.revert"
}

func (r *router) handleUpdateSnapshotAnnotation(w http.ResponseWriter, req *http.Request, id string) {
	defer req.Body.Close()
	snapshot, ok := r.runtime.GetSnapshot(id)
	if !ok {
		writeError(w, http.StatusNotFound, "snapshot_not_found", "快照不存在")
		return
	}
	var body updateSnapshotAnnotationRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "快照备注格式不正确")
		return
	}
	if len(body.Tags) > 12 || len(body.DisplayName) > 120 || len(body.Description) > 500 || hasOversizedSnapshotTag(body.Tags) {
		writeError(w, http.StatusBadRequest, "invalid_snapshot_annotation", "快照显示名、描述或标签数量超出限制")
		return
	}
	session := currentSession(req)
	updated, err := r.store.UpsertSnapshotAnnotation(req.Context(), snapshot, repository.SnapshotAnnotationInput{DisplayName: body.DisplayName, Description: body.Description, Tags: body.Tags}, session.User.ID)
	if err != nil {
		r.logger.Error("update snapshot annotation failed", "error", err, "snapshot", id)
		writeError(w, http.StatusInternalServerError, "update_snapshot_annotation_failed", "保存快照备注失败")
		return
	}
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "snapshot.annotation.update", "snapshot", id, repository.ClientIP(req), map[string]any{"snapshot": snapshot.Name, "vm": snapshot.VMName})
	writeJSON(w, http.StatusOK, updated)
}

func hasOversizedSnapshotTag(tags []string) bool {
	for _, tag := range tags {
		if len([]rune(strings.TrimSpace(tag))) > 40 {
			return true
		}
	}
	return false
}
