package router

import (
	"context"
	"os"
	"strings"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/pkg/agent"
)

func (r *router) runStorageVolumeCloneTask(taskID string, userID string, clientIP string, agentRecord domain.Agent, cfg agent.Config, poolName string, request agent.StorageVolumeCloneRequest) {
	go func() {
		claimed, err := r.store.ClaimTask(context.Background(), taskID)
		if err != nil || !claimed {
			message := "启动克隆任务失败"
			if err != nil {
				r.logger.Error("claim storage volume clone task failed", "error", err, "agent", agentRecord.ID, "pool", poolName)
			}
			_ = r.store.FinishTask(context.Background(), taskID, "failed", map[string]any{"agent": agentRecord.Name, "pool": poolName, "message": message}, message)
			r.runtime.BroadcastPayload("storage.volume.failed", map[string]any{"operation": "clone", "message": message})
			return
		}
		progress := map[string]any{"agent": agentRecord.Name, "pool": poolName, "source": request.SourceName, "volume": request.Name, "message": "正在克隆镜像"}
		_ = r.store.UpdateTaskProgress(context.Background(), taskID, progress, "")
		item, err := agent.NewClient(agentRecord.TLSInsecure).CloneStorageVolume(context.Background(), cfg, poolName, request)
		if err != nil {
			message := agent.UserFacingErrorMessage(err)
			r.logger.Error("agent clone storage volume failed", "error", err, "agent", agentRecord.ID, "pool", poolName)
			_ = r.store.FinishTask(context.Background(), taskID, "failed", progress, message)
			_ = r.store.WriteAudit(context.Background(), userID, "storage_volume.clone.failed", "agent", agentRecord.ID, clientIP, map[string]any{"agent": agentRecord.Name, "pool": poolName, "source": request.SourceName, "volume": request.Name, "error": message})
			r.runtime.BroadcastPayload("storage.volume.failed", map[string]any{"operation": "clone", "message": friendlyStorageVolumeMessage(message)})
			return
		}
		done := map[string]any{"agent": agentRecord.Name, "pool": poolName, "source": request.SourceName, "volume": item.Name, "message": "镜像克隆完成"}
		_ = r.store.FinishTask(context.Background(), taskID, "completed", done, "")
		_ = r.store.WriteAudit(context.Background(), userID, "storage_volume.clone", "agent", agentRecord.ID, clientIP, map[string]any{"agent": agentRecord.Name, "pool": poolName, "source": request.SourceName, "volume": item.Name})
		r.runtime.BroadcastPayload("storage.volume.completed", map[string]any{"operation": "clone", "message": item.Name + " 已克隆", "pool": poolName, "volume": item.Name})
		r.broadcastStoragePoolUpdated(agentRecord.ID, poolName)
	}()
}

func (r *router) runStorageVolumeUploadTask(taskID string, userID string, clientIP string, agentRecord domain.Agent, cfg agent.Config, poolName string, volumeName string, fileName string, tmpPath string) {
	go func() {
		defer os.Remove(tmpPath)
		claimed, err := r.store.ClaimTask(context.Background(), taskID)
		if err != nil || !claimed {
			message := "启动上传任务失败"
			if err != nil {
				r.logger.Error("claim storage volume upload task failed", "error", err, "agent", agentRecord.ID, "pool", poolName)
			}
			_ = r.store.FinishTask(context.Background(), taskID, "failed", map[string]any{"agent": agentRecord.Name, "pool": poolName, "message": message}, message)
			r.runtime.BroadcastPayload("storage.volume.failed", map[string]any{"operation": "upload", "message": message})
			return
		}
		displayName := strings.TrimSpace(volumeName)
		if displayName == "" {
			displayName = fileName
		}
		progress := map[string]any{"agent": agentRecord.Name, "pool": poolName, "volume": displayName, "message": "正在上传 ISO"}
		_ = r.store.UpdateTaskProgress(context.Background(), taskID, progress, "")
		file, err := os.Open(tmpPath)
		if err != nil {
			message := "读取上传文件失败"
			_ = r.store.FinishTask(context.Background(), taskID, "failed", progress, message)
			r.runtime.BroadcastPayload("storage.volume.failed", map[string]any{"operation": "upload", "message": message})
			return
		}
		defer file.Close()
		item, err := agent.NewClient(agentRecord.TLSInsecure).UploadStorageVolume(context.Background(), cfg, poolName, volumeName, fileName, file)
		if err != nil {
			message := agent.UserFacingErrorMessage(err)
			r.logger.Error("agent upload storage volume failed", "error", err, "agent", agentRecord.ID, "pool", poolName)
			_ = r.store.FinishTask(context.Background(), taskID, "failed", progress, message)
			_ = r.store.WriteAudit(context.Background(), userID, "storage_volume.upload.failed", "agent", agentRecord.ID, clientIP, map[string]any{"agent": agentRecord.Name, "pool": poolName, "volume": displayName, "error": message})
			r.runtime.BroadcastPayload("storage.volume.failed", map[string]any{"operation": "upload", "message": friendlyStorageVolumeMessage(message)})
			return
		}
		done := map[string]any{"agent": agentRecord.Name, "pool": poolName, "volume": item.Name, "message": "ISO 上传完成"}
		_ = r.store.FinishTask(context.Background(), taskID, "completed", done, "")
		_ = r.store.WriteAudit(context.Background(), userID, "storage_volume.upload", "agent", agentRecord.ID, clientIP, map[string]any{"agent": agentRecord.Name, "pool": poolName, "volume": item.Name})
		r.runtime.BroadcastPayload("storage.volume.completed", map[string]any{"operation": "upload", "message": item.Name + " 已上传", "pool": poolName, "volume": item.Name})
		r.broadcastStoragePoolUpdated(agentRecord.ID, poolName)
	}()
}
