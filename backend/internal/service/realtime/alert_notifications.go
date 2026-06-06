package realtime

import (
	"context"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/internal/service/notification"
)

func (s *Service) queueProblemNotification(ctx context.Context, alert domain.Alert) {
	if err := s.store.EnsureAlertNotificationDeliveries(ctx, alert, notification.EventTypeProblem); err != nil {
		s.logger.Warn("queue problem alert notification failed", "alert", alert.ID, "error", err)
	}
}

func (s *Service) queueRecoveryNotifications(ctx context.Context, alerts []domain.Alert) {
	for _, alert := range alerts {
		if err := s.store.EnsureAlertNotificationDeliveries(ctx, alert, notification.EventTypeRecovery); err != nil {
			s.logger.Warn("queue recovery alert notification failed", "alert", alert.ID, "error", err)
		}
	}
}

func (s *Service) resolveActiveAlert(ctx context.Context, sourceType string, sourceID string, title string) {
	alerts, err := s.store.ResolveActiveAlertReturning(ctx, sourceType, sourceID, title)
	if err != nil {
		s.logger.Warn("resolve active alert failed", "sourceType", sourceType, "sourceId", sourceID, "title", title, "error", err)
		return
	}
	s.queueRecoveryNotifications(ctx, alerts)
}

func (s *Service) resolveActiveAlertsBySource(ctx context.Context, sourceType string, sourceID string) {
	alerts, err := s.store.ResolveActiveAlertsBySourceReturning(ctx, sourceType, sourceID)
	if err != nil {
		s.logger.Warn("resolve active alerts by source failed", "sourceType", sourceType, "sourceId", sourceID, "error", err)
		return
	}
	s.queueRecoveryNotifications(ctx, alerts)
}

func (s *Service) QueueResolvedAlertNotifications(ctx context.Context, alerts []domain.Alert) {
	s.queueRecoveryNotifications(ctx, alerts)
	s.notifyPendingAlerts(ctx)
}
