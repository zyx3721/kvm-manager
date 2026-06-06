export type VMStatus = 'running' | 'stopped' | 'paused' | 'error' | 'unknown';

export function normalizeVMStatus(status: string | undefined | null): VMStatus {
  const value = (status || '').trim().toLowerCase();
  switch (value) {
    case 'running':
      return 'running';
    case 'paused':
    case 'suspended':
      return 'paused';
    case 'stopped':
    case 'shutoff':
    case 'shut off':
      return 'stopped';
    case 'error':
    case 'failed':
    case 'crashed':
      return 'error';
    default:
      return 'unknown';
  }
}

export function isVMRunning(status: string | undefined | null) {
  return normalizeVMStatus(status) === 'running';
}

export function isVMPaused(status: string | undefined | null) {
  return normalizeVMStatus(status) === 'paused';
}

export function isVMStopped(status: string | undefined | null) {
  return normalizeVMStatus(status) === 'stopped';
}

export function vmStatusLabel(status: string | undefined | null) {
  switch (normalizeVMStatus(status)) {
    case 'running':
      return '运行中';
    case 'paused':
      return '已暂停';
    case 'stopped':
      return '已停止';
    case 'error':
      return '异常';
    default:
      return status || '-';
  }
}

export function defaultMigrationMode(status: string | undefined | null) {
  return isVMRunning(status) ? 'live' : 'cold';
}
