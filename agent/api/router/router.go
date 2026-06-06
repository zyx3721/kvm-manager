package router

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"kvm-manager/agent/internal/kvm"
	"kvm-manager/agent/internal/security"
)

var snapshotNamePattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,80}$`)

type Router struct {
	provider kvm.Provider
	logger   *slog.Logger
	auth     security.Authenticator
}

func New(provider kvm.Provider, logger *slog.Logger, auth security.Authenticator) http.Handler {
	r := &Router{provider: provider, logger: logger, auth: auth}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", r.handleHealth)
	mux.Handle("GET /v1/host", auth.Middleware(http.HandlerFunc(r.handleHost)))
	mux.Handle("GET /v1/host/interfaces", auth.Middleware(http.HandlerFunc(r.handleHostInterfaces)))
	mux.Handle("POST /v1/host/interfaces", auth.Middleware(http.HandlerFunc(r.handleHostInterfaces)))
	mux.Handle("GET /v1/host/interfaces/", auth.Middleware(http.HandlerFunc(r.handleHostInterfaceRoute)))
	mux.Handle("PUT /v1/host/interfaces/", auth.Middleware(http.HandlerFunc(r.handleHostInterfaceRoute)))
	mux.Handle("DELETE /v1/host/interfaces/", auth.Middleware(http.HandlerFunc(r.handleHostInterfaceRoute)))
	mux.Handle("GET /v1/vms", auth.Middleware(http.HandlerFunc(r.handleListVMs)))
	mux.Handle("POST /v1/vms", auth.Middleware(http.HandlerFunc(r.handleCreateVM)))
	mux.Handle("GET /v1/storage-pools", auth.Middleware(http.HandlerFunc(r.handleStoragePools)))
	mux.Handle("POST /v1/storage-pools", auth.Middleware(http.HandlerFunc(r.handleStoragePools)))
	mux.Handle("GET /v1/storage-pools/", auth.Middleware(http.HandlerFunc(r.handleStoragePoolRoute)))
	mux.Handle("POST /v1/storage-pools/", auth.Middleware(http.HandlerFunc(r.handleStoragePoolRoute)))
	mux.Handle("PUT /v1/storage-pools/", auth.Middleware(http.HandlerFunc(r.handleStoragePoolRoute)))
	mux.Handle("DELETE /v1/storage-pools/", auth.Middleware(http.HandlerFunc(r.handleStoragePoolRoute)))
	mux.Handle("GET /v1/network-pools", auth.Middleware(http.HandlerFunc(r.handleNetworkPools)))
	mux.Handle("POST /v1/network-pools", auth.Middleware(http.HandlerFunc(r.handleNetworkPools)))
	mux.Handle("PUT /v1/network-pools/", auth.Middleware(http.HandlerFunc(r.handleNetworkPoolRoute)))
	mux.Handle("DELETE /v1/network-pools/", auth.Middleware(http.HandlerFunc(r.handleNetworkPoolRoute)))
	mux.Handle("POST /v1/migration/check", auth.Middleware(http.HandlerFunc(r.handleMigrationCheck)))
	mux.Handle("POST /v1/migration/ssh-key", auth.Middleware(http.HandlerFunc(r.handleMigrationSSHKey)))
	mux.Handle("POST /v1/migration/hostname", auth.Middleware(http.HandlerFunc(r.handleMigrationHostname)))
	mux.Handle("GET /v1/vms/", auth.Middleware(http.HandlerFunc(r.handleVMRoute)))
	mux.Handle("PUT /v1/vms/", auth.Middleware(http.HandlerFunc(r.handleVMRoute)))
	mux.Handle("POST /v1/vms/", auth.Middleware(http.HandlerFunc(r.handleVMRoute)))
	mux.Handle("DELETE /v1/vms/", auth.Middleware(http.HandlerFunc(r.handleVMRoute)))
	return r.withRequestLog(mux)
}

func (r *Router) handleStoragePools(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		items, err := r.provider.ListStoragePools()
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "list_storage_pools_failed", "读取存储池失败")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
	case http.MethodPost:
		defer req.Body.Close()
		var body kvm.StoragePoolCreateRequest
		if err := decodeJSONBody(w, req, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "存储池参数格式不正确")
			return
		}
		item, err := r.provider.CreateStoragePool(body)
		if err != nil {
			r.logger.Error("storage pool create failed", "error", err)
			writeError(w, http.StatusServiceUnavailable, "storage_pool_create_failed", operationErrorMessage("创建存储池失败", err))
			return
		}
		writeJSON(w, http.StatusOK, item)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
	}
}

func (r *Router) handleStoragePoolRoute(w http.ResponseWriter, req *http.Request) {
	pool, action, ok := parseStoragePoolPath(req.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	if req.Method == http.MethodGet && action == "iso-files" {
		items, err := r.provider.ListISOFiles(pool)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "list_iso_files_failed", "读取 ISO 文件失败")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
		return
	}
	if req.Method == http.MethodGet && action == "volumes" {
		items, err := r.provider.ListStorageVolumes(pool)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "list_storage_volumes_failed", "读取存储卷失败")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
		return
	}
	if req.Method == http.MethodPost && action == "volumes" {
		r.handleCreateStorageVolume(w, req, pool)
		return
	}
	if req.Method == http.MethodPost && action == "volumes/upload" {
		r.handleUploadStorageVolume(w, req, pool)
		return
	}
	if req.Method == http.MethodPost && action == "volumes/clone" {
		r.handleCloneStorageVolume(w, req, pool)
		return
	}
	if req.Method == http.MethodDelete && action == "volumes" {
		r.handleDeleteStorageVolume(w, req, pool)
		return
	}
	if req.Method == http.MethodDelete && action == "delete" {
		r.handleDeleteStoragePool(w, req, pool)
		return
	}
	if req.Method == http.MethodPut && action == "state" {
		r.handleUpdateStoragePoolState(w, req, pool)
		return
	}
	if req.Method == http.MethodPut && action == "autostart" {
		r.handleUpdateStoragePoolAutostart(w, req, pool)
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
}

func (r *Router) handleCreateStorageVolume(w http.ResponseWriter, req *http.Request, poolName string) {
	defer req.Body.Close()
	var body kvm.StorageVolumeCreateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "存储卷参数格式不正确")
		return
	}
	volume, err := r.provider.CreateStorageVolume(poolName, body)
	if err != nil {
		r.logger.Error("storage volume create failed", "pool", poolName, "error", err)
		writeError(w, http.StatusServiceUnavailable, "storage_volume_create_failed", operationErrorMessage("创建存储卷失败", err))
		return
	}
	writeJSON(w, http.StatusOK, volume)
}

func (r *Router) handleUploadStorageVolume(w http.ResponseWriter, req *http.Request, poolName string) {
	if err := req.ParseMultipartForm(64 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "上传文件参数不正确")
		return
	}
	file, header, err := req.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "请选择要上传的文件")
		return
	}
	defer file.Close()
	name := strings.TrimSpace(req.FormValue("name"))
	if name == "" && header != nil {
		name = header.Filename
	}
	request := kvm.StorageVolumeCreateRequest{
		Name:          name,
		Format:        "raw",
		CapacityBytes: header.Size,
	}
	volume, err := r.provider.UploadStorageVolume(poolName, request, file)
	if err != nil {
		r.logger.Error("storage volume upload failed", "pool", poolName, "volume", name, "error", err)
		writeError(w, http.StatusServiceUnavailable, "storage_volume_upload_failed", operationErrorMessage("上传 ISO 失败", err))
		return
	}
	writeJSON(w, http.StatusOK, volume)
}

func (r *Router) handleCloneStorageVolume(w http.ResponseWriter, req *http.Request, poolName string) {
	defer req.Body.Close()
	var body kvm.StorageVolumeCloneRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "克隆存储卷参数格式不正确")
		return
	}
	volume, err := r.provider.CloneStorageVolume(poolName, body)
	if err != nil {
		r.logger.Error("storage volume clone failed", "pool", poolName, "source", body.SourceName, "target", body.Name, "error", err)
		writeError(w, http.StatusServiceUnavailable, "storage_volume_clone_failed", operationErrorMessage("克隆存储卷失败", err))
		return
	}
	writeJSON(w, http.StatusOK, volume)
}

func (r *Router) handleDeleteStorageVolume(w http.ResponseWriter, req *http.Request, poolName string) {
	volumeName := strings.TrimSpace(req.URL.Query().Get("name"))
	if volumeName == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "存储卷名称不能为空")
		return
	}
	if err := r.provider.DeleteStorageVolume(poolName, volumeName); err != nil {
		r.logger.Error("storage volume delete failed", "pool", poolName, "volume", volumeName, "error", err)
		writeError(w, http.StatusServiceUnavailable, "storage_volume_delete_failed", operationErrorMessage("删除存储卷失败", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleDeleteStoragePool(w http.ResponseWriter, req *http.Request, poolName string) {
	if err := r.provider.DeleteStoragePool(poolName); err != nil {
		r.logger.Error("storage pool delete failed", "pool", poolName, "error", err)
		writeError(w, http.StatusServiceUnavailable, "storage_pool_delete_failed", operationErrorMessage("删除存储池失败", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleNetworkPools(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		items, err := r.provider.ListNetworkPools()
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "list_network_pools_failed", "读取网络池失败")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
	case http.MethodPost:
		defer req.Body.Close()
		var body kvm.NetworkPoolCreateRequest
		if err := decodeJSONBody(w, req, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "网络池参数格式不正确")
			return
		}
		item, err := r.provider.CreateNetworkPool(body)
		if err != nil {
			r.logger.Error("network pool create failed", "error", err)
			writeError(w, http.StatusServiceUnavailable, "network_pool_create_failed", operationErrorMessage("创建网络池失败", err))
			return
		}
		writeJSON(w, http.StatusOK, item)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
	}
}

func (r *Router) handleNetworkPoolRoute(w http.ResponseWriter, req *http.Request) {
	pool, action, ok := parseNetworkPoolPath(req.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	if req.Method == http.MethodPut && action == "state" {
		r.handleUpdateNetworkPoolState(w, req, pool)
		return
	}
	if req.Method == http.MethodPut && action == "autostart" {
		r.handleUpdateNetworkPoolAutostart(w, req, pool)
		return
	}
	if req.Method == http.MethodDelete && action == "delete" {
		r.handleDeleteNetworkPool(w, req, pool)
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
}

func (r *Router) handleDeleteNetworkPool(w http.ResponseWriter, req *http.Request, poolName string) {
	if err := r.provider.DeleteNetworkPool(poolName); err != nil {
		r.logger.Error("network pool delete failed", "pool", poolName, "error", err)
		writeError(w, http.StatusServiceUnavailable, "network_pool_delete_failed", operationErrorMessage("删除网络池失败", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleUpdateStoragePoolState(w http.ResponseWriter, req *http.Request, poolName string) {
	defer req.Body.Close()
	var body kvm.PoolStateUpdateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "存储池状态参数格式不正确")
		return
	}
	if err := r.provider.UpdateStoragePoolState(poolName, body); err != nil {
		r.logger.Error("storage pool state update failed", "pool", poolName, "error", err)
		writeError(w, http.StatusServiceUnavailable, "storage_pool_state_update_failed", operationErrorMessage("修改存储池状态失败", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"active": body.Active})
}

func (r *Router) handleUpdateStoragePoolAutostart(w http.ResponseWriter, req *http.Request, poolName string) {
	defer req.Body.Close()
	var body kvm.PoolAutostartUpdateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "存储池自启动参数格式不正确")
		return
	}
	if err := r.provider.UpdateStoragePoolAutostart(poolName, body); err != nil {
		r.logger.Error("storage pool autostart update failed", "pool", poolName, "error", err)
		writeError(w, http.StatusServiceUnavailable, "storage_pool_autostart_update_failed", operationErrorMessage("修改存储池自启动失败", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"autostart": body.Autostart})
}

func (r *Router) handleUpdateNetworkPoolState(w http.ResponseWriter, req *http.Request, poolName string) {
	defer req.Body.Close()
	var body kvm.PoolStateUpdateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "网络池状态参数格式不正确")
		return
	}
	if err := r.provider.UpdateNetworkPoolState(poolName, body); err != nil {
		r.logger.Error("network pool state update failed", "pool", poolName, "error", err)
		writeError(w, http.StatusServiceUnavailable, "network_pool_state_update_failed", operationErrorMessage("修改网络池状态失败", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"active": body.Active})
}

func (r *Router) handleUpdateNetworkPoolAutostart(w http.ResponseWriter, req *http.Request, poolName string) {
	defer req.Body.Close()
	var body kvm.PoolAutostartUpdateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "网络池自启动参数格式不正确")
		return
	}
	if err := r.provider.UpdateNetworkPoolAutostart(poolName, body); err != nil {
		r.logger.Error("network pool autostart update failed", "pool", poolName, "error", err)
		writeError(w, http.StatusServiceUnavailable, "network_pool_autostart_update_failed", operationErrorMessage("修改网络池自启动失败", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"autostart": body.Autostart})
}

func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (r *Router) handleHost(w http.ResponseWriter, req *http.Request) {
	info, err := r.provider.HostInfo()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "host_unavailable", "宿主机信息不可用")
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (r *Router) handleListVMs(w http.ResponseWriter, req *http.Request) {
	var (
		vms []kvm.VirtualMachine
		err error
	)
	if strings.EqualFold(strings.TrimSpace(req.URL.Query().Get("level")), "fast") {
		vms, err = r.provider.ListVMsFast()
	} else {
		vms, err = r.provider.ListVMs()
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "list_vms_failed", "读取虚拟机列表失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": vms, "total": len(vms)})
}

func (r *Router) handleVMRoute(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet {
		if name, ok := parseVMConsolePath(req.URL.Path); ok {
			r.handleConsoleWS(w, req, name)
			return
		}
	}
	if req.Method == http.MethodPost {
		if vmName, snapshotName, snapshotAction, ok := parseSnapshotActionPath(req.URL.Path); ok {
			if err := r.runSnapshotAction(vmName, snapshotName, snapshotAction); err != nil {
				r.logger.Error("snapshot action failed", "vm", vmName, "snapshot", snapshotName, "action", snapshotAction, "error", err)
				writeError(w, http.StatusServiceUnavailable, "snapshot_action_failed", operationErrorMessage(snapshotActionSummary(snapshotAction), err))
				return
			}
			writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "action": snapshotAction, "name": vmName, "snapshot": snapshotName})
			return
		}
	}
	name, action, ok := parseVMPath(req.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	if req.Method == http.MethodGet && action == "snapshots" {
		snapshots, err := r.provider.ListSnapshots(name)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "list_snapshots_failed", "读取快照列表失败")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": snapshots, "total": len(snapshots)})
		return
	}
	if req.Method == http.MethodGet && action == "refresh" {
		vm, err := r.provider.VM(name)
		if err != nil {
			r.logger.Error("vm refresh failed", "vm", name, "error", err)
			writeError(w, http.StatusServiceUnavailable, "vm_refresh_failed", operationErrorMessage("刷新虚拟机信息失败", err))
			return
		}
		writeJSON(w, http.StatusOK, vm)
		return
	}
	if req.Method == http.MethodGet && action == "config" {
		config, err := r.provider.VMConfig(name)
		if err != nil {
			r.logger.Error("vm config failed", "vm", name, "error", err)
			writeError(w, http.StatusServiceUnavailable, "vm_config_failed", "读取虚拟机配置失败")
			return
		}
		writeJSON(w, http.StatusOK, config)
		return
	}
	if req.Method == http.MethodGet && action == "console" {
		info, err := r.provider.ConsoleInfo(name)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "console_info_failed", "读取控制台配置失败")
			return
		}
		writeJSON(w, http.StatusOK, info)
		return
	}
	if req.Method == http.MethodPut && action == "config" {
		r.handleUpdateVMConfig(w, req, name)
		return
	}
	if req.Method == http.MethodPut && action == "console" {
		r.handleUpdateVMConsole(w, req, name)
		return
	}
	if req.Method == http.MethodPut && action == "rename" {
		r.handleRenameVM(w, req, name)
		return
	}
	if req.Method == http.MethodPut && action == "autostart" {
		r.handleUpdateVMAutostart(w, req, name)
		return
	}
	if req.Method == http.MethodPut && action == "media" {
		r.handleConnectVMMedia(w, req, name)
		return
	}
	if req.Method == http.MethodDelete && action == "media" {
		r.handleDisconnectVMMedia(w, req, name)
		return
	}
	if req.Method == http.MethodPut && action == "xml" {
		r.handleUpdateVMXML(w, req, name)
		return
	}
	if req.Method == http.MethodPut && action == "devices" {
		r.handleUpdateVMDevices(w, req, name)
		return
	}
	if req.Method == http.MethodPost && action == "clone" {
		r.handleCloneVM(w, req, name)
		return
	}
	if req.Method == http.MethodPost && action == "migrate" {
		r.handleMigrateVM(w, req, name)
		return
	}
	if req.Method == http.MethodPost && action == "snapshots" {
		r.handleCreateSnapshot(w, req, name)
		return
	}
	if req.Method == http.MethodPost {
		if err := r.runAction(name, action); err != nil {
			writeError(w, http.StatusServiceUnavailable, "vm_action_failed", operationErrorMessage("虚拟机操作失败", err))
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "action": action, "name": name})
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
}

func (r *Router) handleCreateVM(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	var body kvm.VMCreateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "创建虚拟机参数格式不正确")
		return
	}
	config, err := r.provider.CreateVM(body)
	if err != nil {
		r.logger.Error("vm create failed", "name", body.Name, "error", err)
		writeError(w, http.StatusServiceUnavailable, "vm_create_failed", operationErrorMessage("创建虚拟机失败", err))
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (r *Router) handleMigrateVM(w http.ResponseWriter, req *http.Request, name string) {
	defer req.Body.Close()
	var body kvm.VMMigrateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "迁移虚拟机参数格式不正确")
		return
	}
	if err := r.provider.MigrateVM(name, body); err != nil {
		r.logger.Error("vm migrate failed", "vm", name, "destination", body.DestinationURI, "error", err)
		writeError(w, http.StatusServiceUnavailable, "vm_migrate_failed", operationErrorMessage("迁移虚拟机失败", err))
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "action": "migrate", "name": name})
}

func (r *Router) handleMigrationCheck(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	var body kvm.MigrationConnectionCheckRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "迁移通道检测参数格式不正确")
		return
	}
	result, err := r.provider.CheckMigrationConnection(body)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "migration_check_failed", operationErrorMessage("迁移通道检测失败", err))
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (r *Router) handleMigrationSSHKey(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	var body kvm.MigrationSSHKeySetupRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "迁移 SSH 免密配置参数格式不正确")
		return
	}
	result, err := r.provider.SetupMigrationSSHKey(body)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "migration_ssh_key_failed", operationErrorMessage("迁移 SSH 免密配置失败", err))
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (r *Router) handleMigrationHostname(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	var body kvm.MigrationHostnameSetupRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "迁移目标主机名配置参数格式不正确")
		return
	}
	result, err := r.provider.SetupMigrationHostname(body)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "migration_hostname_failed", operationErrorMessage("配置迁移目标主机名失败", err))
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (r *Router) handleCreateSnapshot(w http.ResponseWriter, req *http.Request, vmName string) {
	defer req.Body.Close()
	var body kvm.SnapshotCreateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "快照参数格式不正确")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.Description = strings.TrimSpace(body.Description)
	if !snapshotNamePattern.MatchString(body.Name) {
		writeError(w, http.StatusBadRequest, "invalid_snapshot_name", "快照名称只能包含字母、数字、点、下划线、冒号和连字符，长度 1-80")
		return
	}
	if err := r.provider.CreateSnapshot(vmName, body); err != nil {
		r.logger.Error("snapshot create failed", "vm", vmName, "snapshot", body.Name, "error", err)
		writeError(w, http.StatusServiceUnavailable, "snapshot_create_failed", operationErrorMessage("创建快照失败", err))
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "action": "create", "name": vmName, "snapshot": body.Name})
}

func (r *Router) runAction(name, action string) error {
	switch action {
	case "start":
		return r.provider.StartVM(name)
	case "shutdown":
		return r.provider.ShutdownVM(name)
	case "suspend":
		return r.provider.PauseVM(name)
	case "resume":
		return r.provider.ResumeVM(name)
	case "reboot":
		return r.provider.RebootVM(name)
	case "reset":
		return r.provider.ResetVM(name)
	case "destroy":
		return r.provider.DestroyVM(name)
	case "delete":
		return r.provider.DeleteVM(name)
	case "force-delete":
		return r.provider.ForceDeleteVM(name)
	default:
		return http.ErrNotSupported
	}
}

func (r *Router) runSnapshotAction(vmName, snapshotName, action string) error {
	switch action {
	case "revert":
		return r.provider.RevertSnapshot(vmName, snapshotName)
	case "delete":
		return r.provider.DeleteSnapshot(vmName, snapshotName)
	default:
		return http.ErrNotSupported
	}
}

func snapshotActionSummary(action string) string {
	switch action {
	case "revert":
		return "恢复快照失败"
	case "delete":
		return "删除快照失败"
	default:
		return "快照操作失败"
	}
}

func (r *Router) withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		next.ServeHTTP(w, req)
		r.logger.Info("agent request", "method", req.Method, "path", req.URL.Path, "remote", req.RemoteAddr)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
