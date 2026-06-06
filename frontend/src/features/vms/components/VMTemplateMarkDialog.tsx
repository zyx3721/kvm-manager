import { useState } from 'react';
import { BadgeCheckIcon, XIcon } from 'lucide-react';
import { toast } from 'sonner';

import { DialogPortal } from '../../../components/kvm/DialogPortal';
import { markVMTemplate, type VirtualMachine } from '../../../lib/api';
import { isVMRunning } from '../utils/vmStatus';
import { Field, PrimaryButton, fieldStyle, inputClass } from './create/CreateFormShared';

export function VMTemplateMarkDialog({
  vm,
  onClose,
  onMarked,
}: {
  vm: VirtualMachine;
  onClose: () => void;
  onMarked: () => void;
}) {
  const [name, setName] = useState(vm.templateName || vm.name);
  const [description, setDescription] = useState(vm.templateDescription || '');
  const [busy, setBusy] = useState(false);

  async function submit() {
    const templateName = name.trim();
    if (!templateName) return toast.warning('请输入模板名称');
    if (isVMRunning(vm.status)) return toast.warning('虚拟机正在运行，无法标记为模板，请先关闭虚拟机后再操作');
    setBusy(true);
    try {
      await markVMTemplate(vm.id, { name: templateName, description: description.trim() });
      toast.success(`${vm.name} 已标记为模板`);
      onMarked();
      onClose();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '标记虚拟机模板失败');
    } finally {
      setBusy(false);
    }
  }

  return (
    <DialogPortal>
      <div className="kvm-dialog-backdrop fixed inset-0 z-50 flex items-center justify-center px-3 py-5">
        <div className="kvm-dialog-panel flex w-[min(92vw,520px)] flex-col overflow-hidden rounded-2xl shadow-2xl">
          <header className="flex min-h-14 items-center justify-between border-b px-4 py-2.5" style={{ borderColor: 'var(--kvm-border)' }}>
            <div className="flex items-center gap-2">
              <BadgeCheckIcon size={17} style={{ color: '#5eead4' }} />
              <h2 className="text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>标记为虚拟机模板</h2>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="kvm-action-button flex h-8 w-8 items-center justify-center rounded-lg border"
              style={{ background: 'var(--kvm-control-bg)', borderColor: 'var(--kvm-border)', color: 'var(--kvm-text-muted)' }}
              aria-label="关闭模板标记窗口"
            >
              <XIcon size={15} />
            </button>
          </header>
          <main className="space-y-4 p-4" style={{ background: 'var(--kvm-control-bg-soft)' }}>
            <div className="rounded-xl border p-4" style={{ background: 'var(--kvm-card)', borderColor: 'var(--kvm-border)' }}>
              <div className="mb-3 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                源虚拟机：<span className="font-mono" style={{ color: 'var(--kvm-text)' }}>{vm.name}</span>
              </div>
              <div className="grid gap-3">
                <Field label="模板名称">
                  <input value={name} disabled={busy} onChange={event => setName(event.target.value)} className={inputClass} style={fieldStyle} />
                </Field>
                <Field label="模板描述">
                  <input value={description} disabled={busy} onChange={event => setDescription(event.target.value)} className={inputClass} style={fieldStyle} />
                </Field>
              </div>
              <div className="mt-3 rounded-lg border px-3 py-2 text-xs" style={{ borderColor: 'var(--kvm-border)', background: 'var(--kvm-control-bg)', color: 'var(--kvm-text-muted)' }}>
                标记只写入模板身份，不复制虚拟机 CPU、内存、磁盘等详情
              </div>
            </div>
            <div className="flex justify-end">
              <PrimaryButton label={busy ? '保存中' : '保存'} disabled={busy} onClick={() => void submit()} />
            </div>
          </main>
        </div>
      </div>
    </DialogPortal>
  );
}

