package realtime

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"kvm-manager/backend/internal/repository"
)

func (s *Service) StartScheduledRefresh(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		s.logger.Info("scheduled runtime refresh disabled")
		return
	}
	go func() {
		s.logger.Info("scheduled runtime refresh started", slog.String("interval", interval.String()))
		if err := s.enqueueScheduledRefresh(ctx); err != nil {
			s.logger.Warn("enqueue initial scheduled refresh failed", "error", err)
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.enqueueScheduledRefresh(ctx); err != nil {
					s.logger.Warn("enqueue scheduled refresh failed", "error", err)
				}
			}
		}
	}()
}

func (s *Service) StartScheduledDeepRefresh(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		s.logger.Info("scheduled deep runtime refresh disabled")
		return
	}
	go func() {
		s.logger.Info("scheduled deep runtime refresh started", slog.String("interval", interval.String()))
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.enqueueScheduledDeepRefresh(ctx); err != nil {
					s.logger.Warn("enqueue scheduled deep refresh failed", "error", err)
				}
			}
		}
	}()
}

func (s *Service) enqueueScheduledRefresh(ctx context.Context) error {
	if _, err := s.store.FindActiveTaskByType(ctx, RefreshAllTaskType); err == nil {
		return nil
	} else if !errorsIsNotFound(err) {
		return err
	}
	if _, err := s.store.FindActiveTaskByType(ctx, RefreshFastTaskType); err == nil {
		return nil
	} else if !errorsIsNotFound(err) {
		return err
	}
	hasAgents, err := s.hasRegisteredAgents(ctx)
	if err != nil {
		return err
	}
	if !hasAgents {
		return nil
	}
	task, err := s.store.CreateTask(ctx, RefreshFastTaskType, "queued", "runtime", "", NewQueuedRefreshProgress(), "", "")
	if err != nil {
		return err
	}
	if err := s.EnqueueRefreshTask(ctx, task); err != nil {
		_ = s.store.FinishTask(ctx, task.ID, "failed", NewQueuedRefreshProgress(), refreshTaskErrorMessage(err))
		return err
	}
	s.Broadcast("sync.queued")
	return nil
}

func (s *Service) enqueueScheduledDeepRefresh(ctx context.Context) error {
	if _, err := s.store.FindActiveTaskByType(ctx, RefreshAllTaskType); err == nil {
		return nil
	} else if !errorsIsNotFound(err) {
		return err
	}
	if _, err := s.store.FindActiveTaskByType(ctx, RefreshFastTaskType); err == nil {
		return nil
	} else if !errorsIsNotFound(err) {
		return err
	}
	hasAgents, err := s.hasRegisteredAgents(ctx)
	if err != nil {
		return err
	}
	if !hasAgents {
		return nil
	}
	task, err := s.store.CreateTask(ctx, RefreshAllTaskType, "queued", "runtime", "", NewQueuedRefreshProgress(), "", "")
	if err != nil {
		return err
	}
	if err := s.EnqueueRefreshTask(ctx, task); err != nil {
		_ = s.store.FinishTask(ctx, task.ID, "failed", NewQueuedRefreshProgress(), refreshTaskErrorMessage(err))
		return err
	}
	s.Broadcast("sync.queued")
	return nil
}

func (s *Service) hasRegisteredAgents(ctx context.Context) (bool, error) {
	agents, err := s.store.ListAgents(ctx)
	if err != nil {
		return false, err
	}
	return len(agents) > 0, nil
}

func errorsIsNotFound(err error) bool {
	return errors.Is(err, repository.ErrNotFound)
}
