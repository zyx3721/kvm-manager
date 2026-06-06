package router

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/pkg/agent"
	"kvm-manager/backend/pkg/tokencrypto"
)

type storagePoolCreateRequest struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Path       string `json:"path"`
	Device     string `json:"device"`
	SourceHost string `json:"sourceHost"`
	SourcePath string `json:"sourcePath"`
	Format     string `json:"format"`
}

type storageVolumeCreateRequest struct {
	Name             string `json:"name"`
	Format           string `json:"format"`
	CapacityBytes    int64  `json:"capacityBytes"`
	PreallocMetadata bool   `json:"preallocMetadata"`
}

type storageVolumeCloneRequest struct {
	Name             string `json:"name"`
	SourceName       string `json:"sourceName"`
	Format           string `json:"format"`
	Convert          bool   `json:"convert"`
	PreallocMetadata bool   `json:"preallocMetadata"`
}

type networkPoolCreateRequest struct {
	Name         string `json:"name"`
	Subnet       string `json:"subnet"`
	DHCP         bool   `json:"dhcp"`
	FixedAddress bool   `json:"fixedAddress"`
	Type         string `json:"type"`
	Bridge       string `json:"bridge"`
	OpenVSwitch  bool   `json:"openVSwitch"`
}

type poolStateUpdateRequest struct {
	Active bool `json:"active"`
}

type poolAutostartUpdateRequest struct {
	Autostart bool `json:"autostart"`
}

func (r *router) handleStoragePoolRoute(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet {
		if !r.ensureStoragePoolReadPermission(w, req) {
			return
		}
	} else if !r.ensurePermission(w, req, "storage.manage") {
		return
	}
	agentID, action, ok := parseHostResourcePath(req.URL.Path, "/api/storage-pools/")
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
		items, err := client.ListStoragePools(req.Context(), cfg)
		if err != nil {
			r.logger.Error("agent list storage pools failed", "error", err, "agent", agentID)
			writeError(w, http.StatusServiceUnavailable, "agent_storage_pools_failed", agent.UserFacingErrorMessage(err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
		return
	}
	if req.Method == http.MethodPost && action == "" {
		r.handleCreateStoragePool(w, req, agentRecord, cfg, client)
		return
	}
	if req.Method == http.MethodGet && strings.HasPrefix(action, "iso-files/") {
		poolName := strings.TrimPrefix(action, "iso-files/")
		items, err := client.ListISOFiles(req.Context(), cfg, poolName)
		if err != nil {
			r.logger.Error("agent list iso files failed", "error", err, "agent", agentID, "pool", poolName)
			writeError(w, http.StatusServiceUnavailable, "agent_iso_files_failed", agent.UserFacingErrorMessage(err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
		return
	}
	if req.Method == http.MethodGet && strings.HasPrefix(action, "volumes/") {
		poolName := strings.TrimPrefix(action, "volumes/")
		items, err := client.ListStorageVolumes(req.Context(), cfg, poolName)
		if err != nil {
			r.logger.Error("agent list storage volumes failed", "error", err, "agent", agentID, "pool", poolName)
			writeError(w, http.StatusServiceUnavailable, "agent_storage_volumes_failed", agent.UserFacingErrorMessage(err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
		return
	}
	if req.Method == http.MethodPost && strings.HasPrefix(action, "volumes/") && strings.HasSuffix(action, "/upload") {
		poolName := strings.TrimSuffix(strings.TrimPrefix(action, "volumes/"), "/upload")
		r.handleUploadStorageVolume(w, req, agentRecord, cfg, client, poolName)
		return
	}
	if req.Method == http.MethodPost && strings.HasPrefix(action, "volumes/") && strings.HasSuffix(action, "/clone") {
		poolName := strings.TrimSuffix(strings.TrimPrefix(action, "volumes/"), "/clone")
		r.handleCloneStorageVolume(w, req, agentRecord, cfg, client, poolName)
		return
	}
	if req.Method == http.MethodPost && strings.HasPrefix(action, "volumes/") {
		r.handleCreateStorageVolume(w, req, agentRecord, cfg, client, strings.TrimPrefix(action, "volumes/"))
		return
	}
	if req.Method == http.MethodDelete && strings.HasPrefix(action, "volumes/") {
		r.handleDeleteStorageVolume(w, req, agentRecord, cfg, client, strings.TrimPrefix(action, "volumes/"))
		return
	}
	if req.Method == http.MethodDelete && strings.HasPrefix(action, "delete/") {
		r.handleDeleteStoragePool(w, req, agentRecord, cfg, client, strings.TrimPrefix(action, "delete/"))
		return
	}
	if req.Method == http.MethodPut && strings.HasPrefix(action, "state/") {
		r.handleUpdateStoragePoolState(w, req, agentRecord, cfg, client, strings.TrimPrefix(action, "state/"))
		return
	}
	if req.Method == http.MethodPut && strings.HasPrefix(action, "autostart/") {
		r.handleUpdateStoragePoolAutostart(w, req, agentRecord, cfg, client, strings.TrimPrefix(action, "autostart/"))
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
}

func (r *router) ensureStoragePoolReadPermission(w http.ResponseWriter, req *http.Request) bool {
	user := currentSession(req).User
	if hasAssociatedStoragePoolReadPermission(user) {
		return true
	}
	writeError(w, http.StatusForbidden, "permission_denied", "当前用户无权执行此操作")
	return false
}

func (r *router) handleCreateStoragePool(w http.ResponseWriter, req *http.Request, agentRecord domain.Agent, cfg agent.Config, client *agent.Client) {
	defer req.Body.Close()
	var body storagePoolCreateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "存储池参数格式不正确")
		return
	}
	item, err := client.CreateStoragePool(req.Context(), cfg, agent.StoragePoolCreateRequest{
		Name:       strings.TrimSpace(body.Name),
		Type:       strings.TrimSpace(body.Type),
		Path:       strings.TrimSpace(body.Path),
		Device:     strings.TrimSpace(body.Device),
		SourceHost: strings.TrimSpace(body.SourceHost),
		SourcePath: strings.TrimSpace(body.SourcePath),
		Format:     strings.TrimSpace(body.Format),
	})
	if err != nil {
		r.logger.Error("agent create storage pool failed", "error", err, "agent", agentRecord.ID)
		writeError(w, http.StatusServiceUnavailable, "agent_storage_pool_create_failed", agent.UserFacingErrorMessage(err))
		return
	}
	session := currentSession(req)
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "storage_pool.create", "agent", agentRecord.ID, repository.ClientIP(req), map[string]any{"agent": agentRecord.Name, "name": item.Name, "type": item.Type})
	r.broadcastStoragePoolUpdated(agentRecord.ID, item.Name)
	writeJSON(w, http.StatusOK, item)
}

func (r *router) handleDeleteStorageVolume(w http.ResponseWriter, req *http.Request, agentRecord domain.Agent, cfg agent.Config, client *agent.Client, poolName string) {
	volumeName := strings.TrimSpace(req.URL.Query().Get("name"))
	if volumeName == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "存储卷名称不能为空")
		return
	}
	if err := client.DeleteStorageVolume(req.Context(), cfg, poolName, volumeName); err != nil {
		r.logger.Error("agent delete storage volume failed", "error", err, "agent", agentRecord.ID, "pool", poolName, "volume", volumeName)
		writeError(w, http.StatusServiceUnavailable, "agent_storage_volume_delete_failed", agent.UserFacingErrorMessage(err))
		return
	}
	session := currentSession(req)
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "storage_volume.delete", "agent", agentRecord.ID, repository.ClientIP(req), map[string]any{"agent": agentRecord.Name, "pool": poolName, "volume": volumeName})
	r.broadcastStoragePoolUpdated(agentRecord.ID, poolName)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *router) handleCreateStorageVolume(w http.ResponseWriter, req *http.Request, agentRecord domain.Agent, cfg agent.Config, client *agent.Client, poolName string) {
	defer req.Body.Close()
	var body storageVolumeCreateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "存储卷参数格式不正确")
		return
	}
	item, err := client.CreateStorageVolume(req.Context(), cfg, poolName, agent.StorageVolumeCreateRequest{
		Name:             strings.TrimSpace(body.Name),
		Format:           strings.TrimSpace(body.Format),
		CapacityBytes:    body.CapacityBytes,
		PreallocMetadata: body.PreallocMetadata,
	})
	if err != nil {
		r.logger.Error("agent create storage volume failed", "error", err, "agent", agentRecord.ID, "pool", poolName)
		writeError(w, http.StatusServiceUnavailable, "agent_storage_volume_create_failed", agent.UserFacingErrorMessage(err))
		return
	}
	session := currentSession(req)
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "storage_volume.create", "agent", agentRecord.ID, repository.ClientIP(req), map[string]any{"agent": agentRecord.Name, "pool": poolName, "volume": item.Name})
	r.broadcastStoragePoolUpdated(agentRecord.ID, poolName)
	writeJSON(w, http.StatusOK, item)
}

func (r *router) handleCloneStorageVolume(w http.ResponseWriter, req *http.Request, agentRecord domain.Agent, cfg agent.Config, client *agent.Client, poolName string) {
	defer req.Body.Close()
	var body storageVolumeCloneRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "存储卷克隆参数格式不正确")
		return
	}
	cloneRequest := agent.StorageVolumeCloneRequest{
		Name:             strings.TrimSpace(body.Name),
		SourceName:       strings.TrimSpace(body.SourceName),
		Format:           strings.TrimSpace(body.Format),
		Convert:          body.Convert,
		PreallocMetadata: body.PreallocMetadata,
	}
	if cloneRequest.Name == "" || cloneRequest.SourceName == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "克隆镜像名称不能为空")
		return
	}
	if storageVolumeNameExists(req.Context(), client, cfg, poolName, cloneRequest.Name) {
		writeError(w, http.StatusConflict, "storage_volume_exists", "镜像名称已存在，请更换名称")
		return
	}
	session := currentSession(req)
	task, err := r.store.CreateTask(req.Context(), "storage_volume.clone", "queued", "agent", agentRecord.ID, map[string]any{"agent": agentRecord.Name, "pool": poolName, "source": cloneRequest.SourceName, "volume": cloneRequest.Name, "message": "克隆镜像排队中"}, session.User.ID, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_storage_volume_clone_task_failed", "创建克隆任务失败")
		return
	}
	r.runStorageVolumeCloneTask(task.ID, session.User.ID, repository.ClientIP(req), agentRecord, cfg, poolName, cloneRequest)
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "queued", "task": task})
}

func (r *router) handleUploadStorageVolume(w http.ResponseWriter, req *http.Request, agentRecord domain.Agent, cfg agent.Config, client *agent.Client, poolName string) {
	if err := req.ParseMultipartForm(128 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "上传文件参数不正确")
		return
	}
	file, header, err := req.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "请选择要上传的文件")
		return
	}
	defer file.Close()
	volumeName := strings.TrimSpace(req.FormValue("name"))
	fileName := ""
	if header != nil {
		fileName = header.Filename
	}
	tmp, err := os.CreateTemp("", "kvm-manager-iso-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_upload_temp_failed", "创建上传缓存失败")
		return
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, file); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		writeError(w, http.StatusInternalServerError, "save_upload_temp_failed", "保存上传文件失败")
		return
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		writeError(w, http.StatusInternalServerError, "save_upload_temp_failed", "保存上传文件失败")
		return
	}
	session := currentSession(req)
	displayName := strings.TrimSpace(volumeName)
	if displayName == "" {
		displayName = fileName
	}
	if !strings.HasSuffix(strings.ToLower(displayName), ".iso") {
		_ = os.Remove(tmpPath)
		writeError(w, http.StatusBadRequest, "invalid_iso_name", "ISO 名称必须以 .iso 结尾")
		return
	}
	if storageVolumeNameExists(req.Context(), client, cfg, poolName, displayName) {
		_ = os.Remove(tmpPath)
		writeError(w, http.StatusConflict, "storage_volume_exists", "镜像名称已存在，请更换名称")
		return
	}
	task, err := r.store.CreateTask(req.Context(), "storage_volume.upload", "queued", "agent", agentRecord.ID, map[string]any{"agent": agentRecord.Name, "pool": poolName, "volume": displayName, "message": "上传 ISO 排队中"}, session.User.ID, "")
	if err != nil {
		_ = os.Remove(tmpPath)
		writeError(w, http.StatusInternalServerError, "create_storage_volume_upload_task_failed", "创建上传任务失败")
		return
	}
	r.runStorageVolumeUploadTask(task.ID, session.User.ID, repository.ClientIP(req), agentRecord, cfg, poolName, volumeName, fileName, tmpPath)
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "queued", "task": task})
}

func friendlyStorageVolumeMessage(message string) string {
	compact := strings.Join(strings.Fields(strings.TrimSpace(message)), " ")
	lower := strings.ToLower(compact)
	switch {
	case strings.Contains(lower, "target volume already exists") || strings.Contains(lower, "exists already") || strings.Contains(lower, "already exists"):
		return "镜像名称已存在，请更换名称"
	case strings.Contains(lower, "failed to get shared \"write\" lock") || strings.Contains(lower, "failed to get \"write\" lock") || strings.Contains(lower, "is already in use"):
		return "当前镜像正在被虚拟机使用，无法克隆。请先关闭相关虚拟机，或通过快照创建一致性副本"
	case strings.Contains(lower, "permission denied"):
		return "宿主机权限不足，无法操作该镜像"
	case strings.Contains(lower, "no space left"):
		return "宿主机存储空间不足，无法完成操作"
	case strings.Contains(lower, "qemu-img") || strings.Contains(lower, "virsh") || strings.Contains(lower, "error:"):
		return "宿主机命令执行失败，请检查镜像状态、格式和权限"
	default:
		if compact == "" {
			return "存储卷操作失败"
		}
		if len(compact) > 120 {
			return compact[:120] + "..."
		}
		return compact
	}
}

func storageVolumeNameExists(ctx context.Context, client *agent.Client, cfg agent.Config, poolName string, volumeName string) bool {
	target := strings.ToLower(strings.TrimSpace(volumeName))
	if target == "" {
		return false
	}
	volumes, err := client.ListStorageVolumes(ctx, cfg, poolName)
	if err != nil {
		return false
	}
	for _, volume := range volumes {
		if strings.ToLower(strings.TrimSpace(volume.Name)) == target {
			return true
		}
	}
	return false
}

func (r *router) handleDeleteStoragePool(w http.ResponseWriter, req *http.Request, agentRecord domain.Agent, cfg agent.Config, client *agent.Client, poolName string) {
	if strings.TrimSpace(poolName) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "存储池名称不能为空")
		return
	}
	if err := client.DeleteStoragePool(req.Context(), cfg, poolName); err != nil {
		r.logger.Error("agent delete storage pool failed", "error", err, "agent", agentRecord.ID, "pool", poolName)
		writeError(w, http.StatusServiceUnavailable, "agent_storage_pool_delete_failed", agent.UserFacingErrorMessage(err))
		return
	}
	session := currentSession(req)
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "storage_pool.delete", "agent", agentRecord.ID, repository.ClientIP(req), map[string]any{"agent": agentRecord.Name, "pool": poolName})
	r.broadcastStoragePoolUpdated(agentRecord.ID, poolName)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *router) handleUpdateStoragePoolState(w http.ResponseWriter, req *http.Request, agentRecord domain.Agent, cfg agent.Config, client *agent.Client, poolName string) {
	defer req.Body.Close()
	var body poolStateUpdateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "存储池状态参数格式不正确")
		return
	}
	if err := client.UpdateStoragePoolState(req.Context(), cfg, poolName, agent.PoolStateUpdateRequest{Active: body.Active}); err != nil {
		r.logger.Error("agent update storage pool state failed", "error", err, "agent", agentRecord.ID, "pool", poolName)
		writeError(w, http.StatusServiceUnavailable, "agent_storage_pool_state_failed", agent.UserFacingErrorMessage(err))
		return
	}
	session := currentSession(req)
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "storage_pool.state.update", "agent", agentRecord.ID, repository.ClientIP(req), map[string]any{"agent": agentRecord.Name, "name": poolName, "active": body.Active})
	r.broadcastStoragePoolUpdated(agentRecord.ID, poolName)
	writeJSON(w, http.StatusOK, map[string]bool{"active": body.Active})
}

func (r *router) handleUpdateStoragePoolAutostart(w http.ResponseWriter, req *http.Request, agentRecord domain.Agent, cfg agent.Config, client *agent.Client, poolName string) {
	defer req.Body.Close()
	var body poolAutostartUpdateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "存储池自启动参数格式不正确")
		return
	}
	if err := client.UpdateStoragePoolAutostart(req.Context(), cfg, poolName, agent.PoolAutostartUpdateRequest{Autostart: body.Autostart}); err != nil {
		r.logger.Error("agent update storage pool autostart failed", "error", err, "agent", agentRecord.ID, "pool", poolName)
		writeError(w, http.StatusServiceUnavailable, "agent_storage_pool_autostart_failed", agent.UserFacingErrorMessage(err))
		return
	}
	session := currentSession(req)
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "storage_pool.autostart.update", "agent", agentRecord.ID, repository.ClientIP(req), map[string]any{"agent": agentRecord.Name, "name": poolName, "autostart": body.Autostart})
	r.broadcastStoragePoolUpdated(agentRecord.ID, poolName)
	writeJSON(w, http.StatusOK, map[string]bool{"autostart": body.Autostart})
}

func (r *router) handleNetworkPoolRoute(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet {
		if !r.ensureNetworkPoolReadPermission(w, req) {
			return
		}
	} else if !r.ensurePermission(w, req, "network.manage") {
		return
	}
	agentID, action, ok := parseHostResourcePath(req.URL.Path, "/api/network-pools/")
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
		items, err := client.ListNetworkPools(req.Context(), cfg)
		if err != nil {
			r.logger.Error("agent list network pools failed", "error", err, "agent", agentID)
			writeError(w, http.StatusServiceUnavailable, "agent_network_pools_failed", agent.UserFacingErrorMessage(err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
		return
	}
	if req.Method == http.MethodPost && action == "" {
		r.handleCreateNetworkPool(w, req, agentRecord, cfg, client)
		return
	}
	if req.Method == http.MethodPut && strings.HasPrefix(action, "state/") {
		r.handleUpdateNetworkPoolState(w, req, agentRecord, cfg, client, strings.TrimPrefix(action, "state/"))
		return
	}
	if req.Method == http.MethodPut && strings.HasPrefix(action, "autostart/") {
		r.handleUpdateNetworkPoolAutostart(w, req, agentRecord, cfg, client, strings.TrimPrefix(action, "autostart/"))
		return
	}
	if req.Method == http.MethodDelete && strings.HasPrefix(action, "delete/") {
		r.handleDeleteNetworkPool(w, req, agentRecord, cfg, client, strings.TrimPrefix(action, "delete/"))
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
}

func (r *router) ensureNetworkPoolReadPermission(w http.ResponseWriter, req *http.Request) bool {
	user := currentSession(req).User
	if hasAssociatedNetworkPoolReadPermission(user) {
		return true
	}
	writeError(w, http.StatusForbidden, "permission_denied", "当前用户无权执行此操作")
	return false
}

func (r *router) handleCreateNetworkPool(w http.ResponseWriter, req *http.Request, agentRecord domain.Agent, cfg agent.Config, client *agent.Client) {
	defer req.Body.Close()
	var body networkPoolCreateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "网络池参数格式不正确")
		return
	}
	item, err := client.CreateNetworkPool(req.Context(), cfg, agent.NetworkPoolCreateRequest{
		Name:         strings.TrimSpace(body.Name),
		Subnet:       strings.TrimSpace(body.Subnet),
		DHCP:         body.DHCP,
		FixedAddress: body.FixedAddress,
		Type:         strings.TrimSpace(body.Type),
		Bridge:       strings.TrimSpace(body.Bridge),
		OpenVSwitch:  body.OpenVSwitch,
	})
	if err != nil {
		r.logger.Error("agent create network pool failed", "error", err, "agent", agentRecord.ID)
		writeError(w, http.StatusServiceUnavailable, "agent_network_pool_create_failed", agent.UserFacingErrorMessage(err))
		return
	}
	session := currentSession(req)
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "network_pool.create", "agent", agentRecord.ID, repository.ClientIP(req), map[string]any{"agent": agentRecord.Name, "name": item.Name, "type": item.Forward})
	r.broadcastNetworkPoolUpdated(agentRecord.ID, item.Name)
	writeJSON(w, http.StatusOK, item)
}

func (r *router) handleDeleteNetworkPool(w http.ResponseWriter, req *http.Request, agentRecord domain.Agent, cfg agent.Config, client *agent.Client, poolName string) {
	if strings.TrimSpace(poolName) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "网络池名称不能为空")
		return
	}
	if err := client.DeleteNetworkPool(req.Context(), cfg, poolName); err != nil {
		r.logger.Error("agent delete network pool failed", "error", err, "agent", agentRecord.ID, "pool", poolName)
		writeError(w, http.StatusServiceUnavailable, "agent_network_pool_delete_failed", agent.UserFacingErrorMessage(err))
		return
	}
	session := currentSession(req)
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "network_pool.delete", "agent", agentRecord.ID, repository.ClientIP(req), map[string]any{"agent": agentRecord.Name, "pool": poolName})
	r.broadcastNetworkPoolUpdated(agentRecord.ID, poolName)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *router) handleUpdateNetworkPoolState(w http.ResponseWriter, req *http.Request, agentRecord domain.Agent, cfg agent.Config, client *agent.Client, poolName string) {
	defer req.Body.Close()
	var body poolStateUpdateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "网络池状态参数格式不正确")
		return
	}
	if err := client.UpdateNetworkPoolState(req.Context(), cfg, poolName, agent.PoolStateUpdateRequest{Active: body.Active}); err != nil {
		r.logger.Error("agent update network pool state failed", "error", err, "agent", agentRecord.ID, "pool", poolName)
		writeError(w, http.StatusServiceUnavailable, "agent_network_pool_state_failed", agent.UserFacingErrorMessage(err))
		return
	}
	session := currentSession(req)
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "network_pool.state.update", "agent", agentRecord.ID, repository.ClientIP(req), map[string]any{"agent": agentRecord.Name, "name": poolName, "active": body.Active})
	r.broadcastNetworkPoolUpdated(agentRecord.ID, poolName)
	writeJSON(w, http.StatusOK, map[string]bool{"active": body.Active})
}

func (r *router) handleUpdateNetworkPoolAutostart(w http.ResponseWriter, req *http.Request, agentRecord domain.Agent, cfg agent.Config, client *agent.Client, poolName string) {
	defer req.Body.Close()
	var body poolAutostartUpdateRequest
	if err := decodeJSONBody(w, req, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "网络池自启动参数格式不正确")
		return
	}
	if err := client.UpdateNetworkPoolAutostart(req.Context(), cfg, poolName, agent.PoolAutostartUpdateRequest{Autostart: body.Autostart}); err != nil {
		r.logger.Error("agent update network pool autostart failed", "error", err, "agent", agentRecord.ID, "pool", poolName)
		writeError(w, http.StatusServiceUnavailable, "agent_network_pool_autostart_failed", agent.UserFacingErrorMessage(err))
		return
	}
	session := currentSession(req)
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "network_pool.autostart.update", "agent", agentRecord.ID, repository.ClientIP(req), map[string]any{"agent": agentRecord.Name, "name": poolName, "autostart": body.Autostart})
	r.broadcastNetworkPoolUpdated(agentRecord.ID, poolName)
	writeJSON(w, http.StatusOK, map[string]bool{"autostart": body.Autostart})
}

func (r *router) broadcastStoragePoolUpdated(agentID string, poolName string) {
	r.runtime.BroadcastPayload("storage.pool.updated", map[string]any{"agentId": agentID, "pool": poolName})
	r.runtime.Broadcast("runtime.updated")
}

func (r *router) broadcastNetworkPoolUpdated(agentID string, poolName string) {
	r.runtime.BroadcastPayload("network.pool.updated", map[string]any{"agentId": agentID, "pool": poolName})
	r.runtime.Broadcast("runtime.updated")
}

func (r *router) agentTokenForHost(w http.ResponseWriter, req *http.Request, id string) (domain.Agent, string, bool) {
	item, err := r.store.GetAgent(req.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent_not_found", "宿主机对应 Agent 不存在")
		return domain.Agent{}, "", false
	}
	token, err := tokencrypto.Open(r.cfg.JWT.Secret, item.TokenCiphertext)
	if err != nil || token == "" {
		writeError(w, http.StatusBadRequest, "agent_token_unavailable", "Agent 令牌不可用于执行操作，请重新保存 Agent")
		return domain.Agent{}, "", false
	}
	return item, token, true
}

func parseHostResourcePath(path string, prefix string) (agentID string, action string, ok bool) {
	trimmed := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if trimmed == "" || strings.HasPrefix(trimmed, prefix) {
		return "", "", false
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) == 1 {
		return parts[0], "", parts[0] != ""
	}
	return parts[0], strings.Join(parts[1:], "/"), parts[0] != ""
}
