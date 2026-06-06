package router

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/pkg/agent"
)

type updateVMConfigRequest struct {
	Description       string `json:"description"`
	CurrentCPU        int    `json:"currentCpu"`
	MaximumCPU        int    `json:"maximumCpu"`
	CurrentMemoryMB   int64  `json:"currentMemoryMB"`
	MaximumMemoryMB   int64  `json:"maximumMemoryMB"`
	MemoryStatsPeriod int    `json:"memoryStatsPeriod"`
}

type renameVMRequest struct {
	Name string `json:"name"`
}

type updateVMAutostartRequest struct {
	Autostart bool `json:"autostart"`
}

type updateVMConsoleRequest struct {
	PasswordEnabled bool   `json:"passwordEnabled"`
	Password        string `json:"password"`
}

type connectVMMediaRequest struct {
	Target  string `json:"target"`
	ISOPath string `json:"isoPath"`
}

type disconnectVMMediaRequest struct {
	Target string `json:"target"`
}

type updateVMXMLRequest struct {
	XML string `json:"xml"`
}

type cloneVMRequest struct {
	Name            string                    `json:"name"`
	Description     string                    `json:"description"`
	Autostart       bool                      `json:"autostart"`
	CurrentCPU      int                       `json:"currentCpu"`
	MaximumCPU      int                       `json:"maximumCpu"`
	CurrentMemoryMB int64                     `json:"currentMemoryMB"`
	MaximumMemoryMB int64                     `json:"maximumMemoryMB"`
	CDROMPolicy     string                    `json:"cdromPolicy"`
	Interfaces      []cloneVMInterfaceRequest `json:"interfaces"`
	Disks           []cloneVMDiskRequest      `json:"disks"`
}

type cloneVMInterfaceRequest struct {
	Name   string `json:"name"`
	MAC    string `json:"mac"`
	Source string `json:"source"`
}

type cloneVMDiskRequest struct {
	Name             string `json:"name"`
	Pool             string `json:"pool"`
	SourcePath       string `json:"sourcePath"`
	TargetName       string `json:"targetName"`
	PreallocMetadata bool   `json:"preallocMetadata"`
}

func (r *router) handleGetVMConfig(w http.ResponseWriter, req *http.Request, id string) {
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	agentRecord, token, ok := r.agentTokenForVM(w, req, vm)
	if !ok {
		return
	}
	client := agent.NewClient(agentRecord.TLSInsecure)
	config, err := client.VMConfig(req.Context(), agent.Config{Endpoint: agentRecord.Endpoint, Token: token, TLSInsecure: agentRecord.TLSInsecure}, vm.Name)
	if err != nil {
		r.logger.Error("agent vm config failed", "error", err, "vm", id)
		writeError(w, http.StatusServiceUnavailable, "agent_vm_config_failed", agent.UserFacingErrorMessage(err))
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (r *router) handleUpdateVMConfig(w http.ResponseWriter, req *http.Request, id string) {
	defer req.Body.Close()
	var body updateVMConfigRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "虚拟机配置参数格式不正确")
		return
	}
	body.Description = strings.TrimSpace(body.Description)
	if err := validateVMConfigUpdate(body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_vm_config", err.Error())
		return
	}
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	agentRecord, token, ok := r.agentTokenForVM(w, req, vm)
	if !ok {
		return
	}
	client := agent.NewClient(agentRecord.TLSInsecure)
	cfg := agent.Config{Endpoint: agentRecord.Endpoint, Token: token, TLSInsecure: agentRecord.TLSInsecure}
	request := agent.VMConfigUpdateRequest{
		Description:       body.Description,
		CurrentCPU:        body.CurrentCPU,
		MaximumCPU:        body.MaximumCPU,
		CurrentMemoryMB:   body.CurrentMemoryMB,
		MaximumMemoryMB:   body.MaximumMemoryMB,
		MemoryStatsPeriod: body.MemoryStatsPeriod,
	}
	config, err := client.UpdateVMConfig(req.Context(), cfg, vm.Name, request)
	if err != nil {
		r.logger.Error("agent vm config update failed", "error", err, "vm", id)
		writeError(w, http.StatusServiceUnavailable, "agent_vm_config_update_failed", agent.UserFacingErrorMessage(err))
		return
	}
	r.syncVMRuntimeAsync(agentRecord.ID, token, id, vm.Name)
	session := currentSession(req)
	task, err := r.store.CreateTask(req.Context(), "vm.config.update", "completed", "virtual_machine", id, map[string]any{"vm": vm.Name, "agent": agentRecord.Name, "currentCpu": request.CurrentCPU, "maximumCpu": request.MaximumCPU, "currentMemoryMB": request.CurrentMemoryMB, "maximumMemoryMB": request.MaximumMemoryMB, "memoryStatsPeriod": request.MemoryStatsPeriod}, session.User.ID, "")
	if err != nil {
		r.logger.Warn("create vm config update task failed", "error", err)
	}
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "vm.config.update", "virtual_machine", id, repository.ClientIP(req), map[string]any{"name": vm.Name, "agent": agentRecord.Name, "currentCpu": request.CurrentCPU, "maximumCpu": request.MaximumCPU, "currentMemoryMB": request.CurrentMemoryMB, "maximumMemoryMB": request.MaximumMemoryMB, "memoryStatsPeriod": request.MemoryStatsPeriod})
	writeJSON(w, http.StatusOK, map[string]any{"config": config, "task": task})
}

func (r *router) handleRenameVM(w http.ResponseWriter, req *http.Request, id string) {
	defer req.Body.Close()
	var body renameVMRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "虚拟机名称参数格式不正确")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid_vm_name", "虚拟机名称不能为空")
		return
	}
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	if strings.EqualFold(strings.TrimSpace(vm.Status), "running") {
		writeError(w, http.StatusBadRequest, "vm_running", "虚拟机正在运行，无法修改名称，请先关闭虚拟机后再操作")
		return
	}
	agentRecord, token, ok := r.agentTokenForVM(w, req, vm)
	if !ok {
		return
	}
	client := agent.NewClient(agentRecord.TLSInsecure)
	cfg := agent.Config{Endpoint: agentRecord.Endpoint, Token: token, TLSInsecure: agentRecord.TLSInsecure}
	if body.Name == vm.Name {
		config, err := client.VMConfig(req.Context(), cfg, vm.Name)
		if err != nil {
			r.logger.Error("agent vm config failed before rename", "error", err, "vm", id)
			writeError(w, http.StatusServiceUnavailable, "agent_vm_config_failed", agent.UserFacingErrorMessage(err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"config": config, "task": nil})
		return
	}
	if status, code, message := validateVMRenameTarget(req.Context(), client, cfg, body.Name); message != "" {
		writeError(w, status, code, message)
		return
	}
	config, err := client.RenameVM(req.Context(), cfg, vm.Name, agent.VMRenameRequest{Name: body.Name})
	if err != nil {
		r.logger.Error("agent vm rename failed", "error", err, "vm", id, "target", body.Name)
		writeError(w, http.StatusServiceUnavailable, "agent_vm_rename_failed", agent.UserFacingErrorMessage(err))
		return
	}
	if err := r.runtime.SyncAgentWithToken(req.Context(), agentRecord.ID, token); err != nil {
		r.logger.Warn("sync agent after vm rename failed", "error", err, "agent", agentRecord.ID)
	}
	r.runtime.Broadcast("runtime.updated")
	session := currentSession(req)
	task, err := r.store.CreateTask(req.Context(), "vm.rename", "completed", "virtual_machine", id, map[string]any{"vm": vm.Name, "newName": body.Name, "agent": agentRecord.Name}, session.User.ID, "")
	if err != nil {
		r.logger.Warn("create vm rename task failed", "error", err)
	}
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "vm.rename", "virtual_machine", id, repository.ClientIP(req), map[string]any{"name": vm.Name, "newName": body.Name, "agent": agentRecord.Name})
	writeJSON(w, http.StatusOK, map[string]any{"config": config, "task": task})
}

func (r *router) handleUpdateVMAutostart(w http.ResponseWriter, req *http.Request, id string) {
	defer req.Body.Close()
	var body updateVMAutostartRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "虚拟机自启动参数格式不正确")
		return
	}
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	agentRecord, token, ok := r.agentTokenForVM(w, req, vm)
	if !ok {
		return
	}
	client := agent.NewClient(agentRecord.TLSInsecure)
	if err := client.UpdateVMAutostart(req.Context(), agent.Config{Endpoint: agentRecord.Endpoint, Token: token, TLSInsecure: agentRecord.TLSInsecure}, vm.Name, agent.VMAutostartUpdateRequest{Autostart: body.Autostart}); err != nil {
		r.logger.Error("agent vm autostart update failed", "error", err, "vm", id)
		writeError(w, http.StatusServiceUnavailable, "agent_vm_autostart_update_failed", agent.UserFacingErrorMessage(err))
		return
	}
	session := currentSession(req)
	task, err := r.store.CreateTask(req.Context(), "vm.autostart.update", "completed", "virtual_machine", id, map[string]any{"vm": vm.Name, "agent": agentRecord.Name, "autostart": body.Autostart}, session.User.ID, "")
	if err != nil {
		r.logger.Warn("create vm autostart update task failed", "error", err)
	}
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "vm.autostart.update", "virtual_machine", id, repository.ClientIP(req), map[string]any{"name": vm.Name, "agent": agentRecord.Name, "autostart": body.Autostart})
	r.syncVMRuntimeAsync(agentRecord.ID, token, id, vm.Name)
	writeJSON(w, http.StatusOK, map[string]any{"autostart": body.Autostart, "task": task})
}

func (r *router) handleUpdateVMConsole(w http.ResponseWriter, req *http.Request, id string) {
	defer req.Body.Close()
	var body updateVMConsoleRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "控制台配置参数格式不正确")
		return
	}
	body.Password = strings.TrimSpace(body.Password)
	if body.PasswordEnabled && body.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_console_password", "请输入控制台密码")
		return
	}
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	agentRecord, token, ok := r.agentTokenForVM(w, req, vm)
	if !ok {
		return
	}
	if !body.PasswordEnabled && strings.EqualFold(vm.Status, "running") {
		currentConfig, err := agent.NewClient(agentRecord.TLSInsecure).VMConfig(req.Context(), agent.Config{Endpoint: agentRecord.Endpoint, Token: token, TLSInsecure: agentRecord.TLSInsecure}, vm.Name)
		if err != nil {
			r.logger.Error("agent vm console config check failed", "error", err, "vm", id)
			writeError(w, http.StatusServiceUnavailable, "agent_vm_console_check_failed", agent.UserFacingErrorMessage(err))
			return
		}
		if currentConfig.Graphics.PasswordEnabled {
			writeError(w, http.StatusBadRequest, "running_console_password_disable_unsupported", "运行中的虚拟机不支持关闭控制台密码，请先关闭虚拟机后再操作")
			return
		}
	}
	request := agent.VMConsoleUpdateRequest{PasswordEnabled: body.PasswordEnabled, Password: body.Password}
	config, err := agent.NewClient(agentRecord.TLSInsecure).UpdateVMConsole(req.Context(), agent.Config{Endpoint: agentRecord.Endpoint, Token: token, TLSInsecure: agentRecord.TLSInsecure}, vm.Name, request)
	if err != nil {
		r.logger.Error("agent vm console update failed", "error", err, "vm", id)
		writeError(w, http.StatusServiceUnavailable, "agent_vm_console_update_failed", agent.UserFacingErrorMessage(err))
		return
	}
	session := currentSession(req)
	task, err := r.store.CreateTask(req.Context(), "vm.console.update", "completed", "virtual_machine", id, map[string]any{"vm": vm.Name, "agent": agentRecord.Name, "passwordEnabled": body.PasswordEnabled}, session.User.ID, "")
	if err != nil {
		r.logger.Warn("create vm console update task failed", "error", err)
	}
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "vm.console.update", "virtual_machine", id, repository.ClientIP(req), map[string]any{"name": vm.Name, "agent": agentRecord.Name, "passwordEnabled": body.PasswordEnabled})
	writeJSON(w, http.StatusOK, map[string]any{"config": config, "task": task})
}

func (r *router) handleConnectVMMedia(w http.ResponseWriter, req *http.Request, id string) {
	defer req.Body.Close()
	var body connectVMMediaRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "介质连接参数格式不正确")
		return
	}
	body.Target = strings.TrimSpace(body.Target)
	body.ISOPath = strings.TrimSpace(body.ISOPath)
	if body.Target == "" {
		writeError(w, http.StatusBadRequest, "invalid_vm_media", "请选择要连接的光驱")
		return
	}
	if body.ISOPath == "" {
		writeError(w, http.StatusBadRequest, "invalid_vm_media", "请选择 ISO 文件")
		return
	}
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	agentRecord, token, ok := r.agentTokenForVM(w, req, vm)
	if !ok {
		return
	}
	client := agent.NewClient(agentRecord.TLSInsecure)
	config, err := client.ConnectVMMedia(req.Context(), agent.Config{Endpoint: agentRecord.Endpoint, Token: token, TLSInsecure: agentRecord.TLSInsecure}, vm.Name, agent.VMMediaConnectRequest{Target: body.Target, ISOPath: body.ISOPath})
	if err != nil {
		r.logger.Error("agent vm media connect failed", "error", err, "vm", id, "target", body.Target)
		writeError(w, http.StatusServiceUnavailable, "agent_vm_media_connect_failed", agent.UserFacingErrorMessage(err))
		return
	}
	session := currentSession(req)
	task, err := r.store.CreateTask(req.Context(), "vm.media.connect", "completed", "virtual_machine", id, map[string]any{"vm": vm.Name, "agent": agentRecord.Name, "target": body.Target, "isoPath": body.ISOPath}, session.User.ID, "")
	if err != nil {
		r.logger.Warn("create vm media connect task failed", "error", err)
	}
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "vm.media.connect", "virtual_machine", id, repository.ClientIP(req), map[string]any{"name": vm.Name, "agent": agentRecord.Name, "target": body.Target, "isoPath": body.ISOPath})
	r.syncVMRuntimeAsync(agentRecord.ID, token, id, vm.Name)
	writeJSON(w, http.StatusOK, map[string]any{"config": config, "task": task})
}

func (r *router) handleDisconnectVMMedia(w http.ResponseWriter, req *http.Request, id string) {
	defer req.Body.Close()
	var body disconnectVMMediaRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "介质断开参数格式不正确")
		return
	}
	body.Target = strings.TrimSpace(body.Target)
	if body.Target == "" {
		writeError(w, http.StatusBadRequest, "invalid_vm_media", "请选择要断开的光驱")
		return
	}
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	agentRecord, token, ok := r.agentTokenForVM(w, req, vm)
	if !ok {
		return
	}
	client := agent.NewClient(agentRecord.TLSInsecure)
	config, err := client.DisconnectVMMedia(req.Context(), agent.Config{Endpoint: agentRecord.Endpoint, Token: token, TLSInsecure: agentRecord.TLSInsecure}, vm.Name, agent.VMMediaDisconnectRequest{Target: body.Target})
	if err != nil {
		r.logger.Error("agent vm media disconnect failed", "error", err, "vm", id, "target", body.Target)
		writeError(w, http.StatusServiceUnavailable, "agent_vm_media_disconnect_failed", agent.UserFacingErrorMessage(err))
		return
	}
	session := currentSession(req)
	task, err := r.store.CreateTask(req.Context(), "vm.media.disconnect", "completed", "virtual_machine", id, map[string]any{"vm": vm.Name, "agent": agentRecord.Name, "target": body.Target}, session.User.ID, "")
	if err != nil {
		r.logger.Warn("create vm media disconnect task failed", "error", err)
	}
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "vm.media.disconnect", "virtual_machine", id, repository.ClientIP(req), map[string]any{"name": vm.Name, "agent": agentRecord.Name, "target": body.Target})
	r.syncVMRuntimeAsync(agentRecord.ID, token, id, vm.Name)
	writeJSON(w, http.StatusOK, map[string]any{"config": config, "task": task})
}

func (r *router) handleUpdateVMXML(w http.ResponseWriter, req *http.Request, id string) {
	defer req.Body.Close()
	var body updateVMXMLRequest
	if err := decodeJSONBodyLimit(w, req, 4<<20, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "虚拟机 XML 参数格式不正确")
		return
	}
	body.XML = strings.TrimSpace(body.XML)
	if body.XML == "" {
		writeError(w, http.StatusBadRequest, "invalid_vm_xml", "虚拟机 XML 不能为空")
		return
	}
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	if strings.EqualFold(strings.TrimSpace(vm.Status), "running") {
		writeError(w, http.StatusBadRequest, "vm_running", "虚拟机正在运行，无法修改 XML，请先关闭虚拟机后再操作")
		return
	}
	agentRecord, token, ok := r.agentTokenForVM(w, req, vm)
	if !ok {
		return
	}
	client := agent.NewClient(agentRecord.TLSInsecure)
	config, err := client.UpdateVMXML(req.Context(), agent.Config{Endpoint: agentRecord.Endpoint, Token: token, TLSInsecure: agentRecord.TLSInsecure}, vm.Name, agent.VMXMLUpdateRequest{XML: body.XML})
	if err != nil {
		r.logger.Error("agent vm xml update failed", "error", err, "vm", id)
		writeError(w, http.StatusServiceUnavailable, "agent_vm_xml_update_failed", agent.UserFacingErrorMessage(err))
		return
	}
	session := currentSession(req)
	task, err := r.store.CreateTask(req.Context(), "vm.xml.update", "completed", "virtual_machine", id, map[string]any{"vm": vm.Name, "agent": agentRecord.Name}, session.User.ID, "")
	if err != nil {
		r.logger.Warn("create vm xml update task failed", "error", err)
	}
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "vm.xml.update", "virtual_machine", id, repository.ClientIP(req), map[string]any{"name": vm.Name, "agent": agentRecord.Name})
	r.syncVMRuntimeAsync(agentRecord.ID, token, id, vm.Name)
	writeJSON(w, http.StatusOK, map[string]any{"config": config, "task": task})
}

func validateVMRenameTarget(ctx context.Context, client *agent.Client, cfg agent.Config, name string) (int, string, string) {
	vms, err := client.ListVMsFast(ctx, cfg)
	if err != nil {
		return http.StatusServiceUnavailable, "agent_vm_rename_precheck_failed", "检查虚拟机名称失败：" + agent.UserFacingErrorMessage(err)
	}
	for _, vm := range vms {
		if vm.Name == name {
			return http.StatusBadRequest, "vm_rename_name_exists", "虚拟机名称已存在，请更换名称"
		}
	}
	return 0, "", ""
}

func (r *router) syncVMRuntimeAsync(agentID string, token string, vmID string, vmName string) {
	go func() {
		if _, err := r.runtime.SyncVMWithToken(context.Background(), agentID, token, vmID, vmName); err != nil {
			r.logger.Warn("async vm sync after config change failed", "error", err, "agent", agentID, "vm", vmID)
			return
		}
		r.runtime.Broadcast("runtime.updated")
	}()
}

func validateVMConfigUpdate(body updateVMConfigRequest) error {
	if body.CurrentCPU <= 0 || body.MaximumCPU <= 0 {
		return errors.New("CPU 分配必须大于 0")
	}
	if body.CurrentCPU > body.MaximumCPU {
		return errors.New("逻辑 CPU 当前分配不能大于最大分配")
	}
	if body.CurrentMemoryMB <= 0 || body.MaximumMemoryMB <= 0 {
		return errors.New("内存分配必须大于 0")
	}
	if body.CurrentMemoryMB > body.MaximumMemoryMB {
		return errors.New("总内存当前分配不能大于最大分配")
	}
	if body.MemoryStatsPeriod < 0 || body.MemoryStatsPeriod > 86400 {
		return errors.New("内存统计周期必须在 0 到 86400 秒之间")
	}
	if len(body.Description) > 2048 {
		return errors.New("描述不能超过 2048 个字符")
	}
	return nil
}
