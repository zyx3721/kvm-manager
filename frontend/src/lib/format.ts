const GB = 1024 ** 3;
const TB = 1024 ** 4;
const units = ['B', 'KB', 'MB', 'GB', 'TB'] as const;

export function formatBytes(bytes: number, unit: 'GB' | 'TB' = 'GB', precision?: number) {
  const divisor = unit === 'TB' ? TB : GB;
  return `${(bytes / divisor).toFixed(precision ?? 1)} ${unit}`;
}

export function formatMemoryBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 MB';
  if (bytes < GB) return `${Math.round(bytes / 1024 / 1024)} MB`;
  return formatBytes(bytes, 'GB');
}

export function formatBytesAuto(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return '-';
  let value = bytes;
  let index = 0;
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024;
    index += 1;
  }
  const precision = index <= 1 || value >= 100 ? 0 : 1;
  return `${value.toFixed(precision)} ${units[index]}`;
}

export function formatBytesAutoFixed(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return '-';
  let value = bytes;
  let index = 0;
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024;
    index += 1;
  }
  return `${value.toFixed(1)} ${units[index]}`;
}

export function formatSpec(cpuCores: number, memoryBytes: number, diskBytes?: number) {
  const base = `${cpuCores} vCPU / ${formatBytes(memoryBytes, 'GB')}`;
  return diskBytes ? `${base} / ${formatBytes(diskBytes, 'GB')}` : base;
}

export function formatUptime(seconds: number) {
  if (!seconds) return '-';
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days}天 ${hours}时`;
  return `${hours}时 ${minutes}分`;
}

export function formatThroughput(bytesPerSecond: number) {
  const mb = bytesPerSecond / 1024 / 1024;
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB/s`;
  return `${Math.round(mb)} MB/s`;
}

export function formatTimeAgo(iso: string) {
  const ms = Date.now() - new Date(iso).getTime();
  const minutes = Math.max(0, Math.floor(ms / 60000));
  if (minutes < 60) return `${minutes} 分钟前`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时前`;
  return `${Math.floor(hours / 24)} 天前`;
}

export function metadataMessage(metadata: Record<string, unknown> | string, fallback: string) {
  if (typeof metadata === 'string') {
    try {
      const parsed = JSON.parse(metadata) as Record<string, unknown>;
      return String(parsed.message || fallback);
    } catch {
      return fallback;
    }
  }
  return String(metadata.message || fallback);
}

export function metadataLevel(metadata: Record<string, unknown> | string) {
  if (typeof metadata === 'string') {
    try {
      const parsed = JSON.parse(metadata) as Record<string, unknown>;
      return String(parsed.level || 'info');
    } catch {
      return 'info';
    }
  }
  return String(metadata.level || 'info');
}

export function formatOSType(osType: string, vmName?: string) {
  const value = osType.trim();
  const lower = value.toLowerCase();
  if (!value || lower === 'hvm' || lower === 'xen' || lower === 'exe' || lower === 'linux') {
    const name = (vmName || '').toLowerCase();
    if (name.includes('win')) return 'Windows';
    if (name.includes('centos') || name.includes('ct')) return 'CentOS';
    if (name.includes('ubuntu')) return 'Ubuntu';
    if (name.includes('debian')) return 'Debian';
    if (name.includes('rocky')) return 'Rocky Linux';
    if (name.includes('alma')) return 'AlmaLinux';
    return '未知';
  }
  return value;
}
