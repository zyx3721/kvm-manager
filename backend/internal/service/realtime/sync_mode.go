package realtime

import (
	"context"
	"sync"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/pkg/agent"
)

type SyncMode string

const (
	SyncFast SyncMode = "fast"
	SyncFull SyncMode = "full"
	SyncVMs  SyncMode = "vms"
)

type syncAgentResult struct {
	ID    string
	Name  string
	Error string
}

func (s *Service) syncAgents(ctx context.Context, agents []domain.Agent, mode SyncMode) []syncAgentResult {
	if len(agents) == 0 {
		return nil
	}
	concurrency := s.syncConcurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(agents) {
		concurrency = len(agents)
	}
	jobs := make(chan domain.Agent)
	results := make(chan syncAgentResult, len(agents))
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				result := syncAgentResult{ID: item.ID, Name: item.Name}
				if err := s.SyncAgentWithMode(ctx, item, mode); err != nil {
					result.Error = agent.UserFacingErrorMessage(err)
				}
				results <- result
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, item := range agents {
			select {
			case <-ctx.Done():
				return
			case jobs <- item:
			}
		}
	}()
	wg.Wait()
	close(results)
	items := make([]syncAgentResult, 0, len(agents))
	for result := range results {
		items = append(items, result)
	}
	return items
}
