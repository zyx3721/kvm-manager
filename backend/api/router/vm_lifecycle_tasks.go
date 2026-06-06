package router

import (
	"context"
	"strings"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/pkg/agent"
)

func (r *router) runVMCreateTask(taskID string, userID string, clientIP string, agentRecord domain.Agent, cfg agent.Config, request agent.VMCreateRequest) {
	go func() {
		claimed, err := r.store.ClaimTask(context.Background(), taskID)
		if err != nil || !claimed {
			message := "启动虚拟机创建任务失败"
			if err != nil {
				r.logger.Error("claim vm create task failed", "error", err, "agent", agentRecord.ID, "vm", request.Name)
			}
			_ = r.store.FinishTask(context.Background(), taskID, "failed", map[string]any{"agent": agentRecord.Name, "vm": request.Name, "message": message}, message)
			r.runtime.BroadcastPayload("vm.create.failed", map[string]any{"message": message, "vm": request.Name})
			return
		}
		progress := map[string]any{"agent": agentRecord.Name, "vm": request.Name, "message": "正在创建虚拟机"}
		_ = r.store.UpdateTaskProgress(context.Background(), taskID, progress, "")
		config, err := agent.NewClient(agentRecord.TLSInsecure).CreateVM(context.Background(), cfg, request)
		if err != nil {
			message := friendlyVMCreateTaskMessage(request, agent.UserFacingErrorMessage(err))
			r.logger.Error("agent create vm failed", "error", err, "agent", agentRecord.ID, "vm", request.Name)
			_ = r.store.FinishTask(context.Background(), taskID, "failed", progress, message)
			_ = r.store.WriteAudit(context.Background(), userID, "vm.create.failed", "agent", agentRecord.ID, clientIP, map[string]any{"agent": agentRecord.Name, "vm": request.Name, "error": message})
			r.runtime.BroadcastPayload("vm.create.failed", map[string]any{"message": message, "vm": request.Name})
			return
		}
		done := map[string]any{"agent": agentRecord.Name, "vm": config.Name, "message": "虚拟机创建完成"}
		_ = r.store.FinishTask(context.Background(), taskID, "completed", done, "")
		_ = r.store.WriteAudit(context.Background(), userID, "vm.create", "agent", agentRecord.ID, clientIP, map[string]any{"agent": agentRecord.Name, "vm": config.Name})
		r.runtime.BroadcastPayload("vm.create.completed", map[string]any{"message": config.Name + " 已创建", "vm": config.Name})
		if err := r.runtime.SyncAgentWithTokenFast(context.Background(), agentRecord.ID, cfg.Token); err != nil {
			r.logger.Warn("fast sync agent after vm create failed", "error", err, "agent", agentRecord.ID)
		}
		r.runtime.Broadcast("runtime.updated")
		r.runtime.SyncAgentWithTokenDelayedFull(context.Background(), agentRecord.ID, cfg.Token)
	}()
}

func friendlyVMCreateTaskMessage(request agent.VMCreateRequest, message string) string {
	if request.CreateMode != "template" || !isQemuImageWriteLockMessage(message) {
		return message
	}
	if strings.TrimSpace(request.Template.SourceName) == "" {
		return "当前模板文件正在被运行中的虚拟机使用，无法克隆。请先关闭使用该模板文件的虚拟机后再创建"
	}
	return "模板文件 " + request.Template.SourceName + " 正在被运行中的虚拟机使用，无法克隆。请先关闭使用该模板文件的虚拟机后再创建"
}

func isQemuImageWriteLockMessage(message string) bool {
	compact := strings.Join(strings.Fields(strings.TrimSpace(message)), " ")
	lower := strings.ToLower(compact)
	return strings.Contains(lower, `failed to get shared "write" lock`) ||
		strings.Contains(lower, `failed to get "write" lock`) ||
		strings.Contains(lower, "is another process using the image") ||
		strings.Contains(lower, "is already in use")
}

func (r *router) runVMMigrateTask(taskID string, userID string, clientIP string, vm domain.VirtualMachine, sourceAgent domain.Agent, sourceToken string, targetAgent domain.Agent, targetToken string, request agent.VMMigrateRequest) {
	go func() {
		claimed, err := r.store.ClaimTask(context.Background(), taskID)
		if err != nil || !claimed {
			message := "启动虚拟机迁移任务失败"
			if err != nil {
				r.logger.Error("claim vm migrate task failed", "error", err, "vm", vm.ID)
			}
			_ = r.store.FinishTask(context.Background(), taskID, "failed", map[string]any{"vm": vm.Name, "message": message}, message)
			r.runtime.BroadcastPayload("vm.migrate.failed", map[string]any{"message": message, "vm": vm.Name})
			return
		}
		progress := map[string]any{"vm": vm.Name, "sourceAgent": sourceAgent.Name, "targetAgent": targetAgent.Name, "message": "正在迁移虚拟机"}
		_ = r.store.UpdateTaskProgress(context.Background(), taskID, progress, "")
		cfg := agent.Config{Endpoint: sourceAgent.Endpoint, Token: sourceToken, TLSInsecure: sourceAgent.TLSInsecure}
		if err := agent.NewClient(sourceAgent.TLSInsecure).MigrateVM(context.Background(), cfg, vm.Name, request); err != nil {
			message := agent.UserFacingErrorMessage(err)
			r.logger.Error("agent migrate vm failed", "error", err, "vm", vm.ID)
			_ = r.store.FinishTask(context.Background(), taskID, "failed", progress, message)
			_ = r.store.WriteAudit(context.Background(), userID, "vm.migrate.failed", "virtual_machine", vm.ID, clientIP, map[string]any{"vm": vm.Name, "sourceAgent": sourceAgent.Name, "targetAgent": targetAgent.Name, "error": message})
			r.runtime.BroadcastPayload("vm.migrate.failed", map[string]any{"message": message, "vm": vm.Name})
			return
		}
		done := map[string]any{"vm": vm.Name, "sourceAgent": sourceAgent.Name, "targetAgent": targetAgent.Name, "message": "虚拟机迁移完成"}
		_ = r.store.FinishTask(context.Background(), taskID, "completed", done, "")
		_ = r.store.WriteAudit(context.Background(), userID, "vm.migrate", "virtual_machine", vm.ID, clientIP, map[string]any{"vm": vm.Name, "sourceAgent": sourceAgent.Name, "targetAgent": targetAgent.Name})
		r.runtime.BroadcastPayload("vm.migrate.completed", map[string]any{"message": vm.Name + " 已迁移", "vm": vm.Name})
		if err := r.runtime.SyncAgentWithTokenFast(context.Background(), sourceAgent.ID, sourceToken); err != nil {
			r.logger.Warn("fast sync source agent after vm migrate failed", "error", err, "agent", sourceAgent.ID)
		}
		if err := r.runtime.SyncAgentWithTokenFast(context.Background(), targetAgent.ID, targetToken); err != nil {
			r.logger.Warn("fast sync target agent after vm migrate failed", "error", err, "agent", targetAgent.ID)
		}
		r.runtime.Broadcast("runtime.updated")
		go func() {
			if err := r.runtime.SyncAgentWithToken(context.Background(), sourceAgent.ID, sourceToken); err != nil {
				r.logger.Warn("full sync source agent after vm migrate failed", "error", err, "agent", sourceAgent.ID)
			}
			if err := r.runtime.SyncAgentWithToken(context.Background(), targetAgent.ID, targetToken); err != nil {
				r.logger.Warn("full sync target agent after vm migrate failed", "error", err, "agent", targetAgent.ID)
			}
			r.runtime.Broadcast("runtime.updated")
		}()
	}()
}
