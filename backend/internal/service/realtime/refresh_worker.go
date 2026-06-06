package realtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"kvm-manager/backend/internal/domain"
)

const (
	RefreshAllTaskType  = "runtime.refresh.all"
	RefreshFastTaskType = "runtime.refresh.fast"
)

type RefreshProgress struct {
	TotalAgents  int                  `json:"totalAgents"`
	SyncedAgents int                  `json:"syncedAgents"`
	FailedAgents int                  `json:"failedAgents"`
	CurrentAgent string               `json:"currentAgent"`
	Message      string               `json:"message"`
	StartedAt    string               `json:"startedAt,omitempty"`
	FinishedAt   string               `json:"finishedAt,omitempty"`
	AgentResults []RefreshAgentResult `json:"agentResults"`
}

type RefreshAgentResult struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func NewQueuedRefreshProgress() RefreshProgress {
	return RefreshProgress{Message: "刷新任务已排队", AgentResults: []RefreshAgentResult{}}
}

func (s *Service) StartRefreshWorker(ctx context.Context) {
	s.workerOnce.Do(func() {
		for _, taskType := range []string{RefreshAllTaskType, RefreshFastTaskType} {
			if err := s.store.FailRunningTasksByType(ctx, taskType, "后端重启，刷新任务未完成"); err != nil {
				s.logger.Warn("fail stale refresh tasks failed", "type", taskType, "error", err)
			}
		}
		go s.runRefreshWorker(ctx)
		go s.requeueRefreshTasks(ctx, RefreshAllTaskType)
		go s.requeueRefreshTasks(ctx, RefreshFastTaskType)
	})
}

func (s *Service) EnqueueRefreshTask(ctx context.Context, task domain.Task) error {
	select {
	case s.refreshQueue <- task.ID:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("刷新队列已满")
	}
}

func (s *Service) requeueRefreshTasks(ctx context.Context, taskType string) {
	tasks, err := s.store.ListQueuedTasksByType(ctx, taskType)
	if err != nil {
		s.logger.Warn("list queued refresh tasks failed", "error", err)
		return
	}
	for _, task := range tasks {
		if err := s.EnqueueRefreshTask(ctx, task); err != nil {
			s.logger.Warn("requeue refresh task failed", "task", task.ID, "error", err)
		}
	}
}

func (s *Service) runRefreshWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case taskID := <-s.refreshQueue:
			s.runRefreshTask(ctx, taskID)
		}
	}
}

func (s *Service) runRefreshTask(parent context.Context, taskID string) {
	claimed, err := s.store.ClaimTask(parent, taskID)
	if err != nil {
		s.logger.Warn("claim refresh task failed", "task", taskID, "error", err)
		return
	}
	if !claimed {
		return
	}

	task, err := s.store.GetTask(parent, taskID)
	if err != nil {
		s.logger.Warn("get refresh task failed", "task", taskID, "error", err)
		return
	}
	mode := SyncFull
	if task.Type == RefreshFastTaskType {
		mode = SyncFast
	}

	agents, err := s.store.ListAgents(parent)
	progress := RefreshProgress{StartedAt: time.Now().UTC().Format(time.RFC3339), AgentResults: []RefreshAgentResult{}}
	if err != nil {
		progress.Message = "读取 Agent 列表失败"
		_ = s.store.FinishTask(parent, taskID, "failed", progress, refreshTaskErrorMessage(err))
		s.Broadcast("sync.failed")
		return
	}

	progress.TotalAgents = len(agents)
	progress.Message = "刷新任务运行中"
	_ = s.store.UpdateTaskProgress(parent, taskID, progress, "")
	s.Broadcast("sync.started")

	results := s.syncAgents(parent, agents, mode)
	for _, result := range results {
		select {
		case <-parent.Done():
			progress.Message = "刷新任务已停止"
			progress.FinishedAt = time.Now().UTC().Format(time.RFC3339)
			_ = s.store.FinishTask(context.Background(), taskID, "failed", progress, refreshTaskErrorMessage(parent.Err()))
			return
		default:
		}

		progress.CurrentAgent = result.Name
		progress.Message = "已同步 Agent " + result.Name
		_ = s.store.UpdateTaskProgress(parent, taskID, progress, "")
		s.Broadcast("sync.progress")

		if result.Error != "" {
			progress.FailedAgents++
			progress.AgentResults = append(progress.AgentResults, RefreshAgentResult{ID: result.ID, Name: result.Name, Status: "failed", Error: result.Error})
		} else {
			progress.SyncedAgents++
			progress.AgentResults = append(progress.AgentResults, RefreshAgentResult{ID: result.ID, Name: result.Name, Status: "completed"})
		}
		progress.Message = refreshProgressMessage(progress)
		_ = s.store.UpdateTaskProgress(parent, taskID, progress, "")
		s.Broadcast("sync.progress")
	}

	progress.CurrentAgent = ""
	progress.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	progress.Message = refreshProgressMessage(progress)
	status := "completed"
	errorMessage := ""
	if progress.TotalAgents > 0 && progress.SyncedAgents == 0 && progress.FailedAgents > 0 {
		status = "failed"
		errorMessage = joinRefreshErrors(progress.AgentResults)
	}
	if err := s.store.FinishTask(parent, taskID, status, progress, errorMessage); err != nil {
		s.logger.Warn("finish refresh task failed", "task", taskID, "error", err)
	}
	s.Broadcast("runtime.updated")
	s.Broadcast("sync.finished")
}

func refreshTaskErrorMessage(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled):
		return "刷新任务已取消"
	case errors.Is(err, context.DeadlineExceeded):
		return "刷新任务超时"
	default:
		return err.Error()
	}
}

func refreshProgressMessage(progress RefreshProgress) string {
	if progress.TotalAgents == 0 {
		return "暂无可同步的 Agent"
	}
	return fmt.Sprintf("刷新已完成 %d/%d，异常 %d", progress.SyncedAgents, progress.TotalAgents, progress.FailedAgents)
}

func joinRefreshErrors(results []RefreshAgentResult) string {
	errors := make([]string, 0)
	for _, result := range results {
		if result.Error != "" {
			errors = append(errors, result.Name+": "+result.Error)
		}
	}
	return strings.Join(errors, "; ")
}
