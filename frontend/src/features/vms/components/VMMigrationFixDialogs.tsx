import { useState } from 'react';
import { KeyRoundIcon, XIcon } from 'lucide-react';
import { toast } from 'sonner';

import {
  setupVMMigrationHostname,
  setupVMMigrationSSHKey,
  type VMMigratePayload,
} from '../../../lib/api';
import { fieldStyle, inputClass } from './edit/EditShared';
import { PrimaryButton } from './VMEditControls';

type MigrationFixDialogProps = {
  vmId: string;
  vmName: string;
  payload: VMMigratePayload;
  message: string;
  onClose: () => void;
  onConfigured: () => void;
};

export function MigrationSSHKeyDialog({ vmId, vmName, payload, message, onClose, onConfigured }: MigrationFixDialogProps) {
  const [username, setUsername] = useState('root');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit() {
    if (!username.trim()) return toast.warning('请输入目标宿主机 SSH 用户');
    if (!password) return toast.warning('请输入目标宿主机 SSH 密码');
    setBusy(true);
    try {
      await setupVMMigrationSSHKey(vmId, {
        targetAgentId: payload.targetAgentId,
        destinationUri: payload.destinationUri,
        username: username.trim(),
        password,
      });
      toast.success('迁移 SSH 免密已配置');
      onConfigured();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '配置迁移 SSH 免密失败');
    } finally {
      setBusy(false);
    }
  }

  return (
    <MigrationFixShell title="配置迁移 SSH 免密" iconColor="#fbbf24" ariaClose="关闭迁移 SSH 免密窗口" onClose={onClose}>
      <div className="rounded-xl border p-4" style={{ background: 'var(--kvm-card)', borderColor: 'var(--kvm-border)' }}>
        <FixMessage message={message || '源宿主机无法免密连接目标 libvirt，请输入目标宿主机 SSH 密码完成免密配置'} vmName={vmName} />
        <div className="grid gap-3">
          <Field label="目标 SSH 用户">
            <input value={username} disabled={busy} onChange={event => setUsername(event.target.value)} className={inputClass} style={fieldStyle} />
          </Field>
          <Field label="目标 SSH 密码">
            <input type="password" value={password} disabled={busy} onChange={event => setPassword(event.target.value)} className={inputClass} style={fieldStyle} />
          </Field>
        </div>
        <FixNotice>密码仅用于本次写入源宿主机公钥，不会保存到平台数据库</FixNotice>
      </div>
      <DialogActions label={busy ? '配置中' : '配置并重新预检'} disabled={busy} onClick={submit} />
    </MigrationFixShell>
  );
}

export function MigrationHostnameDialog({ vmId, vmName, payload, message, onClose, onConfigured }: MigrationFixDialogProps) {
  const [hostname, setHostname] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit() {
    const value = hostname.trim().toLowerCase();
    if (!value) return toast.warning('请输入目标宿主机主机名');
    if (value === 'localhost' || value === 'localhost.localdomain' || value.startsWith('127.') || value === '::1') {
      return toast.warning('目标宿主机主机名不能为 localhost');
    }
    setBusy(true);
    try {
      await setupVMMigrationHostname(vmId, {
        targetAgentId: payload.targetAgentId,
        destinationUri: payload.destinationUri,
        hostname: value,
      });
      toast.success('迁移目标主机名已配置');
      onConfigured();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '配置迁移目标主机名失败');
    } finally {
      setBusy(false);
    }
  }

  return (
    <MigrationFixShell title="修复迁移目标主机名" iconColor="#60a5fa" ariaClose="关闭迁移目标主机名窗口" onClose={onClose}>
      <div className="rounded-xl border p-4" style={{ background: 'var(--kvm-card)', borderColor: 'var(--kvm-border)' }}>
        <FixMessage message={message || '目标宿主机主机名解析为 localhost，请输入真实主机名并写入源目标 hosts 解析'} vmName={vmName} />
        <Field label="目标宿主机主机名">
          <input value={hostname} disabled={busy} onChange={event => setHostname(event.target.value)} placeholder="kvm02" className={inputClass} style={fieldStyle} />
        </Field>
        <FixNotice>将设置目标宿主机 hostname，并在源宿主机和目标宿主机 /etc/hosts 写入目标 IP 与主机名解析</FixNotice>
      </div>
      <DialogActions label={busy ? '配置中' : '配置并重新预检'} disabled={busy} onClick={submit} />
    </MigrationFixShell>
  );
}

function MigrationFixShell({
  title,
  iconColor,
  ariaClose,
  onClose,
  children,
}: {
  title: string;
  iconColor: string;
  ariaClose: string;
  onClose: () => void;
  children: React.ReactNode;
}) {
  return (
    <div className="kvm-dialog-backdrop fixed inset-0 z-[60] flex items-center justify-center px-3 py-5">
      <div className="kvm-dialog-panel flex w-[min(92vw,520px)] flex-col overflow-hidden rounded-2xl shadow-2xl">
        <header className="flex min-h-14 items-center justify-between border-b px-4 py-2.5" style={{ borderColor: 'var(--kvm-border)' }}>
          <div className="flex items-center gap-2">
            <KeyRoundIcon size={17} style={{ color: iconColor }} />
            <h2 className="text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>{title}</h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="kvm-action-button flex h-8 w-8 items-center justify-center rounded-lg border"
            style={{ background: 'var(--kvm-control-bg)', borderColor: 'var(--kvm-border)', color: 'var(--kvm-text-muted)' }}
            aria-label={ariaClose}
          >
            <XIcon size={15} />
          </button>
        </header>
        <main className="space-y-4 p-4" style={{ background: 'var(--kvm-control-bg-soft)' }}>{children}</main>
      </div>
    </div>
  );
}

function FixMessage({ message, vmName }: { message: string; vmName: string }) {
  return (
    <>
      <div className="mb-3 text-xs leading-5" style={{ color: 'var(--kvm-text-muted)' }}>{message}</div>
      <div className="mb-3 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
        迁移对象：<span className="font-mono" style={{ color: 'var(--kvm-text)' }}>{vmName}</span>
      </div>
    </>
  );
}

function FixNotice({ children }: { children: React.ReactNode }) {
  return (
    <div className="mt-3 rounded-lg border px-3 py-2 text-xs leading-5" style={{ borderColor: 'var(--kvm-border)', background: 'var(--kvm-control-bg)', color: 'var(--kvm-text-muted)' }}>
      {children}
    </div>
  );
}

function DialogActions({ label, disabled, onClick }: { label: string; disabled: boolean; onClick: () => void }) {
  return (
    <div className="flex justify-end">
      <PrimaryButton label={label} disabled={disabled} onClick={() => void onClick()} />
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <label><div className="mb-1.5 text-xs font-semibold" style={{ color: 'var(--kvm-text-muted)' }}>{label}</div>{children}</label>;
}
