export function StateBadge({ active }: { active: boolean }) {
  return (
    <span
      className="inline-flex h-7 items-center rounded-full border px-2.5 text-xs font-semibold"
      style={{
        background: active ? 'rgba(16,185,129,0.12)' : 'rgba(239,68,68,0.1)',
        borderColor: active ? 'rgba(16,185,129,0.34)' : 'rgba(239,68,68,0.3)',
        color: active ? 'var(--kvm-status-green-text)' : 'var(--kvm-status-red-text)',
      }}
    >
      {active ? '运行中' : '已停止'}
    </span>
  );
}

export function AutostartBadge({ enabled }: { enabled: boolean }) {
  return (
    <span
      className="inline-flex h-7 items-center rounded-full border px-2.5 text-xs font-semibold"
      style={{
        background: enabled ? 'rgba(59,130,246,0.12)' : 'rgba(148,163,184,0.1)',
        borderColor: enabled ? 'rgba(59,130,246,0.34)' : 'rgba(148,163,184,0.26)',
        color: enabled ? 'var(--kvm-status-blue-text)' : 'var(--kvm-status-gray-text)',
      }}
    >
      {enabled ? '已启用' : '未启用'}
    </span>
  );
}

export function isPoolActive(state: string) {
  const normalized = state.toLowerCase();
  return normalized === 'running' || normalized === 'yes' || normalized === 'active';
}
