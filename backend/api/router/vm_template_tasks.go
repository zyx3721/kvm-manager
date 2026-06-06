package router

import (
	"context"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/pkg/agent"
)

func (r *router) runVMTemplateCreateTask(taskID string, userID string, clientIP string, templateVM domain.VirtualMachine, agentRecord domain.Agent, cfg agent.Config, request agent.VMCloneRequest) {
	go func() {
		claimed, err := r.store.ClaimTask(context.Background(), taskID)
		if err != nil || !claimed {
			message := "启动模板创建任务失败"
			if err != nil {
				r.logger.Error("claim vm template create task failed", "error", err, "agent", agentRecord.ID, "vm", templateVM.ID)
			}
			_ = r.store.FinishTask(context.Background(), taskID, "failed", map[string]any{"agent": agentRecord.Name, "template": templateDisplayName(templateVM), "vm": request.Name, "message": message}, message)
			r.runtime.BroadcastPayload("vm.template.create.failed", map[string]any{"message": message, "template": templateDisplayName(templateVM), "vm": request.Name})
			return
		}
		progress := map[string]any{"agent": agentRecord.Name, "template": templateDisplayName(templateVM), "vm": request.Name, "message": "正在从模板创建虚拟机"}
		_ = r.store.UpdateTaskProgress(context.Background(), taskID, progress, "")
		config, err := agent.NewClient(agentRecord.TLSInsecure).CloneVM(context.Background(), cfg, templateVM.Name, request)
		if err != nil {
			message := agent.UserFacingErrorMessage(err)
			r.logger.Error("agent create vm from template failed", "error", err, "agent", agentRecord.ID, "vm", templateVM.ID)
			_ = r.store.FinishTask(context.Background(), taskID, "failed", progress, message)
			_ = r.store.WriteAudit(context.Background(), userID, "vm.template.create.failed", "virtual_machine", templateVM.ID, clientIP, map[string]any{"agent": agentRecord.Name, "template": templateDisplayName(templateVM), "vm": request.Name, "error": message})
			r.runtime.BroadcastPayload("vm.template.create.failed", map[string]any{"message": message, "template": templateDisplayName(templateVM), "vm": request.Name})
			return
		}
		done := map[string]any{"agent": agentRecord.Name, "template": templateDisplayName(templateVM), "vm": config.Name, "message": "从模板创建虚拟机完成"}
		_ = r.store.FinishTask(context.Background(), taskID, "completed", done, "")
		_ = r.store.WriteAudit(context.Background(), userID, "vm.template.create", "virtual_machine", templateVM.ID, clientIP, map[string]any{"agent": agentRecord.Name, "template": templateDisplayName(templateVM), "vm": config.Name})
		r.runtime.BroadcastPayload("vm.template.create.completed", map[string]any{"message": config.Name + " 已从模板创建", "template": templateDisplayName(templateVM), "vm": config.Name})
		if err := r.runtime.SyncAgentWithTokenFast(context.Background(), agentRecord.ID, cfg.Token); err != nil {
			r.logger.Warn("fast sync agent after vm template create failed", "error", err, "agent", agentRecord.ID)
		}
		r.runtime.Broadcast("runtime.updated")
		r.runtime.SyncAgentWithTokenDelayedFull(context.Background(), agentRecord.ID, cfg.Token)
	}()
}
