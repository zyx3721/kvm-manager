CREATE TABLE IF NOT EXISTS host_metric_rollups (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  bucket_size TEXT NOT NULL,
  bucket_at TIMESTAMPTZ NOT NULL,
  agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  host_name TEXT NOT NULL,
  cpu_usage INT NOT NULL,
  memory_usage INT NOT NULL,
  storage_usage INT NOT NULL,
  vm_count INT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(bucket_size, bucket_at, agent_id)
);

CREATE TABLE IF NOT EXISTS vm_metric_rollups (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  bucket_size TEXT NOT NULL,
  bucket_at TIMESTAMPTZ NOT NULL,
  agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  vm_id TEXT NOT NULL,
  vm_name TEXT NOT NULL,
  status TEXT NOT NULL,
  cpu_usage INT NOT NULL,
  memory_usage INT NOT NULL,
  disk_usage INT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(bucket_size, bucket_at, vm_id)
);

CREATE INDEX IF NOT EXISTS idx_host_metric_rollups_agent_time ON host_metric_rollups(agent_id, bucket_size, bucket_at DESC);
CREATE INDEX IF NOT EXISTS idx_vm_metric_rollups_vm_time ON vm_metric_rollups(vm_id, bucket_size, bucket_at DESC);

