import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  ActivityIcon,
  AlertTriangleIcon,
  BoxesIcon,
  CpuIcon,
  HardDriveIcon,
  MemoryStickIcon,
  PlugZapIcon,
  RefreshCwIcon,
  ServerIcon,
  Trash2Icon,
  WifiIcon,
  XIcon,
} from 'lucide-react';
import {
  createAgent,
  deleteAgent,
  fetchAgents,
  fetchHosts,
  fetchVMs,
  probeAgentConnection,
  syncAgent,
  testAgentConnection,
  type AgentHostInfo,
  type AgentRecord,
  type Host,
  type VirtualMachine,
} from '../../lib/api';
import { formatBytes } from '../../lib/format';
import { DialogPortal } from '../../components/kvm/DialogPortal';
import { KvmTooltip, StatusBadge } from '../../components/kvm/StatusBadge';
import { HostTrendDialog } from '../../components/kvm/HostTrendDialog';
import { CheckToggle } from '../vms/components/VMEditControls';
import { onKvmRefresh } from '../../lib/refresh';
import { can } from '../../lib/permissions';
import { toast } from 'sonner';
import { AgentProbeResult } from './components/AgentProbeResult';
import { HostIconButton } from './components/HostIconButton';
import { ResourceRow } from './components/ResourceRow';
import { fallbackPercent, sumBy } from './utils';

export default function Hosts() {
  const [hosts, setHosts] = useState<Host[]>([]);
  const [vms, setVMs] = useState<VirtualMachine[]>([]);
  const [agents, setAgents] = useState<AgentRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [agentBusy, setAgentBusy] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<AgentRecord | null>(null);
  const [agentForm, setAgentForm] = useState({
    name: '',
    endpoint: '',
    token: '',
    tlsInsecure: false,
  });
  const [agentProbeResult, setAgentProbeResult] = useState<
    { type: 'success'; host: AgentHostInfo } | { type: 'error'; message: string } | null
  >(null);
  const [trendHost, setTrendHost] = useState<Host | null>(null);
  const [error, setError] = useState('');
  const canReadHosts = can('hosts.read');
  const canReadVMs = can('vms.read');
  const canReadAgents = can('agents.read');
  const canManageAgents = can('agents.manage');

  const loadHosts = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [hostResponse, agentResponse, vmResponse] = await Promise.all([
        canReadHosts ? fetchHosts() : Promise.resolve({ items: [] }),
        canReadAgents || canManageAgents ? fetchAgents() : Promise.resolve({ items: [] }),
        canReadVMs ? fetchVMs() : Promise.resolve({ items: [] }),
      ]);
      setHosts(hostResponse.items);
      setVMs(vmResponse.items);
      setAgents(agentResponse.items);
    } catch (err) {
      const message = err instanceof Error ? err.message : '读取宿主机列表失败';
      toast.error(message);
      setError(isPermissionMessage(message) ? '' : message);
    } finally {
      setLoading(false);
    }
  }, [canManageAgents, canReadAgents, canReadHosts, canReadVMs]);

  useEffect(() => {
    const timer = window.setTimeout(() => void loadHosts(), 0);
    const unsubscribe = onKvmRefresh(() => void loadHosts());
    return () => {
      window.clearTimeout(timer);
      unsubscribe();
    };
  }, [loadHosts]);

  const handleCreateAgent = async () => {
    setAgentBusy('create');
    setError('');
    setAgentProbeResult(null);
    let created: AgentRecord | null = null;
    try {
      await probeAgentConnection({
        endpoint: agentForm.endpoint,
        token: agentForm.token,
        tlsInsecure: agentForm.tlsInsecure,
      });
      created = await createAgent(agentForm);
      await syncAgent(created.id, agentForm.token);
      setAgentForm({ name: '', endpoint: '', token: '', tlsInsecure: false });
      await loadHosts();
    } catch (err) {
      if (created) {
        await deleteAgent(created.id).catch(() => undefined);
      }
      setAgentProbeResult({
        type: 'error',
        message: err instanceof Error ? err.message : 'Agent 操作失败',
      });
    } finally {
      setAgentBusy('');
    }
  };

  const handleProbeAgent = async () => {
    if (!agentForm.endpoint || !agentForm.token) {
      setAgentProbeResult({ type: 'error', message: '请先填写 Agent 地址和令牌，再执行测试' });
      return;
    }
    setAgentBusy('probe');
    setError('');
    setAgentProbeResult(null);
    try {
      const response = await probeAgentConnection({
        endpoint: agentForm.endpoint,
        token: agentForm.token,
        tlsInsecure: agentForm.tlsInsecure,
      });
      setAgentProbeResult({ type: 'success', host: response.host });
    } catch (err) {
      setAgentProbeResult({
        type: 'error',
        message: err instanceof Error ? err.message : 'Agent 测试失败',
      });
    } finally {
      setAgentBusy('');
    }
  };

  const handleDeleteAgent = async (agent: AgentRecord) => {
    setDeleteTarget(agent);
  };

  const handleTestSavedAgent = async (agent: AgentRecord) => {
    setAgentBusy(agent.id + 'test');
    setError('');
    try {
      await testAgentConnection(agent.id);
      await loadHosts();
      toast.success(`${agent.name} 连接测试成功`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Agent 连接测试失败');
    } finally {
      setAgentBusy('');
    }
  };

  const handleSyncSavedAgent = async (agent: AgentRecord) => {
    setAgentBusy(agent.id + 'sync');
    setError('');
    try {
      await syncAgent(agent.id);
      await loadHosts();
      toast.success(`${agent.name} 同步完成`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Agent 同步失败');
    } finally {
      setAgentBusy('');
    }
  };

  const confirmDeleteAgent = async () => {
    if (!deleteTarget) return;
    const agent = deleteTarget;
    setAgentBusy(agent.id + 'delete');
    setError('');
    try {
      await deleteAgent(agent.id);
      setDeleteTarget(null);
      await loadHosts();
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除 Agent 失败');
    } finally {
      setAgentBusy('');
    }
  };

  const vmsByHost = useMemo(() => {
    const grouped = new Map<string, VirtualMachine[]>();
    vms.forEach(vm => {
      grouped.set(vm.hostId, [...(grouped.get(vm.hostId) || []), vm]);
    });
    return grouped;
  }, [vms]);

  return (
    <div data-cmp="Hosts" className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold" style={{ color: 'var(--kvm-text)' }}>
            宿主机
          </h1>
          <p className="mt-1 text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
            物理节点资源池管理，共 {hosts.length} 台宿主机
          </p>
        </div>
      </div>

      {(canReadAgents || canManageAgents) && <section
        className="rounded-xl p-5"
        style={{ background: 'var(--kvm-card)', border: '1px solid var(--kvm-border)' }}
      >
        <div className="mb-4 flex items-center justify-between">
          <div>
            <h2
              className="flex items-center gap-2 text-sm font-semibold"
              style={{ color: 'var(--kvm-text)' }}
            >
              <PlugZapIcon size={16} />
              Agent 接入
            </h2>
            <p className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
              连接远端 KVM Agent 后，将同步宿主机与虚拟机真实数据
            </p>
          </div>
          <span className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
            {agents.length} 个 Agent
          </span>
        </div>

        {canManageAgents && <div className="grid grid-cols-1 gap-3 lg:grid-cols-[1fr_1.4fr_1fr_auto_auto_auto]">
          <input
            value={agentForm.name}
            onChange={event => setAgentForm(form => ({ ...form, name: event.target.value }))}
            placeholder="Agent 名称"
            className="rounded-lg px-3 py-2 text-sm outline-none"
            style={{
              background: 'var(--kvm-control-bg)',
              border: '1px solid var(--kvm-border)',
              color: 'var(--kvm-text)',
            }}
          />
          <input
            value={agentForm.endpoint}
            onChange={event => setAgentForm(form => ({ ...form, endpoint: event.target.value }))}
            placeholder="http://host:9443"
            className="rounded-lg px-3 py-2 text-sm outline-none"
            style={{
              background: 'var(--kvm-control-bg)',
              border: '1px solid var(--kvm-border)',
              color: 'var(--kvm-text)',
            }}
          />
          <input
            value={agentForm.token}
            onChange={event => setAgentForm(form => ({ ...form, token: event.target.value }))}
            placeholder="令牌"
            type="password"
            className="rounded-lg px-3 py-2 text-sm outline-none"
            style={{
              background: 'var(--kvm-control-bg)',
              border: '1px solid var(--kvm-border)',
              color: 'var(--kvm-text)',
            }}
          />
          <div className="flex items-center">
            <CheckToggle
              checked={agentForm.tlsInsecure}
              onChange={checked => setAgentForm(form => ({ ...form, tlsInsecure: checked }))}
              label="跳过 TLS 校验"
            />
          </div>
          <button
            type="button"
            onClick={handleProbeAgent}
            disabled={agentBusy === 'probe' || !agentForm.endpoint || !agentForm.token}
            className="kvm-action-button rounded-lg px-4 py-2 text-sm disabled:opacity-60"
            style={{
              background: 'rgba(59,130,246,0.12)',
              color: '#3b82f6',
              border: '1px solid rgba(59,130,246,0.28)',
            }}
          >
            {agentBusy === 'probe' ? '测试中...' : '测试'}
          </button>
          <button
            type="button"
            onClick={handleCreateAgent}
            disabled={
              agentBusy === 'create' || !agentForm.name || !agentForm.endpoint || !agentForm.token
            }
            className="kvm-action-button rounded-lg px-4 py-2 text-sm disabled:opacity-60"
            style={{
              background: 'rgba(16,185,129,0.12)',
              color: '#10b981',
              border: '1px solid rgba(16,185,129,0.28)',
            }}
          >
            {agentBusy === 'create' ? '保存中...' : '保存'}
          </button>
        </div>}

        {canManageAgents && agentProbeResult && <AgentProbeResult result={agentProbeResult} />}

        {agents.length > 0 && (
          <div
            className="kvm-hidden-scrollbar mt-4 space-y-2 overflow-y-auto pr-1"
            style={{ maxHeight: agents.length >= 5 ? '260px' : undefined }}
          >
            {agents.map(agent => (
              <div
                key={agent.id}
                className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-lg px-3 py-2 text-sm"
                style={{
                  background: 'rgba(255,255,255,0.03)',
                  border: '1px solid var(--kvm-border)',
                  color: 'var(--kvm-text)',
                }}
              >
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="truncate font-medium">{agent.name}</span>
                    <StatusBadge status={agent.status} />
                  </div>
                  <div
                    className="mt-1 truncate font-mono text-xs"
                    style={{ color: 'var(--kvm-text-muted)' }}
                  >
                    {agent.endpoint}
                  </div>
                </div>
                <div className="flex items-center justify-end gap-2">
                  {canManageAgents && <HostIconButton
                    label="测试连接"
                    variant="test"
                    disabled={agentBusy === agent.id + 'test'}
                    onClick={() => void handleTestSavedAgent(agent)}
                  >
                    <WifiIcon size={13} />
                  </HostIconButton>}
                  {canManageAgents && <HostIconButton
                    label="同步 Agent"
                    variant="sync"
                    disabled={agentBusy === agent.id + 'sync'}
                    onClick={() => void handleSyncSavedAgent(agent)}
                  >
                    <RefreshCwIcon
                      size={13}
                      className={agentBusy === agent.id + 'sync' ? 'animate-spin' : ''}
                    />
                  </HostIconButton>}
                  {canManageAgents && <HostIconButton
                    label="删除 Agent"
                    danger
                    disabled={agentBusy === agent.id + 'delete'}
                    onClick={() => handleDeleteAgent(agent)}
                  >
                    <Trash2Icon size={13} />
                  </HostIconButton>}
                </div>
              </div>
            ))}
          </div>
        )}
      </section>}

      {error && (
        <div
          className="rounded-xl p-4 text-sm"
          style={{
            background: 'rgba(245,158,11,0.1)',
            border: '1px solid rgba(245,158,11,0.25)',
            color: '#f59e0b',
          }}
        >
          {error}
        </div>
      )}
      {loading && hosts.length === 0 && (
        <div className="text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
          正在加载宿主机...
        </div>
      )}
      {!loading && canReadHosts && hosts.length === 0 && (
        <div
          className="kvm-empty-state rounded-xl p-6 text-center text-sm"
          style={{ color: 'var(--kvm-text-muted)' }}
        >
          暂无宿主机数据，请先在上方连接远端 Agent
        </div>
      )}

      {canReadHosts && <section className="space-y-3">
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
          {hosts.map(host => {
                const hostVMs = vmsByHost.get(host.id) || [];
                const cpuValue = fallbackPercent(
                  host.cpuUsage,
                  sumBy(hostVMs, vm => vm.cpuCores),
                  host.cpuCores
                );
                const memoryValue = fallbackPercent(
                  host.memoryUsage,
                  sumBy(hostVMs, vm => vm.memoryBytes),
                  host.memoryBytes
                );
                const storageValue = fallbackPercent(
                  host.storageUsage,
                  sumBy(hostVMs, vm => vm.diskBytes),
                  host.storageBytes
                );
                return (
                  <div
                    key={host.id}
                    className="rounded-xl p-5 kvm-card-hover"
                    style={{ background: 'var(--kvm-card)', border: '1px solid var(--kvm-border)' }}
                  >
                    <div className="mb-5 flex items-start justify-between">
                      <div>
                        <div className="mb-1 flex items-center gap-2">
                          <ServerIcon size={16} style={{ color: '#3b82f6' }} />
                          <h3 className="font-semibold" style={{ color: 'var(--kvm-text)' }}>
                            {host.name}
                          </h3>
                          <StatusBadge status={host.status} />
                          <HostIconButton
                            label="监控"
                            variant="monitor"
                            tooltipPlacement="bottom"
                            onClick={() => setTrendHost(host)}
                          >
                            <ActivityIcon size={13} />
                          </HostIconButton>
                        </div>
                        <div
                          className="font-mono text-xs"
                          style={{ color: 'var(--kvm-text-muted)' }}
                        >
                          {host.hostname || host.address} · {host.address}
                        </div>
                      </div>
                      <div
                        className="text-right text-xs"
                        style={{ color: 'var(--kvm-text-muted)' }}
                      >
                        <div
                          className="flex items-center gap-1 justify-end"
                          style={{ color: 'var(--kvm-text)' }}
                        >
                          <BoxesIcon size={14} />
                          {host.vmCount}
                        </div>
                        虚拟机
                      </div>
                    </div>

                    <div className="space-y-4">
                      <ResourceRow
                        icon={CpuIcon}
                        label={`CPU · ${host.cpuCores} cores`}
                        value={cpuValue}
                      />
                      <ResourceRow
                        icon={MemoryStickIcon}
                        label={`内存 · ${formatBytes(host.memoryBytes, 'GB')}`}
                        value={memoryValue}
                      />
                      <ResourceRow
                        icon={HardDriveIcon}
                        label={`存储 · ${formatBytes(host.storageBytes, 'GB')}`}
                        value={storageValue}
                      />
                    </div>

                    <div
                      className="mt-5 flex items-center justify-between border-t pt-4 text-xs"
                      style={{ borderColor: 'var(--kvm-border)', color: 'var(--kvm-text-muted)' }}
                    >
                      <KvmTooltip
                        label={host.kvmFullVersion || host.kvmVersion}
                        placement="top"
                        align="start"
                        multiline
                        disabled={!host.kvmFullVersion && !host.kvmVersion}
                      >
                        <span className="font-mono">{host.kvmVersion || 'libvirt unknown'}</span>
                      </KvmTooltip>
                    </div>
                  </div>
                );
              })}
        </div>
      </section>}

      {deleteTarget && (
        <DialogPortal>
        <div
          className="kvm-dialog-backdrop fixed inset-0 z-50 flex items-center justify-center px-4"
          role="dialog"
          aria-modal="true"
          aria-labelledby="delete-agent-title"
        >
          <div
            className="kvm-dialog-panel w-full max-w-md rounded-xl p-5 shadow-2xl"
            style={{ borderColor: 'rgba(239,68,68,0.34)' }}
          >
            <div className="mb-4 flex items-start justify-between gap-4">
              <div className="flex items-center gap-3">
                <span
                  className="flex h-10 w-10 items-center justify-center rounded-lg"
                  style={{
                    background: 'rgba(239,68,68,0.12)',
                    color: '#ef4444',
                    border: '1px solid rgba(239,68,68,0.28)',
                  }}
                >
                  <AlertTriangleIcon size={18} />
                </span>
                <div>
                  <h3
                    id="delete-agent-title"
                    className="text-base font-semibold"
                    style={{ color: 'var(--kvm-text)' }}
                  >
                    删除 Agent
                  </h3>
                  <p className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                    将删除接入登记，并清理该 Agent 同步的宿主机和虚拟机数据
                  </p>
                </div>
              </div>
              <button
                type="button"
                onClick={() => setDeleteTarget(null)}
                className="kvm-action-button flex h-8 w-8 items-center justify-center rounded-md border"
                style={{
                  borderColor: 'var(--kvm-border)',
                  color: 'var(--kvm-text-muted)',
                  background: 'rgba(255,255,255,0.03)',
                }}
                aria-label="关闭删除确认"
              >
                <XIcon size={15} />
              </button>
            </div>
            <div
              className="rounded-lg p-3 text-sm"
              style={{
                background: 'rgba(255,255,255,0.035)',
                border: '1px solid var(--kvm-border)',
                color: 'var(--kvm-text)',
              }}
            >
              <div className="font-medium">{deleteTarget.name}</div>
              <div
                className="mt-1 truncate font-mono text-xs"
                style={{ color: 'var(--kvm-text-muted)' }}
              >
                {deleteTarget.endpoint}
              </div>
            </div>
            <div className="mt-5 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setDeleteTarget(null)}
                disabled={agentBusy === deleteTarget.id + 'delete'}
                className="kvm-action-button rounded-lg px-4 py-2 text-sm disabled:opacity-60"
                style={{
                  background: 'rgba(255,255,255,0.03)',
                  color: 'var(--kvm-text-muted)',
                  border: '1px solid var(--kvm-border)',
                }}
              >
                取消
              </button>
              <button
                type="button"
                onClick={confirmDeleteAgent}
                disabled={agentBusy === deleteTarget.id + 'delete'}
                className="kvm-action-button kvm-danger-button rounded-lg px-4 py-2 text-sm disabled:opacity-60"
                style={{
                  background: 'rgba(239,68,68,0.12)',
                  color: '#ef4444',
                  border: '1px solid rgba(239,68,68,0.35)',
                }}
              >
                {agentBusy === deleteTarget.id + 'delete' ? '删除中...' : '确认删除'}
              </button>
            </div>
          </div>
        </div>
        </DialogPortal>
      )}
      {trendHost && <HostTrendDialog host={trendHost} onClose={() => setTrendHost(null)} />}
    </div>
  );
}

function isPermissionMessage(message: string) {
  return message.includes('当前用户无权执行此操作');
}
