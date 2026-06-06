ALTER TABLE host_metric_samples
  ADD COLUMN IF NOT EXISTS disk_read_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS disk_write_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS network_rx_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS network_tx_bytes_per_second BIGINT NOT NULL DEFAULT 0;

ALTER TABLE host_metric_rollups
  ADD COLUMN IF NOT EXISTS disk_read_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS disk_write_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS network_rx_bytes_per_second BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS network_tx_bytes_per_second BIGINT NOT NULL DEFAULT 0;
