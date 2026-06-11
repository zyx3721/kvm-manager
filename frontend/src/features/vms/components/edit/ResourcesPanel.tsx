import { useEffect, useState } from 'react';

import { CpuIcon, HardDriveIcon, MemoryStickIcon } from 'lucide-react';
import { toast } from 'sonner';

import { updateVMConfig, type VMConfig, type VirtualMachine } from '../../../../lib/api';
import { formatBytes } from '../../../../lib/format';
import { AllocationControl, CheckToggle, PrimaryButton } from '../VMEditControls';
import { CardSection, FieldText, FormGrid, SummaryCard } from './EditShared';
import {
  bytesToMB,
  memoryOptions,
  memoryRangeOptions,
  nearestMemoryOption,
  numberRange,
  parsePositiveInteger,
} from './editUtils';
import { isVMRunning } from '../../utils/vmStatus';

export function ResourcesPanel({
  vm,
  config,
  memoryMB,
  maxCpu,
  onConfigChange,
}: {
  vm: VirtualMachine;
  config: VMConfig | null;
  memoryMB: number;
  maxCpu: number;
  onConfigChange: (config: VMConfig) => void;
}) {
  const [currentCpu, setCurrentCpu] = useState(String(config?.currentCpu || vm.cpuCores));
  const [maximumCpu, setMaximumCpu] = useState(String(config?.maximumCpu || maxCpu));
  const [currentMemory, setCurrentMemory] = useState(String(nearestMemoryOption(memoryMB)));
  const [maximumMemory, setMaximumMemory] = useState(
    String(nearestMemoryOption(bytesToMB(config?.maximumMemoryBytes || vm.memoryBytes)))
  );
  const [customCurrentCpu, setCustomCurrentCpu] = useState(false);
  const [customMaximumCpu, setCustomMaximumCpu] = useState(false);
  const [customCurrentMemory, setCustomCurrentMemory] = useState(false);
  const [customMaximumMemory, setCustomMaximumMemory] = useState(false);
  const [memoryStatsEnabled, setMemoryStatsEnabled] = useState(
    (config?.memoryStatsPeriod ?? 5) > 0
  );
  const [memoryStatsPeriod, setMemoryStatsPeriod] = useState(
    String(config?.memoryStatsPeriod || 5)
  );
  const [saving, setSaving] = useState(false);
  const running = isVMRunning(config?.status || vm.status);
  const hostCpu = config?.hostCpu && config.hostCpu > 0 ? config.hostCpu : 0;
  const hostMemoryBytes =
    config?.hostMemoryBytes && config.hostMemoryBytes > 0 ? config.hostMemoryBytes : 0;
  const hostMemoryMB = hostMemoryBytes > 0 ? bytesToMB(hostMemoryBytes) : 0;
  const currentCpuFloor = running ? config?.currentCpu || 1 : 1;
  const currentCpuLimit = running ? config?.maximumCpu || maxCpu : maxCpu;
  const currentMemoryFloor = running ? bytesToMB(config?.currentMemoryBytes || vm.memoryBytes) : 0;
  const currentMemoryLimit = running
    ? bytesToMB(config?.maximumMemoryBytes || vm.memoryBytes)
    : hostMemoryMB || memoryOptions[memoryOptions.length - 1];
  const stoppedMemoryOptions = memoryRangeOptions(currentMemoryLimit);
  const currentMemoryOptions = running
    ? memoryOptions.filter(item => item >= currentMemoryFloor && item <= currentMemoryLimit)
    : stoppedMemoryOptions;

  useEffect(() => {
    if (!config) return;
    setCurrentCpu(String(config.currentCpu || vm.cpuCores));
    setMaximumCpu(String(config.maximumCpu || maxCpu));
    const nextCurrentMemory = bytesToMB(config.currentMemoryBytes);
    const nextMaximumMemory = bytesToMB(config.maximumMemoryBytes);
    setCurrentMemory(String(nextCurrentMemory));
    setMaximumMemory(String(nextMaximumMemory));
    setCustomCurrentCpu(false);
    setCustomMaximumCpu(false);
    setCustomCurrentMemory(!memoryOptions.includes(nextCurrentMemory));
    setCustomMaximumMemory(!memoryOptions.includes(nextMaximumMemory));
    setMemoryStatsEnabled((config.memoryStatsPeriod || 0) > 0);
    setMemoryStatsPeriod(String(config.memoryStatsPeriod || 5));
  }, [config, maxCpu, vm.cpuCores]);

  async function handleSubmit() {
    if (!config) return toast.warning('虚拟机配置尚未加载完成');
    const currentCpuValue = parsePositiveInteger('逻辑 CPU 当前分配', currentCpu, customCurrentCpu);
    const maximumCpuValue = parsePositiveInteger('逻辑 CPU 最大分配', maximumCpu, customMaximumCpu);
    const currentMemoryValue = parsePositiveInteger(
      '总内存当前分配',
      currentMemory,
      customCurrentMemory
    );
    const maximumMemoryValue = parsePositiveInteger(
      '总内存最大分配',
      maximumMemory,
      customMaximumMemory
    );
    if (!currentCpuValue || !maximumCpuValue || !currentMemoryValue || !maximumMemoryValue) return;
    const memoryStatsPeriodValue = memoryStatsEnabled
      ? parsePositiveInteger('内存统计周期', memoryStatsPeriod, true)
      : 0;
    if (memoryStatsPeriodValue === null) return;
    if (memoryStatsPeriodValue > 86400) return toast.error('内存统计周期不能超过 86400 秒');
    if (currentCpuValue > maximumCpuValue) return toast.error('逻辑 CPU 当前分配不能大于最大分配');
    if (currentMemoryValue > maximumMemoryValue)
      return toast.error('总内存当前分配不能大于最大分配');
    if (running && maximumCpuValue !== config.maximumCpu)
      return toast.error('虚拟机正在运行，最大 CPU 需关机后修改');
    if (running && maximumMemoryValue !== bytesToMB(config.maximumMemoryBytes))
      return toast.error('虚拟机正在运行，最大内存需关机后修改');
    if (running && currentCpuValue < config.currentCpu)
      return toast.error('虚拟机正在运行，CPU 只能扩容不能缩容');
    if (running && currentMemoryValue < bytesToMB(config.currentMemoryBytes))
      return toast.error('虚拟机正在运行，内存只能扩容不能缩容');
    if (running && currentCpuValue > config.maximumCpu)
      return toast.error('当前 CPU 不能超过已预留的最大 CPU');
    if (running && currentMemoryValue > bytesToMB(config.maximumMemoryBytes))
      return toast.error('当前内存不能超过已预留的最大内存');
    if (hostCpu > 0 && maximumCpuValue > hostCpu)
      return toast.error('逻辑 CPU 最大分配不能超过宿主机逻辑 CPU');
    if (hostMemoryMB > 0 && maximumMemoryValue > hostMemoryMB)
      return toast.error('总内存最大分配不能超过宿主机总内存');
    const unchanged =
      config &&
      currentCpuValue === (config.currentCpu || vm.cpuCores) &&
      maximumCpuValue === (config.maximumCpu || maxCpu) &&
      currentMemoryValue === bytesToMB(config.currentMemoryBytes) &&
      maximumMemoryValue === bytesToMB(config.maximumMemoryBytes) &&
      memoryStatsPeriodValue === (config.memoryStatsPeriod || 0);
    if (unchanged) return toast.warning('请先修改配置');
    setSaving(true);
    try {
      const result = await updateVMConfig(vm.id, {
        description: config.description || '',
        currentCpu: currentCpuValue,
        maximumCpu: maximumCpuValue,
        currentMemoryMB: currentMemoryValue,
        maximumMemoryMB: maximumMemoryValue,
        memoryStatsPeriod: memoryStatsPeriodValue,
      });
      onConfigChange(result.config);
      toast.success('虚拟机资源配置已修改');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '虚拟机资源配置修改失败');
    } finally {
      setSaving(false);
    }
  }

  return (
    <section className="mx-auto max-w-3xl space-y-4">
      <div className="grid gap-3 md:grid-cols-3">
        <SummaryCard
          icon={CpuIcon}
          label="CPU"
          value={`${config?.currentCpu || vm.cpuCores} / ${config?.maximumCpu || maxCpu} vCPU`}
          color="#60a5fa"
        />
        <SummaryCard
          icon={MemoryStickIcon}
          label="内存"
          value={`${formatBytes(config?.currentMemoryBytes || vm.memoryBytes, 'GB')} / ${formatBytes(config?.maximumMemoryBytes || vm.memoryBytes, 'GB')}`}
          color="#2dd4bf"
        />
        <SummaryCard
          icon={HardDriveIcon}
          label="磁盘"
          value={formatBytes(vm.diskBytes, 'GB')}
          color="#f59e0b"
        />
      </div>

      <CardSection title={`宿主机逻辑 CPU: ${config?.hostCpu || maxCpu}`}>
        {running && (
          <div
            className="mb-3 rounded-lg border px-3 py-2 text-xs"
            style={{
              background: 'rgba(245,158,11,0.08)',
              borderColor: 'rgba(245,158,11,0.28)',
              color: '#fbbf24',
            }}
          >
            虚拟机正在运行，仅支持在已预留上限内热扩容当前 CPU 与内存
          </div>
        )}
        <FormGrid align="start">
          <FieldText>当前分配</FieldText>
          <AllocationControl
            value={currentCpu}
            custom={customCurrentCpu}
            values={numberRange(currentCpuFloor, currentCpuLimit)}
            disabled={saving}
            onValueChange={setCurrentCpu}
            onCustomChange={setCustomCurrentCpu}
          />
          <FieldText>最大分配</FieldText>
          <AllocationControl
            value={maximumCpu}
            custom={customMaximumCpu}
            values={numberRange(vm.cpuCores, maxCpu)}
            disabled={running}
            onValueChange={setMaximumCpu}
            onCustomChange={setCustomMaximumCpu}
          />
        </FormGrid>
      </CardSection>

      <CardSection
        title={`宿主机总内存: ${formatBytes(Math.max(config?.hostMemoryBytes || 0, 1), 'GB')}`}
      >
        <FormGrid align="start">
          <FieldText className="pt-2">当前分配 (MB)</FieldText>
          <AllocationControl
            value={currentMemory}
            custom={customCurrentMemory}
            values={currentMemoryOptions.length ? currentMemoryOptions : [currentMemoryFloor]}
            disabled={saving}
            onValueChange={setCurrentMemory}
            onCustomChange={setCustomCurrentMemory}
          />
          <FieldText className="pt-2">最大分配 (MB)</FieldText>
          <AllocationControl
            value={maximumMemory}
            custom={customMaximumMemory}
            values={running ? memoryOptions : stoppedMemoryOptions}
            disabled={running}
            onValueChange={setMaximumMemory}
            onCustomChange={setCustomMaximumMemory}
          />
          <FieldText className="pt-2">统计周期</FieldText>
          <div className="space-y-2">
            <div className="flex max-w-xs items-center gap-2">
              <input
                value={memoryStatsEnabled ? memoryStatsPeriod : ''}
                inputMode="numeric"
                disabled={saving || !memoryStatsEnabled}
                onChange={event => setMemoryStatsPeriod(event.target.value)}
                className="h-9 w-28 rounded-lg px-3 text-sm outline-none disabled:opacity-60"
                style={{
                  background: 'var(--kvm-control-bg)',
                  border: '1px solid var(--kvm-border)',
                  color: 'var(--kvm-text)',
                  boxShadow: 'inset 0 1px 0 rgba(255,255,255,0.045)',
                }}
              />
              <span className="text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
                秒
              </span>
            </div>
            <CheckToggle
              checked={memoryStatsEnabled}
              disabled={saving}
              label="启用内存统计周期"
              onChange={setMemoryStatsEnabled}
            />
          </div>
        </FormGrid>
      </CardSection>

      <div className="flex justify-end">
        <PrimaryButton
          label={saving ? '修改中...' : '修改'}
          disabled={saving}
          onClick={() => void handleSubmit()}
        />
      </div>
    </section>
  );
}
