import { useEffect, useMemo, useState } from 'react';
import { Disc3Icon, RefreshCwIcon } from 'lucide-react';
import { toast } from 'sonner';

import {
  connectVMMedia,
  disconnectVMMedia,
  fetchISOFiles,
  fetchStoragePools,
  type ISOFile,
  type StoragePool,
  type VMConfig,
  type VirtualMachine,
} from '../../../../lib/api';
import { formatBytesAutoFixed } from '../../../../lib/format';
import { SelectMenu } from '../../../../components/kvm/SelectMenu';
import { CardSection } from './EditShared';
import { isVMRunning } from '../../utils/vmStatus';

export function MediaPanel({
  vm,
  config,
  onConfigChange,
}: {
  vm: VirtualMachine;
  config: VMConfig | null;
  onConfigChange: (config: VMConfig) => void;
}) {
  const [pools, setPools] = useState<StoragePool[]>([]);
  const [selectedPool, setSelectedPool] = useState('');
  const [isoFiles, setISOFiles] = useState<ISOFile[]>([]);
  const [selectedISO, setSelectedISO] = useState('');
  const [selectedTarget, setSelectedTarget] = useState('');
  const [loading, setLoading] = useState(false);
  const [connecting, setConnecting] = useState(false);
  const [disconnecting, setDisconnecting] = useState(false);
  const running = isVMRunning(config?.status || vm.status);
  const cdroms = useMemo(() => config?.cdroms || [], [config?.cdroms]);
  const connectedCDROM = useMemo(() => cdroms.find(cdrom => cdrom.connected) || null, [cdroms]);
  const selectedCDROM = useMemo(
    () => connectedCDROM || cdroms.find(cdrom => cdrom.name === selectedTarget) || null,
    [cdroms, connectedCDROM, selectedTarget]
  );
  const mediaConnected = Boolean(connectedCDROM);
  const displayCDROMs = cdroms.length
    ? cdroms
    : [{ name: 'CDROM 1', path: '', bus: '', connected: false }];
  const controlsDisabled = running || mediaConnected || loading || connecting || disconnecting;

  const isoPools = useMemo(
    () => pools.filter(pool => ['dir', 'iso'].includes((pool.type || '').toLowerCase())),
    [pools]
  );

  async function loadPools() {
    if (!vm.hostId) return;
    setLoading(true);
    try {
      const body = await fetchStoragePools(vm.hostId);
      setPools(body.items);
      const current =
        selectedPool ||
        body.items.find(pool => ['iso', 'dir'].includes((pool.type || '').toLowerCase()))?.name ||
        '';
      setSelectedPool(current);
      if (current) {
        const isoBody = await fetchISOFiles(vm.hostId, current);
        setISOFiles(isoBody.items);
        setSelectedISO(isoBody.items[0]?.path || '');
      } else {
        setISOFiles([]);
        setSelectedISO('');
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '读取 ISO 镜像失败');
    } finally {
      setLoading(false);
    }
  }

  async function handlePoolChange(poolName: string) {
    if (running || mediaConnected) return;
    setSelectedPool(poolName);
    setLoading(true);
    try {
      const body = await fetchISOFiles(vm.hostId, poolName);
      setISOFiles(body.items);
      setSelectedISO(body.items[0]?.path || '');
    } catch (error) {
      setISOFiles([]);
      setSelectedISO('');
      toast.error(error instanceof Error ? error.message : '读取 ISO 镜像失败');
    } finally {
      setLoading(false);
    }
  }

  async function handleConnect() {
    if (running) {
      toast.warning('虚拟机正在运行，不支持连接介质，请先关闭虚拟机后再操作');
      return;
    }
    if (mediaConnected) {
      toast.warning('请先断开当前介质后再连接新的 ISO');
      return;
    }
    if (!config) {
      toast.warning('虚拟机配置尚未加载完成');
      return;
    }
    if (!selectedTarget) {
      toast.warning('请选择要连接的光驱');
      return;
    }
    if (!selectedISO) {
      toast.warning('请选择 ISO 文件');
      return;
    }
    setConnecting(true);
    try {
      const result = await connectVMMedia(vm.id, { target: selectedTarget, isoPath: selectedISO });
      onConfigChange(result.config);
      toast.success('介质连接成功');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '连接介质失败');
    } finally {
      setConnecting(false);
    }
  }

  async function handleDisconnect() {
    if (running) {
      toast.warning('虚拟机正在运行，不支持断开介质，请先关闭虚拟机后再操作');
      return;
    }
    if (!config) {
      toast.warning('虚拟机配置尚未加载完成');
      return;
    }
    if (!selectedTarget) {
      toast.warning('请选择要断开的光驱');
      return;
    }
    if (!mediaConnected) {
      toast.warning('当前光驱未连接介质');
      return;
    }
    setDisconnecting(true);
    try {
      const result = await disconnectVMMedia(vm.id, { target: selectedTarget });
      if (config) {
        onConfigChange(disconnectLocalCDROM(config, selectedTarget));
      }
      toast.success('介质已断开');
      onConfigChange(result.config);
      if (pools.length === 0) {
        void loadPools();
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '断开介质失败');
    } finally {
      setDisconnecting(false);
    }
  }

  useEffect(() => {
    void loadPools();
  }, [vm.hostId]);

  useEffect(() => {
    if (!config || cdroms.length === 0) {
      setSelectedTarget('');
      return;
    }
    if (connectedCDROM && selectedTarget !== connectedCDROM.name) {
      setSelectedTarget(connectedCDROM.name || '');
      return;
    }
    if (!selectedTarget || !cdroms.some(cdrom => cdrom.name === selectedTarget)) {
      setSelectedTarget(cdroms[0].name || '');
    }
  }, [cdroms, config, connectedCDROM, selectedTarget]);

  return (
    <section className="mx-auto max-w-3xl space-y-4">
      <CardSection title="ISO 镜像选择">
        <div className="space-y-4">
          {running && (
            <div
              className="rounded-lg border px-3 py-2 text-xs"
              style={{
                background: 'rgba(245,158,11,0.08)',
                borderColor: 'rgba(245,158,11,0.28)',
                color: '#fbbf24',
              }}
            >
              虚拟机正在运行，介质连接和断开需先关闭虚拟机后再操作
            </div>
          )}
          <div
            className="rounded-lg border p-3 text-sm"
            style={{ borderColor: 'var(--kvm-border)', background: 'var(--kvm-control-bg-soft)' }}
          >
            <div
              className="mb-2 flex items-center gap-2 font-semibold"
              style={{ color: 'var(--kvm-text)' }}
            >
              <Disc3Icon size={15} />
              当前介质
            </div>
            <div className="space-y-1.5">
              {displayCDROMs.map(cdrom => (
                <div
                  key={cdrom.name || cdrom.path || 'cdrom'}
                  className="grid gap-2 text-xs md:grid-cols-[90px_1fr]"
                >
                  <span className="pl-10" style={{ color: 'var(--kvm-text)' }}>
                    {cdrom.name || 'CDROM'}
                  </span>
                  <span className="break-all font-mono" style={{ color: 'var(--kvm-text-muted)' }}>
                    {cdrom.connected
                      ? `${cdrom.path || '-'}${cdrom.bus ? ` (${cdrom.bus})` : ''}`
                      : '未连接介质'}
                  </span>
                </div>
              ))}
            </div>
          </div>
          <div className="grid gap-3 md:grid-cols-[140px_1fr] md:items-center">
            <div className="text-right text-sm" style={{ color: 'var(--kvm-text)' }}>
              目标光驱
            </div>
            <SelectMenu
              value={selectedTarget}
              disabled={cdroms.length === 0 || controlsDisabled}
              placeholder="暂无可用光驱"
              options={cdroms.map(cdrom => ({
                value: cdrom.name,
                label: `${cdrom.name || 'CDROM'}${cdrom.bus ? ` · ${cdrom.bus}` : ''}`,
              }))}
              menuClassName="!max-h-[120px]"
              onChange={setSelectedTarget}
            />
          </div>
          <div className="grid gap-3 md:grid-cols-[140px_1fr] md:items-center">
            <div className="text-right text-sm" style={{ color: 'var(--kvm-text)' }}>
              存储池
            </div>
            <div className="flex gap-2">
              <SelectMenu
                value={selectedPool}
                disabled={controlsDisabled}
                placeholder="暂无 ISO 存储池"
                options={isoPools.map(pool => ({
                  value: pool.name,
                  label: `${pool.name} · ${pool.path || pool.type}`,
                }))}
                className="flex-1"
                menuClassName="!max-h-[168px]"
                onChange={value => void handlePoolChange(value)}
              />
              <button
                type="button"
                disabled={controlsDisabled}
                onClick={() => void loadPools()}
                className="kvm-action-button inline-flex h-10 items-center gap-2 rounded-lg border px-3 text-sm disabled:cursor-not-allowed disabled:opacity-50"
                style={buttonStyle}
              >
                <RefreshCwIcon size={15} />
                刷新
              </button>
            </div>
          </div>
          <div className="grid gap-3 md:grid-cols-[140px_1fr] md:items-center">
            <div className="text-right text-sm" style={{ color: 'var(--kvm-text)' }}>
              ISO 文件
            </div>
            <SelectMenu
              value={selectedISO}
              disabled={controlsDisabled || !selectedPool || isoFiles.length === 0}
              placeholder={loading ? '正在读取 ISO 文件' : '暂无 ISO 文件'}
              options={isoFiles.map(file => ({
                value: file.path,
                label: `${file.name} · ${formatBytesAutoFixed(file.bytes)}`,
              }))}
              maxVisibleItems={3}
              onChange={setSelectedISO}
            />
          </div>
          <div
            className="rounded-lg border p-3 text-sm"
            style={{
              borderColor: 'var(--kvm-border)',
              background: 'var(--kvm-control-bg-soft)',
              color: 'var(--kvm-text-muted)',
            }}
          >
            <div
              className="mb-2 flex items-center gap-2 font-semibold"
              style={{ color: 'var(--kvm-text)' }}
            >
              <Disc3Icon size={15} />
              当前选择
            </div>
            <div className="pl-10 break-all font-mono text-xs">
              {selectedISO || '未选择 ISO 镜像'}
            </div>
          </div>
          <div className="flex flex-wrap items-center justify-end gap-3">
            <button
              type="button"
              disabled={running || connecting || disconnecting}
              onClick={() => void (mediaConnected ? handleDisconnect() : handleConnect())}
              className="kvm-action-button inline-flex h-10 items-center gap-2 rounded-lg border px-4 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-50"
              style={buttonStyle}
            >
              <Disc3Icon size={15} />
              {mediaConnected
                ? disconnecting
                  ? '正在断开'
                  : '断开'
                : connecting
                  ? '正在连接'
                  : '连接'}
            </button>
          </div>
        </div>
      </CardSection>
    </section>
  );
}

const buttonStyle = {
  background: 'var(--kvm-control-bg)',
  borderColor: 'var(--kvm-border)',
  color: 'var(--kvm-text)',
};

function disconnectLocalCDROM(config: VMConfig, target: string): VMConfig {
  return {
    ...config,
    cdroms: config.cdroms.map(cdrom =>
      cdrom.name === target ? { ...cdrom, path: '', connected: false } : cdrom
    ),
  };
}
