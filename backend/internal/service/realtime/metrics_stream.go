package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/internal/repository"
)

const (
	metricStreamKey   = "kvm:metrics:samples"
	metricStreamGroup = "kvm-manager-metric-writers"
)

type MetricSampleEvent struct {
	Host repository.HostMetricSample `json:"host"`
	VMs  []repository.VMMetricSample `json:"vms"`
}

func buildMetricEvent(agentID string, host domain.Host, vms map[string]domain.VirtualMachine, collectedAt time.Time) MetricSampleEvent {
	items := make([]repository.VMMetricSample, 0, len(vms))
	for _, vm := range vms {
		items = append(items, repository.VMMetricSample{
			AgentID:              agentID,
			VMID:                 vm.ID,
			VMName:               vm.Name,
			Status:               vm.Status,
			CPUUsage:             vm.CPUUsage,
			CPUUsageAvailable:    vm.CPUUsageAvailable,
			MemoryUsage:          vm.MemoryUsage,
			MemoryUsageAvailable: vm.MemoryUsageAvailable,
			DiskUsage:            vm.DiskUsage,
			DiskUsageAvailable:   vm.DiskUsageAvailable,
			DiskReadBytesPerSec:  vm.DiskReadBytesPerSec,
			DiskWriteBytesPerSec: vm.DiskWriteBytesPerSec,
			NetworkRxBytesPerSec: vm.NetworkRxBytesPerSec,
			NetworkTxBytesPerSec: vm.NetworkTxBytesPerSec,
			UptimeSeconds:        vm.UptimeSeconds,
			CollectedAt:          collectedAt,
		})
	}
	return MetricSampleEvent{
		Host: repository.HostMetricSample{
			AgentID:              agentID,
			HostName:             host.Name,
			CPUUsage:             host.CPUUsage,
			MemoryUsage:          host.MemoryUsage,
			MemoryBytes:          host.MemoryBytes,
			StorageUsage:         host.StorageUsage,
			StorageBytes:         host.StorageBytes,
			DiskReadBytesPerSec:  host.DiskReadBytesPerSec,
			DiskWriteBytesPerSec: host.DiskWriteBytesPerSec,
			NetworkRxBytesPerSec: host.NetworkRxBytesPerSec,
			NetworkTxBytesPerSec: host.NetworkTxBytesPerSec,
			VMCount:              host.VMCount,
			CollectedAt:          collectedAt,
		},
		VMs: items,
	}
}

func appendMetricEvent(ctx context.Context, client redis.Cmdable, event MetricSampleEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return client.XAdd(ctx, &redis.XAddArgs{
		Stream: metricStreamKey,
		Values: map[string]any{"payload": string(payload)},
	}).Err()
}

func trimMetricStream(ctx context.Context, client redis.Cmdable, maxLen int64) error {
	if maxLen <= 0 {
		return nil
	}
	return client.XTrimMaxLenApprox(ctx, metricStreamKey, maxLen, 1000).Err()
}

func ensureMetricStreamGroup(ctx context.Context, client redis.Cmdable) error {
	err := client.XGroupCreateMkStream(ctx, metricStreamKey, metricStreamGroup, "0").Err()
	if err == nil || stringsContains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return err
}

func (s *Service) StartMetricWriter(ctx context.Context, client redis.Cmdable) {
	if client == nil {
		return
	}
	go s.runMetricWriter(ctx, client)
}

func (s *Service) runMetricWriter(ctx context.Context, client redis.Cmdable) {
	if err := ensureMetricStreamGroup(ctx, client); err != nil {
		s.logger.Warn("ensure metric stream group failed", "error", err)
		return
	}
	consumer := fmt.Sprintf("writer-%d", time.Now().UnixNano())
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		streams, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    metricStreamGroup,
			Consumer: consumer,
			Streams:  []string{metricStreamKey, ">"},
			Count:    10,
			Block:    5 * time.Second,
		}).Result()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			s.logger.Warn("read metric stream failed", "error", err)
			continue
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				if err := s.consumeMetricMessage(ctx, client, message); err != nil {
					s.logger.Warn("consume metric message failed", "message", message.ID, "error", err)
				}
			}
		}
		s.claimPendingMetricMessages(ctx, client, consumer)
	}
}

func (s *Service) claimPendingMetricMessages(ctx context.Context, client redis.Cmdable, consumer string) {
	messages, _, err := client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   metricStreamKey,
		Group:    metricStreamGroup,
		Consumer: consumer,
		MinIdle:  30 * time.Second,
		Start:    "0-0",
		Count:    10,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return
	}
	if err != nil {
		s.logger.Warn("claim pending metric messages failed", "error", err)
		return
	}
	for _, message := range messages {
		if err := s.consumeMetricMessage(ctx, client, message); err != nil {
			s.logger.Warn("consume pending metric message failed", "message", message.ID, "error", err)
		}
	}
}

func (s *Service) consumeMetricMessage(ctx context.Context, client redis.Cmdable, message redis.XMessage) error {
	raw, ok := message.Values["payload"].(string)
	if !ok || raw == "" {
		return client.XAck(ctx, metricStreamKey, metricStreamGroup, message.ID).Err()
	}
	var event MetricSampleEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		return err
	}
	if err := s.store.InsertMetricSamples(ctx, event.Host, event.VMs); err != nil {
		return err
	}
	return client.XAck(ctx, metricStreamKey, metricStreamGroup, message.ID).Err()
}

func stringsContains(value string, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
