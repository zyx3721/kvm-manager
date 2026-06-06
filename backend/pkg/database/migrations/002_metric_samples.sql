CREATE TABLE IF NOT EXISTS host_metric_samples (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  host_name TEXT NOT NULL,
  cpu_usage INT NOT NULL,
  memory_usage INT NOT NULL,
  memory_bytes BIGINT NOT NULL,
  storage_usage INT NOT NULL,
  storage_bytes BIGINT NOT NULL,
  vm_count INT NOT NULL,
  collected_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS vm_metric_samples (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  vm_id TEXT NOT NULL,
  vm_name TEXT NOT NULL,
  status TEXT NOT NULL,
  cpu_usage INT NOT NULL,
  cpu_usage_available BOOLEAN NOT NULL,
  memory_usage INT NOT NULL,
  memory_usage_available BOOLEAN NOT NULL,
  disk_usage INT NOT NULL,
  disk_usage_available BOOLEAN NOT NULL,
  uptime_seconds BIGINT NOT NULL,
  collected_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_host_metric_samples_agent_time ON host_metric_samples(agent_id, collected_at DESC);
CREATE INDEX IF NOT EXISTS idx_vm_metric_samples_vm_time ON vm_metric_samples(vm_id, collected_at DESC);
CREATE INDEX IF NOT EXISTS idx_host_metric_samples_time_brin ON host_metric_samples USING BRIN(collected_at);
CREATE INDEX IF NOT EXISTS idx_vm_metric_samples_time_brin ON vm_metric_samples USING BRIN(collected_at);

