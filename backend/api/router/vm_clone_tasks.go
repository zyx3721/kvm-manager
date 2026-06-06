package router

import (
	"context"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/pkg/agent"
)

func (r *router) runVMCloneTask(taskID string, userID string, clientIP string, sourceVMID string, sourceVMName string, agentRecord domain.Agent, cfg agent.Config, request agent.VMCloneRequest) {
	go func() {
		claimed, err := r.store.ClaimTask(context.Background(), taskID)
		if err != nil || !claimed {
			message := "启动虚拟机克隆任务失败"
			if err != nil {
				r.logger.Error("claim vm clone task failed", "error", err, "agent", agentRecord.ID, "vm", sourceVMID)
			}
			_ = r.store.FinishTask(context.Background(), taskID, "failed", map[string]any{"agent": agentRecord.Name, "vm": sourceVMName, "clone": request.Name, "message": message}, message)
			r.runtime.BroadcastPayload("vm.clone.failed", map[string]any{"message": message, "vm": sourceVMName, "clone": request.Name})
			return
		}
		progress := map[string]any{"agent": agentRecord.Name, "vm": sourceVMName, "clone": request.Name, "message": "正在克隆虚拟机"}
		_ = r.store.UpdateTaskProgress(context.Background(), taskID, progress, "")
		config, err := agent.NewClient(agentRecord.TLSInsecure).CloneVM(context.Background(), cfg, sourceVMName, request)
		if err != nil {
			message := agent.UserFacingErrorMessage(err)
			r.logger.Error("agent clone vm failed", "error", err, "agent", agentRecord.ID, "vm", sourceVMID)
			_ = r.store.FinishTask(context.Background(), taskID, "failed", progress, message)
			_ = r.store.WriteAudit(context.Background(), userID, "vm.clone.failed", "virtual_machine", sourceVMID, clientIP, map[string]any{"agent": agentRecord.Name, "vm": sourceVMName, "clone": request.Name, "error": message})
			r.runtime.BroadcastPayload("vm.clone.failed", map[string]any{"message": message, "vm": sourceVMName, "clone": request.Name})
			return
		}
		done := map[string]any{"agent": agentRecord.Name, "vm": sourceVMName, "clone": config.Name, "message": "虚拟机克隆完成"}
		_ = r.store.FinishTask(context.Background(), taskID, "completed", done, "")
		_ = r.store.WriteAudit(context.Background(), userID, "vm.clone", "virtual_machine", sourceVMID, clientIP, map[string]any{"agent": agentRecord.Name, "vm": sourceVMName, "clone": config.Name})
		r.runtime.BroadcastPayload("vm.clone.completed", map[string]any{"message": config.Name + " 已克隆", "vm": sourceVMName, "clone": config.Name})
		if err := r.runtime.SyncAgentWithTokenFast(context.Background(), agentRecord.ID, cfg.Token); err != nil {
			r.logger.Warn("fast sync agent after vm clone failed", "error", err, "agent", agentRecord.ID)
		}
		r.runtime.Broadcast("runtime.updated")
		r.runtime.SyncAgentWithTokenDelayedFull(context.Background(), agentRecord.ID, cfg.Token)
	}()
}
