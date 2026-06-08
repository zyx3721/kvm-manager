package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"kvm-manager/backend/internal/domain"
)

const (
	redisHostsKey          = "kvm:runtime:hosts"
	redisVMsKey            = "kvm:runtime:vms"
	redisSnapshotsKey      = "kvm:runtime:snapshots"
	agentDeleteMarkerTTL   = 10 * time.Minute
	agentDeleteMarkerValue = "1"
)

var errAgentRuntimeDeleted = errors.New("agent runtime deleted")

type RedisRuntimeStore struct {
	client redis.Cmdable
}

func NewRedisRuntimeStore(client redis.Cmdable) *RedisRuntimeStore {
	return &RedisRuntimeStore{client: client}
}

func (r *RedisRuntimeStore) SaveAgentRuntime(ctx context.Context, agentID string, host domain.Host, vms map[string]domain.VirtualMachine, snapshots map[string]domain.Snapshot, replaceSnapshots bool) error {
	deleted, err := r.isAgentDeleted(ctx, agentID)
	if err != nil {
		return err
	}
	if deleted {
		return errAgentRuntimeDeleted
	}
	pipe := r.client.TxPipeline()
	pipe.SAdd(ctx, redisHostsKey, agentID)
	pipe.Set(ctx, redisHostKey(agentID), mustJSON(host), 0)

	oldVMIDs, err := r.client.SMembers(ctx, redisAgentVMsKey(agentID)).Result()
	if err != nil {
		return err
	}
	for _, vmID := range oldVMIDs {
		if replaceSnapshots {
			oldSnapshotIDs, err := r.client.SMembers(ctx, redisVMSnapshotsKey(vmID)).Result()
			if err != nil {
				return err
			}
			for _, snapshotID := range oldSnapshotIDs {
				pipe.Del(ctx, redisSnapshotKey(snapshotID))
				pipe.SRem(ctx, redisSnapshotsKey, snapshotID)
			}
			pipe.Del(ctx, redisVMSnapshotsKey(vmID))
		}
		pipe.Del(ctx, redisVMKey(vmID))
		pipe.SRem(ctx, redisVMsKey, vmID)
	}
	pipe.Del(ctx, redisAgentVMsKey(agentID))

	for id, vm := range vms {
		pipe.SAdd(ctx, redisVMsKey, id)
		pipe.SAdd(ctx, redisAgentVMsKey(agentID), id)
		pipe.Set(ctx, redisVMKey(id), mustJSON(vm), 0)
	}
	if replaceSnapshots {
		for id, snapshot := range snapshots {
			pipe.SAdd(ctx, redisSnapshotsKey, id)
			pipe.SAdd(ctx, redisVMSnapshotsKey(snapshot.VMID), id)
			pipe.Set(ctx, redisSnapshotKey(id), mustJSON(snapshot), 0)
		}
	}
	_, err = pipe.Exec(ctx)
	return r.rejectDeletedAgentRuntimeWrite(ctx, agentID, err)
}

func (r *RedisRuntimeStore) UpdateVMRuntime(ctx context.Context, agentID string, host domain.Host, vm domain.VirtualMachine) error {
	deleted, err := r.isAgentDeleted(ctx, agentID)
	if err != nil {
		return err
	}
	if deleted {
		return errAgentRuntimeDeleted
	}
	pipe := r.client.TxPipeline()
	oldVMIDs, err := r.client.SMembers(ctx, redisAgentVMsKey(agentID)).Result()
	if err != nil {
		return err
	}
	for _, vmID := range oldVMIDs {
		if vmID == vm.ID {
			continue
		}
		current, ok, err := r.GetVM(ctx, vmID)
		if err != nil {
			return err
		}
		if ok && current.Name == vm.Name {
			pipe.Del(ctx, redisVMKey(vmID))
			pipe.SRem(ctx, redisVMsKey, vmID)
			pipe.SRem(ctx, redisAgentVMsKey(agentID), vmID)
		}
	}
	pipe.SAdd(ctx, redisHostsKey, agentID)
	pipe.Set(ctx, redisHostKey(agentID), mustJSON(host), 0)
	pipe.SAdd(ctx, redisVMsKey, vm.ID)
	pipe.SAdd(ctx, redisAgentVMsKey(agentID), vm.ID)
	pipe.Set(ctx, redisVMKey(vm.ID), mustJSON(vm), 0)
	_, err = pipe.Exec(ctx)
	return r.rejectDeletedAgentRuntimeWrite(ctx, agentID, err)
}

func (r *RedisRuntimeStore) RemoveVMRuntime(ctx context.Context, agentID string, vmID string) error {
	pipe := r.client.TxPipeline()
	oldSnapshotIDs, err := r.client.SMembers(ctx, redisVMSnapshotsKey(vmID)).Result()
	if err != nil {
		return err
	}
	for _, snapshotID := range oldSnapshotIDs {
		pipe.Del(ctx, redisSnapshotKey(snapshotID))
		pipe.SRem(ctx, redisSnapshotsKey, snapshotID)
	}
	pipe.Del(ctx, redisVMSnapshotsKey(vmID), redisVMKey(vmID))
	pipe.SRem(ctx, redisVMsKey, vmID)
	if agentID != "" {
		pipe.SRem(ctx, redisAgentVMsKey(agentID), vmID)
		if host, ok, err := r.GetHost(ctx, agentID); err != nil {
			return err
		} else if ok && host.VMCount > 0 {
			host.VMCount--
			pipe.Set(ctx, redisHostKey(agentID), mustJSON(host), 0)
		}
	}
	_, err = pipe.Exec(ctx)
	return r.rejectDeletedAgentRuntimeWrite(ctx, agentID, err)
}

func (r *RedisRuntimeStore) ReplaceAgentSnapshots(ctx context.Context, agentID string, snapshots map[string]domain.Snapshot) error {
	deleted, err := r.isAgentDeleted(ctx, agentID)
	if err != nil {
		return err
	}
	if deleted {
		return errAgentRuntimeDeleted
	}
	vmIDs, err := r.client.SMembers(ctx, redisAgentVMsKey(agentID)).Result()
	if err != nil {
		return err
	}
	pipe := r.client.TxPipeline()
	for _, vmID := range vmIDs {
		oldSnapshotIDs, err := r.client.SMembers(ctx, redisVMSnapshotsKey(vmID)).Result()
		if err != nil {
			return err
		}
		for _, snapshotID := range oldSnapshotIDs {
			pipe.Del(ctx, redisSnapshotKey(snapshotID))
			pipe.SRem(ctx, redisSnapshotsKey, snapshotID)
		}
		pipe.Del(ctx, redisVMSnapshotsKey(vmID))
	}
	for id, snapshot := range snapshots {
		pipe.SAdd(ctx, redisSnapshotsKey, id)
		pipe.SAdd(ctx, redisVMSnapshotsKey(snapshot.VMID), id)
		pipe.Set(ctx, redisSnapshotKey(id), mustJSON(snapshot), 0)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (r *RedisRuntimeStore) RemoveAgent(ctx context.Context, agentID string) error {
	oldVMIDs, err := r.client.SMembers(ctx, redisAgentVMsKey(agentID)).Result()
	if err != nil {
		return err
	}
	pipe := r.client.TxPipeline()
	pipe.Set(ctx, redisAgentDeleteMarkerKey(agentID), agentDeleteMarkerValue, agentDeleteMarkerTTL)
	pipe.Del(ctx, redisHostKey(agentID), redisAgentVMsKey(agentID))
	pipe.SRem(ctx, redisHostsKey, agentID)
	for _, vmID := range oldVMIDs {
		oldSnapshotIDs, err := r.client.SMembers(ctx, redisVMSnapshotsKey(vmID)).Result()
		if err != nil {
			return err
		}
		for _, snapshotID := range oldSnapshotIDs {
			pipe.Del(ctx, redisSnapshotKey(snapshotID))
			pipe.SRem(ctx, redisSnapshotsKey, snapshotID)
		}
		pipe.Del(ctx, redisVMSnapshotsKey(vmID), redisVMKey(vmID))
		pipe.SRem(ctx, redisVMsKey, vmID)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (r *RedisRuntimeStore) isAgentDeleted(ctx context.Context, agentID string) (bool, error) {
	value, err := r.client.Exists(ctx, redisAgentDeleteMarkerKey(agentID)).Result()
	if err != nil {
		return false, err
	}
	return value > 0, nil
}

func (r *RedisRuntimeStore) rejectDeletedAgentRuntimeWrite(ctx context.Context, agentID string, writeErr error) error {
	if writeErr != nil {
		return writeErr
	}
	deleted, err := r.isAgentDeleted(ctx, agentID)
	if err != nil {
		return err
	}
	if !deleted {
		return nil
	}
	if err := r.RemoveAgent(ctx, agentID); err != nil {
		return err
	}
	return errAgentRuntimeDeleted
}

func (r *RedisRuntimeStore) ListHosts(ctx context.Context) ([]domain.Host, error) {
	ids, err := r.client.SMembers(ctx, redisHostsKey).Result()
	if err != nil {
		return nil, err
	}
	hosts := make([]domain.Host, 0, len(ids))
	for _, id := range ids {
		value, err := r.client.Get(ctx, redisHostKey(id)).Bytes()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			return nil, err
		}
		var host domain.Host
		if err := json.Unmarshal(value, &host); err != nil {
			return nil, err
		}
		hosts = append(hosts, host)
	}
	sortHosts(hosts)
	return hosts, nil
}

func (r *RedisRuntimeStore) GetHost(ctx context.Context, agentID string) (domain.Host, bool, error) {
	value, err := r.client.Get(ctx, redisHostKey(agentID)).Bytes()
	if err == redis.Nil {
		return domain.Host{}, false, nil
	}
	if err != nil {
		return domain.Host{}, false, err
	}
	var host domain.Host
	if err := json.Unmarshal(value, &host); err != nil {
		return domain.Host{}, false, err
	}
	return host, true, nil
}

func (r *RedisRuntimeStore) ListVMs(ctx context.Context, status string, query string, hostID string) ([]domain.VirtualMachine, error) {
	if status == "all" {
		status = ""
	}
	query = strings.ToLower(strings.TrimSpace(query))
	hostID = strings.TrimSpace(hostID)
	ids, err := r.client.SMembers(ctx, redisVMsKey).Result()
	if err != nil {
		return nil, err
	}
	items := make([]domain.VirtualMachine, 0, len(ids))
	for _, id := range ids {
		vm, ok, err := r.GetVM(ctx, id)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if status != "" && vm.Status != status {
			continue
		}
		if hostID != "" && vm.HostID != hostID {
			continue
		}
		if query != "" && !vmMatchesQuery(vm, query) {
			continue
		}
		items = append(items, vm)
	}
	sortVMs(items)
	return items, nil
}

func (r *RedisRuntimeStore) GetVM(ctx context.Context, id string) (domain.VirtualMachine, bool, error) {
	value, err := r.client.Get(ctx, redisVMKey(id)).Bytes()
	if err == redis.Nil {
		return domain.VirtualMachine{}, false, nil
	}
	if err != nil {
		return domain.VirtualMachine{}, false, err
	}
	var vm domain.VirtualMachine
	if err := json.Unmarshal(value, &vm); err != nil {
		return domain.VirtualMachine{}, false, err
	}
	return vm, true, nil
}

func (r *RedisRuntimeStore) ListSnapshots(ctx context.Context) ([]domain.Snapshot, error) {
	ids, err := r.client.SMembers(ctx, redisSnapshotsKey).Result()
	if err != nil {
		return nil, err
	}
	items := make([]domain.Snapshot, 0, len(ids))
	for _, id := range ids {
		value, err := r.client.Get(ctx, redisSnapshotKey(id)).Bytes()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			return nil, err
		}
		var snapshot domain.Snapshot
		if err := json.Unmarshal(value, &snapshot); err != nil {
			return nil, err
		}
		items = append(items, snapshot)
	}
	sortSnapshots(items)
	return items, nil
}

func (r *RedisRuntimeStore) GetSnapshot(ctx context.Context, id string) (domain.Snapshot, bool, error) {
	value, err := r.client.Get(ctx, redisSnapshotKey(id)).Bytes()
	if err == redis.Nil {
		return domain.Snapshot{}, false, nil
	}
	if err != nil {
		return domain.Snapshot{}, false, err
	}
	var snapshot domain.Snapshot
	if err := json.Unmarshal(value, &snapshot); err != nil {
		return domain.Snapshot{}, false, err
	}
	return snapshot, true, nil
}

func mustJSON(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func redisHostKey(agentID string) string     { return "kvm:runtime:host:" + agentID }
func redisAgentVMsKey(agentID string) string { return "kvm:runtime:vms:agent:" + agentID }
func redisAgentDeleteMarkerKey(agentID string) string {
	return "kvm:runtime:agent-deleted:" + agentID
}
func redisVMKey(vmID string) string             { return "kvm:runtime:vm:" + vmID }
func redisVMSnapshotsKey(vmID string) string    { return "kvm:runtime:snapshots:vm:" + vmID }
func redisSnapshotKey(snapshotID string) string { return "kvm:runtime:snapshot:" + snapshotID }

func sortHosts(hosts []domain.Host) {
	sort.Slice(hosts, func(i, j int) bool {
		leftRank := hostStatusRank(hosts[i].Status)
		rightRank := hostStatusRank(hosts[j].Status)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return strings.ToLower(hosts[i].Name) < strings.ToLower(hosts[j].Name)
	})
}

func sortVMs(vms []domain.VirtualMachine) {
	sort.Slice(vms, func(i, j int) bool {
		leftRank := vmStatusRank(vms[i].Status)
		rightRank := vmStatusRank(vms[j].Status)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return strings.ToLower(vms[i].Name) < strings.ToLower(vms[j].Name)
	})
}

func sortSnapshots(snapshots []domain.Snapshot) {
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt) })
}

func hostStatusRank(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "online":
		return 0
	case "degraded":
		return 1
	case "maintenance":
		return 2
	case "offline":
		return 3
	case "unknown":
		return 4
	default:
		return 5
	}
}

func vmStatusRank(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running":
		return 0
	case "paused":
		return 1
	case "stopped":
		return 2
	case "error":
		return 3
	case "unknown":
		return 4
	default:
		return 5
	}
}
