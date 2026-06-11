import { useState } from 'react';

import { FileStackIcon, Layers3Icon } from 'lucide-react';

import type { Host, VirtualMachine, VMCreatePayload } from '../../../../lib/api';
import { DiskTemplateCreatePanel } from './DiskTemplateCreatePanel';
import { TemplateVMCreatePanel } from './TemplateVMCreatePanel';

type TemplateCreateKind = 'vm' | 'disk';
type Option = { value: string; label: string; tooltip?: string };

export function TemplateCreatePanel({
  templates,
  selectedId,
  agentId,
  hostOptions,
  selectedHost,
  hostMemoryMB,
  storageOptions,
  networkOptions,
  osTypeOptions,
  cpuModels,
  buses,
  isoBuses,
  networkModelOptions,
  graphicsOptions,
  firmwareOptions,
  busy,
  onSelectedIdChange,
  onAgentChange,
  onSubmit,
  onQueued,
}: {
  templates: VirtualMachine[];
  selectedId: string;
  agentId: string;
  hostOptions: Option[];
  selectedHost?: Host;
  hostMemoryMB: number;
  storageOptions: Option[];
  networkOptions: Option[];
  osTypeOptions: Option[];
  cpuModels: Option[];
  buses: Option[];
  isoBuses: Option[];
  networkModelOptions: Option[];
  graphicsOptions: Option[];
  firmwareOptions: Option[];
  busy: boolean;
  onSelectedIdChange: (value: string) => void;
  onAgentChange: (value: string) => void;
  onSubmit: (result: {
    payload?: VMCreatePayload;
    warning?: string;
    vmName?: string;
  }) => Promise<void>;
  onQueued: () => void;
}) {
  const [kind, setKind] = useState<TemplateCreateKind>('vm');

  return (
    <div className="space-y-4">
      <div className="grid gap-3 md:grid-cols-2">
        <TemplateKindCard
          active={kind === 'vm'}
          icon={Layers3Icon}
          title="虚拟机模板"
          summary="从已标记模板克隆整机配置与磁盘"
          meta={`${templates.length} 个模板`}
          onClick={() => setKind('vm')}
        />
        <TemplateKindCard
          active={kind === 'disk'}
          icon={FileStackIcon}
          title="磁盘模板文件"
          summary="从 qcow2/img/raw 等模板卷创建系统盘"
          meta="选择存储卷"
          onClick={() => setKind('disk')}
        />
      </div>
      {kind === 'vm' ? (
        <TemplateVMCreatePanel
          templates={templates}
          selectedId={selectedId}
          onSelectedIdChange={onSelectedIdChange}
          onQueued={onQueued}
        />
      ) : (
        <DiskTemplateCreatePanel
          agentId={agentId}
          hostOptions={hostOptions}
          selectedHost={selectedHost}
          hostMemoryMB={hostMemoryMB}
          storageOptions={storageOptions}
          networkOptions={networkOptions}
          osTypeOptions={osTypeOptions}
          cpuModels={cpuModels}
          buses={buses}
          isoBuses={isoBuses}
          networkModelOptions={networkModelOptions}
          graphicsOptions={graphicsOptions}
          firmwareOptions={firmwareOptions}
          busy={busy}
          onAgentChange={onAgentChange}
          onSubmit={onSubmit}
        />
      )}
    </div>
  );
}

function TemplateKindCard({
  active,
  icon: Icon,
  title,
  summary,
  meta,
  onClick,
}: {
  active: boolean;
  icon: typeof Layers3Icon;
  title: string;
  summary: string;
  meta: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className="kvm-action-button group min-h-[92px] rounded-xl border p-4 text-left transition-all duration-150"
      style={{
        background: active ? 'rgba(59,130,246,0.12)' : 'var(--kvm-control-bg-soft)',
        borderColor: active ? 'rgba(147,197,253,0.55)' : 'var(--kvm-border)',
        boxShadow: active
          ? 'inset 0 1px 0 rgba(255,255,255,0.08), 0 14px 28px rgba(59,130,246,0.12)'
          : 'none',
      }}
    >
      <div className="flex items-start gap-3">
        <span
          className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border"
          style={{
            background: active ? 'rgba(59,130,246,0.18)' : 'var(--kvm-control-bg)',
            borderColor: active ? 'rgba(147,197,253,0.5)' : 'var(--kvm-border)',
            color: active ? '#93c5fd' : 'var(--kvm-text-muted)',
          }}
        >
          <Icon size={18} />
        </span>
        <span className="min-w-0 flex-1">
          <span className="block text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
            {title}
          </span>
          <span className="mt-1 block text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
            {summary}
          </span>
          <span
            className="mt-2 inline-flex rounded-md border px-2 py-0.5 text-[11px]"
            style={{
              borderColor: 'var(--kvm-border)',
              color: active ? '#93c5fd' : 'var(--kvm-text-muted)',
            }}
          >
            {meta}
          </span>
        </span>
      </div>
    </button>
  );
}
