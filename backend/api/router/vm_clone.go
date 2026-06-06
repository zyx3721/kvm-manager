package router

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/pkg/agent"
)

func (r *router) handleCloneVM(w http.ResponseWriter, req *http.Request, id string) {
	defer req.Body.Close()
	var body cloneVMRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "虚拟机克隆参数格式不正确")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid_vm_clone", "克隆名称不能为空")
		return
	}
	if len(body.Disks) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_vm_clone", "请选择要克隆的磁盘")
		return
	}
	if err := validateVMCloneRequest(body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_vm_clone", err.Error())
		return
	}
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	if strings.EqualFold(strings.TrimSpace(vm.Status), "running") {
		writeError(w, http.StatusBadRequest, "vm_running", "虚拟机正在运行，无法克隆，请先关闭虚拟机后再操作")
		return
	}
	agentRecord, token, ok := r.agentTokenForVM(w, req, vm)
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
	task, err := r.store.CreateTask(req.Context(), "vm.clone", "queued", "virtual_machine", id, map[string]any{"vm": vm.Name, "clone": cloneRequest.Name, "agent": agentRecord.Name, "message": "克隆虚拟机排队中"}, session.User.ID, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_vm_clone_task_failed", "创建虚拟机克隆任务失败")
		return
	}
	r.runVMCloneTask(task.ID, session.User.ID, repository.ClientIP(req), id, vm.Name, agentRecord, cfg, cloneRequest)
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "queued", "task": task})
}

func buildVMCloneAgentRequest(body cloneVMRequest) agent.VMCloneRequest {
	cloneRequest := agent.VMCloneRequest{
		Name:            body.Name,
		Description:     strings.TrimSpace(body.Description),
		Autostart:       body.Autostart,
		CurrentCPU:      body.CurrentCPU,
		MaximumCPU:      body.MaximumCPU,
		CurrentMemoryMB: body.CurrentMemoryMB,
		MaximumMemoryMB: body.MaximumMemoryMB,
		CDROMPolicy:     strings.TrimSpace(body.CDROMPolicy),
	}
	if cloneRequest.Autostart {
		cloneRequest.CDROMPolicy = "disconnect"
	}
	for _, iface := range body.Interfaces {
		cloneRequest.Interfaces = append(cloneRequest.Interfaces, agent.VMCloneInterfaceRequest{
			Name:   strings.TrimSpace(iface.Name),
			MAC:    strings.TrimSpace(iface.MAC),
			Source: strings.TrimSpace(iface.Source),
		})
	}
	for _, disk := range body.Disks {
		cloneRequest.Disks = append(cloneRequest.Disks, agent.VMCloneDiskRequest{
			Name:             strings.TrimSpace(disk.Name),
			Pool:             strings.TrimSpace(disk.Pool),
			SourcePath:       strings.TrimSpace(disk.SourcePath),
			TargetName:       strings.TrimSpace(disk.TargetName),
			PreallocMetadata: disk.PreallocMetadata,
		})
	}
	return cloneRequest
}

func validateVMCloneRequest(body cloneVMRequest) error {
	if len(body.Description) > 2048 {
		return errors.New("描述不能超过 2048 个字符")
	}
	if body.CurrentCPU < 0 || body.MaximumCPU < 0 || body.CurrentMemoryMB < 0 || body.MaximumMemoryMB < 0 {
		return errors.New("CPU 与内存配置不能为负数")
	}
	if body.CurrentCPU > 0 && body.MaximumCPU > 0 && body.CurrentCPU > body.MaximumCPU {
		return errors.New("当前 CPU 不能大于最大 CPU")
	}
	if body.CurrentMemoryMB > 0 && body.MaximumMemoryMB > 0 && body.CurrentMemoryMB > body.MaximumMemoryMB {
		return errors.New("当前内存不能大于最大内存")
	}
	policy := strings.ToLower(strings.TrimSpace(body.CDROMPolicy))
	if policy != "" && policy != "inherit" && policy != "disconnect" {
		return errors.New("介质策略不正确")
	}
	for _, iface := range body.Interfaces {
		if strings.TrimSpace(iface.MAC) == "" || strings.TrimSpace(iface.Source) == "" {
			return errors.New("网络设备 MAC 和网络池不能为空")
		}
	}
	for _, disk := range body.Disks {
		if strings.TrimSpace(disk.Name) == "" || strings.TrimSpace(disk.Pool) == "" || strings.TrimSpace(disk.SourcePath) == "" || strings.TrimSpace(disk.TargetName) == "" {
			return errors.New("磁盘名称、存储池、源路径和目标卷名不能为空")
		}
		sourceExtension := volumeExtension(disk.SourcePath)
		targetExtension := volumeExtension(disk.TargetName)
		if sourceExtension != "" && !strings.EqualFold(sourceExtension, targetExtension) {
			return errors.New("磁盘 " + strings.TrimSpace(disk.Name) + " 的目标卷扩展名必须和源磁盘 " + sourceExtension + " 一致")
		}
	}
	return nil
}

func volumeExtension(name string) string {
	baseName := strings.TrimSpace(name)
	if index := strings.LastIndexAny(baseName, `/\`); index >= 0 {
		baseName = baseName[index+1:]
	}
	if dotIndex := strings.LastIndex(baseName, "."); dotIndex > 0 {
		return baseName[dotIndex:]
	}
	return ""
}

func validateVMCloneTargets(ctx context.Context, client *agent.Client, cfg agent.Config, request agent.VMCloneRequest) (int, string, string) {
	if status, code, message := validateRequestedHostResources(ctx, client, cfg, request.MaximumCPU, request.MaximumMemoryMB); message != "" {
		return status, code, message
	}
	vms, err := client.ListVMsFast(ctx, cfg)
	if err != nil {
		return http.StatusServiceUnavailable, "agent_vm_clone_precheck_failed", "检查克隆虚拟机名称失败：" + agent.UserFacingErrorMessage(err)
	}
	for _, vm := range vms {
		if vm.Name == request.Name {
			return http.StatusBadRequest, "vm_clone_name_exists", "克隆名称已存在，请更换克隆名称"
		}
	}
	targetsByPool := make(map[string]map[string]bool)
	for _, disk := range request.Disks {
		pool := strings.TrimSpace(disk.Pool)
		targetName := strings.TrimSpace(disk.TargetName)
		if targetsByPool[pool] == nil {
			targetsByPool[pool] = make(map[string]bool)
		}
		if targetsByPool[pool][targetName] {
			return http.StatusBadRequest, "vm_clone_disk_target_duplicated", "存储池 " + pool + " 中的目标卷名重复：" + targetName
		}
		targetsByPool[pool][targetName] = true
	}
	for pool, targets := range targetsByPool {
		volumes, err := client.ListStorageVolumes(ctx, cfg, pool)
		if err != nil {
			return http.StatusServiceUnavailable, "agent_storage_volume_precheck_failed", "检查存储池 " + pool + " 中的卷失败：" + agent.UserFacingErrorMessage(err)
		}
		for _, volume := range volumes {
			if targets[volume.Name] {
				return http.StatusBadRequest, "vm_clone_disk_target_exists", "存储池 " + pool + " 中已存在卷 " + volume.Name + "，请更换存储配置名称"
			}
		}
	}
	return 0, "", ""
}
