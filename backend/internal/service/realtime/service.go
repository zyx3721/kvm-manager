package realtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"kvm-manager/backend/internal/domain"
	"kvm-manager/backend/internal/repository"
	"kvm-manager/backend/pkg/agent"
	"kvm-manager/backend/pkg/tokencrypto"
)

type AlertNotifier interface {
	NotifyDelivery(ctx context.Context, delivery domain.AlertNotificationDelivery)
}

const (
	defaultSyncFastTimeout = 12 * time.Second
	defaultSyncFullTimeout = 60 * time.Second
	vmActionFullSyncDelay  = 8 * time.Second
	offlineFailureLimit    = 3
	resourceAlertLimit     = 85
	alertConsecutiveLimit  = 3
)

type alertRuntimeConfig struct {
	ResourceLimit         int
	ConsecutiveLimit      int
	OfflineFailureLimit   int
	NotificationBatchSize int
}

type Event struct {
	Type    string         `json:"type"`
	At      string         `json:"at"`
	Payload map[string]any `json:"payload,omitempty"`
}

type Service struct {
	store              *repository.Store
	logger             *slog.Logger
	secret             string
	runtimeStore       *RedisRuntimeStore
	metricStream       redis.Cmdable
	notifier           AlertNotifier
	refreshQueue       chan string
	syncFastTimeout    time.Duration
	syncFullTimeout    time.Duration
	syncConcurrency    int
	metricStreamMaxLen int64
	workerOnce         sync.Once
	mu                 sync.RWMutex
	subs               map[chan Event]struct{}
	alertMu            sync.Mutex
	alertCounts        map[string]int
}

type Options struct {
	SyncFastTimeout    time.Duration
	SyncFullTimeout    time.Duration
	SyncConcurrency    int
	MetricStreamMaxLen int64
}

func (s *Service) SetNotifier(notifier AlertNotifier) {
	s.notifier = notifier
}

func New(store *repository.Store, logger *slog.Logger, secret string, runtimeStore *RedisRuntimeStore, metricStream redis.Cmdable, syncConcurrency int, metricStreamMaxLen int64) *Service {
	return NewWithOptions(store, logger, secret, runtimeStore, metricStream, Options{
		SyncConcurrency:    syncConcurrency,
		MetricStreamMaxLen: metricStreamMaxLen,
	})
}

func NewWithOptions(store *repository.Store, logger *slog.Logger, secret string, runtimeStore *RedisRuntimeStore, metricStream redis.Cmdable, options Options) *Service {
	if options.SyncFastTimeout <= 0 {
		options.SyncFastTimeout = defaultSyncFastTimeout
	}
	if options.SyncFullTimeout <= 0 {
		options.SyncFullTimeout = defaultSyncFullTimeout
	}
	return &Service{
		store:              store,
		logger:             logger,
		secret:             secret,
		runtimeStore:       runtimeStore,
		metricStream:       metricStream,
		refreshQueue:       make(chan string, 32),
		syncFastTimeout:    options.SyncFastTimeout,
		syncFullTimeout:    options.SyncFullTimeout,
		syncConcurrency:    options.SyncConcurrency,
		metricStreamMaxLen: options.MetricStreamMaxLen,
		subs:               map[chan Event]struct{}{},
		alertCounts:        map[string]int{},
	}
}

func (s *Service) Subscribe() (chan Event, func()) {
	ch := make(chan Event, 8)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		if _, ok := s.subs[ch]; ok {
			delete(s.subs, ch)
			close(ch)
		}
		s.mu.Unlock()
	}
}

func (s *Service) Broadcast(eventType string) {
	s.BroadcastPayload(eventType, nil)
}

func (s *Service) BroadcastPayload(eventType string, payload map[string]any) {
	event := Event{Type: eventType, At: time.Now().UTC().Format(time.RFC3339), Payload: payload}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for ch := range s.subs {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *Service) SyncAll(ctx context.Context) {
	agents, err := s.store.ListAgents(ctx)
	if err != nil {
		s.logger.Warn("list agents for realtime sync failed", "error", err)
		return
	}
	changed := false
	for _, item := range agents {
		if err := s.SyncAgent(ctx, item); err != nil {
			s.logger.Warn("realtime agent sync failed", "agent", item.ID, "error", err)
		} else {
			changed = true
		}
	}
	if changed {
		s.Broadcast("runtime.updated")
	}
}

func (s *Service) SyncAgent(ctx context.Context, item domain.Agent) error {
	return s.SyncAgentWithMode(ctx, item, SyncFull)
}

func (s *Service) SyncAgentWithMode(ctx context.Context, item domain.Agent, mode SyncMode) error {
	token, err := tokencrypto.Open(s.secret, item.TokenCiphertext)
	if err != nil || token == "" {
		message := "Agent 令牌不可用于自动刷新，请重新保存 Agent"
		if err != nil {
			message = agent.UserFacingErrorMessage(err)
		}
		s.recordAgentSyncFailure(ctx, item, message)
		return fmt.Errorf("%s", message)
	}
	return s.syncWithToken(ctx, item, token, mode)
}

func (s *Service) recordAgentSyncFailure(ctx context.Context, item domain.Agent, message string) {
	config := s.loadAlertRuntimeConfig(ctx)
	_ = s.store.UpdateAgentSyncFailure(ctx, item.ID, message, config.OfflineFailureLimit)
	if item.FailureCount+1 < config.OfflineFailureLimit {
		return
	}
	offlineAlertTitle := fmt.Sprintf("Agent %s 离线", item.Name)
	_ = s.store.UpsertActiveAlert(ctx, "critical", "agent", item.ID, offlineAlertTitle, "连续同步失败，Agent 已标记为离线", map[string]any{
		"agent":        item.Name,
		"endpoint":     item.Endpoint,
		"lastError":    message,
		"failureCount": item.FailureCount + 1,
	})
	s.notifyPendingAlerts(ctx)
}
func (s *Service) SyncAgentWithToken(ctx context.Context, id string, token string) error {
	return s.SyncAgentWithTokenMode(ctx, id, token, SyncFull)
}

func (s *Service) SyncAgentWithTokenFast(ctx context.Context, id string, token string) error {
	return s.SyncAgentWithTokenMode(ctx, id, token, SyncFast)
}

func (s *Service) SyncAgentWithTokenMode(ctx context.Context, id string, token string, mode SyncMode) error {
	item, err := s.store.GetAgent(ctx, id)
	if err != nil {
		return err
	}
	return s.syncWithToken(ctx, item, token, mode)
}

func (s *Service) SyncAgentWithTokenDelayedFull(ctx context.Context, id string, token string) {
	go func() {
		timer := time.NewTimer(vmActionFullSyncDelay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if err := s.SyncAgentWithTokenMode(context.Background(), id, token, SyncFull); err != nil {
			s.logger.Warn("delayed full sync after vm action failed", "agent", id, "error", err)
			return
		}
		s.Broadcast("runtime.updated")
	}()
}

func (s *Service) UpdateVMStatus(id string, status string) (domain.VirtualMachine, bool) {
	vm, ok, err := s.runtimeStore.GetVM(context.Background(), id)
	if err != nil {
		s.logger.Warn("get vm from runtime cache failed", "vm", id, "error", err)
		return domain.VirtualMachine{}, false
	}
	if !ok {
		return domain.VirtualMachine{}, false
	}
	vm.Status = status
	vm.UpdatedAt = time.Now().UTC()
	hosts, err := s.runtimeStore.ListHosts(context.Background())
	if err != nil {
		s.logger.Warn("list hosts from runtime cache failed", "error", err)
		return domain.VirtualMachine{}, false
	}
	for _, host := range hosts {
		if host.ID == vm.HostID {
			if err := s.runtimeStore.UpdateVMRuntime(context.Background(), vm.HostID, host, vm); err != nil {
				s.logger.Warn("update vm runtime status failed", "vm", id, "error", err)
				return domain.VirtualMachine{}, false
			}
			vm = s.applyTemplateMarkToVM(context.Background(), vm)
			return vm, true
		}
	}
	return domain.VirtualMachine{}, false
}

func (s *Service) RemoveVM(id string, agentID string) bool {
	if err := s.runtimeStore.RemoveVMRuntime(context.Background(), agentID, id); err != nil {
		s.logger.Warn("remove vm from runtime cache failed", "vm", id, "agent", agentID, "error", err)
		return false
	}
	return true
}

func (s *Service) SyncVMWithToken(ctx context.Context, id string, token string, vmID string, vmName string) (domain.VirtualMachine, error) {
	item, err := s.store.GetAgent(ctx, id)
	if err != nil {
		return domain.VirtualMachine{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, s.syncFullTimeout)
	defer cancel()
	_ = s.store.MarkAgentSyncStarted(ctx, item.ID)
	client := agent.NewClientWithTimeout(item.TLSInsecure, s.syncFullTimeout)
	cfg := agent.Config{Endpoint: item.Endpoint, Token: token, TLSInsecure: item.TLSInsecure}
	info, err := client.HostInfo(ctx, cfg)
	if err != nil {
		s.recordAgentSyncFailure(ctx, item, agent.UserFacingErrorMessage(err))
		s.Broadcast("sync.failed")
		return domain.VirtualMachine{}, err
	}
	remote, err := client.VM(ctx, cfg, vmName)
	if err != nil {
		s.recordAgentSyncFailure(ctx, item, agent.UserFacingErrorMessage(err))
		s.Broadcast("sync.failed")
		return domain.VirtualMachine{}, err
	}
	hostVMCount := 1
	if hosts, err := s.runtimeStore.ListHosts(ctx); err == nil {
		for _, current := range hosts {
			if current.ID == item.ID {
				hostVMCount = current.VMCount
				break
			}
		}
	}
	now := time.Now().UTC()
	host := domain.Host{
		ID:                   item.ID,
		Name:                 item.Name,
		Address:              firstNonEmpty(info.HostAddress, item.Endpoint),
		Hostname:             info.Hostname,
		Cluster:              "default",
		Status:               normalizeHostStatus(info.Status),
		CPUCores:             info.CPUCores,
		CPUUsage:             clampPercent(info.CPUUsage),
		MemoryBytes:          info.MemoryBytes,
		MemoryUsage:          clampPercent(info.MemoryUsage),
		StorageBytes:         info.StorageBytes,
		StorageUsage:         clampPercent(info.StorageUsage),
		DiskReadBytesPerSec:  info.DiskReadBytesPerSec,
		DiskWriteBytesPerSec: info.DiskWriteBytesPerSec,
		NetworkRxBytesPerSec: info.NetworkRxBytesPerSec,
		NetworkTxBytesPerSec: info.NetworkTxBytesPerSec,
		VMCount:              hostVMCount,
		KVMVersion:           info.KVMVersion,
		KVMFullVersion:       info.KVMFullVersion,
		CreatedAt:            item.CreatedAt,
		UpdatedAt:            now,
	}
	refreshed := buildRuntimeVMs(item, info, []agent.VirtualMachine{remote}, now)
	vm, ok := refreshed[vmRuntimeID(item.ID, remote.Name, remote.UUID)]
	if !ok {
		return domain.VirtualMachine{}, fmt.Errorf("vm refresh returned empty result")
	}
	if strings.TrimSpace(vmID) != "" && vm.ID != vmID {
		vm.ID = vmID
	}
	vm = s.applyTemplateMarkToVM(ctx, vm)
	if ok, err := s.ensureAgentStillRegistered(ctx, item.ID); err != nil {
		return domain.VirtualMachine{}, err
	} else if !ok {
		return domain.VirtualMachine{}, repository.ErrNotFound
	}
	if err := s.runtimeStore.UpdateVMRuntime(ctx, item.ID, host, vm); err != nil {
		if errors.Is(err, errAgentRuntimeDeleted) {
			return domain.VirtualMachine{}, repository.ErrNotFound
		}
		return domain.VirtualMachine{}, err
	}
	if s.metricStream != nil {
		if err := appendMetricEvent(ctx, s.metricStream, buildMetricEvent(item.ID, host, map[string]domain.VirtualMachine{vm.ID: vm}, now)); err != nil {
			s.logger.Warn("append metric sample event failed", "agent", item.ID, "error", err)
		}
		if err := trimMetricStream(ctx, s.metricStream, s.metricStreamMaxLen); err != nil {
			s.logger.Warn("trim metric stream failed", "error", err)
		}
	}
	s.evaluateRuntimeAlerts(ctx, item, host, map[string]domain.VirtualMachine{vm.ID: vm})
	s.notifyPendingAlerts(ctx)
	_ = s.store.UpdateAgentSyncSuccess(ctx, item.ID, info.KVMVersion, info.Capabilities)
	s.resolveActiveAlertsBySource(ctx, "agent", item.ID)
	s.notifyPendingAlerts(ctx)
	return vm, nil
}

func (s *Service) syncWithToken(ctx context.Context, item domain.Agent, token string, mode SyncMode) error {
	syncTimeout := s.syncFastTimeout
	if mode == SyncFull {
		syncTimeout = s.syncFullTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()
	_ = s.store.MarkAgentSyncStarted(ctx, item.ID)
	fastClient := agent.NewClientWithTimeout(item.TLSInsecure, s.syncFastTimeout)
	fullClient := agent.NewClientWithTimeout(item.TLSInsecure, s.syncFullTimeout)
	client := fastClient
	if mode == SyncFull {
		client = fullClient
	}
	cfg := agent.Config{Endpoint: item.Endpoint, Token: token, TLSInsecure: item.TLSInsecure}
	info, err := client.HostInfo(ctx, cfg)
	if err != nil {
		s.recordAgentSyncFailure(ctx, item, agent.UserFacingErrorMessage(err))
		s.Broadcast("sync.failed")
		return err
	}
	var remoteVMs []agent.VirtualMachine
	if mode == SyncFast {
		remoteVMs, err = fastClient.ListVMsFast(ctx, cfg)
	} else {
		remoteVMs, err = fullClient.ListVMs(ctx, cfg)
	}
	if err != nil {
		s.recordAgentSyncFailure(ctx, item, agent.UserFacingErrorMessage(err))
		s.Broadcast("sync.failed")
		return err
	}

	now := time.Now().UTC()
	host := domain.Host{
		ID:                   item.ID,
		Name:                 item.Name,
		Address:              firstNonEmpty(info.HostAddress, item.Endpoint),
		Hostname:             info.Hostname,
		Cluster:              "default",
		Status:               normalizeHostStatus(info.Status),
		CPUCores:             info.CPUCores,
		CPUUsage:             clampPercent(info.CPUUsage),
		MemoryBytes:          info.MemoryBytes,
		MemoryUsage:          clampPercent(info.MemoryUsage),
		StorageBytes:         info.StorageBytes,
		StorageUsage:         clampPercent(info.StorageUsage),
		DiskReadBytesPerSec:  info.DiskReadBytesPerSec,
		DiskWriteBytesPerSec: info.DiskWriteBytesPerSec,
		NetworkRxBytesPerSec: info.NetworkRxBytesPerSec,
		NetworkTxBytesPerSec: info.NetworkTxBytesPerSec,
		VMCount:              len(remoteVMs),
		KVMVersion:           info.KVMVersion,
		KVMFullVersion:       info.KVMFullVersion,
		CreatedAt:            item.CreatedAt,
		UpdatedAt:            now,
	}

	vms := buildRuntimeVMs(item, info, remoteVMs, now)
	if mode == SyncFull {
		s.mergeRuntimeDetails(ctx, vms)
	}
	snapshots := make(map[string]domain.Snapshot)
	for _, remote := range remoteVMs {
		id := vmRuntimeID(item.ID, remote.Name, remote.UUID)
		if mode == SyncFull {
			remoteSnapshots, err := fullClient.ListSnapshots(ctx, cfg, remote.Name)
			if err != nil {
				s.logger.Warn("list vm snapshots failed", "agent", item.ID, "vm", remote.Name, "error", err)
				continue
			}
			for _, remoteSnapshot := range remoteSnapshots {
				snapshots[snapshotRuntimeID(id, remoteSnapshot.Name)] = buildRuntimeSnapshot(item, remote, id, remoteSnapshot, now)
			}
		}
	}

	if mode == SyncFast {
		s.mergeFastRuntime(ctx, vms)
	}
	if ok, err := s.ensureAgentStillRegistered(ctx, item.ID); err != nil {
		return err
	} else if !ok {
		return repository.ErrNotFound
	}
	if err := s.runtimeStore.SaveAgentRuntime(ctx, item.ID, host, vms, snapshots, mode == SyncFull); err != nil {
		if errors.Is(err, errAgentRuntimeDeleted) {
			return repository.ErrNotFound
		}
		return err
	}
	if s.metricStream != nil {
		if err := appendMetricEvent(ctx, s.metricStream, buildMetricEvent(item.ID, host, vms, now)); err != nil {
			s.logger.Warn("append metric sample event failed", "agent", item.ID, "error", err)
		}
		if err := trimMetricStream(ctx, s.metricStream, s.metricStreamMaxLen); err != nil {
			s.logger.Warn("trim metric stream failed", "error", err)
		}
	}

	s.evaluateRuntimeAlerts(ctx, item, host, vms)
	s.notifyPendingAlerts(ctx)
	_ = s.store.UpdateAgentSyncSuccess(ctx, item.ID, info.KVMVersion, info.Capabilities)
	s.resolveActiveAlertsBySource(ctx, "agent", item.ID)
	s.notifyPendingAlerts(ctx)
	return nil
}

func (s *Service) ensureAgentStillRegistered(ctx context.Context, agentID string) (bool, error) {
	if _, err := s.store.GetAgent(ctx, agentID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			if removeErr := s.runtimeStore.RemoveAgent(ctx, agentID); removeErr != nil {
				s.logger.Warn("remove deleted agent runtime cache failed", "agent", agentID, "error", removeErr)
				return false, removeErr
			}
			s.logger.Info("skip runtime cache update for deleted agent", "agent", agentID)
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *Service) evaluateRuntimeAlerts(ctx context.Context, item domain.Agent, host domain.Host, vms map[string]domain.VirtualMachine) {
	config := s.loadAlertRuntimeConfig(ctx)
	s.evaluateHostThresholdAlert(ctx, item, "CPU", host.CPUUsage, config.ResourceLimit, config.ConsecutiveLimit, "cpu")
	s.evaluateHostThresholdAlert(ctx, item, "内存", host.MemoryUsage, config.ResourceLimit, config.ConsecutiveLimit, "memory")
	s.evaluateHostThresholdAlert(ctx, item, "存储", host.StorageUsage, config.ResourceLimit, config.ConsecutiveLimit, "storage")
	for _, vm := range vms {
		s.evaluateVMStateAlert(ctx, item, vm)
		s.evaluateVMThresholdAlert(ctx, item, vm, "CPU", vm.CPUUsage, vm.CPUUsageAvailable, config.ResourceLimit, config.ConsecutiveLimit, "cpu")
		s.evaluateVMThresholdAlert(ctx, item, vm, "内存", vm.MemoryUsage, vm.MemoryUsageAvailable, config.ResourceLimit, config.ConsecutiveLimit, "memory")
		s.evaluateVMThresholdAlert(ctx, item, vm, "磁盘", vm.DiskUsage, vm.DiskUsageAvailable, config.ResourceLimit, config.ConsecutiveLimit, "disk")
	}
}

func (s *Service) notifyPendingAlerts(ctx context.Context) {
	if s.notifier == nil {
		return
	}
	config := s.loadAlertRuntimeConfig(ctx)
	alerts, err := s.store.ListPendingAlertNotifications(ctx, config.NotificationBatchSize)
	if err != nil {
		s.logger.Warn("list pending alert notifications failed", "error", err)
		return
	}
	for _, alert := range alerts {
		s.queueProblemNotification(ctx, alert)
	}
	deliveries, err := s.store.ListPendingAlertNotificationDeliveries(ctx, config.NotificationBatchSize)
	if err != nil {
		s.logger.Warn("list pending alert notifications failed", "error", err)
		return
	}
	for _, delivery := range deliveries {
		s.notifier.NotifyDelivery(ctx, delivery)
	}
}

func (s *Service) evaluateHostThresholdAlert(ctx context.Context, item domain.Agent, metric string, value int, limit int, consecutiveLimit int, key string) {
	title := "宿主机" + metric + "使用率过高"
	sourceID := item.ID + ":" + key
	if value >= limit {
		count := s.recordThresholdAlertSample("host", sourceID, title, true)
		if count >= consecutiveLimit {
			_ = s.store.UpsertActiveAlert(ctx, "warning", "host", sourceID, title, fmt.Sprintf("宿主机 %s %s 使用率达到 %d%%", item.Name, metric, value), map[string]any{"agent": item.Name, "metric": key, "value": value, "limit": limit, "consecutive": count})
		}
		return
	}
	s.recordThresholdAlertSample("host", sourceID, title, false)
	s.resolveActiveAlert(ctx, "host", sourceID, title)
}

func (s *Service) evaluateVMStateAlert(ctx context.Context, item domain.Agent, vm domain.VirtualMachine) {
	title := "虚拟机状态异常"
	if vm.Status == "error" || vm.Status == "unknown" {
		_ = s.store.UpsertActiveAlert(ctx, "critical", "virtual_machine", vm.ID+":state", title, fmt.Sprintf("虚拟机 %s 当前状态为 %s", vm.Name, vm.Status), vmAlertMetadata(item, vm, map[string]any{"status": vm.Status}))
		return
	}
	s.resolveActiveAlert(ctx, "virtual_machine", vm.ID+":state", title)
}

func (s *Service) evaluateVMThresholdAlert(ctx context.Context, item domain.Agent, vm domain.VirtualMachine, metric string, value int, available bool, limit int, consecutiveLimit int, key string) {
	title := "虚拟机" + metric + "使用率过高"
	sourceID := vm.ID + ":" + key
	if available && value >= limit {
		count := s.recordThresholdAlertSample("virtual_machine", sourceID, title, true)
		if count >= consecutiveLimit {
			_ = s.store.UpsertActiveAlert(ctx, "warning", "virtual_machine", sourceID, title, fmt.Sprintf("虚拟机 %s %s 使用率达到 %d%%", vm.Name, metric, value), vmAlertMetadata(item, vm, map[string]any{"metric": key, "value": value, "limit": limit, "consecutive": count}))
		}
		return
	}
	s.recordThresholdAlertSample("virtual_machine", sourceID, title, false)
	s.resolveActiveAlert(ctx, "virtual_machine", sourceID, title)
}

func (s *Service) recordThresholdAlertSample(sourceType string, sourceID string, title string, exceeded bool) int {
	counterKey := sourceType + "|" + sourceID + "|" + title
	s.alertMu.Lock()
	defer s.alertMu.Unlock()
	if !exceeded {
		delete(s.alertCounts, counterKey)
		return 0
	}
	s.alertCounts[counterKey]++
	return s.alertCounts[counterKey]
}

func (s *Service) loadAlertRuntimeConfig(ctx context.Context) alertRuntimeConfig {
	config := alertRuntimeConfig{
		ResourceLimit:         resourceAlertLimit,
		ConsecutiveLimit:      alertConsecutiveLimit,
		OfflineFailureLimit:   offlineFailureLimit,
		NotificationBatchSize: 50,
	}
	if s == nil || s.store == nil {
		return config
	}
	baseConfig, err := s.store.GetSystemBaseConfig(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("load runtime alert settings failed", "error", err)
		}
		return config
	}
	config.ResourceLimit = clampConfigInt(baseConfig.ResourceCriticalThreshold, 1, 100, resourceAlertLimit)
	config.ConsecutiveLimit = clampConfigInt(baseConfig.ResourceAlertConsecutiveCount, 1, 20, alertConsecutiveLimit)
	config.OfflineFailureLimit = clampConfigInt(baseConfig.AgentOfflineFailureCount, 1, 20, offlineFailureLimit)
	config.NotificationBatchSize = clampConfigInt(baseConfig.AlertNotificationBatchSize, 10, 100, 50)
	return config
}

func (s *Service) RemoveAgent(agentID string) {
	if err := s.runtimeStore.RemoveAgent(context.Background(), agentID); err != nil {
		s.logger.Warn("remove agent runtime cache failed", "agent", agentID, "error", err)
	}
}
func (s *Service) ListHosts() []domain.Host {
	hosts, err := s.runtimeStore.ListHosts(context.Background())
	if err != nil {
		s.logger.Warn("list hosts from runtime cache failed", "error", err)
		return []domain.Host{}
	}
	return s.filterRegisteredHosts(context.Background(), hosts)
}

func (s *Service) ListVMs(status string, query string, hostID ...string) []domain.VirtualMachine {
	filterHostID := ""
	if len(hostID) > 0 {
		filterHostID = hostID[0]
	}
	items, err := s.runtimeStore.ListVMs(context.Background(), status, query, filterHostID)
	if err != nil {
		s.logger.Warn("list vms from runtime cache failed", "error", err)
		return []domain.VirtualMachine{}
	}
	items = s.filterRegisteredVMs(context.Background(), items)
	s.applyTemplateMarksToList(context.Background(), items)
	return items
}

func (s *Service) filterRegisteredHosts(ctx context.Context, hosts []domain.Host) []domain.Host {
	agentIDs, ok := s.registeredAgentIDs(ctx)
	if !ok {
		return hosts
	}
	filtered := make([]domain.Host, 0, len(hosts))
	for _, host := range hosts {
		if _, exists := agentIDs[host.ID]; exists {
			filtered = append(filtered, host)
			continue
		}
		s.removeOrphanAgentRuntime(ctx, host.ID)
	}
	return filtered
}

func (s *Service) filterRegisteredVMs(ctx context.Context, vms []domain.VirtualMachine) []domain.VirtualMachine {
	agentIDs, ok := s.registeredAgentIDs(ctx)
	if !ok {
		return vms
	}
	filtered := make([]domain.VirtualMachine, 0, len(vms))
	removedAgents := map[string]struct{}{}
	for _, vm := range vms {
		if _, exists := agentIDs[vm.HostID]; exists {
			filtered = append(filtered, vm)
			continue
		}
		if vm.HostID != "" {
			removedAgents[vm.HostID] = struct{}{}
		}
	}
	for agentID := range removedAgents {
		s.removeOrphanAgentRuntime(ctx, agentID)
	}
	return filtered
}

func (s *Service) registeredAgentIDs(ctx context.Context) (map[string]struct{}, bool) {
	if s.store == nil {
		return nil, false
	}
	agents, err := s.store.ListAgents(ctx)
	if err != nil {
		s.logger.Warn("list registered agents for runtime filter failed", "error", err)
		return nil, false
	}
	items := make(map[string]struct{}, len(agents))
	for _, agent := range agents {
		items[agent.ID] = struct{}{}
	}
	return items, true
}

func (s *Service) removeOrphanAgentRuntime(ctx context.Context, agentID string) {
	if agentID == "" {
		return
	}
	if err := s.runtimeStore.RemoveAgent(ctx, agentID); err != nil {
		s.logger.Warn("remove orphan agent runtime cache failed", "agent", agentID, "error", err)
	}
}

func (s *Service) GetVM(id string) (domain.VirtualMachine, bool) {
	vm, ok, err := s.runtimeStore.GetVM(context.Background(), id)
	if err != nil {
		s.logger.Warn("get vm from runtime cache failed", "vm", id, "error", err)
		return domain.VirtualMachine{}, false
	}
	if ok {
		vm = s.applyTemplateMarkToVM(context.Background(), vm)
	}
	return vm, ok
}

func (s *Service) ListVMTemplates(status string, query string, hostID ...string) []domain.VirtualMachine {
	items := s.ListVMs(status, query, hostID...)
	templates := make([]domain.VirtualMachine, 0, len(items))
	for _, vm := range items {
		if vm.IsTemplate {
			templates = append(templates, vm)
		}
	}
	return templates
}

func (s *Service) ListSnapshots() []domain.Snapshot {
	items, err := s.runtimeStore.ListSnapshots(context.Background())
	if err != nil {
		s.logger.Warn("list snapshots from runtime cache failed", "error", err)
		return []domain.Snapshot{}
	}
	return items
}

func (s *Service) GetSnapshot(id string) (domain.Snapshot, bool) {
	snapshot, ok, err := s.runtimeStore.GetSnapshot(context.Background(), id)
	if err != nil {
		s.logger.Warn("get snapshot from runtime cache failed", "snapshot", id, "error", err)
		return domain.Snapshot{}, false
	}
	return snapshot, ok
}

func (s *Service) SyncSnapshotsWithToken(ctx context.Context, agentID string, token string) error {
	item, err := s.store.GetAgent(ctx, agentID)
	if err != nil {
		return err
	}
	cfg := agent.Config{Endpoint: item.Endpoint, Token: token, TLSInsecure: item.TLSInsecure}
	fullClient := agent.NewClient(item.TLSInsecure)
	vms, err := s.runtimeStore.ListVMs(ctx, "", "", agentID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	snapshots := make(map[string]domain.Snapshot)
	for _, vm := range vms {
		remoteSnapshots, err := fullClient.ListSnapshots(ctx, cfg, vm.Name)
		if err != nil {
			s.logger.Warn("list vm snapshots failed", "agent", item.ID, "vm", vm.Name, "error", err)
			continue
		}
		remoteVM := agent.VirtualMachine{Name: vm.Name}
		for _, remoteSnapshot := range remoteSnapshots {
			snapshots[snapshotRuntimeID(vm.ID, remoteSnapshot.Name)] = buildRuntimeSnapshot(item, remoteVM, vm.ID, remoteSnapshot, now)
		}
	}
	if err := s.runtimeStore.ReplaceAgentSnapshots(ctx, agentID, snapshots); err != nil {
		if errors.Is(err, errAgentRuntimeDeleted) {
			return repository.ErrNotFound
		}
		return err
	}
	return nil
}

func buildRuntimeSnapshot(item domain.Agent, remote agent.VirtualMachine, vmID string, remoteSnapshot agent.Snapshot, now time.Time) domain.Snapshot {
	createdAt := parseAgentSnapshotTime(remoteSnapshot.CreatedAt, now)
	snapshotID := snapshotRuntimeID(vmID, remoteSnapshot.Name)
	return domain.Snapshot{
		ID:          snapshotID,
		HostID:      item.ID,
		HostName:    item.Name,
		VMID:        vmID,
		VMName:      remote.Name,
		Name:        remoteSnapshot.Name,
		Description: "",
		SizeBytes:   0,
		Type:        "snapshot",
		Status:      normalizeSnapshotStatus(remoteSnapshot.Status),
		CreatedAt:   createdAt,
		UpdatedAt:   now,
	}
}

func buildRuntimeVMs(item domain.Agent, info agent.HostInfo, remoteVMs []agent.VirtualMachine, now time.Time) map[string]domain.VirtualMachine {
	vms := make(map[string]domain.VirtualMachine, len(remoteVMs))
	for _, remote := range remoteVMs {
		id := vmRuntimeID(item.ID, remote.Name, remote.UUID)
		vms[id] = domain.VirtualMachine{
			ID:                   id,
			HostID:               item.ID,
			HostName:             firstNonEmpty(info.HostAddress, item.Endpoint),
			Name:                 remote.Name,
			UUID:                 remote.UUID,
			Description:          remote.Description,
			OSType:               remote.OSType,
			Status:               normalizeVMStatus(remote.Status),
			CPUCores:             remote.CPUCores,
			MemoryBytes:          remote.MemoryBytes,
			DiskBytes:            remote.DiskBytes,
			DiskUsedBytes:        remote.DiskUsedBytes,
			Disks:                convertVMDiskList(remote.Disks),
			PrimaryIP:            remote.PrimaryIP,
			CPUUsage:             clampPercent(remote.CPUUsage),
			CPUUsageAvailable:    remote.CPUUsageAvailable,
			MemoryUsage:          clampPercent(remote.MemoryUsage),
			MemoryUsageAvailable: remote.MemoryUsageAvailable,
			DiskUsage:            clampPercent(remote.DiskUsage),
			DiskUsageAvailable:   remote.DiskUsageAvailable,
			DiskReadBytesPerSec:  remote.DiskReadBytesPerSec,
			DiskWriteBytesPerSec: remote.DiskWriteBytesPerSec,
			NetworkRxBytesPerSec: remote.NetworkRxBytesPerSec,
			NetworkTxBytesPerSec: remote.NetworkTxBytesPerSec,
			UptimeSeconds:        remote.UptimeSeconds,
			CreatedAt:            item.CreatedAt,
			UpdatedAt:            now,
		}
	}
	return vms
}

func (s *Service) applyTemplateMarksToList(ctx context.Context, items []domain.VirtualMachine) {
	if len(items) == 0 {
		return
	}
	byID := make(map[string]domain.VirtualMachine, len(items))
	for _, vm := range items {
		byID[vm.ID] = vm
	}
	s.applyTemplateMarks(ctx, byID)
	for index := range items {
		items[index] = byID[items[index].ID]
	}
}

func (s *Service) applyTemplateMarks(ctx context.Context, vms map[string]domain.VirtualMachine) {
	if len(vms) == 0 || s.store == nil {
		return
	}
	marks, err := s.store.ListVMTemplateMarks(ctx)
	if err != nil {
		s.logger.Warn("list vm template marks failed", "error", err)
		return
	}
	marksByKey := make(map[string]domain.VMTemplateMark, len(marks))
	for _, mark := range marks {
		marksByKey[mark.AgentID+":"+mark.VMUUID] = mark
	}
	for id, vm := range vms {
		if strings.TrimSpace(vm.UUID) == "" {
			continue
		}
		mark, ok := marksByKey[vm.HostID+":"+vm.UUID]
		if !ok {
			continue
		}
		vm.IsTemplate = true
		vm.TemplateID = mark.ID
		vm.TemplateName = mark.Name
		vm.TemplateDescription = mark.Description
		vms[id] = vm
	}
}

func (s *Service) applyTemplateMarkToVM(ctx context.Context, vm domain.VirtualMachine) domain.VirtualMachine {
	if s.store == nil || strings.TrimSpace(vm.UUID) == "" {
		return vm
	}
	mark, err := s.store.GetVMTemplateMark(ctx, vm.HostID, vm.UUID)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			s.logger.Warn("get vm template mark failed", "error", err, "vm", vm.ID)
		}
		return vm
	}
	vm.IsTemplate = true
	vm.TemplateID = mark.ID
	vm.TemplateName = mark.Name
	vm.TemplateDescription = mark.Description
	return vm
}

func (s *Service) DashboardSummary(recentEvents []domain.AuditLog, activeAlerts []domain.Alert) domain.DashboardSummary {
	hosts := s.ListHosts()
	vms := s.ListVMs("", "")
	summary := domain.DashboardSummary{StatusCounts: map[string]int{}, RecentEvents: recentEvents, ActiveAlerts: activeAlerts}
	summary.TotalHosts = len(hosts)
	for _, host := range hosts {
		if host.Status == "online" {
			summary.OnlineHosts++
		}
		summary.TotalVCPUs += host.CPUCores
		summary.TotalMemory += host.MemoryBytes
		summary.UsedMemory += host.MemoryBytes * int64(host.MemoryUsage) / 100
		summary.TotalDisk += host.StorageBytes
		summary.UsedDisk += host.StorageBytes * int64(host.StorageUsage) / 100
		summary.AverageCPU += host.CPUUsage
		summary.AverageMemory += host.MemoryUsage
	}
	if len(hosts) > 0 {
		summary.AverageCPU = summary.AverageCPU / len(hosts)
		summary.AverageMemory = summary.AverageMemory / len(hosts)
	}
	applyDashboardVMStats(&summary, vms)
	return summary
}

func applyDashboardVMStats(summary *domain.DashboardSummary, vms []domain.VirtualMachine) {
	nonTemplateVMs := make([]domain.VirtualMachine, 0, len(vms))
	for _, vm := range vms {
		if vm.IsTemplate {
			continue
		}
		nonTemplateVMs = append(nonTemplateVMs, vm)
		summary.StatusCounts[vm.Status]++
		summary.TotalVMs++
		summary.UsedVCPUs += vm.CPUCores
	}
	summary.RecentVMs = nonTemplateVMs
	if len(summary.RecentVMs) > 6 {
		summary.RecentVMs = summary.RecentVMs[:6]
	}
	summary.RunningVMs = summary.StatusCounts["running"]
	summary.StoppedVMs = summary.StatusCounts["stopped"]
	summary.PausedVMs = summary.StatusCounts["paused"]
	summary.ErrorVMs = summary.StatusCounts["error"]
}

func snapshotRuntimeID(vmID string, name string) string {
	return vmID + ":snapshot:" + name
}

func convertVMDiskList(items []agent.VMDisk) []domain.VMDisk {
	disks := make([]domain.VMDisk, 0, len(items))
	for _, item := range items {
		disks = append(disks, domain.VMDisk{
			Name:      item.Name,
			Path:      item.Path,
			Bytes:     item.Bytes,
			UsedBytes: item.UsedBytes,
		})
	}
	return disks
}

func vmRuntimeID(agentID, name, uuid string) string {
	key := strings.TrimSpace(uuid)
	if key == "" {
		key = name
	}
	return agentID + ":" + key
}

func vmMatchesQuery(vm domain.VirtualMachine, query string) bool {
	return strings.Contains(strings.ToLower(vm.Name), query) ||
		strings.Contains(strings.ToLower(vm.PrimaryIP), query) ||
		strings.Contains(strings.ToLower(vm.OSType), query) ||
		strings.Contains(strings.ToLower(vm.HostName), query)
}

func normalizeHostStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "online", "degraded", "maintenance":
		return strings.ToLower(strings.TrimSpace(status))
	case "offline":
		return "offline"
	default:
		return "online"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeVMStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running", "stopped", "paused", "error":
		return strings.ToLower(strings.TrimSpace(status))
	case "shut off", "shutoff", "off":
		return "stopped"
	default:
		if strings.TrimSpace(status) == "" {
			return "unknown"
		}
		return strings.ToLower(strings.TrimSpace(status))
	}
}

func normalizeSnapshotStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready", "creating", "failed":
		return strings.ToLower(strings.TrimSpace(status))
	case "current":
		return "ready"
	default:
		return "ready"
	}
}

func parseAgentSnapshotTime(value string, fallback time.Time) time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05 MST",
		time.RFC3339,
	} {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return parsed.UTC()
		}
	}
	return fallback
}

func clampPercent(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func clampConfigInt(value int, min int, max int, fallback int) int {
	if value < min || value > max {
		return fallback
	}
	return value
}
