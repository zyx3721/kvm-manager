import { useState } from 'react';
import { toast } from 'sonner';
import { DialogFrame } from './DialogFrame';
import { inputClass, fieldStyle } from './edit/EditShared';
import { PrimaryButton } from './VMEditControls';
import { vmConsoleUrl, type VirtualMachine } from '../../../lib/api';
import { isVMRunning } from '../utils/vmStatus';

export function ConsolePasswordDialog({
  vm,
  busy,
  onClose,
  onConfirm,
}: {
  vm: VirtualMachine;
  busy?: boolean;
  onClose: () => void;
  onConfirm: (password: string) => void;
}) {
  const [password, setPassword] = useState('');
  const [validating, setValidating] = useState(false);

  async function handleConfirm() {
    const nextPassword = password.trim();
    if (!nextPassword || busy || validating) return;
    if (!isVMRunning(vm.status)) {
      onConfirm(nextPassword);
      return;
    }
    setValidating(true);
    try {
      await verifyConsolePassword(vm.id, nextPassword);
      onConfirm(nextPassword);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '控制台密码不正确');
    } finally {
      setValidating(false);
    }
  }

  const disabled = Boolean(busy || validating || !password.trim());

  return (
    <DialogFrame title="控制台密码" tone="normal" onClose={onClose}>
      <div className="space-y-4">
        <div
          className="rounded-lg border p-3"
          style={{ borderColor: 'var(--kvm-border)', background: 'var(--kvm-control-bg-soft)' }}
        >
          <div className="text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
            {vm.name}
          </div>
          <div className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
            该虚拟机已配置 VNC 访问密码
          </div>
        </div>
        <label>
          <div className="mb-1.5 text-xs font-semibold" style={{ color: 'var(--kvm-text-muted)' }}>
            访问密码
          </div>
          <input
            value={password}
            disabled={busy || validating}
            type="password"
            autoComplete="current-password"
            onChange={event => setPassword(event.target.value)}
            className={inputClass}
            style={fieldStyle}
          />
        </label>
        <div className="flex justify-end pt-3">
          <PrimaryButton
            label={busy || validating ? '验证中' : '打开控制台'}
            disabled={disabled}
            onClick={() => void handleConfirm()}
          />
        </div>
      </div>
    </DialogFrame>
  );
}

async function verifyConsolePassword(vmId: string, password: string) {
  const module = await import('@novnc/novnc');
  const target = document.createElement('div');
  target.style.position = 'fixed';
  target.style.left = '-10000px';
  target.style.top = '-10000px';
  target.style.width = '1px';
  target.style.height = '1px';
  document.body.appendChild(target);
  let rfb: ConsolePasswordProbe | null = null;
  try {
    await new Promise<void>((resolve, reject) => {
      let settled = false;
      const finish = (error?: Error) => {
        if (settled) return;
        settled = true;
        window.clearTimeout(timeoutId);
        rfb?.removeEventListener('connect', onConnect);
        rfb?.removeEventListener('disconnect', onDisconnect);
        rfb?.removeEventListener('credentialsrequired', onCredentials);
        rfb?.removeEventListener('securityfailure', onSecurityFailure);
        rfb?.disconnect();
        if (error) reject(error);
        else resolve();
      };
      const timeoutId = window.setTimeout(() => finish(new Error('控制台密码验证超时')), 6000);
      const onConnect = () => finish();
      const onDisconnect = () => finish(new Error('控制台密码不正确'));
      const onCredentials = () => finish(new Error('控制台密码不正确'));
      const onSecurityFailure = () => finish(new Error('控制台密码不正确'));
      rfb = new module.default(target, vmConsoleUrl(vmId), {
        credentials: { password },
        wsProtocols: ['binary'],
      }) as ConsolePasswordProbe;
      rfb.viewOnly = true;
      rfb.scaleViewport = true;
      rfb.resizeSession = false;
      rfb.addEventListener('connect', onConnect);
      rfb.addEventListener('disconnect', onDisconnect);
      rfb.addEventListener('credentialsrequired', onCredentials);
      rfb.addEventListener('securityfailure', onSecurityFailure);
    });
  } finally {
    rfb?.disconnect();
    target.remove();
  }
}

type ConsolePasswordProbe = {
  disconnect: () => void;
  viewOnly: boolean;
  scaleViewport: boolean;
  resizeSession: boolean;
  addEventListener: (type: string, listener: (event: Event) => void) => void;
  removeEventListener: (type: string, listener: (event: Event) => void) => void;
};
