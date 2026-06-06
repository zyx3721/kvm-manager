import { PlusIcon } from 'lucide-react';

import { SelectMenu } from '../../../../components/kvm/SelectMenu';
import { XMLTextEditor } from '../XMLTextEditor';
import type { CreateDiskDraft } from './CreateExtraDiskCard';
import { CreateExtraDiskCard } from './CreateExtraDiskCard';
import {
  Field,
  MetadataToggleRow,
  NumberField,
  Panel,
  PasswordInput,
  PrimaryButton,
  Toggle,
  fieldStyle,
  inputClass,
} from './CreateFormShared';

type Option = { value: string; label: string; tooltip?: string };

export function XMLCreatePanel({
  agentId,
  hostOptions,
  xml,
  busy,
  onAgentChange,
  onXMLChange,
}: {
  agentId: string;
  hostOptions: Option[];
  xml: string;
  busy: boolean;
  onAgentChange: (value: string) => void;
  onXMLChange: (value: string) => void;
}) {
  return (
    <Panel title="XML 创建">
      <div className="mb-3 flex items-center gap-3">
        <div className="shrink-0 text-xs font-semibold" style={{ color: 'var(--kvm-text-muted)' }}>
          宿主机
        </div>
        <div className="min-w-0 flex-1">
          <SelectMenu
            value={agentId}
            options={hostOptions}
            placeholder="选择宿主机"
            onChange={onAgentChange}
          />
        </div>
      </div>
      <XMLTextEditor
        value={xml}
        disabled={busy}
        className="h-[min(48vh,460px)]"
        heightClassName="h-full"
        onChange={onXMLChange}
      />
    </Panel>
  );
}

export function BasicInfoPanel({
  agentId,
  hostOptions,
  name,
  description,
  osType,
  osTypeOptions,
  autostart,
  busy,
  onAgentChange,
  onNameChange,
  onDescriptionChange,
  onOSTypeChange,
  onAutostartChange,
}: {
  agentId: string;
  hostOptions: Option[];
  name: string;
  description: string;
  osType: string;
  osTypeOptions: Option[];
  autostart: boolean;
  busy: boolean;
  onAgentChange: (value: string) => void;
  onNameChange: (value: string) => void;
  onDescriptionChange: (value: string) => void;
  onOSTypeChange: (value: string) => void;
  onAutostartChange: (value: boolean) => void;
}) {
  return (
    <Panel title="基础信息">
      <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-[minmax(160px,0.8fr)_minmax(150px,0.85fr)_minmax(200px,1.1fr)_minmax(140px,0.95fr)_minmax(142px,0.7fr)]">
        <Field label="宿主机">
          <SelectMenu
            value={agentId}
            options={hostOptions}
            placeholder="选择宿主机"
            onChange={onAgentChange}
          />
        </Field>
        <Field label="虚拟机名称">
          <input
            value={name}
            disabled={busy}
            onChange={event => onNameChange(event.target.value)}
            className={inputClass}
            style={fieldStyle}
          />
        </Field>
        <Field label="描述">
          <input
            value={description}
            disabled={busy}
            onChange={event => onDescriptionChange(event.target.value)}
            className={inputClass}
            style={fieldStyle}
          />
        </Field>
        <Field label="操作系统类型">
          <SelectMenu
            value={osType}
            options={osTypeOptions}
            placeholder="选择系统"
            onChange={onOSTypeChange}
          />
        </Field>
        <Field label="创建后直接启动">
          <Toggle
            checked={autostart}
            disabled={busy}
            label={autostart ? '已启用' : '不启用'}
            onChange={onAutostartChange}
          />
        </Field>
      </div>
    </Panel>
  );
}

export function ComputePanel({
  cpu,
  maxCPU,
  memory,
  maxMemory,
  cpuModel,
  cpuModels,
  hostCpu,
  hostMemoryMB,
  onCPUChange,
  onMaxCPUChange,
  onMemoryChange,
  onMaxMemoryChange,
  onCPUModelChange,
}: {
  cpu: number;
  maxCPU: number;
  memory: number;
  maxMemory: number;
  cpuModel: string;
  cpuModels: Option[];
  hostCpu?: number;
  hostMemoryMB: number;
  onCPUChange: (value: number) => void;
  onMaxCPUChange: (value: number) => void;
  onMemoryChange: (value: number) => void;
  onMaxMemoryChange: (value: number) => void;
  onCPUModelChange: (value: string) => void;
}) {
  return (
    <Panel title="计算资源">
      <div className="grid gap-3 md:grid-cols-5">
        <NumberField label="CPU" value={cpu} onChange={onCPUChange} />
        <NumberField label="最大 CPU" value={maxCPU} onChange={onMaxCPUChange} />
        <NumberField label="内存 MB" value={memory} onChange={onMemoryChange} />
        <NumberField label="最大内存 MB" value={maxMemory} onChange={onMaxMemoryChange} />
        <Field label="CPU 模式">
          <SelectMenu
            value={cpuModel}
            options={cpuModels}
            placeholder="CPU 模式"
            optionTooltipPlacement="right"
            onChange={onCPUModelChange}
          />
        </Field>
      </div>
      {(hostCpu || hostMemoryMB > 0) && (
        <div className="mt-2 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
          宿主机上限：{hostCpu || '-'} vCPU / {formatHostMemoryLimit(hostMemoryMB)}
        </div>
      )}
    </Panel>
  );
}

function formatHostMemoryLimit(valueMB: number) {
  if (valueMB <= 0) return '-';
  const valueGB = valueMB / 1024;
  return `${valueGB.toFixed(1)} GB`;
}

export function BlankStoragePanel(props: {
  busy: boolean;
  storagePoolsLength: number;
  systemDiskTarget: string;
  diskPool: string;
  diskFormat: string;
  diskBus: string;
  diskSize: number;
  diskName: string;
  preallocMetadata: boolean;
  extraDisks: CreateDiskDraft[];
  storageOptions: Option[];
  formats: Option[];
  buses: Option[];
  isoPool: string;
  isoPath: string;
  isoBus: string;
  isoOptions: Option[];
  isoBuses: Option[];
  onDiskPoolChange: (value: string) => void;
  onDiskFormatChange: (value: string) => void;
  onDiskBusChange: (value: string) => void;
  onDiskSizeChange: (value: number) => void;
  onDiskNameChange: (value: string) => void;
  onDiskNameTouched: () => void;
  onPreallocChange: (value: boolean) => void;
  onISOPoolChange: (value: string) => void;
  onISOPathChange: (value: string) => void;
  onISOBusChange: (value: string) => void;
  onAddExtraDisk: () => void;
  onUpdateExtraDisk: (id: string, capacityGB: string) => void;
  onRemoveExtraDisk: (id: string) => void;
}) {
  return (
    <Panel title="存储与镜像">
      <div className="space-y-3">
        <div
          className="rounded-xl border p-3"
          style={{ background: 'var(--kvm-control-bg-soft)', borderColor: 'var(--kvm-border)' }}
        >
          <div className="mb-3 flex items-center justify-between gap-3">
            <div className="text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
              系统盘
            </div>
            <span
              className="rounded-md border px-2 py-0.5 font-mono text-xs"
              style={{
                background: 'var(--kvm-control-bg)',
                borderColor: 'var(--kvm-border)',
                color: 'var(--kvm-text-muted)',
              }}
            >
              {props.systemDiskTarget}
            </span>
          </div>
          <div className="grid gap-3 md:grid-cols-5">
            <Field label="存储池">
              <SelectMenu
                value={props.diskPool}
                options={props.storageOptions}
                placeholder="选择存储池"
                onChange={props.onDiskPoolChange}
              />
            </Field>
            <Field label="磁盘格式">
              <SelectMenu
                value={props.diskFormat}
                options={props.formats}
                placeholder="格式"
                onChange={props.onDiskFormatChange}
              />
            </Field>
            <Field label="磁盘总线">
              <SelectMenu
                value={props.diskBus}
                options={props.buses}
                placeholder="总线"
                onChange={props.onDiskBusChange}
              />
            </Field>
            <NumberField label="容量 GB" value={props.diskSize} onChange={props.onDiskSizeChange} />
            <Field label="卷名称">
              <input
                value={props.diskName}
                disabled={props.busy}
                onChange={event => {
                  props.onDiskNameTouched();
                  props.onDiskNameChange(event.target.value);
                }}
                className={inputClass}
                style={fieldStyle}
              />
            </Field>
            <Field label="ISO 池">
              <SelectMenu
                value={props.isoPool}
                options={props.storageOptions}
                placeholder="选择 ISO 池"
                onChange={props.onISOPoolChange}
              />
            </Field>
            <Field label="ISO 镜像">
              <SelectMenu
                value={props.isoPath}
                options={props.isoOptions}
                placeholder="选择 ISO"
                maxVisibleItems={5}
                onChange={props.onISOPathChange}
              />
            </Field>
            <Field label="光驱总线">
              <SelectMenu
                value={props.isoBus}
                options={props.isoBuses}
                placeholder="光驱总线"
                onChange={props.onISOBusChange}
              />
            </Field>
          </div>
          <MetadataToggleRow
            format={props.diskFormat}
            checked={props.preallocMetadata}
            disabled={props.busy}
            onChange={props.onPreallocChange}
          />
        </div>
        {props.extraDisks.map((disk, index) => (
          <CreateExtraDiskCard
            key={disk.id}
            disk={disk}
            index={index + 1}
            disabled={props.busy}
            storageOptions={props.storageOptions}
            metadataDisabled
            onCapacityChange={capacityGB => props.onUpdateExtraDisk(disk.id, capacityGB)}
            onRemove={() => props.onRemoveExtraDisk(disk.id)}
          />
        ))}
        <button
          type="button"
          disabled={props.busy || props.storagePoolsLength === 0}
          onClick={props.onAddExtraDisk}
          className="kvm-action-button inline-flex h-9 items-center gap-2 rounded-lg border px-3 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-60"
          style={{
            background: 'rgba(45,212,191,0.1)',
            borderColor: 'rgba(45,212,191,0.32)',
            color: 'var(--kvm-check-toggle-active-text)',
          }}
        >
          <PlusIcon size={15} />
          添加新磁盘
        </button>
      </div>
    </Panel>
  );
}

export function NetworkBootPanel(props: {
  networkSource: string;
  networkOptions: Option[];
  networkModel: string;
  networkModelOptions: Option[];
  firmware: string;
  firmwareOptions: Option[];
  graphics: string;
  graphicsOptions: Option[];
  consolePasswordEnabled: boolean;
  consolePassword: string;
  consolePasswordVisible: boolean;
  busy: boolean;
  onNetworkSourceChange: (value: string) => void;
  onNetworkModelChange: (value: string) => void;
  onFirmwareChange: (value: string) => void;
  onGraphicsChange: (value: string) => void;
  onConsolePasswordEnabledChange: (value: boolean) => void;
  onConsolePasswordChange: (value: string) => void;
  onConsolePasswordVisibleChange: (value: boolean) => void;
}) {
  return (
    <Panel title="网络与启动">
      <div className="grid gap-3 md:grid-cols-5">
        <Field label="网络池">
          <SelectMenu
            value={props.networkSource}
            options={props.networkOptions}
            placeholder="选择网络池"
            onChange={props.onNetworkSourceChange}
          />
        </Field>
        <Field label="网卡模型">
          <SelectMenu
            value={props.networkModel}
            options={props.networkModelOptions}
            placeholder="网卡模型"
            onChange={props.onNetworkModelChange}
          />
        </Field>
        <Field label="固件">
          <SelectMenu
            value={props.firmware}
            options={props.firmwareOptions}
            placeholder="固件"
            onChange={props.onFirmwareChange}
          />
        </Field>
        <Field label="控制台类型">
          <SelectMenu
            value={props.graphics}
            options={props.graphicsOptions}
            placeholder="控制台"
            onChange={props.onGraphicsChange}
          />
        </Field>
        <Field label="控制台密码">
          <Toggle
            checked={props.consolePasswordEnabled}
            disabled={props.busy}
            label={props.consolePasswordEnabled ? '已启用' : '未启用'}
            onChange={props.onConsolePasswordEnabledChange}
          />
        </Field>
      </div>
      {props.consolePasswordEnabled && (
        <div className="mt-3 max-w-xs">
          <Field label="VNC 访问密码">
            <PasswordInput
              value={props.consolePassword}
              disabled={props.busy}
              visible={props.consolePasswordVisible}
              onVisibleChange={props.onConsolePasswordVisibleChange}
              onChange={props.onConsolePasswordChange}
            />
          </Field>
        </div>
      )}
    </Panel>
  );
}

export { PrimaryButton };
