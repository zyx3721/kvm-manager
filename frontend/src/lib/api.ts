import { clearSession, getAuthToken, markSessionActivity } from './auth';

export type ApiErrorPayload = {
  error?: string;
  message?: string;
};

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

type RequestOptions = RequestInit & {
  auth?: boolean;
};

const businessUnauthorizedCodes = new Set(['invalid_agent_token', 'invalid_old_password']);

async function readError(response: Response) {
  try {
    const body = (await response.json()) as ApiErrorPayload;
    return new ApiError(
      response.status,
      body.error || 'request_failed',
      body.message || `请求失败：${response.status}`
    );
  } catch {
    return new ApiError(response.status, 'request_failed', `请求失败：${response.status}`);
  }
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers(options.headers);
  const token = getAuthToken();

  const isFormData = typeof FormData !== 'undefined' && options.body instanceof FormData;
  if (!headers.has('Content-Type') && options.body && !isFormData) {
    headers.set('Content-Type', 'application/json');
  }
  if (options.auth !== false && token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const response = await fetch(path, { ...options, headers });
  if (options.auth !== false && response.ok) {
    markSessionActivity();
  }
  if (!response.ok) {
    const error = await readError(response);
    if (response.status === 401 && !businessUnauthorizedCodes.has(error.code)) {
      clearSession();
      if (window.location.pathname !== '/login') {
        window.location.assign('/login');
      }
    }
    throw error;
  }

  if (response.status === 204) {
    return undefined as T;
  }
  return response.json() as Promise<T>;
}

export type ApiUser = {
  id: string;
  username: string;
  email: string;
  displayName: string;
  role: string;
  source?: string;
  roles?: UserRole[];
  permissions?: string[];
  disabled?: boolean;
  lastLoginAt?: string;
  created_at?: string;
  updated_at?: string;
};

export type UserRole = {
  id: string;
  key: string;
  name: string;
  description: string;
  permissions: string[];
  builtin: boolean;
  created_at: string;
  updated_at: string;
};

export type UserGroup = {
  id: string;
  name: string;
  description: string;
  disabled: boolean;
  members?: ApiUser[];
  roles?: UserRole[];
  created_at: string;
  updated_at: string;
};

export type UserPermission = {
  key: string;
  name: string;
  description: string;
  category: string;
  impliedReadPermission?: string;
};

export type ManagedUserPayload = {
  username: string;
  password?: string;
  email: string;
  displayName: string;
  roleKeys: string[];
  disabled: boolean;
};

export type UserRolePayload = {
  key: string;
  name: string;
  description: string;
  permissions: string[];
};

export type UserGroupPayload = {
  name: string;
  description: string;
  disabled: boolean;
  memberIds: string[];
  roleKeys: string[];
};

export type SessionResponse = {
  token: string;
  expires_at: string;
  user: ApiUser;
};

export type PublicAuthProvider = {
  id: string;
  type: string;
  name: string;
  enabled: boolean;
};

export type SystemBaseConfig = {
  siteName: string;
  loginName: string;
  appName: string;
  appSubtitle: string;
  iconData: string;
  passwordResetCodeTtlMinutes: number;
  passwordResetCaptchaTtlMinutes: number;
  passwordResetSendCooldownMinutes: number;
  passwordResetRateLimitMinutes: number;
  resourceWarningThreshold: number;
  resourceCriticalThreshold: number;
  resourceAlertConsecutiveCount: number;
  agentOfflineFailureCount: number;
  alertNotificationTimeoutSeconds: number;
  alertNotificationMaxRetryCount: number;
  alertNotificationRetryBaseSeconds: number;
  alertNotificationRetryMaxMinutes: number;
  alertNotificationBatchSize: number;
  created_at?: string;
  updated_at?: string;
};

export type ChangePasswordPayload = {
  old_password: string;
  new_password: string;
  confirm_password: string;
};

export type PasswordResetCaptcha = {
  token: string;
  question: string;
  expiresAt: string;
  generatedAt: string;
};

export type PasswordResetChannel = {
  id: string;
  name: string;
  description: string;
  requiresTo: boolean;
};

export type VirtualMachine = {
  id: string;
  hostId: string;
  hostName: string;
  name: string;
  uuid: string;
  description?: string;
  osType: string;
  status: string;
  cpuCores: number;
  memoryBytes: number;
  diskBytes: number;
  diskUsedBytes: number;
  disks: VMDisk[];
  primaryIp: string;
  cpuUsage: number;
  cpuUsageAvailable: boolean;
  memoryUsage: number;
  memoryUsageAvailable: boolean;
  diskUsage: number;
  diskUsageAvailable: boolean;
  diskReadBytesPerSecond: number;
  diskWriteBytesPerSecond: number;
  networkRxBytesPerSecond: number;
  networkTxBytesPerSecond: number;
  uptimeSeconds: number;
  isTemplate?: boolean;
  templateId?: string;
  templateName?: string;
  templateDescription?: string;
  created_at: string;
  updated_at: string;
};

export type VMDisk = {
  name: string;
  path: string;
  bytes: number;
  usedBytes: number;
};

export type VMConfig = {
  name: string;
  uuid: string;
  osType: string;
  status: string;
  description: string;
  autostart: boolean;
  currentCpu: number;
  maximumCpu: number;
  hostCpu: number;
  currentMemoryBytes: number;
  maximumMemoryBytes: number;
  hostMemoryBytes: number;
  memoryStatsPeriod: number;
  disks: VMConfigDisk[];
  interfaces: VMConfigInterface[];
  cdroms: VMConfigCDROM[];
  graphics: VMConfigGraphics;
  xml: string;
};

export type VMConfigUpdatePayload = {
  description: string;
  currentCpu: number;
  maximumCpu: number;
  currentMemoryMB: number;
  maximumMemoryMB: number;
  memoryStatsPeriod: number;
};

export type VMRenamePayload = {
  name: string;
};

export type VMConsoleInfo = {
  type: string;
  listen: string;
  port: number;
  passwordEnabled: boolean;
};

export type VMConfigGraphics = {
  type: string;
  listen: string;
  port: string;
  passwordEnabled: boolean;
};

export type VMConsoleUpdatePayload = {
  passwordEnabled: boolean;
  password: string;
};

export type VMMediaConnectPayload = {
  target: string;
  isoPath: string;
};

export type VMMediaDisconnectPayload = {
  target: string;
};

export type VMXMLUpdatePayload = {
  xml: string;
};

export type VMDeviceUpdatePayload = {
  interfaces: VMDeviceInterfacePayload[];
  newInterfaces: VMDeviceNewInterfacePayload[];
  deletedInterfaces: VMDeviceDeleteInterfacePayload[];
  diskResizes: VMDeviceDiskResizePayload[];
  newDisks: VMDeviceNewDiskPayload[];
  deletedDisks: VMDeviceDeleteDiskPayload[];
};

export type VMDeviceInterfacePayload = {
  name: string;
  mac: string;
  source: string;
};

export type VMDeviceNewInterfacePayload = {
  source: string;
  model: string;
};

export type VMDeviceDeleteInterfacePayload = {
  name: string;
  mac: string;
};

export type VMDeviceDiskResizePayload = {
  name: string;
  capacityBytes: number;
};

export type VMDeviceDeleteDiskPayload = {
  name: string;
};

export type VMDeviceNewDiskPayload = {
  name: string;
  pool: string;
  target: string;
  bus: string;
  format: string;
  capacityBytes: number;
  preallocMetadata?: boolean;
};

export type VMClonePayload = {
  name: string;
  description: string;
  autostart: boolean;
  currentCpu: number;
  maximumCpu: number;
  currentMemoryMB: number;
  maximumMemoryMB: number;
  cdromPolicy: 'inherit' | 'disconnect';
  interfaces: VMCloneInterfacePayload[];
  disks: VMCloneDiskPayload[];
};

export type VMTemplateMarkPayload = {
  name: string;
  description: string;
};

export type VMCreatePayload = {
  agentId: string;
  createMode?: 'blank' | 'template' | 'xml';
  name: string;
  description: string;
  autostart: boolean;
  currentCpu: number;
  maximumCpu: number;
  currentMemoryMB: number;
  maximumMemoryMB: number;
  cpuModel: string;
  osType: string;
  disks: VMCreateDiskPayload[];
  diskName: string;
  diskPool: string;
  diskFormat: string;
  diskBus: string;
  diskCapacityGB: number;
  preallocMetadata: boolean;
  isoPath: string;
  isoBus: string;
  networkSource: string;
  networkModel: string;
  graphics: string;
  consolePassword: string;
  bootFirmware: string;
  template?: VMCreateTemplatePayload;
  xml?: string;
};

export type VMCreateDiskPayload = {
  name: string;
  pool: string;
  format: string;
  bus: string;
  capacityGB: number;
  preallocMetadata: boolean;
};

export type VMCreateTemplatePayload = {
  sourcePool: string;
  sourceName: string;
  targetPool: string;
  targetName: string;
  bus: string;
  format: string;
  convert: boolean;
  preallocMetadata: boolean;
};

export type VMMigratePayload = {
  targetAgentId: string;
  destinationUri: string;
  live: boolean;
  copyDisks: boolean;
  persistent: boolean;
  undefineSource: boolean;
  autoConverge: boolean;
  postCopy: boolean;
};

export type VMMigrationSSHKeyPayload = {
  targetAgentId: string;
  destinationUri: string;
  username: string;
  password: string;
};

export type VMMigrationHostnamePayload = {
  targetAgentId: string;
  destinationUri: string;
  hostname: string;
};

export type VMMigrationPrecheckItem = {
  key: string;
  label: string;
  status: 'passed' | 'failed' | 'skipped';
  message: string;
  code?: string;
};

export type VMMigrationPrecheckReport = {
  passed: boolean;
  items: VMMigrationPrecheckItem[];
};

export type VMCloneInterfacePayload = {
  name: string;
  mac: string;
  source: string;
};

export type VMCloneDiskPayload = {
  name: string;
  pool: string;
  sourcePath: string;
  targetName: string;
  preallocMetadata: boolean;
};

export type VMConfigDisk = {
  name: string;
  path: string;
  sourcePath?: string;
  pool: string;
  bus: string;
  device: string;
  type: string;
  bytes: number;
};

export type VMConfigInterface = {
  name: string;
  mac: string;
  type: string;
  source: string;
  model: string;
};

export type VMConfigCDROM = {
  name: string;
  path: string;
  bus: string;
  connected: boolean;
};

export type Host = {
  id: string;
  name: string;
  address: string;
  hostname: string;
  cluster: string;
  status: 'online' | 'offline' | 'maintenance' | 'degraded' | 'unknown';
  cpuCores: number;
  cpuUsage: number;
  memoryBytes: number;
  memoryUsage: number;
  storageBytes: number;
  storageUsage: number;
  diskReadBytesPerSecond: number;
  diskWriteBytesPerSecond: number;
  networkRxBytesPerSecond: number;
  networkTxBytesPerSecond: number;
  vmCount: number;
  kvmVersion: string;
  kvmFullVersion: string;
  created_at: string;
  updated_at: string;
};

export type StoragePool = {
  name: string;
  type: string;
  state: string;
  autostart: boolean;
  path: string;
  capacitySource: string;
  capacity: number;
  allocation: number;
  available: number;
  volumeCount: number;
};

export type HostInterface = {
  name: string;
  type: string;
  mac: string;
  ipv4: string;
  ipv4Mode: string;
  ipv6: string;
  ipv6Mode: string;
  bridgeDevice: string;
  bootMode: string;
  status: string;
  stp: string;
  delay: string;
};

export type HostInterfaceCreatePayload = {
  name: string;
  startMode: string;
  device: string;
  type: string;
  stp: string;
  delay: string;
  ipv4Mode: string;
  ipv4Address: string;
  ipv4Gateway: string;
  ipv6Mode: string;
  ipv6Address: string;
  ipv6Gateway: string;
  applySystemConfig: boolean;
  dnsServers: string[];
};

export type HostInterfaceDevice = {
  name: string;
};

export type StoragePoolCreatePayload = {
  name: string;
  type: string;
  path?: string;
  device?: string;
  sourceHost?: string;
  sourcePath?: string;
  format?: string;
};

export type ISOFile = {
  name: string;
  path: string;
  bytes: number;
  pool: string;
};

export type StorageVolume = {
  name: string;
  path: string;
  type: string;
  format: string;
  capacity: number;
  allocation: number;
  pool: string;
  cloneSupported: boolean;
  deleteSupported: boolean;
};

export type StorageVolumeCreatePayload = {
  name: string;
  format: string;
  capacityBytes: number;
  preallocMetadata?: boolean;
};

export type StorageVolumeClonePayload = {
  name: string;
  sourceName: string;
  format: string;
  convert: boolean;
  preallocMetadata?: boolean;
};

export type AsyncTaskResponse = {
  status: string;
  task: Task;
};

export type NetworkPool = {
  name: string;
  state: string;
  autostart: boolean;
  bridge: string;
  forward: string;
  subnet: string;
  dhcp: boolean;
  dhcpStart: string;
  dhcpEnd: string;
  fixedAddresses: NetworkFixedAddress[];
  openVSwitch: boolean;
};

export type NetworkFixedAddress = {
  address: string;
  mac: string;
};

export type NetworkPoolCreatePayload = {
  name: string;
  subnet?: string;
  dhcp: boolean;
  fixedAddress?: boolean;
  type: string;
  bridge?: string;
  openVSwitch?: boolean;
};

export type PoolStatePayload = {
  active: boolean;
};

export type PoolAutostartPayload = {
  autostart: boolean;
};

export type AgentRecord = {
  id: string;
  name: string;
  endpoint: string;
  tlsInsecure: boolean;
  status: string;
  version: string;
  capabilities: unknown;
  lastHeartbeatAt?: string;
  lastError: string;
  created_at: string;
  updated_at: string;
};

export type AgentHostInfo = {
  hostname: string;
  status: string;
  kvmVersion: string;
  kvmFullVersion: string;
  cpuCores: number;
  cpuUsage: number;
  memoryBytes: number;
  memoryUsage: number;
  storageBytes: number;
  storageUsage: number;
  diskReadBytesPerSecond: number;
  diskWriteBytesPerSecond: number;
  networkRxBytesPerSecond: number;
  networkTxBytesPerSecond: number;
  capabilities: string[];
};

export type Snapshot = {
  id: string;
  hostId: string;
  hostName: string;
  vmId: string;
  vmName: string;
  name: string;
  displayName: string;
  description: string;
  tags: string[];
  sizeBytes: number;
  type: 'snapshot' | 'backup';
  status: 'ready' | 'creating' | 'failed';
  created_at: string;
  updated_at: string;
};

export type AuditLog = {
  id: string;
  userId: string;
  username: string;
  action: string;
  resourceType: string;
  resourceId: string;
  ipAddress: string;
  metadata: Record<string, unknown> | string;
  created_at: string;
};

export type Alert = {
  id: string;
  level: string;
  status: string;
  sourceType: string;
  sourceId: string;
  title: string;
  message: string;
  metadata: Record<string, unknown> | string;
  firstSeenAt: string;
  lastSeenAt: string;
  resolvedAt?: string;
  notificationSentAt?: string;
  readAt?: string;
  dismissedAt?: string;
  created_at: string;
  updated_at: string;
};

export type NotificationChannel = {
  id: string;
  enabled: boolean;
  passwordResetEnabled: boolean;
  config: Record<string, unknown> | string;
  created_at: string;
  updated_at: string;
};

export type AlertNotificationDelivery = {
  id: string;
  alert: Alert;
  eventType: string;
  channelId: string;
  status: string;
  payload: Record<string, unknown> | string;
  error: string;
  retryCount: number;
  nextRetryAt: string;
  lastAttemptAt?: string;
  sentAt?: string;
  created_at: string;
  updated_at: string;
};

export type NotificationTemplatePreview = {
  problemSubject: string;
  problemText: string;
  problemWebhook?: Record<string, unknown>;
  recoverySubject: string;
  recoveryText: string;
  recoveryWebhook?: Record<string, unknown>;
  contentType?: string;
  messageType?: string;
  problemTitle?: string;
  recoveryTitle?: string;
  problemColor?: string;
  recoveryColor?: string;
};

export type AuthProvider = {
  id: string;
  type: string;
  name: string;
  enabled: boolean;
  config: Record<string, unknown> | string;
  created_at: string;
  updated_at: string;
};

export type Task = {
  id: string;
  type: string;
  status: string;
  targetType: string;
  targetId: string;
  payload: Record<string, unknown>;
  errorMessage?: string;
  createdBy: string;
  created_at: string;
  updated_at: string;
  finished_at?: string;
};

export type RefreshProgress = {
  totalAgents?: number;
  syncedAgents?: number;
  failedAgents?: number;
  currentAgent?: string;
  message?: string;
};
export type MetricPoint = {
  time: string;
  cpu: number;
  memory: number;
  storage?: number;
  disk?: number;
  diskReadBytesPerSecond?: number;
  diskWriteBytesPerSecond?: number;
  networkRxBytesPerSecond?: number;
  networkTxBytesPerSecond?: number;
  vmCount?: number;
};

export type MetricSeriesResponse = {
  range: string;
  bucket: string;
  items: MetricPoint[];
};

export type DashboardSummary = {
  totalHosts: number;
  onlineHosts: number;
  totalVMs: number;
  runningVMs: number;
  stoppedVMs: number;
  pausedVMs: number;
  errorVMs: number;
  totalVCPUs: number;
  usedVCPUs: number;
  averageCpu: number;
  averageMemory: number;
  totalMemoryBytes: number;
  usedMemoryBytes: number;
  totalDiskBytes: number;
  usedDiskBytes: number;
  statusCounts: Record<string, number>;
  recentEvents: AuditLog[];
  recentVMs: VirtualMachine[];
  activeAlerts: Alert[];
};

export type ListResponse<T> = {
  items: T[];
  total: number;
};

export function fetchDashboardSummary() {
  return apiRequest<DashboardSummary>('/api/dashboard/summary');
}

export function fetchPublicAuthProviders() {
  return apiRequest<ListResponse<PublicAuthProvider>>('/api/auth/providers', { auth: false });
}

export function fetchPublicSystemBaseConfig() {
  return apiRequest<SystemBaseConfig>('/api/public/base-config', { auth: false });
}

export function fetchSystemBaseConfig() {
  return apiRequest<SystemBaseConfig>('/api/settings/base-config');
}

export function updateSystemBaseConfig(payload: SystemBaseConfig) {
  return apiRequest<SystemBaseConfig>('/api/settings/base-config', {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function fetchHostMetricSeries(
  agentId = '',
  range = '1h',
  window?: { start?: string; end?: string }
) {
  const target = agentId ? encodeURIComponent(agentId) : 'all';
  const query = new URLSearchParams({ range });
  if (window?.start) query.set('start', window.start);
  if (window?.end) query.set('end', window.end);
  return apiRequest<MetricSeriesResponse>(`/api/metrics/hosts/${target}?${query}`);
}

export function fetchVMMetricSeries(
  vmId: string,
  range = '1h',
  window?: { start?: string; end?: string }
) {
  const query = new URLSearchParams({ range });
  if (window?.start) query.set('start', window.start);
  if (window?.end) query.set('end', window.end);
  return apiRequest<MetricSeriesResponse>(`/api/metrics/vms/${encodeURIComponent(vmId)}?${query}`);
}

export function changePassword(payload: ChangePasswordPayload) {
  return apiRequest<{ message: string }>('/api/auth/password', {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function fetchVMs(params: { status?: string; q?: string; hostId?: string } = {}) {
  const query = new URLSearchParams();
  if (params.status && params.status !== 'all') query.set('status', params.status);
  if (params.q) query.set('q', params.q);
  if (params.hostId && params.hostId !== 'all') query.set('hostId', params.hostId);
  const suffix = query.toString() ? `?${query}` : '';
  return apiRequest<ListResponse<VirtualMachine>>(`/api/vms${suffix}`);
}

export function refreshVM(id: string) {
  return apiRequest<{ status: string; vm: VirtualMachine }>(
    `/api/vms/${encodeURIComponent(id)}/refresh`,
    {
      method: 'POST',
    }
  );
}

export function fetchVMConfig(id: string) {
  return apiRequest<VMConfig>(`/api/vms/${encodeURIComponent(id)}/config`);
}

export function fetchVMConsoleInfo(id: string) {
  return apiRequest<VMConsoleInfo>(`/api/vms/${encodeURIComponent(id)}/console`);
}

export function updateVMConfig(id: string, payload: VMConfigUpdatePayload) {
  return apiRequest<{ config: VMConfig; task: unknown }>(
    `/api/vms/${encodeURIComponent(id)}/config`,
    {
      method: 'PUT',
      body: JSON.stringify(payload),
    }
  );
}

export function renameVM(id: string, payload: VMRenamePayload) {
  return apiRequest<{ config: VMConfig; task: unknown }>(
    `/api/vms/${encodeURIComponent(id)}/rename`,
    {
      method: 'PUT',
      body: JSON.stringify(payload),
    }
  );
}

export function updateVMAutostart(id: string, autostart: boolean) {
  return apiRequest<{ autostart: boolean; task: unknown }>(
    `/api/vms/${encodeURIComponent(id)}/autostart`,
    {
      method: 'PUT',
      body: JSON.stringify({ autostart }),
    }
  );
}

export function updateVMConsole(id: string, payload: VMConsoleUpdatePayload) {
  return apiRequest<{ config: VMConfig; task: unknown }>(
    `/api/vms/${encodeURIComponent(id)}/console`,
    {
      method: 'PUT',
      body: JSON.stringify(payload),
    }
  );
}

export function connectVMMedia(id: string, payload: VMMediaConnectPayload) {
  return apiRequest<{ config: VMConfig; task: unknown }>(
    `/api/vms/${encodeURIComponent(id)}/media`,
    {
      method: 'PUT',
      body: JSON.stringify(payload),
    }
  );
}

export function disconnectVMMedia(id: string, payload: VMMediaDisconnectPayload) {
  return apiRequest<{ config: VMConfig; task: unknown }>(
    `/api/vms/${encodeURIComponent(id)}/media`,
    {
      method: 'DELETE',
      body: JSON.stringify(payload),
    }
  );
}

export function updateVMXML(id: string, payload: VMXMLUpdatePayload) {
  return apiRequest<{ config: VMConfig; task: unknown }>(`/api/vms/${encodeURIComponent(id)}/xml`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function updateVMDevices(id: string, payload: VMDeviceUpdatePayload) {
  return apiRequest<{ config: VMConfig; task: unknown }>(
    `/api/vms/${encodeURIComponent(id)}/devices`,
    {
      method: 'PUT',
      body: JSON.stringify(payload),
    }
  );
}

export function cloneVM(id: string, payload: VMClonePayload) {
  return apiRequest<AsyncTaskResponse>(`/api/vms/${encodeURIComponent(id)}/clone`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function createVMFromTemplate(id: string, payload: VMClonePayload) {
  return apiRequest<AsyncTaskResponse>(`/api/vms/${encodeURIComponent(id)}/template-create`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function createVM(payload: VMCreatePayload) {
  return apiRequest<AsyncTaskResponse>('/api/vms', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function markVMTemplate(id: string, payload: VMTemplateMarkPayload) {
  return apiRequest<{ template: unknown; task: unknown }>(
    `/api/vms/${encodeURIComponent(id)}/template-mark`,
    {
      method: 'POST',
      body: JSON.stringify(payload),
    }
  );
}

export function unmarkVMTemplate(id: string) {
  return apiRequest<{ status: string; task: unknown }>(
    `/api/vms/${encodeURIComponent(id)}/template-mark`,
    {
      method: 'DELETE',
    }
  );
}

export function migrateVM(id: string, payload: VMMigratePayload) {
  return apiRequest<AsyncTaskResponse>(`/api/vms/${encodeURIComponent(id)}/migrate`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function precheckVMMigration(id: string, payload: VMMigratePayload) {
  return apiRequest<VMMigrationPrecheckReport>(
    `/api/vms/${encodeURIComponent(id)}/migrate-precheck`,
    {
      method: 'POST',
      body: JSON.stringify(payload),
    }
  );
}

export function setupVMMigrationSSHKey(id: string, payload: VMMigrationSSHKeyPayload) {
  return apiRequest<{
    status: string;
    result: { ok: boolean; passwordRequired: boolean; message: string };
  }>(`/api/vms/${encodeURIComponent(id)}/migrate-ssh-key`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function setupVMMigrationHostname(id: string, payload: VMMigrationHostnamePayload) {
  return apiRequest<{
    status: string;
    result: { ok: boolean; passwordRequired: boolean; message: string };
  }>(`/api/vms/${encodeURIComponent(id)}/migrate-hostname`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function fetchHosts() {
  return apiRequest<ListResponse<Host>>('/api/hosts');
}

export function fetchHostInterfaces(agentId: string) {
  return apiRequest<ListResponse<HostInterface>>(
    `/api/host-interfaces/${encodeURIComponent(agentId)}`
  );
}

export function createHostInterface(agentId: string, payload: HostInterfaceCreatePayload) {
  return apiRequest<HostInterface>(`/api/host-interfaces/${encodeURIComponent(agentId)}`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function fetchHostInterfaceDevices(agentId: string) {
  return apiRequest<ListResponse<HostInterfaceDevice>>(
    `/api/host-interfaces/${encodeURIComponent(agentId)}/devices/list`
  );
}

export function updateHostInterfaceState(agentId: string, name: string, active: boolean) {
  return apiRequest<PoolStatePayload>(
    `/api/host-interfaces/${encodeURIComponent(agentId)}/state/${encodeURIComponent(name)}`,
    {
      method: 'PUT',
      body: JSON.stringify({ active }),
    }
  );
}

export function deleteHostInterface(agentId: string, name: string) {
  return apiRequest<{ status: string }>(
    `/api/host-interfaces/${encodeURIComponent(agentId)}/delete/${encodeURIComponent(name)}`,
    {
      method: 'DELETE',
    }
  );
}

export function fetchStoragePools(agentId: string) {
  return apiRequest<ListResponse<StoragePool>>(`/api/storage-pools/${encodeURIComponent(agentId)}`);
}

export function createStoragePool(agentId: string, payload: StoragePoolCreatePayload) {
  return apiRequest<StoragePool>(`/api/storage-pools/${encodeURIComponent(agentId)}`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function fetchISOFiles(agentId: string, poolName: string) {
  return apiRequest<ListResponse<ISOFile>>(
    `/api/storage-pools/${encodeURIComponent(agentId)}/iso-files/${encodeURIComponent(poolName)}`
  );
}

export function fetchStorageVolumes(agentId: string, poolName: string) {
  return apiRequest<ListResponse<StorageVolume>>(
    `/api/storage-pools/${encodeURIComponent(agentId)}/volumes/${encodeURIComponent(poolName)}`
  );
}

export function createStorageVolume(
  agentId: string,
  poolName: string,
  payload: StorageVolumeCreatePayload
) {
  return apiRequest<StorageVolume>(
    `/api/storage-pools/${encodeURIComponent(agentId)}/volumes/${encodeURIComponent(poolName)}`,
    {
      method: 'POST',
      body: JSON.stringify(payload),
    }
  );
}

export function cloneStorageVolume(
  agentId: string,
  poolName: string,
  payload: StorageVolumeClonePayload
) {
  return apiRequest<AsyncTaskResponse>(
    `/api/storage-pools/${encodeURIComponent(agentId)}/volumes/${encodeURIComponent(poolName)}/clone`,
    {
      method: 'POST',
      body: JSON.stringify(payload),
    }
  );
}

export function uploadStorageISO(
  agentId: string,
  poolName: string,
  file: File,
  volumeName?: string
) {
  const form = new FormData();
  form.set('file', file);
  if (volumeName) form.set('name', volumeName);
  return apiRequest<AsyncTaskResponse>(
    `/api/storage-pools/${encodeURIComponent(agentId)}/volumes/${encodeURIComponent(poolName)}/upload`,
    {
      method: 'POST',
      body: form,
    }
  );
}

export function deleteStorageVolume(agentId: string, poolName: string, volumeName: string) {
  return apiRequest<{ status: string }>(
    `/api/storage-pools/${encodeURIComponent(agentId)}/volumes/${encodeURIComponent(poolName)}?name=${encodeURIComponent(volumeName)}`,
    { method: 'DELETE' }
  );
}

export function deleteStoragePool(agentId: string, poolName: string) {
  return apiRequest<{ status: string }>(
    `/api/storage-pools/${encodeURIComponent(agentId)}/delete/${encodeURIComponent(poolName)}`,
    { method: 'DELETE' }
  );
}

export function updateStoragePoolState(agentId: string, poolName: string, active: boolean) {
  return apiRequest<PoolStatePayload>(
    `/api/storage-pools/${encodeURIComponent(agentId)}/state/${encodeURIComponent(poolName)}`,
    {
      method: 'PUT',
      body: JSON.stringify({ active }),
    }
  );
}

export function updateStoragePoolAutostart(agentId: string, poolName: string, autostart: boolean) {
  return apiRequest<PoolAutostartPayload>(
    `/api/storage-pools/${encodeURIComponent(agentId)}/autostart/${encodeURIComponent(poolName)}`,
    {
      method: 'PUT',
      body: JSON.stringify({ autostart }),
    }
  );
}

export function fetchNetworkPools(agentId: string) {
  return apiRequest<ListResponse<NetworkPool>>(`/api/network-pools/${encodeURIComponent(agentId)}`);
}

export function createNetworkPool(agentId: string, payload: NetworkPoolCreatePayload) {
  return apiRequest<NetworkPool>(`/api/network-pools/${encodeURIComponent(agentId)}`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function deleteNetworkPool(agentId: string, poolName: string) {
  return apiRequest<{ status: string }>(
    `/api/network-pools/${encodeURIComponent(agentId)}/delete/${encodeURIComponent(poolName)}`,
    { method: 'DELETE' }
  );
}

export function updateNetworkPoolState(agentId: string, poolName: string, active: boolean) {
  return apiRequest<PoolStatePayload>(
    `/api/network-pools/${encodeURIComponent(agentId)}/state/${encodeURIComponent(poolName)}`,
    {
      method: 'PUT',
      body: JSON.stringify({ active }),
    }
  );
}

export function updateNetworkPoolAutostart(agentId: string, poolName: string, autostart: boolean) {
  return apiRequest<PoolAutostartPayload>(
    `/api/network-pools/${encodeURIComponent(agentId)}/autostart/${encodeURIComponent(poolName)}`,
    {
      method: 'PUT',
      body: JSON.stringify({ autostart }),
    }
  );
}

export function fetchSnapshots() {
  return apiRequest<ListResponse<Snapshot>>('/api/snapshots');
}

export function createSnapshot(payload: {
  vmId: string;
  name: string;
  description?: string;
  tags?: string[];
}) {
  return apiRequest<{ snapshot: Snapshot; task: unknown }>('/api/snapshots', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function runSnapshotAction(id: string, action: 'revert' | 'delete') {
  return apiRequest<{ snapshot: Snapshot; task: unknown }>(
    `/api/snapshots/${encodeURIComponent(id)}/${action}`,
    {
      method: 'POST',
    }
  );
}

export function updateSnapshotAnnotation(
  id: string,
  payload: { displayName: string; description: string; tags: string[] }
) {
  return apiRequest<Snapshot>(`/api/snapshots/${encodeURIComponent(id)}/annotation`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function runVMAction(
  id: string,
  action:
    | 'start'
    | 'resume'
    | 'stop'
    | 'force-stop'
    | 'shutdown'
    | 'force-shutdown'
    | 'pause'
    | 'reboot'
    | 'force-reboot'
    | 'delete'
    | 'force-delete'
) {
  return apiRequest<{ vm: VirtualMachine; task: unknown }>(`/api/vms/${id}/${action}`, {
    method: 'POST',
  });
}

export function vmConsoleUrl(id: string) {
  const token = getAuthToken();
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const query = token ? `?token=${encodeURIComponent(token)}` : '';
  return `${protocol}//${window.location.host}/api/vms/${encodeURIComponent(id)}/console/ws${query}`;
}

export function fetchAgents() {
  return apiRequest<ListResponse<AgentRecord>>('/api/agents');
}

export function createAgent(payload: {
  name: string;
  endpoint: string;
  token: string;
  tlsInsecure: boolean;
}) {
  return apiRequest<AgentRecord>('/api/agents', { method: 'POST', body: JSON.stringify(payload) });
}

export function probeAgentConnection(payload: {
  endpoint: string;
  token: string;
  tlsInsecure: boolean;
}) {
  return apiRequest<{ status: string; host: AgentHostInfo }>('/api/agents/test-connection', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function testAgentConnection(id: string, token?: string) {
  return apiRequest<{ status: string; host: AgentHostInfo }>(`/api/agents/${id}/test-connection`, {
    method: 'POST',
    body: token ? JSON.stringify({ token }) : undefined,
  });
}

export function syncAgent(id: string, token?: string) {
  return apiRequest<{ host: Host; syncedVMs: number }>(`/api/agents/${id}/sync`, {
    method: 'POST',
    body: token ? JSON.stringify({ token }) : undefined,
  });
}

export function deleteAgent(id: string) {
  return apiRequest<{ status: string }>(`/api/agents/${id}`, { method: 'DELETE' });
}

export function refreshRuntime() {
  return apiRequest<{ status: string; task: Task }>('/api/refresh', {
    method: 'POST',
  });
}

export function refreshSnapshots() {
  return apiRequest<{ status: string }>('/api/snapshots/refresh', {
    method: 'POST',
  });
}

export function fetchTask(id: string) {
  return apiRequest<{ task: Task }>(`/api/tasks/${encodeURIComponent(id)}`);
}

export type JsonFieldFilter = {
  key?: string;
  value?: string;
};

export function fetchTasks(
  limit: number | 'all' = 50,
  page = 1,
  queryText = '',
  status = '',
  payloadFilter: JsonFieldFilter = {}
) {
  const query = new URLSearchParams({ limit: String(limit), page: String(page) });
  if (queryText.trim()) query.set('q', queryText.trim());
  if (status.trim()) query.set('status', status.trim());
  if (payloadFilter.key?.trim()) query.set('payloadKey', payloadFilter.key.trim());
  if (payloadFilter.value?.trim()) query.set('payloadValue', payloadFilter.value.trim());
  return apiRequest<ListResponse<Task>>(`/api/tasks?${query}`);
}

export function fetchAuditLogs(
  limit: number | 'all' = 50,
  page = 1,
  queryText = '',
  metadataFilter: JsonFieldFilter = {}
) {
  const query = new URLSearchParams({ limit: String(limit), page: String(page) });
  if (queryText.trim()) query.set('q', queryText.trim());
  if (metadataFilter.key?.trim()) query.set('metadataKey', metadataFilter.key.trim());
  if (metadataFilter.value?.trim()) query.set('metadataValue', metadataFilter.value.trim());
  return apiRequest<ListResponse<AuditLog>>(`/api/audit-logs?${query}`);
}

export function runtimeEventsUrl() {
  const token = getAuthToken();
  return token ? `/api/events?token=${encodeURIComponent(token)}` : '/api/events';
}

export function fetchAlerts(
  params: {
    status?: string;
    limit?: number | 'all';
    page?: number;
    q?: string;
    metadata?: JsonFieldFilter;
  } = {}
) {
  const query = new URLSearchParams();
  if (params.status) query.set('status', params.status);
  if (params.limit !== undefined) query.set('limit', String(params.limit));
  if (params.page !== undefined) query.set('page', String(params.page));
  if (params.q?.trim()) query.set('q', params.q.trim());
  if (params.metadata?.key?.trim()) query.set('metadataKey', params.metadata.key.trim());
  if (params.metadata?.value?.trim()) query.set('metadataValue', params.metadata.value.trim());
  const suffix = query.toString() ? `?${query}` : '';
  return apiRequest<ListResponse<Alert>>(`/api/alerts${suffix}`);
}

export function resolveAlert(id: string) {
  return apiRequest<{ status: string }>(`/api/alerts/${encodeURIComponent(id)}/resolve`, {
    method: 'POST',
  });
}

export function fetchAlertDeliveries(id: string) {
  return apiRequest<ListResponse<AlertNotificationDelivery>>(
    `/api/alerts/${encodeURIComponent(id)}/deliveries`
  );
}

export function fetchNotifications(limit = 20) {
  return apiRequest<ListResponse<Alert>>(`/api/notifications?limit=${limit}`);
}

export function fetchUnreadNotificationCount() {
  return apiRequest<{ count: number }>('/api/notifications/unread-count');
}

export function markNotificationRead(id: string) {
  return apiRequest<{ status: string }>(`/api/notifications/${encodeURIComponent(id)}/read`, {
    method: 'POST',
  });
}

export function markAllNotificationsRead() {
  return apiRequest<{ status: string }>('/api/notifications/read-all', { method: 'POST' });
}

export function clearNotifications() {
  return apiRequest<{ status: string }>('/api/notifications/clear', { method: 'POST' });
}

export function fetchNotificationChannels() {
  return apiRequest<ListResponse<NotificationChannel>>('/api/settings/notifications');
}

export function updateNotificationChannel(
  id: string,
  payload: {
    enabled: boolean;
    passwordResetEnabled: boolean;
    clearConfig?: boolean;
    config: Record<string, unknown>;
  }
) {
  return apiRequest<NotificationChannel>(`/api/settings/notifications/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function testNotificationChannel(id: string) {
  return apiRequest<{ status: string }>(
    `/api/settings/notifications/${encodeURIComponent(id)}/test`,
    { method: 'POST' }
  );
}

export function previewNotificationChannel(
  id: string,
  payload: { enabled: boolean; passwordResetEnabled: boolean; config: Record<string, unknown> }
) {
  return apiRequest<NotificationTemplatePreview>(
    `/api/settings/notifications/${encodeURIComponent(id)}/preview`,
    {
      method: 'POST',
      body: JSON.stringify(payload),
    }
  );
}

export function fetchPasswordResetCaptcha() {
  return apiRequest<PasswordResetCaptcha>('/api/auth/password-reset/captcha', { auth: false });
}

export function verifyPasswordResetIdentity(payload: {
  username: string;
  captchaToken: string;
  captchaAnswer: string;
}) {
  return apiRequest<{ channels: PasswordResetChannel[]; verificationToken: string }>(
    '/api/auth/password-reset/verify',
    {
      method: 'POST',
      auth: false,
      body: JSON.stringify(payload),
    }
  );
}

export function sendPasswordResetCode(payload: {
  username: string;
  verificationToken: string;
  channel: string;
  verifyEmail: string;
  to: string;
}) {
  return apiRequest<{ message: string; cooldownSeconds: number; expiresAt: string }>(
    '/api/auth/password-reset/send-code',
    { method: 'POST', auth: false, body: JSON.stringify(payload) }
  );
}

export function confirmPasswordReset(payload: {
  username: string;
  code: string;
  newPassword: string;
  confirmPassword: string;
}) {
  return apiRequest<{ message: string }>('/api/auth/password-reset/confirm', {
    method: 'POST',
    auth: false,
    body: JSON.stringify(payload),
  });
}

export function fetchAuthProviders() {
  return apiRequest<ListResponse<AuthProvider>>('/api/settings/auth-providers');
}

export function updateAuthProvider(
  id: string,
  payload: { name: string; enabled: boolean; config: Record<string, unknown> }
) {
  return apiRequest<AuthProvider>(`/api/settings/auth-providers/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function testAuthProvider(id: string) {
  return apiRequest<{ status: string; matchedUsers: number }>(
    `/api/settings/auth-providers/${encodeURIComponent(id)}/test`,
    { method: 'POST' }
  );
}

export function fetchManagedUsers() {
  return apiRequest<ListResponse<ApiUser>>('/api/settings/users');
}

export function createManagedUser(payload: ManagedUserPayload) {
  return apiRequest<ApiUser>('/api/settings/users', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateManagedUser(id: string, payload: ManagedUserPayload) {
  return apiRequest<ApiUser>(`/api/settings/users/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function deleteManagedUser(id: string) {
  return apiRequest<void>(`/api/settings/users/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  });
}

export function updateManagedUserDisabled(id: string, disabled: boolean) {
  return apiRequest<ApiUser>(`/api/settings/users/${encodeURIComponent(id)}/disabled`, {
    method: 'POST',
    body: JSON.stringify({ disabled }),
  });
}

export function fetchUserRoles() {
  return apiRequest<ListResponse<UserRole>>('/api/settings/roles');
}

export function createUserRole(payload: UserRolePayload) {
  return apiRequest<UserRole>('/api/settings/roles', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateUserRole(id: string, payload: UserRolePayload) {
  return apiRequest<UserRole>(`/api/settings/roles/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function deleteUserRole(id: string) {
  return apiRequest<{ status: string }>(`/api/settings/roles/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  });
}

export function fetchUserGroups() {
  return apiRequest<ListResponse<UserGroup>>('/api/settings/user-groups');
}

export function createUserGroup(payload: UserGroupPayload) {
  return apiRequest<UserGroup>('/api/settings/user-groups', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateUserGroup(id: string, payload: UserGroupPayload) {
  return apiRequest<UserGroup>(`/api/settings/user-groups/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  });
}

export function deleteUserGroup(id: string) {
  return apiRequest<{ status: string }>(`/api/settings/user-groups/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  });
}

export function fetchUserPermissions() {
  return apiRequest<ListResponse<UserPermission>>('/api/settings/permissions');
}
