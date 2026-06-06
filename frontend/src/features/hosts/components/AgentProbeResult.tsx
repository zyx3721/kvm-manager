import { CheckCircle2Icon } from 'lucide-react';

import { type AgentHostInfo } from '../../../lib/api';
import { formatBytes } from '../../../lib/format';

export function AgentProbeResult({
  result,
}: {
  result: { type: 'success'; host: AgentHostInfo } | { type: 'error'; message: string };
}) {
  if (result.type === 'error') {
    return (
      <div
        className="mt-4 rounded-lg p-3 text-sm"
        style={{
          background: 'rgba(239,68,68,0.1)',
          border: '1px solid rgba(239,68,68,0.28)',
          color: '#f87171',
        }}
      >
        {result.message}
      </div>
    );
  }

  const host = result.host;
  return (
    <div
      className="mt-4 rounded-lg p-3 text-sm"
      style={{
        background: 'rgba(16,185,129,0.1)',
        border: '1px solid rgba(16,185,129,0.28)',
        color: '#34d399',
      }}
    >
      <div className="mb-2 flex items-center gap-2 font-medium">
        <CheckCircle2Icon size={15} />
        Agent 测试成功
      </div>
      <div
        className="flex flex-wrap items-center gap-x-8 gap-y-2 text-xs"
        style={{ color: 'var(--kvm-text-muted)' }}
      >
        <span>主机名：{host.hostname || '未知'}</span>
        <span>状态：{host.status || 'unknown'}</span>
        <span>KVM：{host.kvmVersion || 'unknown'}</span>
        <span>
          CPU：{host.cpuCores} 核 / {host.cpuUsage}%
        </span>
        <span>
          内存：{formatBytes(host.memoryBytes, 'GB')} / {host.memoryUsage}%
        </span>
        <span>
          存储：{formatBytes(host.storageBytes, 'GB')} / {host.storageUsage}%
        </span>
      </div>
    </div>
  );
}
