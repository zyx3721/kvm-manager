package router

import (
	"errors"
	"net/http"
	"strings"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/pkg/agent"
)

type markVMTemplateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (r *router) handleMarkVMTemplate(w http.ResponseWriter, req *http.Request, id string) {
	defer req.Body.Close()
	var body markVMTemplateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "模板标记参数格式不正确")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.Description = strings.TrimSpace(body.Description)
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid_vm_template", "模板名称不能为空")
		return
	}
	if len(body.Description) > 2048 {
		writeError(w, http.StatusBadRequest, "invalid_vm_template", "模板描述不能超过 2048 个字符")
		return
	}
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	if strings.TrimSpace(vm.UUID) == "" {
		writeError(w, http.StatusBadRequest, "vm_uuid_required", "当前虚拟机缺少 UUID，无法标记为模板")
		return
	}
	if strings.EqualFold(strings.TrimSpace(vm.Status), "running") {
		writeError(w, http.StatusBadRequest, "vm_running", "虚拟机正在运行，无法标记为模板，请先关闭虚拟机后再操作")
		return
	}
	session := currentSession(req)
	mark, err := r.store.UpsertVMTemplateMark(req.Context(), vm.HostID, vm.UUID, body.Name, body.Description, session.User.ID)
	if err != nil {
		r.logger.Error("mark vm template failed", "error", err, "vm", id)
		writeError(w, http.StatusInternalServerError, "mark_vm_template_failed", "标记虚拟机模板失败")
		return
	}
	task, err := r.store.CreateTask(req.Context(), "vm.template.mark", "completed", "virtual_machine", id, map[string]any{"vm": vm.Name, "template": mark.Name, "message": "虚拟机已标记为模板"}, session.User.ID, "")
	if err != nil {
		r.logger.Warn("create vm template mark task failed", "error", err)
	}
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "vm.template.mark", "virtual_machine", id, repository.ClientIP(req), map[string]any{"name": vm.Name, "template": mark.Name})
	r.runtime.Broadcast("runtime.updated")
	writeJSON(w, http.StatusOK, map[string]any{"template": mark, "task": task})
}

func (r *router) handleUnmarkVMTemplate(w http.ResponseWriter, req *http.Request, id string) {
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	if strings.TrimSpace(vm.UUID) == "" {
		writeError(w, http.StatusBadRequest, "vm_uuid_required", "当前虚拟机缺少 UUID，无法取消模板标记")
		return
	}
	if err := r.store.DeleteVMTemplateMark(req.Context(), vm.HostID, vm.UUID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "vm_template_not_found", "当前虚拟机未标记为模板")
			return
		}
		r.logger.Error("unmark vm template failed", "error", err, "vm", id)
		writeError(w, http.StatusInternalServerError, "unmark_vm_template_failed", "取消虚拟机模板标记失败")
		return
	}
	session := currentSession(req)
	task, err := r.store.CreateTask(req.Context(), "vm.template.unmark", "completed", "virtual_machine", id, map[string]any{"vm": vm.Name, "message": "已取消虚拟机模板标记"}, session.User.ID, "")
	if err != nil {
		r.logger.Warn("create vm template unmark task failed", "error", err)
	}
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "vm.template.unmark", "virtual_machine", id, repository.ClientIP(req), map[string]any{"name": vm.Name, "template": vm.TemplateName})
	r.runtime.Broadcast("runtime.updated")
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "task": task})
}

func (r *router) handleCreateVMFromTemplate(w http.ResponseWriter, req *http.Request, id string) {
	defer req.Body.Close()
	var body cloneVMRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "模板创建参数格式不正确")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid_vm_template_create", "虚拟机名称不能为空")
		return
	}
	if len(body.Disks) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_vm_template_create", "请配置从模板克隆的磁盘")
		return
	}
	if err := validateVMCloneRequest(body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_vm_template_create", err.Error())
		return
	}
	templateVM, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "模板虚拟机不存在")
		return
	}
	if !templateVM.IsTemplate {
		writeError(w, http.StatusBadRequest, "vm_template_required", "请选择已标记的虚拟机模板")
		return
	}
	if strings.EqualFold(strings.TrimSpace(templateVM.Status), "running") {
		writeError(w, http.StatusBadRequest, "vm_running", "模板虚拟机正在运行，无法创建，请先关闭模板虚拟机后再操作")
		return
	}
	agentRecord, token, ok := r.agentTokenForVM(w, req, templateVM)
	if !ok {
		return
	}
	cloneRequest := buildVMCloneAgentRequest(body)
	client := agent.NewClient(agentRecord.TLSInsecure)
	cfg := agent.Config{Endpoint: agentRecord.Endpoint, Token: token, TLSInsecure: agentRecord.TLSInsecure}
	if status, code, message := validateVMCloneTargets(req.Context(), client, cfg, cloneRequest); message != "" {
		writeError(w, status, code, message)
		return
	}
	session := currentSession(req)
	task, err := r.store.CreateTask(req.Context(), "vm.template.create", "queued", "virtual_machine", id, map[string]any{"template": templateDisplayName(templateVM), "vm": cloneRequest.Name, "agent": agentRecord.Name, "message": "从模板创建虚拟机排队中"}, session.User.ID, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_vm_template_task_failed", "创建模板虚拟机任务失败")
		return
	}
	r.runVMTemplateCreateTask(task.ID, session.User.ID, repository.ClientIP(req), templateVM, agentRecord, cfg, cloneRequest)
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "queued", "task": task})
}

func templateDisplayName(vm domain.VirtualMachine) string {
	if strings.TrimSpace(vm.TemplateName) != "" {
		return vm.TemplateName
	}
	return vm.Name
}
