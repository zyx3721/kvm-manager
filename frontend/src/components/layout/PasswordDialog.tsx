import { type FormEvent, useState } from 'react';
import { CheckCircle2Icon, EyeIcon, EyeOffIcon, KeyRoundIcon, XIcon } from 'lucide-react';
import { toast } from 'sonner';
import { ApiError, changePassword } from '../../lib/api';
import { DialogPortal } from '../kvm/DialogPortal';

type PasswordDialogProps = {
  open: boolean;
  onClose: () => void;
  onSuccess?: () => void;
};

type PasswordField = 'old_password' | 'new_password' | 'confirm_password';

const emptyForm = {
  old_password: '',
  new_password: '',
  confirm_password: '',
};

const emptyErrors: Record<PasswordField, boolean> = {
  old_password: false,
  new_password: false,
  confirm_password: false,
};

export default function PasswordDialog({ open, onClose, onSuccess }: PasswordDialogProps) {
  const [form, setForm] = useState(emptyForm);
  const [errors, setErrors] = useState(emptyErrors);
  const [visible, setVisible] = useState(emptyErrors);
  const [message, setMessage] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const closeDialog = () => {
    setForm(emptyForm);
    setErrors(emptyErrors);
    setVisible(emptyErrors);
    setMessage('');
    setSubmitting(false);
    onClose();
  };

  if (!open) return null;

  const updateField = (field: PasswordField, value: string) => {
    setForm(current => ({ ...current, [field]: value }));
    setErrors(current => ({ ...current, [field]: false }));
    setMessage('');
  };

  const toggleVisible = (field: PasswordField) => {
    setVisible(current => ({ ...current, [field]: !current[field] }));
  };

  const validate = () => {
    const nextErrors = { ...emptyErrors };
    if (form.old_password.length < 6) nextErrors.old_password = true;
    if (form.new_password.length < 6) nextErrors.new_password = true;
    if (form.confirm_password.length < 6) nextErrors.confirm_password = true;
    if (nextErrors.old_password || nextErrors.new_password || nextErrors.confirm_password) {
      setErrors(nextErrors);
      setMessage('密码至少 6 个字符');
      return false;
    }
    if (form.new_password === form.old_password) {
      setErrors({ ...emptyErrors, new_password: true });
      setMessage('新密码不能与旧密码相同');
      return false;
    }
    if (form.new_password !== form.confirm_password) {
      setErrors({ ...emptyErrors, confirm_password: true });
      setMessage('新密码与确认密码不一致');
      return false;
    }
    setErrors(emptyErrors);
    setMessage('');
    return true;
  };

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!validate()) return;
    setSubmitting(true);
    try {
      await changePassword(form);
      toast.success('密码已修改，请重新登录');
      closeDialog();
      onSuccess?.();
    } catch (error) {
      const text = error instanceof Error ? error.message : '密码修改失败';
      if (error instanceof ApiError && error.code === 'invalid_old_password') {
        setErrors({ ...emptyErrors, old_password: true });
      }
      setMessage(text);
      toast.error(text);
    } finally {
      setSubmitting(false);
    }
  };

  const fieldClass = (field: PasswordField) =>
    `h-10 w-full rounded-lg border px-3 pr-10 text-sm outline-none transition-colors ${errors[field] ? 'border-red-400/70' : ''}`;

  return (
    <DialogPortal>
      <div
        className="kvm-dialog-backdrop fixed inset-0 z-[70] flex items-center justify-center px-4"
        role="presentation"
        onMouseDown={event => event.target === event.currentTarget && closeDialog()}
      >
        <form
          className="kvm-dialog-panel w-full max-w-[440px] rounded-xl p-5 shadow-2xl"
          role="dialog"
          aria-modal="true"
          aria-labelledby="change-password-title"
          onSubmit={submit}
          style={{ background: 'var(--kvm-card)', border: '1px solid var(--kvm-border)' }}
        >
          <div className="mb-5 flex items-start gap-3">
            <div
              className="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg"
              style={{
                background: 'rgba(59,130,246,0.14)',
                border: '1px solid rgba(59,130,246,0.28)',
                color: '#60a5fa',
              }}
            >
              <KeyRoundIcon size={21} />
            </div>
            <div className="min-w-0 flex-1">
              <h2
                id="change-password-title"
                className="text-base font-semibold"
                style={{ color: 'var(--kvm-text)' }}
              >
                修改密码
              </h2>
              <p className="mt-1 text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
                输入旧密码后设置新的登录密码。
              </p>
            </div>
            <button
              type="button"
              className="kvm-action-button flex h-8 w-8 items-center justify-center rounded-lg"
              onClick={closeDialog}
              aria-label="关闭"
              style={{ color: 'var(--kvm-text-muted)', background: 'rgba(255,255,255,0.04)' }}
            >
              <XIcon size={16} />
            </button>
          </div>

          <div className="space-y-4">
            {(
              [
                ['old_password', '旧密码', '请输入旧密码', 'current-password'],
                ['new_password', '新密码', '请输入新密码', 'new-password'],
                ['confirm_password', '确认密码', '请再次输入新密码', 'new-password'],
              ] as const
            ).map(([field, label, placeholder, autocomplete]) => (
              <label key={field} className="block">
                <span
                  className="mb-1.5 block text-xs font-medium"
                  style={{ color: errors[field] ? '#f87171' : 'var(--kvm-text-muted)' }}
                >
                  {label}
                </span>
                <div className="relative">
                  <input
                    value={form[field]}
                    onChange={event => updateField(field, event.target.value)}
                    type={visible[field] ? 'text' : 'password'}
                    autoComplete={autocomplete}
                    placeholder={placeholder}
                    className={fieldClass(field)}
                    style={{
                      background: 'rgba(255,255,255,0.04)',
                      borderColor: errors[field] ? 'rgba(248,113,113,0.7)' : 'var(--kvm-border)',
                      color: 'var(--kvm-text)',
                    }}
                  />
                  <button
                    type="button"
                    className="kvm-action-button absolute right-1.5 top-1/2 flex h-7 w-7 -translate-y-1/2 items-center justify-center rounded-md"
                    onClick={() => toggleVisible(field)}
                    aria-label={visible[field] ? `隐藏${label}` : `显示${label}`}
                    style={{ color: 'var(--kvm-text-muted)', background: 'transparent' }}
                  >
                    {visible[field] ? <EyeOffIcon size={16} /> : <EyeIcon size={16} />}
                  </button>
                </div>
              </label>
            ))}
          </div>

          {message && (
            <div
              className="mt-4 rounded-lg px-3 py-2 text-sm"
              style={{
                color: '#fca5a5',
                background: 'rgba(239,68,68,0.1)',
                border: '1px solid rgba(239,68,68,0.25)',
              }}
            >
              {message}
            </div>
          )}

          <div className="mt-6 flex justify-end gap-3">
            <button
              type="button"
              className="kvm-action-button rounded-lg px-4 py-2 text-sm"
              onClick={closeDialog}
              style={{
                color: 'var(--kvm-text-muted)',
                background: 'rgba(255,255,255,0.04)',
                border: '1px solid var(--kvm-border)',
              }}
            >
              取消
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="kvm-action-button flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium disabled:opacity-60"
              style={{
                color: '#ffffff',
                background: 'linear-gradient(135deg, #2563eb, #06b6d4)',
                border: '1px solid rgba(96,165,250,0.35)',
              }}
            >
              <CheckCircle2Icon size={16} />
              {submitting ? '提交中...' : '确认修改'}
            </button>
          </div>
        </form>
      </div>
    </DialogPortal>
  );
}
