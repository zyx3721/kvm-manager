package router

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/pkg/agent"
	"kvm-manager/backend/pkg/tokencrypto"
)

type createVMRequest struct {
	AgentID          string                `json:"agentId"`
	CreateMode       string                `json:"createMode"`
	Name             string                `json:"name"`
	Description      string                `json:"description"`
	Autostart        bool                  `json:"autostart"`
	CurrentCPU       int                   `json:"currentCpu"`
	MaximumCPU       int                   `json:"maximumCpu"`
	CurrentMemoryMB  int64                 `json:"currentMemoryMB"`
	MaximumMemoryMB  int64                 `json:"maximumMemoryMB"`
	CPUModel         string                `json:"cpuModel"`
	OSType           string                `json:"osType"`
	Disks            []createVMDiskRequest `json:"disks"`
	DiskName         string                `json:"diskName"`
	DiskPool         string                `json:"diskPool"`
	DiskFormat       string                `json:"diskFormat"`
	DiskBus          string                `json:"diskBus"`
	DiskCapacityGB   int64                 `json:"diskCapacityGB"`
	PreallocMetadata bool                  `json:"preallocMetadata"`
	ISOPath          string                `json:"isoPath"`
	ISOBus           string                `json:"isoBus"`
	NetworkSource    string                `json:"networkSource"`
	NetworkModel     string                `json:"networkModel"`
	Graphics         string                `json:"graphics"`
	ConsolePassword  string                `json:"consolePassword"`
	BootFirmware     string                `json:"bootFirmware"`
	Template         createVMTemplate      `json:"template"`
	XML              string                `json:"xml"`
}

type createVMTemplate struct {
	SourcePool       string `json:"sourcePool"`
	SourceName       string `json:"sourceName"`
	TargetPool       string `json:"targetPool"`
	TargetName       string `json:"targetName"`
	Bus              string `json:"bus"`
	Format           string `json:"format"`
	Convert          bool   `json:"convert"`
	PreallocMetadata bool   `json:"preallocMetadata"`
}

type createVMDiskRequest struct {
	Name             string `json:"name"`
	Pool             string `json:"pool"`
	Format           string `json:"format"`
	Bus              string `json:"bus"`
	CapacityGB       int64  `json:"capacityGB"`
	PreallocMetadata bool   `json:"preallocMetadata"`
}

type migrateVMRequest struct {
	TargetAgentID  string `json:"targetAgentId"`
	DestinationURI string `json:"destinationUri"`
	Live           bool   `json:"live"`
	CopyDisks      bool   `json:"copyDisks"`
	Persistent     bool   `json:"persistent"`
	UndefineSource bool   `json:"undefineSource"`
	AutoConverge   bool   `json:"autoConverge"`
	PostCopy       bool   `json:"postCopy"`
}

type migrationSSHKeySetupRequest struct {
	TargetAgentID  string `json:"targetAgentId"`
	DestinationURI string `json:"destinationUri"`
	Username       string `json:"username"`
	Password       string `json:"password"`
}

type migrationHostnameSetupRequest struct {
	TargetAgentID  string `json:"targetAgentId"`
	DestinationURI string `json:"destinationUri"`
	Hostname       string `json:"hostname"`
}

func (r *router) handleCreateVM(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "vms.create") {
		return
	}
	defer req.Body.Close()
	var body createVMRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "创建虚拟机参数格式不正确")
		return
	}
	if strings.TrimSpace(body.XML) != "" {
		name, err := domainNameFromXML(body.XML)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_vm_create", err.Error())
			return
		}
		body.Name = name
	}
	if err := validateCreateVMRequest(body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_vm_create", err.Error())
		return
	}
	agentRecord, token, ok := r.agentTokenForID(w, req, strings.TrimSpace(body.AgentID))
	if !ok {
		return
	}
	createRequest := agent.VMCreateRequest{
		CreateMode:      normalizedCreateMode(body),
		Name:            strings.TrimSpace(body.Name),
		Description:     strings.TrimSpace(body.Description),
		Autostart:       body.Autostart,
		CurrentCPU:      body.CurrentCPU,
		MaximumCPU:      body.MaximumCPU,
		CurrentMemoryMB: body.CurrentMemoryMB,
		MaximumMemoryMB: body.MaximumMemoryMB,
		CPUModel:        strings.TrimSpace(body.CPUModel),
		OSType:          strings.TrimSpace(body.OSType),
		ISOPath:         strings.TrimSpace(body.ISOPath),
		ISOBus:          strings.TrimSpace(body.ISOBus),
		NetworkSource:   strings.TrimSpace(body.NetworkSource),
		NetworkModel:    strings.TrimSpace(body.NetworkModel),
		Graphics:        strings.TrimSpace(body.Graphics),
		ConsolePassword: strings.TrimSpace(body.ConsolePassword),
		BootFirmware:    strings.TrimSpace(body.BootFirmware),
		Template:        normalizeCreateVMTemplate(body.Template),
		XML:             strings.TrimSpace(body.XML),
	}
	if createRequest.XML == "" && createRequest.CreateMode != "template" {
		disks := normalizeCreateVMDisks(body)
		createRequest.Disks = disks
		createRequest.DiskName = disks[0].Name
		createRequest.DiskPool = disks[0].Pool
		createRequest.DiskFormat = disks[0].Format
		createRequest.DiskBus = disks[0].Bus
		createRequest.DiskCapacityGB = disks[0].CapacityGB
		createRequest.PreallocMetadata = disks[0].PreallocMetadata
	}
	client := agent.NewClient(agentRecord.TLSInsecure)
	cfg := agent.Config{Endpoint: agentRecord.Endpoint, Token: token, TLSInsecure: agentRecord.TLSInsecure}
	if status, code, message := validateVMCreateTargets(req.Context(), client, cfg, createRequest, createRequest.XML == ""); message != "" {
		writeError(w, status, code, message)
		return
	}
	session := currentSession(req)
	task, err := r.store.CreateTask(req.Context(), "vm.create", "queued", "agent", agentRecord.ID, map[string]any{"vm": createRequest.Name, "agent": agentRecord.Name, "message": "创建虚拟机排队中"}, session.User.ID, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_vm_task_failed", "创建虚拟机任务失败")
		return
	}
	r.runVMCreateTask(task.ID, session.User.ID, repository.ClientIP(req), agentRecord, cfg, createRequest)
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "queued", "task": task})
}

func (r *router) handleMigrateVM(w http.ResponseWriter, req *http.Request, id string) {
	defer req.Body.Close()
	var body migrateVMRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "迁移虚拟机参数格式不正确")
		return
	}
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	if err := validateMigrateVMRequest(body, vm); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_vm_migrate", err.Error())
		return
	}
	sourceAgent, sourceToken, ok := r.agentTokenForVM(w, req, vm)
	if !ok {
		return
	}
	targetAgent, targetToken, ok := r.agentTokenForID(w, req, strings.TrimSpace(body.TargetAgentID))
	if !ok {
		return
	}
	destinationURI := strings.TrimSpace(body.DestinationURI)
	if destinationURI == "" {
		destinationURI = defaultMigrationURI(targetAgent)
	}
	migrateRequest := agent.VMMigrateRequest{
		DestinationURI: destinationURI,
		Live:           body.Live,
		CopyDisks:      body.CopyDisks,
		Persistent:     body.Persistent,
		UndefineSource: body.UndefineSource,
		AutoConverge:   body.AutoConverge,
		PostCopy:       body.PostCopy,
	}
	if _, err := url.ParseRequestURI(migrateRequest.DestinationURI); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_vm_migrate", "迁移 URI 不正确")
		return
	}
	session := currentSession(req)
	task, err := r.store.CreateTask(req.Context(), "vm.migrate", "queued", "virtual_machine", id, map[string]any{"vm": vm.Name, "sourceAgent": sourceAgent.Name, "targetAgent": targetAgent.Name, "message": "迁移虚拟机排队中"}, session.User.ID, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_vm_migrate_task_failed", "创建虚拟机迁移任务失败")
		return
	}
	r.runVMMigrateTask(task.ID, session.User.ID, repository.ClientIP(req), vm, sourceAgent, sourceToken, targetAgent, targetToken, migrateRequest)
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "queued", "task": task})
}

func (r *router) handlePrecheckVMMigration(w http.ResponseWriter, req *http.Request, id string) {
	defer req.Body.Close()
	var body migrateVMRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "迁移虚拟机参数格式不正确")
		return
	}
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	if err := validateMigrateVMRequest(body, vm); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_vm_migrate", err.Error())
		return
	}
	sourceAgent, sourceToken, ok := r.agentTokenForVM(w, req, vm)
	if !ok {
		return
	}
	targetAgent, targetToken, ok := r.agentTokenForID(w, req, strings.TrimSpace(body.TargetAgentID))
	if !ok {
		return
	}
	destinationURI := strings.TrimSpace(body.DestinationURI)
	if destinationURI == "" {
		destinationURI = defaultMigrationURI(targetAgent)
	}
	if _, err := url.ParseRequestURI(destinationURI); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_vm_migrate", "迁移 URI 不正确")
		return
	}
	migrateRequest := agent.VMMigrateRequest{
		DestinationURI: destinationURI,
		Live:           body.Live,
		CopyDisks:      body.CopyDisks,
		Persistent:     body.Persistent,
		UndefineSource: body.UndefineSource,
		AutoConverge:   body.AutoConverge,
		PostCopy:       body.PostCopy,
	}
	sourceCfg := agent.Config{Endpoint: sourceAgent.Endpoint, Token: sourceToken, TLSInsecure: sourceAgent.TLSInsecure}
	targetCfg := agent.Config{Endpoint: targetAgent.Endpoint, Token: targetToken, TLSInsecure: targetAgent.TLSInsecure}
	report := runVMMigrationPrecheck(
		req.Context(),
		sourceCfg,
		sourceAgent.TLSInsecure,
		targetCfg,
		targetAgent.TLSInsecure,
		vm,
		migrateRequest,
	)
	writeJSON(w, http.StatusOK, report)
}

func (r *router) handleSetupVMMigrationSSHKey(w http.ResponseWriter, req *http.Request, id string) {
	defer req.Body.Close()
	var body migrationSSHKeySetupRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "迁移 SSH 免密配置参数格式不正确")
		return
	}
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	sourceAgent, sourceToken, ok := r.agentTokenForVM(w, req, vm)
	if !ok {
		return
	}
	targetAgent, _, ok := r.agentTokenForID(w, req, strings.TrimSpace(body.TargetAgentID))
	if !ok {
		return
	}
	destinationURI := strings.TrimSpace(body.DestinationURI)
	if destinationURI == "" {
		destinationURI = defaultMigrationURI(targetAgent)
	}
	if strings.TrimSpace(body.Password) == "" {
		writeError(w, http.StatusBadRequest, "missing_ssh_password", "请输入目标宿主机 SSH 密码")
		return
	}
	sourceCfg := agent.Config{Endpoint: sourceAgent.Endpoint, Token: sourceToken, TLSInsecure: sourceAgent.TLSInsecure}
	result, err := agent.NewClient(sourceAgent.TLSInsecure).SetupMigrationSSHKey(req.Context(), sourceCfg, agent.MigrationSSHKeySetupRequest{
		DestinationURI: destinationURI,
		Username:       strings.TrimSpace(body.Username),
		Password:       body.Password,
	})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "migration_ssh_key_failed", "配置迁移 SSH 免密失败："+agent.UserFacingErrorMessage(err))
		return
	}
	if !result.OK {
		writeError(w, http.StatusBadRequest, "migration_ssh_key_failed", firstNonEmptyString(result.Message, "配置迁移 SSH 免密失败"))
		return
	}
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "vm.migrate.ssh_key.setup", "virtual_machine", id, repository.ClientIP(req), map[string]any{"vm": vm.Name, "sourceAgent": sourceAgent.Name, "targetAgent": targetAgent.Name, "destinationUri": destinationURI, "username": strings.TrimSpace(body.Username)})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "result": result})
}

func (r *router) handleSetupVMMigrationHostname(w http.ResponseWriter, req *http.Request, id string) {
	defer req.Body.Close()
	var body migrationHostnameSetupRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "迁移目标主机名配置参数格式不正确")
		return
	}
	vm, ok := r.runtime.GetVM(id)
	if !ok {
		writeError(w, http.StatusNotFound, "vm_not_found", "虚拟机不存在")
		return
	}
	sourceAgent, sourceToken, ok := r.agentTokenForVM(w, req, vm)
	if !ok {
		return
	}
	targetAgent, _, ok := r.agentTokenForID(w, req, strings.TrimSpace(body.TargetAgentID))
	if !ok {
		return
	}
	destinationURI := strings.TrimSpace(body.DestinationURI)
	if destinationURI == "" {
		destinationURI = defaultMigrationURI(targetAgent)
	}
	if _, err := url.ParseRequestURI(destinationURI); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_vm_migrate", "迁移 URI 不正确")
		return
	}
	if strings.TrimSpace(body.Hostname) == "" {
		writeError(w, http.StatusBadRequest, "missing_hostname", "请输入目标宿主机主机名")
		return
	}
	sourceCfg := agent.Config{Endpoint: sourceAgent.Endpoint, Token: sourceToken, TLSInsecure: sourceAgent.TLSInsecure}
	result, err := agent.NewClient(sourceAgent.TLSInsecure).SetupMigrationHostname(req.Context(), sourceCfg, agent.MigrationHostnameSetupRequest{
		DestinationURI: destinationURI,
		Hostname:       strings.TrimSpace(body.Hostname),
	})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "migration_hostname_failed", "配置迁移目标主机名失败："+agent.UserFacingErrorMessage(err))
		return
	}
	if !result.OK {
		writeError(w, http.StatusBadRequest, "migration_hostname_failed", firstNonEmptyString(result.Message, "配置迁移目标主机名失败"))
		return
	}
	_ = r.store.WriteAudit(req.Context(), currentSession(req).User.ID, "vm.migrate.hostname.setup", "virtual_machine", id, repository.ClientIP(req), map[string]any{"vm": vm.Name, "sourceAgent": sourceAgent.Name, "targetAgent": targetAgent.Name, "destinationUri": destinationURI, "hostname": strings.TrimSpace(body.Hostname)})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "result": result})
}

func (r *router) agentTokenForID(w http.ResponseWriter, req *http.Request, id string) (domain.Agent, string, bool) {
	if id == "" {
		writeError(w, http.StatusBadRequest, "agent_required", "请选择宿主机")
		return domain.Agent{}, "", false
	}
	agentRecord, err := r.store.GetAgent(req.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "agent_not_found", "宿主机 Agent 不存在")
			return domain.Agent{}, "", false
		}
		writeError(w, http.StatusInternalServerError, "get_agent_failed", "读取 Agent 失败")
		return domain.Agent{}, "", false
	}
	token, err := tokencrypto.Open(r.cfg.JWT.Secret, agentRecord.TokenCiphertext)
	if err != nil || strings.TrimSpace(token) == "" {
		r.logger.Error("open agent token failed", "error", err, "agent", agentRecord.ID)
		writeError(w, http.StatusBadRequest, "agent_token_unavailable", "Agent 令牌不可用于执行操作，请重新保存 Agent")
		return domain.Agent{}, "", false
	}
	return agentRecord, token, true
}

func validateCreateVMRequest(body createVMRequest) error {
	if strings.TrimSpace(body.AgentID) == "" {
		return errors.New("请选择宿主机")
	}
	if strings.TrimSpace(body.Name) == "" && strings.TrimSpace(body.XML) == "" {
		return errors.New("虚拟机名称不能为空")
	}
	if len(body.Description) > 2048 {
		return errors.New("描述不能超过 2048 个字符")
	}
	if strings.TrimSpace(body.XML) != "" {
		return nil
	}
	if body.CurrentCPU <= 0 || body.MaximumCPU <= 0 || body.CurrentCPU > body.MaximumCPU {
		return errors.New("CPU 配置不正确")
	}
	if body.CurrentMemoryMB <= 0 || body.MaximumMemoryMB <= 0 || body.CurrentMemoryMB > body.MaximumMemoryMB {
		return errors.New("内存配置不正确")
	}
	switch normalizedCreateMode(body) {
	case "blank":
		if err := validateCreateVMDisks(normalizeCreateVMDisks(body)); err != nil {
			return err
		}
	case "template":
		if err := validateCreateVMTemplate(normalizeCreateVMTemplate(body.Template)); err != nil {
			return err
		}
	default:
		return errors.New("创建模式不支持")
	}
	if strings.TrimSpace(body.NetworkSource) == "" {
		return errors.New("请选择网络池或桥接网络")
	}
	if strings.TrimSpace(body.ISOBus) != "" && !supportedVMCDROMBus(body.ISOBus) {
		return errors.New("ISO 总线类型不支持")
	}
	if strings.TrimSpace(body.NetworkModel) != "" && !supportedVMNetworkModel(body.NetworkModel) {
		return errors.New("网卡模型不支持")
	}
	if strings.TrimSpace(body.Graphics) != "" && !strings.EqualFold(strings.TrimSpace(body.Graphics), "vnc") {
		return errors.New("当前仅支持 VNC 控制台")
	}
	return nil
}

func normalizedCreateMode(body createVMRequest) string {
	if strings.TrimSpace(body.XML) != "" {
		return "xml"
	}
	mode := strings.ToLower(strings.TrimSpace(body.CreateMode))
	if mode == "" {
		return "blank"
	}
	return mode
}

func normalizeCreateVMTemplate(template createVMTemplate) agent.VMCreateTemplate {
	return agent.VMCreateTemplate{
		SourcePool:       strings.TrimSpace(template.SourcePool),
		SourceName:       strings.TrimSpace(template.SourceName),
		TargetPool:       strings.TrimSpace(template.TargetPool),
		TargetName:       strings.TrimSpace(template.TargetName),
		Bus:              strings.TrimSpace(template.Bus),
		Format:           strings.ToLower(strings.TrimSpace(template.Format)),
		Convert:          template.Convert,
		PreallocMetadata: template.PreallocMetadata,
	}
}

func validateCreateVMTemplate(template agent.VMCreateTemplate) error {
	if template.SourcePool == "" || template.SourceName == "" || template.TargetPool == "" || template.TargetName == "" || template.Bus == "" {
		return errors.New("请完整填写模板磁盘配置")
	}
	if strings.ContainsAny(template.SourceName, `/\`) || strings.ContainsAny(template.TargetName, `/\`) {
		return errors.New("模板磁盘卷名称不能包含路径分隔符")
	}
	if !supportedVMDeviceDiskFormat(template.Format) {
		return errors.New("模板磁盘格式不支持")
	}
	if !vmDeviceDiskNameMatchesFormat(template.TargetName, template.Format) {
		return errors.New("目标磁盘卷名称扩展名必须与格式一致，qcow2 使用 .qcow2，其他格式使用 .img")
	}
	if !supportedVMCreateDiskBus(template.Bus) {
		return errors.New("模板磁盘总线不支持")
	}
	return nil
}

func supportedVMCreateDiskBus(bus string) bool {
	switch strings.ToLower(strings.TrimSpace(bus)) {
	case "virtio", "sata", "scsi", "ide":
		return true
	default:
		return false
	}
}

func supportedVMCDROMBus(bus string) bool {
	switch strings.ToLower(strings.TrimSpace(bus)) {
	case "", "sata", "ide", "scsi", "usb":
		return true
	default:
		return false
	}
}

func supportedVMNetworkModel(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "", "virtio", "e1000", "e1000e", "rtl8139", "vmxnet3":
		return true
	default:
		return false
	}
}

func normalizeCreateVMDisks(body createVMRequest) []agent.VMCreateDiskRequest {
	disks := make([]agent.VMCreateDiskRequest, 0, len(body.Disks))
	if len(body.Disks) == 0 {
		body.Disks = []createVMDiskRequest{{
			Name:             body.DiskName,
			Pool:             body.DiskPool,
			Format:           body.DiskFormat,
			Bus:              body.DiskBus,
			CapacityGB:       body.DiskCapacityGB,
			PreallocMetadata: body.PreallocMetadata,
		}}
	}
	for _, disk := range body.Disks {
		disks = append(disks, agent.VMCreateDiskRequest{
			Name:             strings.TrimSpace(disk.Name),
			Pool:             strings.TrimSpace(disk.Pool),
			Format:           strings.ToLower(strings.TrimSpace(disk.Format)),
			Bus:              strings.TrimSpace(disk.Bus),
			CapacityGB:       disk.CapacityGB,
			PreallocMetadata: disk.PreallocMetadata,
		})
	}
	return disks
}

func validateCreateVMDisks(disks []agent.VMCreateDiskRequest) error {
	if len(disks) == 0 {
		return errors.New("请至少配置一块磁盘")
	}
	seen := map[string]struct{}{}
	systemDisk := disks[0]
	for index, disk := range disks {
		label := "磁盘"
		if index == 0 {
			label = "系统盘"
		}
		if disk.Name == "" || disk.Pool == "" || disk.Format == "" || disk.Bus == "" || disk.CapacityGB <= 0 {
			return errors.New(label + "配置不完整")
		}
		if !supportedVMDeviceDiskFormat(disk.Format) {
			return errors.New(label + "格式不支持")
		}
		if !vmDeviceDiskNameMatchesFormat(disk.Name, disk.Format) {
			return errors.New(label + "名称扩展名必须与格式一致，qcow2 使用 .qcow2，其他格式使用 .img")
		}
		if strings.ContainsAny(disk.Name, `/\`) {
			return errors.New(label + "名称不能包含路径分隔符")
		}
		if index > 0 && !createVMDiskSettingsMatchSystem(systemDisk, disk) {
			return errors.New(label + "需与系统盘使用相同的存储池、磁盘格式、磁盘总线和 Metadata 配置")
		}
		key := strings.ToLower(disk.Pool + "/" + disk.Name)
		if _, ok := seen[key]; ok {
			return errors.New("磁盘卷名称不能重复")
		}
		seen[key] = struct{}{}
	}
	return nil
}

func createVMDiskSettingsMatchSystem(systemDisk agent.VMCreateDiskRequest, disk agent.VMCreateDiskRequest) bool {
	return strings.EqualFold(systemDisk.Pool, disk.Pool) &&
		strings.EqualFold(systemDisk.Format, disk.Format) &&
		strings.EqualFold(systemDisk.Bus, disk.Bus) &&
		systemDisk.PreallocMetadata == disk.PreallocMetadata
}

func validateMigrateVMRequest(body migrateVMRequest, vm domain.VirtualMachine) error {
	if strings.TrimSpace(body.TargetAgentID) == "" {
		return errors.New("请选择目标宿主机")
	}
	if strings.TrimSpace(body.TargetAgentID) == vm.HostID {
		return errors.New("目标宿主机不能与源宿主机相同")
	}
	if !body.Live && strings.EqualFold(vm.Status, "running") {
		return errors.New("运行中的虚拟机请选择热迁移，或先关闭虚拟机后再冷迁移")
	}
	if body.Live && !strings.EqualFold(vm.Status, "running") {
		return errors.New("非运行状态的虚拟机请选择冷迁移")
	}
	return nil
}

func validateVMCreateTargets(ctx context.Context, client *agent.Client, cfg agent.Config, request agent.VMCreateRequest, checkResources bool) (int, string, string) {
	if checkResources {
		if status, code, message := validateRequestedHostResources(ctx, client, cfg, request.MaximumCPU, request.MaximumMemoryMB); message != "" {
			return status, code, message
		}
	}
	vms, err := client.ListVMsFast(ctx, cfg)
	if err != nil {
		return http.StatusServiceUnavailable, "agent_vm_create_precheck_failed", "检查虚拟机名称失败：" + agent.UserFacingErrorMessage(err)
	}
	for _, vm := range vms {
		if vm.Name == request.Name {
			return http.StatusBadRequest, "vm_create_name_exists", "虚拟机名称已存在，请更换名称"
		}
	}
	if strings.TrimSpace(request.XML) != "" {
		return 0, "", ""
	}
	if request.CreateMode == "template" {
		return validateVMCreateTemplateTargets(ctx, client, cfg, request.Template)
	}
	disksByPool := map[string][]agent.VMCreateDiskRequest{}
	for _, disk := range request.Disks {
		disksByPool[disk.Pool] = append(disksByPool[disk.Pool], disk)
	}
	for pool, disks := range disksByPool {
		volumes, err := client.ListStorageVolumes(ctx, cfg, pool)
		if err != nil {
			return http.StatusServiceUnavailable, "agent_storage_volume_precheck_failed", "检查存储池卷失败：" + agent.UserFacingErrorMessage(err)
		}
		existing := map[string]struct{}{}
		for _, volume := range volumes {
			existing[volume.Name] = struct{}{}
		}
		for _, disk := range disks {
			if _, ok := existing[disk.Name]; ok {
				return http.StatusBadRequest, "vm_create_disk_exists", "存储池 " + pool + " 中已存在卷 " + disk.Name + "，请更换磁盘卷名称"
			}
		}
	}
	return 0, "", ""
}

func validateVMCreateTemplateTargets(ctx context.Context, client *agent.Client, cfg agent.Config, template agent.VMCreateTemplate) (int, string, string) {
	sourceVolumes, err := client.ListStorageVolumes(ctx, cfg, template.SourcePool)
	if err != nil {
		return http.StatusServiceUnavailable, "agent_template_volume_precheck_failed", "检查模板存储池失败：" + agent.UserFacingErrorMessage(err)
	}
	sourceExists := false
	for _, volume := range sourceVolumes {
		if volume.Name != template.SourceName {
			continue
		}
		sourceExists = true
		if !isTemplateStorageVolume(volume) {
			return http.StatusBadRequest, "vm_create_template_unsupported", "模板文件仅支持 qcow2、img、raw、qcow、qed 格式"
		}
		break
	}
	if !sourceExists {
		return http.StatusBadRequest, "vm_create_template_not_found", "模板文件不存在，请重新选择"
	}
	targetVolumes := sourceVolumes
	if template.TargetPool != template.SourcePool {
		targetVolumes, err = client.ListStorageVolumes(ctx, cfg, template.TargetPool)
		if err != nil {
			return http.StatusServiceUnavailable, "agent_template_target_precheck_failed", "检查目标存储池失败：" + agent.UserFacingErrorMessage(err)
		}
	}
	for _, volume := range targetVolumes {
		if strings.EqualFold(strings.TrimSpace(volume.Name), template.TargetName) {
			return http.StatusBadRequest, "vm_create_disk_exists", "存储池 " + template.TargetPool + " 中已存在卷 " + template.TargetName + "，请更换磁盘卷名称"
		}
	}
	return 0, "", ""
}

func isTemplateStorageVolume(volume agent.StorageVolume) bool {
	if !volume.CloneSupported {
		return false
	}
	format := strings.ToLower(strings.TrimSpace(volume.Format))
	if format == "" {
		format = strings.ToLower(strings.TrimSpace(volume.Type))
	}
	switch format {
	case "qcow2", "raw", "qcow", "qed":
		return true
	}
	name := strings.ToLower(volume.Name)
	path := strings.ToLower(volume.Path)
	for _, extension := range []string{".qcow2", ".img", ".raw", ".qcow", ".qed"} {
		if strings.HasSuffix(name, extension) || strings.HasSuffix(path, extension) {
			return true
		}
	}
	return false
}

func validateRequestedHostResources(ctx context.Context, client *agent.Client, cfg agent.Config, maximumCPU int, maximumMemoryMB int64) (int, string, string) {
	if maximumCPU <= 0 && maximumMemoryMB <= 0 {
		return 0, "", ""
	}
	info, err := client.HostInfo(ctx, cfg)
	if err != nil {
		return http.StatusServiceUnavailable, "agent_host_resource_precheck_failed", "检查宿主机资源失败：" + agent.UserFacingErrorMessage(err)
	}
	return validateHostResourceLimits(info.CPUCores, info.MemoryBytes, maximumCPU, maximumMemoryMB)
}

func validateHostResourceLimits(hostCPU int, hostMemoryBytes int64, maximumCPU int, maximumMemoryMB int64) (int, string, string) {
	if hostCPU > 0 && maximumCPU > hostCPU {
		return http.StatusBadRequest, "vm_resource_exceeds_host", "最大 CPU 不能超过宿主机逻辑 CPU"
	}
	if hostMemoryBytes > 0 && maximumMemoryMB*1024*1024 > hostMemoryBytes {
		return http.StatusBadRequest, "vm_resource_exceeds_host", "最大内存不能超过宿主机总内存"
	}
	return 0, "", ""
}

func defaultMigrationURI(agentRecord domain.Agent) string {
	parsed, err := url.Parse(agentRecord.Endpoint)
	if err != nil {
		return ""
	}
	host := parsed.Hostname()
	if host == "" {
		host, _, _ = net.SplitHostPort(parsed.Host)
	}
	if host == "" {
		return ""
	}
	return "qemu+ssh://" + host + "/system"
}
