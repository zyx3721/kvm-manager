import { useState } from 'react';
import { RefreshCwIcon } from 'lucide-react';
import { toast } from 'sonner';
import { refreshRuntime } from '../../lib/api';
import { KvmTooltip } from '../kvm/StatusBadge';

export function HeaderFullRefreshButton() {
  const [refreshing, setRefreshing] = useState(false);

  const handleRefresh = async () => {
    if (refreshing) return;
    setRefreshing(true);
    try {
      const result = await refreshRuntime();
      toast.success(result.status === 'running' ? '全量刷新任务正在运行' : '全量刷新任务已排队');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '创建全量刷新任务失败');
    } finally {
      setRefreshing(false);
    }
  };

  return (
    <KvmTooltip label="全量刷新所有信息" placement="bottom">
      <button
        type="button"
        onClick={() => void handleRefresh()}
        disabled={refreshing}
        className="kvm-action-button flex h-[42px] w-[42px] items-center justify-center rounded-lg border disabled:cursor-not-allowed disabled:opacity-60"
        style={{
          background: refreshing ? 'rgba(59,130,246,0.1)' : 'var(--kvm-control-bg)',
          borderColor: refreshing ? 'rgba(59,130,246,0.42)' : 'var(--kvm-border)',
          color: refreshing ? 'var(--kvm-accent-text)' : 'var(--kvm-text-muted)',
        }}
        aria-label="全量刷新所有信息"
      >
        <RefreshCwIcon size={17} className={refreshing ? 'animate-spin' : ''} />
      </button>
    </KvmTooltip>
  );
}
