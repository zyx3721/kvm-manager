package realtime

import (
	"context"
	"time"
)

func (s *Service) StartMetricRetention(ctx context.Context, retentionDays int) {
	if retentionDays <= 0 {
		s.logger.Info("metric retention cleanup disabled")
		return
	}
	go func() {
		s.cleanupOldMetrics(ctx, retentionDays)
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.cleanupOldMetrics(ctx, retentionDays)
			}
		}
	}()
}

func (s *Service) cleanupOldMetrics(ctx context.Context, retentionDays int) {
	before := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	if err := s.store.DeleteMetricSamplesBefore(ctx, before); err != nil {
		s.logger.Warn("delete old metric samples failed", "error", err)
	}
}
