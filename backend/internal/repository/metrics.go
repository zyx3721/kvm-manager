package repository

import (
	"context"
	"time"
)

type MetricPoint struct {
	Time      time.Time `json:"time"`
	CPU       int       `json:"cpu"`
	Memory    int       `json:"memory"`
	Storage   int       `json:"storage,omitempty"`
	Disk      int       `json:"disk,omitempty"`
	DiskRead  int64     `json:"diskReadBytesPerSecond,omitempty"`
	DiskWrite int64     `json:"diskWriteBytesPerSecond,omitempty"`
	NetworkRx int64     `json:"networkRxBytesPerSecond,omitempty"`
	NetworkTx int64     `json:"networkTxBytesPerSecond,omitempty"`
	VMCount   int       `json:"vmCount,omitempty"`
}

type HostMetricSample struct {
	AgentID              string
	HostName             string
	CPUUsage             int
	MemoryUsage          int
	MemoryBytes          int64
	StorageUsage         int
	StorageBytes         int64
	DiskReadBytesPerSec  int64
	DiskWriteBytesPerSec int64
	NetworkRxBytesPerSec int64
	NetworkTxBytesPerSec int64
	VMCount              int
	CollectedAt          time.Time
}

type VMMetricSample struct {
	AgentID              string
	VMID                 string
	VMName               string
	Status               string
	CPUUsage             int
	CPUUsageAvailable    bool
	MemoryUsage          int
	MemoryUsageAvailable bool
	DiskUsage            int
	DiskUsageAvailable   bool
	DiskReadBytesPerSec  int64
	DiskWriteBytesPerSec int64
	NetworkRxBytesPerSec int64
	NetworkTxBytesPerSec int64
	UptimeSeconds        int64
	CollectedAt          time.Time
}

func (s *Store) InsertMetricSamples(ctx context.Context, host HostMetricSample, vms []VMMetricSample) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO host_metric_samples(agent_id, host_name, cpu_usage, memory_usage, memory_bytes, storage_usage, storage_bytes, disk_read_bytes_per_second, disk_write_bytes_per_second, network_rx_bytes_per_second, network_tx_bytes_per_second, vm_count, collected_at)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, host.AgentID, host.HostName, host.CPUUsage, host.MemoryUsage, host.MemoryBytes, host.StorageUsage, host.StorageBytes, host.DiskReadBytesPerSec, host.DiskWriteBytesPerSec, host.NetworkRxBytesPerSec, host.NetworkTxBytesPerSec, host.VMCount, host.CollectedAt); err != nil {
		return err
	}

	for _, vm := range vms {
		if _, err := tx.Exec(ctx, `
			INSERT INTO vm_metric_samples(agent_id, vm_id, vm_name, status, cpu_usage, cpu_usage_available, memory_usage, memory_usage_available, disk_usage, disk_usage_available, disk_read_bytes_per_second, disk_write_bytes_per_second, network_rx_bytes_per_second, network_tx_bytes_per_second, uptime_seconds, collected_at)
			VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		`, vm.AgentID, vm.VMID, vm.VMName, vm.Status, vm.CPUUsage, vm.CPUUsageAvailable, vm.MemoryUsage, vm.MemoryUsageAvailable, vm.DiskUsage, vm.DiskUsageAvailable, vm.DiskReadBytesPerSec, vm.DiskWriteBytesPerSec, vm.NetworkRxBytesPerSec, vm.NetworkTxBytesPerSec, vm.UptimeSeconds, vm.CollectedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) ListHostMetricSeries(ctx context.Context, agentID string, since time.Time, until time.Time, bucket time.Duration) ([]MetricPoint, error) {
	if bucket >= 5*time.Minute {
		items, err := s.listHostMetricRollupSeries(ctx, agentID, since, until, bucket)
		if err == nil && len(items) > 0 {
			return items, nil
		}
	}
	rows, err := s.pool.Query(ctx, `
		SELECT to_timestamp(floor(extract(epoch FROM collected_at) / $3) * $3) AS bucket,
		       round(avg(cpu_usage))::int,
		       round(avg(memory_usage))::int,
		       round(avg(storage_usage))::int,
		       round(avg(disk_read_bytes_per_second))::bigint,
		       round(avg(disk_write_bytes_per_second))::bigint,
		       round(avg(network_rx_bytes_per_second))::bigint,
		       round(avg(network_tx_bytes_per_second))::bigint,
		       round(avg(vm_count))::int
		FROM host_metric_samples
		WHERE ($1 = '' OR agent_id::text = $1) AND collected_at >= $2 AND collected_at <= $4
		GROUP BY bucket
		ORDER BY bucket ASC
	`, agentID, since, int(bucket.Seconds()), until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]MetricPoint, 0)
	for rows.Next() {
		var point MetricPoint
		if err := rows.Scan(&point.Time, &point.CPU, &point.Memory, &point.Storage, &point.DiskRead, &point.DiskWrite, &point.NetworkRx, &point.NetworkTx, &point.VMCount); err != nil {
			return nil, err
		}
		items = append(items, point)
	}
	return items, rows.Err()
}

func (s *Store) ListVMMetricSeries(ctx context.Context, vmID string, since time.Time, until time.Time, bucket time.Duration) ([]MetricPoint, error) {
	if bucket >= 5*time.Minute {
		items, err := s.listVMMetricRollupSeries(ctx, vmID, since, until, bucket)
		if err == nil && len(items) > 0 {
			return items, nil
		}
	}
	rows, err := s.pool.Query(ctx, `
		SELECT to_timestamp(floor(extract(epoch FROM collected_at) / $3) * $3) AS bucket,
		       round(avg(cpu_usage))::int,
		       round(avg(memory_usage))::int,
		       round(avg(disk_usage))::int,
		       round(avg(disk_read_bytes_per_second))::bigint,
		       round(avg(disk_write_bytes_per_second))::bigint,
		       round(avg(network_rx_bytes_per_second))::bigint,
		       round(avg(network_tx_bytes_per_second))::bigint
		FROM vm_metric_samples
		WHERE vm_id = $1 AND collected_at >= $2 AND collected_at <= $4
		GROUP BY bucket
		ORDER BY bucket ASC
	`, vmID, since, int(bucket.Seconds()), until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]MetricPoint, 0)
	for rows.Next() {
		var point MetricPoint
		if err := rows.Scan(&point.Time, &point.CPU, &point.Memory, &point.Disk, &point.DiskRead, &point.DiskWrite, &point.NetworkRx, &point.NetworkTx); err != nil {
			return nil, err
		}
		items = append(items, point)
	}
	return items, rows.Err()
}

func (s *Store) UpsertMetricRollups(ctx context.Context, bucketSize string, bucket time.Duration, since time.Time) error {
	seconds := int(bucket.Seconds())
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO host_metric_rollups(bucket_size, bucket_at, agent_id, host_name, cpu_usage, memory_usage, storage_usage, disk_read_bytes_per_second, disk_write_bytes_per_second, network_rx_bytes_per_second, network_tx_bytes_per_second, vm_count)
		SELECT $1,
		       to_timestamp(floor(extract(epoch FROM collected_at) / $2) * $2) AS bucket,
		       agent_id,
		       max(host_name),
		       round(avg(cpu_usage))::int,
		       round(avg(memory_usage))::int,
		       round(avg(storage_usage))::int,
		       round(avg(disk_read_bytes_per_second))::bigint,
		       round(avg(disk_write_bytes_per_second))::bigint,
		       round(avg(network_rx_bytes_per_second))::bigint,
		       round(avg(network_tx_bytes_per_second))::bigint,
		       round(avg(vm_count))::int
		FROM host_metric_samples
		WHERE collected_at >= $3
		GROUP BY bucket, agent_id
		ON CONFLICT(bucket_size, bucket_at, agent_id) DO UPDATE SET
		  host_name=EXCLUDED.host_name,
		  cpu_usage=EXCLUDED.cpu_usage,
		  memory_usage=EXCLUDED.memory_usage,
		  storage_usage=EXCLUDED.storage_usage,
		  disk_read_bytes_per_second=EXCLUDED.disk_read_bytes_per_second,
		  disk_write_bytes_per_second=EXCLUDED.disk_write_bytes_per_second,
		  network_rx_bytes_per_second=EXCLUDED.network_rx_bytes_per_second,
		  network_tx_bytes_per_second=EXCLUDED.network_tx_bytes_per_second,
		  vm_count=EXCLUDED.vm_count
	`, bucketSize, seconds, since); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO vm_metric_rollups(bucket_size, bucket_at, agent_id, vm_id, vm_name, status, cpu_usage, memory_usage, disk_usage, disk_read_bytes_per_second, disk_write_bytes_per_second, network_rx_bytes_per_second, network_tx_bytes_per_second)
		SELECT $1,
		       to_timestamp(floor(extract(epoch FROM collected_at) / $2) * $2) AS bucket,
		       agent_id,
		       vm_id,
		       max(vm_name),
		       max(status),
		       round(avg(cpu_usage))::int,
		       round(avg(memory_usage))::int,
		       round(avg(disk_usage))::int,
		       round(avg(disk_read_bytes_per_second))::bigint,
		       round(avg(disk_write_bytes_per_second))::bigint,
		       round(avg(network_rx_bytes_per_second))::bigint,
		       round(avg(network_tx_bytes_per_second))::bigint
		FROM vm_metric_samples
		WHERE collected_at >= $3
		GROUP BY bucket, agent_id, vm_id
		ON CONFLICT(bucket_size, bucket_at, vm_id) DO UPDATE SET
		  vm_name=EXCLUDED.vm_name,
		  status=EXCLUDED.status,
		  cpu_usage=EXCLUDED.cpu_usage,
		  memory_usage=EXCLUDED.memory_usage,
		  disk_usage=EXCLUDED.disk_usage,
		  disk_read_bytes_per_second=EXCLUDED.disk_read_bytes_per_second,
		  disk_write_bytes_per_second=EXCLUDED.disk_write_bytes_per_second,
		  network_rx_bytes_per_second=EXCLUDED.network_rx_bytes_per_second,
		  network_tx_bytes_per_second=EXCLUDED.network_tx_bytes_per_second
	`, bucketSize, seconds, since)
	return err
}

func (s *Store) listHostMetricRollupSeries(ctx context.Context, agentID string, since time.Time, until time.Time, bucket time.Duration) ([]MetricPoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT bucket_at, round(avg(cpu_usage))::int, round(avg(memory_usage))::int, round(avg(storage_usage))::int, round(avg(disk_read_bytes_per_second))::bigint, round(avg(disk_write_bytes_per_second))::bigint, round(avg(network_rx_bytes_per_second))::bigint, round(avg(network_tx_bytes_per_second))::bigint, round(avg(vm_count))::int
		FROM host_metric_rollups
		WHERE bucket_size=$1 AND ($2 = '' OR agent_id::text = $2) AND bucket_at >= $3 AND bucket_at <= $4
		GROUP BY bucket_at
		ORDER BY bucket_at ASC
	`, bucketLabel(bucket), agentID, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]MetricPoint, 0)
	for rows.Next() {
		var point MetricPoint
		if err := rows.Scan(&point.Time, &point.CPU, &point.Memory, &point.Storage, &point.DiskRead, &point.DiskWrite, &point.NetworkRx, &point.NetworkTx, &point.VMCount); err != nil {
			return nil, err
		}
		items = append(items, point)
	}
	return items, rows.Err()
}

func (s *Store) listVMMetricRollupSeries(ctx context.Context, vmID string, since time.Time, until time.Time, bucket time.Duration) ([]MetricPoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT bucket_at, cpu_usage, memory_usage, disk_usage, disk_read_bytes_per_second, disk_write_bytes_per_second, network_rx_bytes_per_second, network_tx_bytes_per_second
		FROM vm_metric_rollups
		WHERE bucket_size=$1 AND vm_id=$2 AND bucket_at >= $3 AND bucket_at <= $4
		ORDER BY bucket_at ASC
	`, bucketLabel(bucket), vmID, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]MetricPoint, 0)
	for rows.Next() {
		var point MetricPoint
		if err := rows.Scan(&point.Time, &point.CPU, &point.Memory, &point.Disk, &point.DiskRead, &point.DiskWrite, &point.NetworkRx, &point.NetworkTx); err != nil {
			return nil, err
		}
		items = append(items, point)
	}
	return items, rows.Err()
}

func bucketLabel(bucket time.Duration) string {
	switch bucket {
	case 5 * time.Minute:
		return "5m"
	case 30 * time.Minute:
		return "30m"
	case time.Hour:
		return "1h"
	case 24 * time.Hour:
		return "24h"
	default:
		return bucket.String()
	}
}

func (s *Store) DeleteMetricSamplesBefore(ctx context.Context, before time.Time) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM vm_metric_samples WHERE collected_at < $1`, before); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM host_metric_samples WHERE collected_at < $1`, before)
	return err
}
