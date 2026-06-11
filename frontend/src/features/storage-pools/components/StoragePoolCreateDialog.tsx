import { useEffect, useMemo, useState } from 'react';
import { XIcon } from 'lucide-react';
import { toast } from 'sonner';

import { DialogPortal } from '../../../components/kvm/DialogPortal';
import { SelectMenu } from '../../../components/kvm/SelectMenu';
import { type StoragePoolCreatePayload } from '../../../lib/api';
import {
  buttonStyle,
  disabledFieldStyle,
  fieldStyle,
  primaryButtonStyle,
} from './storagePoolStyles';

const storageTypes = [
  { id: 'dir', label: '目录类型卷' },
  { id: 'logical', label: 'LVM类型卷' },
  { id: 'netfs', label: 'NETFS类型卷' },
  { id: 'iscsi', label: 'iSCSI类型卷' },
];

const netfsFormats = ['auto', 'nfs', 'glusterfs', 'cifs'];

export function StoragePoolCreateDialog({
  hostName,
  onClose,
  onSubmit,
}: {
  hostName: string;
  onClose: () => void;
  onSubmit: (payload: StoragePoolCreatePayload) => Promise<void>;
}) {
  const [type, setType] = useState('dir');
  const [name, setName] = useState('');
  const [path, setPath] = useState('');
  const [device, setDevice] = useState('');
  const [sourceHost, setSourceHost] = useState('');
  const [sourcePath, setSourcePath] = useState('');
  const [format, setFormat] = useState('auto');
  const active = useMemo(() => storageTypes.find(item => item.id === type), [type]);

  useEffect(() => {
    setName('');
    setPath('');
    setDevice('');
    setSourceHost('');
    setSourcePath('');
    setFormat('auto');
  }, [type]);

  function submit() {
    const error = validateStoragePoolForm(type, name, path, device, sourceHost, sourcePath);
    if (error) {
      toast.error(error);
      return;
    }
    const payload: StoragePoolCreatePayload = {
      type,
      name: name.trim(),
      path,
      device,
      sourceHost,
      sourcePath,
      format,
    };
    void onSubmit(payload);
  }

  return (
    <DialogPortal>
      <div className="kvm-dialog-backdrop fixed inset-0 z-50 flex items-center justify-center px-3">
        <div className="kvm-dialog-panel w-[min(92vw,640px)] overflow-hidden rounded-2xl">
          <header
            className="flex items-center justify-between border-b px-5 py-4"
            style={{ borderColor: 'var(--kvm-border)' }}
          >
            <div>
              <h2 className="text-base font-semibold" style={{ color: 'var(--kvm-text)' }}>
                创建存储池
              </h2>
              <p className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                {hostName}
              </p>
            </div>
            <button
              type="button"
              onClick={onClose}
              aria-label="关闭"
              className="kvm-action-button inline-flex h-8 w-8 items-center justify-center rounded-lg border"
              style={buttonStyle}
            >
              <XIcon size={15} />
            </button>
          </header>
          <div
            className="flex gap-2 overflow-x-auto border-b px-4 py-2"
            style={{ borderColor: 'var(--kvm-border)', background: 'var(--kvm-table-head-bg)' }}
          >
            {storageTypes.map(item => (
              <button
                key={item.id}
                type="button"
                onClick={() => setType(item.id)}
                className="kvm-action-button h-10 shrink-0 rounded-lg border px-3 text-sm"
                style={{
                  ...buttonStyle,
                  color: type === item.id ? 'var(--kvm-accent-text)' : 'var(--kvm-text-muted)',
                  borderColor: type === item.id ? 'rgba(96,165,250,0.52)' : 'var(--kvm-border)',
                }}
              >
                {item.label}
              </button>
            ))}
          </div>
          <div className="mx-auto max-w-[520px] space-y-4 p-5">
            <Field label="类型">
              <input
                value={type}
                disabled
                className="h-10 w-full cursor-not-allowed rounded-lg px-3 text-sm outline-none"
                style={disabledFieldStyle}
              />
            </Field>
            <Field label="名称">
              <input
                value={name}
                onChange={event => setName(event.target.value)}
                placeholder={namePlaceholder(active?.id)}
                className="h-10 w-full rounded-lg px-3 text-sm outline-none"
                style={fieldStyle}
              />
            </Field>
            {type === 'dir' && (
              <Field label="路径">
                <input
                  value={path}
                  onChange={event => setPath(event.target.value)}
                  placeholder="/var/lib/libvirt/images"
                  className="h-10 w-full rounded-lg px-3 text-sm outline-none"
                  style={fieldStyle}
                />
              </Field>
            )}
            {type === 'logical' && (
              <Field label="路径">
                <input
                  value={device}
                  onChange={event => setDevice(event.target.value)}
                  placeholder="/dev/sdb"
                  className="h-10 w-full rounded-lg px-3 text-sm outline-none"
                  style={fieldStyle}
                />
              </Field>
            )}
            {type === 'netfs' && (
              <>
                <Field label="主机名">
                  <input
                    value={sourceHost}
                    onChange={event => setSourceHost(event.target.value)}
                    placeholder="nfs.example.com"
                    className="h-10 w-full rounded-lg px-3 text-sm outline-none"
                    style={fieldStyle}
                  />
                </Field>
                <Field label="远端路径">
                  <input
                    value={sourcePath}
                    onChange={event => setSourcePath(event.target.value)}
                    placeholder="/srv/storage"
                    className="h-10 w-full rounded-lg px-3 text-sm outline-none"
                    style={fieldStyle}
                  />
                </Field>
                <Field label="本地路径">
                  <input
                    value={path}
                    onChange={event => setPath(event.target.value)}
                    placeholder="/srv/storage"
                    className="h-10 w-full rounded-lg px-3 text-sm outline-none"
                    style={fieldStyle}
                  />
                </Field>
                <Field label="格式">
                  <SelectMenu
                    value={format}
                    placeholder="auto"
                    placement="top"
                    options={netfsFormats.map(item => ({ value: item, label: item }))}
                    onChange={setFormat}
                  />
                </Field>
              </>
            )}
            {type === 'iscsi' && (
              <>
                <Field label="主机名">
                  <input
                    value={sourceHost}
                    onChange={event => setSourceHost(event.target.value)}
                    placeholder="iscsi.example.com"
                    className="h-10 w-full rounded-lg px-3 text-sm outline-none"
                    style={fieldStyle}
                  />
                </Field>
                <Field label="目标 IQN">
                  <input
                    value={sourcePath}
                    onChange={event => setSourcePath(event.target.value)}
                    placeholder="iqn.2026-05.example:storage"
                    className="h-10 w-full rounded-lg px-3 text-sm outline-none"
                    style={fieldStyle}
                  />
                </Field>
                <Field label="本地路径">
                  <input
                    value={path}
                    onChange={event => setPath(event.target.value)}
                    placeholder="/dev/disk/by-path"
                    className="h-10 w-full rounded-lg px-3 text-sm outline-none"
                    style={fieldStyle}
                  />
                </Field>
              </>
            )}
          </div>
          <footer
            className="flex justify-end gap-2 border-t px-5 py-4"
            style={{ borderColor: 'var(--kvm-border)' }}
          >
            <button
              type="button"
              onClick={onClose}
              className="kvm-action-button rounded-lg border px-4 py-2 text-sm"
              style={buttonStyle}
            >
              关闭
            </button>
            <button
              type="button"
              onClick={submit}
              className="kvm-action-button rounded-lg border px-4 py-2 text-sm font-semibold"
              style={primaryButtonStyle}
            >
              创建
            </button>
          </footer>
        </div>
      </div>
    </DialogPortal>
  );
}

function validateStoragePoolForm(
  type: string,
  name: string,
  path: string,
  device: string,
  sourceHost: string,
  sourcePath: string
) {
  if (!name.trim()) return '请填写名称';
  if (type === 'dir' && !isAbsoluteLinuxPath(path)) {
    return '请填写正确的绝对路径';
  }
  if (type === 'logical' && !isAbsoluteLinuxPath(device)) {
    return '请填写正确的设备绝对路径';
  }
  if (type === 'netfs') {
    if (!sourceHost.trim()) return '请填写主机名';
    if (!isAbsoluteLinuxPath(sourcePath)) return '请填写正确的远端路径';
    if (!isAbsoluteLinuxPath(path)) return '请填写正确的本地路径';
  }
  if (type === 'iscsi') {
    if (!sourceHost.trim()) return '请填写主机名';
    if (!sourcePath.trim()) return '请填写目标 IQN';
    if (!isAbsoluteLinuxPath(path)) return '请填写正确的本地路径';
  }
  return '';
}

function isAbsoluteLinuxPath(value: string) {
  const path = value.trim();
  return path.startsWith('/') && !path.includes('\0');
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="grid gap-2 md:grid-cols-[65px_minmax(0,360px)] md:items-center md:justify-center">
      <span className="text-sm font-semibold md:text-right" style={{ color: 'var(--kvm-text)' }}>
        {label}
      </span>
      {children}
    </label>
  );
}

function namePlaceholder(type?: string) {
  switch (type) {
    case 'logical':
      return 'logicalpool';
    case 'netfs':
      return 'netfspool';
    case 'iscsi':
      return 'iscsipool';
    default:
      return 'default';
  }
}
