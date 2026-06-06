package realtime

import (
	"context"
	"time"
)

func (s *Service) StartMetricRollups(ctx context.Context) {
	go func() {
		s.updateMetricRollups(ctx)
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.updateMetricRollups(ctx)
			}
		}
	}()
}

func (s *Service) updateMetricRollups(ctx context.Context) {
	now := time.Now().UTC()
	rollups := []struct {
		label  string
		bucket time.Duration
		since  time.Time
	}{
		{label: "5m", bucket: 5 * time.Minute, since: now.Add(-48 * time.Hour)},
		{label: "30m", bucket: 30 * time.Minute, since: now.Add(-14 * 24 * time.Hour)},
		{label: "1h", bucket: time.Hour, since: now.Add(-60 * 24 * time.Hour)},
		{label: "24h", bucket: 24 * time.Hour, since: now.Add(-180 * 24 * time.Hour)},
	}
	for _, item := range rollups {
		if err := s.store.UpsertMetricRollups(ctx, item.label, item.bucket, item.since); err != nil {
			s.logger.Warn("upsert metric rollups failed", "bucket", item.label, "error", err)
		}
	}
}
