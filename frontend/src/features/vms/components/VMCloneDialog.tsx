import { useEffect, useState } from 'react';

import { CopyPlusIcon, XIcon } from 'lucide-react';

import { fetchVMConfig, type VMConfig, type VirtualMachine } from '../../../lib/api';
import { DialogPortal } from '../../../components/kvm/DialogPortal';
import { StatusBadge } from '../../../components/kvm/StatusBadge';
import { ClonePanel } from './edit/ClonePanel';
import { InlineNotice } from './edit/EditShared';

export function VMCloneDialog({
  vm,
  mode = 'clone',
  onClose,
}: {
  vm: VirtualMachine;
  mode?: 'clone' | 'template';
  onClose: () => void;
}) {
  const [config, setConfig] = useState<VMConfig | null>(null);
  const [configError, setConfigError] = useState('');

  useEffect(() => {
    let ignore = false;
    setConfig(null);
    setConfigError('');
    fetchVMConfig(vm.id)
      .then(item => {
        if (!ignore) setConfig(item);
      })
      .catch(error => {
        if (!ignore) setConfigError(error instanceof Error ? error.message : '读取虚拟机配置失败');
      });
    return () => {
      ignore = true;
    };
  }, [vm.id]);

  return (
    <DialogPortal>
      <div className="kvm-dialog-backdrop fixed inset-0 z-50 flex items-center justify-center px-3 py-5">
        <div className="kvm-dialog-panel flex h-[min(88vh,760px)] w-[min(92vw,980px)] flex-col overflow-hidden rounded-2xl shadow-2xl">
          <header
            className="flex min-h-16 shrink-0 items-center justify-between gap-3 border-b px-4 py-2.5"
            style={{ borderColor: 'var(--kvm-border)' }}
          >
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <span
                  className="h-2 w-2 rounded-full"
                  style={{ background: '#3b82f6', boxShadow: '0 0 18px #3b82f6' }}
                />
                <h2 className="truncate text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
                  {mode === 'template' ? '从模板创建虚拟机' : '克隆虚拟机'}
                </h2>
                <StatusBadge status={vm.status} />
              </div>
              <div
                className="mt-1 flex flex-wrap items-center gap-x-2.5 gap-y-0.5 text-[11px]"
                style={{ color: 'var(--kvm-text-muted)' }}
              >
                <span
                  className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-md border"
                  style={{
                    background: 'rgba(34,197,94,0.12)',
                    borderColor: 'rgba(34,197,94,0.34)',
                    color: '#86efac',
                  }}
                >
                  <CopyPlusIcon size={12} />
                </span>
                <span className="font-mono" style={{ color: 'var(--kvm-text)' }}>
                  {vm.name}
                </span>
                <span>{vm.hostName || '-'}</span>
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
              aria-label="关闭克隆窗口"
            >
              <XIcon size={15} />
            </button>
          </header>

          <main
            className="flex min-h-0 flex-1 flex-col overflow-hidden p-4"
            style={{ background: 'var(--kvm-control-bg-soft)' }}
          >
            {configError && <InlineNotice tone="warning">{configError}</InlineNotice>}
            <div
              className="kvm-dialog-card flex min-h-0 flex-1 overflow-hidden rounded-2xl p-4"
              style={{
                boxShadow: 'inset 0 1px 0 rgba(255,255,255,0.08), 0 18px 40px rgba(15,23,42,0.16)',
              }}
            >
              <ClonePanel vm={vm} config={config} mode={mode} onCloned={onClose} />
            </div>
          </main>
        </div>
      </div>
    </DialogPortal>
  );
}
