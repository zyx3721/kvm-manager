import { useEffect, useMemo, useState } from 'react';

import { toast } from 'sonner';

import { type VMConfig, type VirtualMachine, updateVMXML } from '../../../../lib/api';
import { isVMRunning } from '../../utils/vmStatus';
import { XMLTextEditor } from '../XMLTextEditor';
import { PrimaryButton } from '../VMEditControls';
import { buildPreviewXML } from './editUtils';

export function XMLPanel({
  vm,
  config,
  memoryMB,
  onConfigChange,
}: {
  vm: VirtualMachine;
  config: VMConfig | null;
  memoryMB: number;
  onConfigChange: (config: VMConfig) => void;
}) {
  const sourceXML = useMemo(
    () => config?.xml || buildPreviewXML(vm, memoryMB),
    [config?.xml, memoryMB, vm]
  );
  const [editing, setEditing] = useState(false);
  const [xml, setXML] = useState(sourceXML);
  const [busy, setBusy] = useState(false);
  const running = isVMRunning(vm.status);

  useEffect(() => {
    if (!editing) setXML(sourceXML);
  }, [editing, sourceXML]);

  async function saveXML() {
    if (running) return toast.warning('虚拟机正在运行，无法修改 XML，请先关闭虚拟机后再操作');
    const nextXML = xml.trim();
    if (!nextXML) return toast.warning('虚拟机 XML 不能为空');
    if (!config) return toast.warning('虚拟机配置尚未加载完成');
    if (nextXML === sourceXML.trim()) {
      setEditing(false);
      return toast.warning('请先修改 XML');
    }
    setBusy(true);
    try {
      const response = await updateVMXML(vm.id, { xml: nextXML });
      onConfigChange(response.config);
      setXML(response.config.xml);
      setEditing(false);
      toast.success('虚拟机 XML 已修改');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '虚拟机 XML 修改失败');
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="space-y-4">
      <XMLTextEditor value={xml} readOnly={!editing} disabled={busy} onChange={setXML} />
      <div className="flex justify-end gap-2">
        {editing && (
          <button
            type="button"
            disabled={busy}
            onClick={() => {
              setXML(sourceXML);
              setEditing(false);
            }}
            className="kvm-action-button h-10 rounded-lg border px-5 text-sm font-semibold disabled:opacity-60"
            style={{
              background: 'var(--kvm-control-bg-soft)',
              borderColor: 'var(--kvm-border)',
              color: 'var(--kvm-text-muted)',
            }}
          >
            取消
          </button>
        )}
        <PrimaryButton
          label={editing ? (busy ? '保存中' : '保存') : '编辑'}
          disabled={busy}
          onClick={() => {
            if (!editing) {
              if (running)
                return toast.warning('虚拟机正在运行，无法修改 XML，请先关闭虚拟机后再操作');
              setEditing(true);
              return;
            }
            void saveXML();
          }}
        />
      </div>
    </section>
  );
}
