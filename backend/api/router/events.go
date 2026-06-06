package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/internal/service/realtime"
)

func (r *router) handleEvents(w http.ResponseWriter, req *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "stream_unsupported", "当前连接不支持事件流")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch, unsubscribe := r.runtime.Subscribe()
	defer unsubscribe()
	fmt.Fprint(w, "event: connected\ndata: {\"type\":\"connected\"}\n\n")
	flusher.Flush()
	for {
		select {
		case <-req.Context().Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			payload, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, payload)
			flusher.Flush()
		}
	}
}

func (r *router) handleRefresh(w http.ResponseWriter, req *http.Request) {
	if !r.ensurePermission(w, req, "agents.manage") {
		return
	}
	if task, err := r.store.FindActiveTaskByType(req.Context(), realtime.RefreshAllTaskType); err == nil {
		writeJSON(w, http.StatusAccepted, map[string]any{"status": "running", "task": task})
		return
	} else if !errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "find_refresh_task_failed", "查找刷新任务失败")
		return
	}

	session := currentSession(req)
	task, err := r.store.CreateTask(req.Context(), realtime.RefreshAllTaskType, "queued", "runtime", "", realtime.NewQueuedRefreshProgress(), session.User.ID, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_refresh_task_failed", "创建刷新任务失败")
		return
	}
	if err := r.runtime.EnqueueRefreshTask(req.Context(), task); err != nil {
		_ = r.store.FinishTask(req.Context(), task.ID, "failed", realtime.NewQueuedRefreshProgress(), "刷新队列繁忙，请稍后重试")
		writeError(w, http.StatusServiceUnavailable, "enqueue_refresh_task_failed", "刷新队列繁忙，请稍后重试")
		return
	}
	r.runtime.Broadcast("sync.queued")
	_ = r.store.WriteAudit(req.Context(), session.User.ID, "runtime.refresh", "runtime", task.ID, repository.ClientIP(req), map[string]any{"task": task.ID})
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "queued", "task": task})
}
