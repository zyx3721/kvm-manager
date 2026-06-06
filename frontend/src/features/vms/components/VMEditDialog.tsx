import { useEffect, useState } from 'react';
import {
  CableIcon,
  CpuIcon,
  Disc3Icon,
  FileCode2Icon,
  InfoIcon,
  XIcon,
} from 'lucide-react';
import { fetchVMConfig, type VMConfig, type VMConfigDisk, type VirtualMachine } from '../../../lib/api';
import { formatBytes, formatOSType } from '../../../lib/format';
import { DialogPortal } from '../../../components/kvm/DialogPortal';
import { StatusBadge } from '../../../components/kvm/StatusBadge';
import { BasicInfoPanel } from './edit/BasicInfoPanel';
import { DevicesPanel } from './edit/DevicesPanel';
import { MediaPanel } from './edit/MediaPanel';
import { ResourcesPanel } from './edit/ResourcesPanel';
import { XMLPanel } from './edit/XMLPanel';

type VMEditDialogProps = {
  vm: VirtualMachine;
  onClose: () => void;
  onUpdated?: () => void | Promise<void>;
};

type EditTab = 'basic' | 'resources' | 'media' | 'devices' | 'xml';

type TabItem = {
  key: EditTab;
  label: string;
  icon: typeof CpuIcon;
};

const tabs: TabItem[] = [
  { key: 'basic', label: '基本信息', icon: InfoIcon },
  { key: 'resources', label: 'CPU与内存', icon: CpuIcon },
  { key: 'media', label: '介质', icon: Disc3Icon },
  { key: 'devices', label: '磁盘与网络', icon: CableIcon },
  { key: 'xml', label: 'XML', icon: FileCode2Icon },
];

export function VMEditDialog({ vm, onClose, onUpdated }: VMEditDialogProps) {
  const [activeTab, setActiveTab] = useState<EditTab>('basic');
  const [config, setConfig] = useState<VMConfig | null>(null);
  const memoryMB = Math.max(
    1,
    Math.round((config?.currentMemoryBytes || vm.memoryBytes) / 1024 / 1024)
  );
  const maxCpu = Math.max(config?.hostCpu || 0, config?.maximumCpu || 0, vm.cpuCores, 1);
  const diskCount = config?.disks.length || vm.disks.length;
  const diskLabel = diskCount > 0 ? `${diskCount} 块磁盘` : formatBytes(vm.diskBytes, 'GB');

  useEffect(() => {
    let ignore = false;
    fetchVMConfig(vm.id)
      .then(item => {
        if (!ignore) setConfig(item);
      })
      .catch(() => undefined);
    return () => {
      ignore = true;
    };
  }, [vm.id]);

  function handleConfigChange(nextConfig: VMConfig) {
    setConfig(current => mergeConfigDiskCapacities(nextConfig, current, vm));
    void onUpdated?.();
  }

  return (
    <DialogPortal>
    <div className="kvm-dialog-backdrop fixed inset-0 z-50 flex items-center justify-center px-3 py-5">
      <div className="kvm-dialog-panel flex max-h-[88vh] w-[min(92vw,920px)] flex-col overflow-hidden rounded-2xl shadow-2xl">
        <header
          className="flex min-h-14 shrink-0 items-center justify-between gap-3 border-b px-4 py-2.5"
          style={{ borderColor: 'var(--kvm-border)' }}
        >
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="truncate text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
                编辑虚拟机
              </h2>
              <StatusBadge status={vm.status} />
            </div>
            <div
              className="mt-0.5 flex flex-wrap gap-x-2.5 gap-y-0.5 text-[11px]"
              style={{ color: 'var(--kvm-text-muted)' }}
            >
              <span className="font-mono">{vm.name}</span>
              <span>{vm.hostName || '-'}</span>
              <span>{formatOSType(vm.osType, vm.name)}</span>
              <span>{vm.primaryIp || '-'}</span>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="kvm-action-button flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border"
            style={{
              background: 'var(--kvm-control-bg)',
              borderColor: 'var(--kvm-border)',
              color: 'var(--kvm-text-muted)',
            }}
            aria-label="关闭编辑窗口"
          >
            <XIcon size={15} />
          </button>
        </header>

        <nav
          className="kvm-hidden-scrollbar flex shrink-0 gap-2 overflow-x-auto border-b px-3 py-2"
          style={{
            borderColor: 'var(--kvm-border)',
            background: 'var(--kvm-table-head-bg)',
          }}
          aria-label="虚拟机编辑导航"
        >
          {tabs.map(item => {
            const Icon = item.icon;
            const active = item.key === activeTab;
            return (
              <button
                key={item.key}
                type="button"
                onClick={() => setActiveTab(item.key)}
                className="kvm-action-button flex h-10 shrink-0 items-center gap-2 rounded-xl border px-3 text-xs font-semibold transition-colors"
                style={{
                  background: active ? 'rgba(59,130,246,0.15)' : 'var(--kvm-control-bg-soft)',
                  borderColor: active ? 'rgba(96,165,250,0.48)' : 'rgba(76,103,150,0.24)',
                  color: active ? 'var(--kvm-accent-text)' : 'var(--kvm-text-muted)',
                  boxShadow: active
                    ? '0 12px 24px rgba(15,23,42,0.28), inset 0 1px 0 rgba(255,255,255,0.12)'
                    : 'inset 0 1px 0 rgba(255,255,255,0.04)',
                  transform: active ? 'translateY(-1px)' : 'translateY(0)',
                }}
                aria-current={active ? 'page' : undefined}
              >
                <Icon size={14} />
                {item.label}
              </button>
            );
          })}
        </nav>

        <main
          className="kvm-hidden-scrollbar min-h-0 flex-1 overflow-y-auto p-4"
          style={{ background: 'var(--kvm-control-bg-soft)' }}
        >
          <div className="kvm-dialog-card rounded-2xl p-4">
            {activeTab === 'basic' && (
              <BasicInfoPanel vm={vm} config={config} onConfigChange={handleConfigChange} />
            )}
            {activeTab === 'resources' && (
              <ResourcesPanel
                vm={vm}
                config={config}
                memoryMB={memoryMB}
                maxCpu={maxCpu}
                onConfigChange={handleConfigChange}
              />
            )}
            {activeTab === 'media' && <MediaPanel vm={vm} config={config} onConfigChange={handleConfigChange} />}
            {activeTab === 'devices' && (
              <DevicesPanel vm={vm} config={config} diskLabel={diskLabel} onConfigChange={handleConfigChange} />
            )}
            {activeTab === 'xml' && (
              <XMLPanel vm={vm} config={config} memoryMB={memoryMB} onConfigChange={handleConfigChange} />
            )}
          </div>
        </main>
      </div>
    </div>
    </DialogPortal>
  );
}

function mergeConfigDiskCapacities(nextConfig: VMConfig, currentConfig: VMConfig | null, vm: VirtualMachine): VMConfig {
  const knownDisks = [...(currentConfig?.disks || []), ...vm.disks];
  const capacityByKey = new Map<string, number>();
  for (const disk of knownDisks) {
    const bytes = disk.bytes || 0;
    if (bytes <= 0) continue;
    for (const key of diskCapacityKeys(disk)) {
      capacityByKey.set(key, bytes);
    }
  }
  return {
    ...nextConfig,
    disks: nextConfig.disks.map(disk => {
      if (disk.bytes > 0) return disk;
      for (const key of diskCapacityKeys(disk)) {
        const bytes = capacityByKey.get(key);
        if (bytes && bytes > 0) return { ...disk, bytes };
      }
      return disk;
    }),
  };
}

function diskCapacityKeys(disk: Pick<VMConfigDisk, 'name' | 'path'>) {
  return [disk.name, disk.path].map(value => value.trim()).filter(Boolean);
}
