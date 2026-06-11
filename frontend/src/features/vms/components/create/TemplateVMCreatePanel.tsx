import { useEffect, useMemo, useState } from 'react';

import { Layers3Icon } from 'lucide-react';

import { SelectMenu } from '../../../../components/kvm/SelectMenu';
import { fetchVMConfig, type VMConfig, type VirtualMachine } from '../../../../lib/api';
import { formatOSType } from '../../../../lib/format';
import { ClonePanel } from '../edit/ClonePanel';
import { InlineNotice } from '../edit/EditShared';
import { Panel } from './CreateFormShared';

type Option = { value: string; label: string; tooltip?: string };

export function TemplateVMCreatePanel({
  templates,
  selectedId,
  onSelectedIdChange,
  onQueued,
}: {
  templates: VirtualMachine[];
  selectedId: string;
  onSelectedIdChange: (value: string) => void;
  onQueued: () => void;
}) {
  const selectedTemplate = templates.find(item => item.id === selectedId) || templates[0];
  const [config, setConfig] = useState<VMConfig | null>(null);
  const [configError, setConfigError] = useState('');

  useEffect(() => {
    if (!selectedTemplate) return;
    let ignore = false;
    setConfig(null);
    setConfigError('');
    fetchVMConfig(selectedTemplate.id)
      .then(item => {
        if (!ignore) setConfig(item);
      })
      .catch(error => {
        if (!ignore)
          setConfigError(error instanceof Error ? error.message : '读取模板虚拟机配置失败');
      });
    return () => {
      ignore = true;
    };
  }, [selectedTemplate?.id]);

  const templateOptions = useMemo<Option[]>(
    () =>
      templates.map(item => ({
        value: item.id,
        label: item.templateName || item.name,
        tooltip: `${item.name} · ${item.hostName || '-'} · ${formatOSType(item.osType, item.name)}`,
      })),
    [templates]
  );

  if (!templates.length) {
    return (
      <Panel title="虚拟机模板">
        <div
          className="rounded-xl border px-4 py-8 text-center text-sm"
          style={{ borderColor: 'var(--kvm-border)', color: 'var(--kvm-text-muted)' }}
        >
          暂无已标记的虚拟机模板
        </div>
      </Panel>
    );
  }

  return (
    <div className="space-y-4">
      <Panel title="虚拟机模板">
        <div className="grid gap-3 lg:grid-cols-[minmax(240px,360px)_1fr] lg:items-center">
          <SelectMenu
            value={selectedTemplate?.id || ''}
            options={templateOptions}
            placeholder="选择虚拟机模板"
            maxVisibleItems={6}
            onChange={onSelectedIdChange}
          />
          <div
            className="flex min-w-0 flex-wrap items-center gap-2 text-xs"
            style={{ color: 'var(--kvm-text-muted)' }}
          >
            <span
              className="inline-flex h-7 w-7 items-center justify-center rounded-lg border"
              style={{
                background: 'rgba(45,212,191,0.1)',
                borderColor: 'rgba(45,212,191,0.32)',
                color: 'var(--kvm-check-toggle-active-text)',
              }}
            >
              <Layers3Icon size={15} />
            </span>
            <span className="font-mono" style={{ color: 'var(--kvm-text)' }}>
              {selectedTemplate?.name}
            </span>
            <span>{selectedTemplate?.hostName || '-'}</span>
            <span>{selectedTemplate?.templateDescription || '未填写模板描述'}</span>
          </div>
        </div>
      </Panel>
      {configError && <InlineNotice tone="warning">{configError}</InlineNotice>}
      {selectedTemplate && (
        <div className="kvm-dialog-card min-h-[420px] overflow-hidden rounded-2xl p-4">
          <ClonePanel vm={selectedTemplate} config={config} mode="template" onCloned={onQueued} />
        </div>
      )}
    </div>
  );
}
