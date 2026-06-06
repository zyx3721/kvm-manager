import { type FormEvent, type ReactNode, useEffect, useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import {
  CheckCircle2Icon,
  KeyRoundIcon,
  LockIcon,
  MailIcon,
  MessageSquareIcon,
  MoonIcon,
  RadioTowerIcon,
  RefreshCwIcon,
  SendIcon,
  SunIcon,
  UserIcon,
} from 'lucide-react';
import {
  confirmPasswordReset,
  fetchPasswordResetCaptcha,
  sendPasswordResetCode,
  verifyPasswordResetIdentity,
  type PasswordResetCaptcha,
  type PasswordResetChannel,
} from '../../lib/api';
import { SelectMenu } from '../../components/kvm/SelectMenu';
import { KvmTooltip } from '../../components/kvm/StatusBadge';
import { useBaseConfig } from '../../lib/branding';
import {
  applyKvmTheme,
  getInitialKvmTheme,
  persistKvmTheme,
  toggleKvmTheme,
  type KvmTheme,
} from '../../lib/utils';

type ResetStep = 'identity' | 'delivery';

const channelIcons: Record<string, typeof MailIcon> = {
  email: MailIcon,
  lark: SendIcon,
  wechat: MessageSquareIcon,
  dingtalk: RadioTowerIcon,
};

export default function ForgotPasswordPage() {
  const navigate = useNavigate();
  const [theme, setTheme] = useState<KvmTheme>(getInitialKvmTheme);
  const [step, setStep] = useState<ResetStep>('identity');
  const [username, setUsername] = useState('');
  const [captcha, setCaptcha] = useState<PasswordResetCaptcha | null>(null);
  const [captchaAnswer, setCaptchaAnswer] = useState('');
  const [channels, setChannels] = useState<PasswordResetChannel[]>([]);
  const [verificationToken, setVerificationToken] = useState('');
  const [channel, setChannel] = useState('');
  const [verifyEmail, setVerifyEmail] = useState('');
  const [to, setTo] = useState('');
  const [code, setCode] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [busy, setBusy] = useState('');
  const [error, setError] = useState('');
  const [cooldown, setCooldown] = useState(0);
  const baseConfig = useBaseConfig();

  const selectedChannel = useMemo(
    () => channels.find(item => item.id === channel),
    [channel, channels]
  );
  const canSendCode = step === 'delivery' && selectedChannel && cooldown === 0 && busy === '';

  useEffect(() => {
    applyKvmTheme(theme);
    persistKvmTheme(theme);
  }, [theme]);

  useEffect(() => {
    void loadCaptcha();
  }, []);

  useEffect(() => {
    if (!captcha?.expiresAt || step !== 'identity') return;
    const expiresAt = new Date(captcha.expiresAt).getTime();
    const delay = Math.max(1000, expiresAt - Date.now());
    const timer = window.setTimeout(() => {
      void loadCaptcha();
    }, delay);
    return () => window.clearTimeout(timer);
  }, [captcha?.expiresAt, step]);

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = window.setTimeout(() => setCooldown(value => Math.max(0, value - 1)), 1000);
    return () => window.clearTimeout(timer);
  }, [cooldown]);

  async function loadCaptcha() {
    try {
      const next = await fetchPasswordResetCaptcha();
      setCaptcha(next);
      setCaptchaAnswer('');
    } catch (err) {
      setError(err instanceof Error ? err.message : '生成验证码失败');
    }
  }

  async function handleVerify(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const normalizedUsername = username.trim();
    if (!normalizedUsername) {
      setError('请输入用户名');
      return;
    }
    if (!captcha || !captchaAnswer.trim()) {
      setError('请输入验证码');
      return;
    }
    setBusy('verify');
    setError('');
    try {
      const response = await verifyPasswordResetIdentity({
        username: normalizedUsername,
        captchaToken: captcha.token,
        captchaAnswer: captchaAnswer.trim(),
      });
      if (response.channels.length === 0) {
        setError('当前没有可用的找回密码媒介');
        return;
      }
      setUsername(normalizedUsername);
      setChannels(response.channels);
      setVerificationToken(response.verificationToken);
      setChannel(response.channels[0]?.id ?? '');
      setStep('delivery');
    } catch (err) {
      setError(err instanceof Error ? err.message : '校验失败，请稍后重试');
      void loadCaptcha();
    } finally {
      setBusy('');
    }
  }

  async function handleSendCode() {
    if (!selectedChannel) return;
    if (!verifyEmail.trim()) {
      setError('请输入验证邮箱');
      return;
    }
    setBusy('send');
    setError('');
    try {
      const result = await sendPasswordResetCode({
        username,
        verificationToken,
        channel: selectedChannel.id,
        verifyEmail: verifyEmail.trim(),
        to: selectedChannel.requiresTo ? to.trim() : '',
      });
      setCooldown(result.cooldownSeconds || 60);
      toast.success('找回密码验证码已发送');
    } catch (err) {
      setError(err instanceof Error ? err.message : '发送验证码失败');
    } finally {
      setBusy('');
    }
  }

  async function handleReset(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!code.trim()) {
      setError('请输入找回密码验证码');
      return;
    }
    if (newPassword.length < 6 || confirmPassword.length < 6) {
      setError('密码至少 6 个字符');
      return;
    }
    if (newPassword !== confirmPassword) {
      setError('新密码与确认密码不一致');
      return;
    }
    setBusy('reset');
    setError('');
    try {
      await confirmPasswordReset({
        username,
        code: code.trim(),
        newPassword,
        confirmPassword,
      });
      toast.success('密码已重置，请重新登录');
      navigate('/login', { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : '密码重置失败');
    } finally {
      setBusy('');
    }
  }

  return (
    <main
      data-cmp="ForgotPasswordPage"
      className="relative flex min-h-dvh items-center justify-center overflow-hidden px-4 py-8 sm:px-6"
      style={{
        background:
          'radial-gradient(circle at 50% 0%, rgba(20,184,166,0.22), transparent 30%), radial-gradient(circle at 10% 26%, rgba(59,130,246,0.16), transparent 28%), radial-gradient(circle at 86% 78%, rgba(16,185,129,0.15), transparent 30%), var(--kvm-login-bg)',
        color: 'var(--kvm-text)',
      }}
    >
      <ThemeToggle theme={theme} onToggle={() => setTheme(toggleKvmTheme)} />
      <section className="relative z-10 flex w-full max-w-[460px] flex-col items-center gap-5">
        <div className="kvm-login-reveal flex flex-col items-center text-center">
          <img
            className="mb-4 h-20 w-20 drop-shadow-[0_18px_42px_rgba(6,182,212,0.18)]"
            src={baseConfig.iconData}
            alt={baseConfig.loginName}
          />
          <p className="kvm-gradient-text text-2xl font-bold tracking-wide">{baseConfig.loginName}</p>
          <p
            className="mt-1 text-xs uppercase tracking-[0.28em]"
            style={{ color: 'var(--kvm-text-muted)' }}
          >
            Infrastructure Control Plane
          </p>
        </div>

        <section
          className="kvm-login-frame kvm-login-reveal w-full rounded-[28px] p-1"
        >
          <div
            className="rounded-[24px] p-6 sm:p-8"
            style={{
              background: 'var(--kvm-login-panel-bg)',
              border: '1px solid var(--kvm-border)',
              backdropFilter: 'blur(18px)',
              boxShadow: 'var(--kvm-login-panel-shadow)',
            }}
          >
            <div className="mb-7 text-center">
              <div>
                <h1
                  className="text-2xl font-bold"
                  style={{ color: 'var(--kvm-text)', textShadow: '0 0 22px rgba(6,182,212,0.16)' }}
                >
                  忘记密码
                </h1>
                <p className="mt-2 text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
                  {step === 'identity'
                    ? '请输入您需要找回密码的用户名'
                    : '选择通知媒介接收验证码，然后设置新密码'}
                </p>
              </div>
            </div>

            {step === 'identity' ? (
              <form className="space-y-5" onSubmit={handleVerify} noValidate>
                <Field
                  label="用户名"
                  required
                  icon={UserIcon}
                  action={
                    <Link
                      to="/login"
                      className="text-xs font-semibold transition-colors"
                      style={{ color: 'var(--kvm-accent-text)' }}
                    >
                      返回登录
                    </Link>
                  }
                >
                  <input
                    value={username}
                    onChange={event => setUsername(event.target.value)}
                    autoComplete="username"
                    placeholder="请输入用户名"
                    className="h-12 w-full rounded-2xl py-3 pl-12 pr-4 text-sm outline-none transition-all"
                    style={inputStyle}
                  />
                </Field>
                <Field label="验证码" required icon={KeyRoundIcon}>
                  <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_132px]">
                    <input
                      value={captchaAnswer}
                      onChange={event => setCaptchaAnswer(event.target.value)}
                      inputMode="numeric"
                      placeholder="请输入验证码"
                      className="h-12 w-full rounded-2xl py-3 pl-12 pr-4 text-sm outline-none transition-all"
                      style={inputStyle}
                    />
                    <button
                      type="button"
                      onClick={() => void loadCaptcha()}
                      className="kvm-action-button flex h-12 items-center justify-center gap-2 rounded-2xl border px-4 text-sm font-semibold"
                      style={{
                        color: 'var(--kvm-text)',
                        background: 'rgba(255,255,255,0.045)',
                        borderColor: 'var(--kvm-border)',
                      }}
                    >
                      <span className="text-base tracking-[0.12em]">{captcha?.question ?? '--'}</span>
                      <RefreshCwIcon size={15} style={{ color: 'var(--kvm-text-muted)' }} />
                    </button>
                  </div>
                </Field>
                <ErrorLine error={error} />
                <PrimaryButton busy={busy === 'verify'} label="提交" />
              </form>
            ) : (
              <form className="space-y-5" onSubmit={handleReset} noValidate>
                <Field
                  label="通知媒介"
                  required
                  icon={SendIcon}
                  hideIcon
                  action={
                    <span className="text-xs font-semibold" style={{ color: 'var(--kvm-text-muted)' }}>
                      当前账号：<span style={{ color: 'var(--kvm-text)' }}>{username}</span>
                    </span>
                  }
                >
                  <SelectMenu
                    value={channel}
                    options={channels.map(item => {
                      const Icon = channelIcons[item.id] ?? SendIcon;
                      return {
                        value: item.id,
                        label: (
                          <span className="inline-flex items-center gap-2">
                            <Icon size={15} className="shrink-0" />
                            {item.name}
                          </span>
                        ),
                        searchLabel: item.name,
                        tooltip: item.description,
                      };
                    })}
                    placeholder="请选择通知媒介"
                    onChange={value => {
                      setChannel(value);
                      setTo('');
                    }}
                    className="w-full"
                  />
                </Field>
                <Field label="验证邮箱" required icon={MailIcon}>
                  <input
                    value={verifyEmail}
                    onChange={event => {
                      setVerifyEmail(event.target.value);
                      setTo(event.target.value);
                    }}
                    autoComplete="email"
                    placeholder="请输入当前账号配置的邮箱"
                    className="h-12 w-full rounded-2xl py-3 pl-12 pr-4 text-sm outline-none transition-all"
                    style={inputStyle}
                  />
                </Field>
                <Field label="重置验证码" required icon={KeyRoundIcon}>
                  <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_96px]">
                    <input
                      value={code}
                      onChange={event => setCode(event.target.value)}
                      inputMode="numeric"
                      placeholder="请输入收到的验证码"
                      className="h-12 w-full rounded-2xl py-3 pl-12 pr-4 text-sm outline-none transition-all"
                      style={inputStyle}
                    />
                    <button
                      type="button"
                      disabled={!canSendCode}
                      onClick={() => void handleSendCode()}
                      className="kvm-login-submit flex h-12 items-center justify-center gap-2 rounded-2xl text-sm font-semibold transition-all disabled:cursor-not-allowed disabled:opacity-60"
                      style={{
                        background: 'linear-gradient(135deg, #0f766e, #14b8a6)',
                        color: '#fff',
                        boxShadow: '0 18px 42px rgba(20,184,166,0.26)',
                      }}
                    >
                      {busy === 'send' ? '发送中' : cooldown > 0 ? `${cooldown}s` : '发送'}
                    </button>
                  </div>
                </Field>
                <Field label="新密码" required icon={LockIcon}>
                  <input
                    type="password"
                    value={newPassword}
                    onChange={event => setNewPassword(event.target.value)}
                    autoComplete="new-password"
                    placeholder="至少 6 个字符"
                    className="h-12 w-full rounded-2xl py-3 pl-12 pr-4 text-sm outline-none transition-all"
                    style={inputStyle}
                  />
                </Field>
                <Field label="确认密码" required icon={CheckCircle2Icon}>
                  <input
                    type="password"
                    value={confirmPassword}
                    onChange={event => setConfirmPassword(event.target.value)}
                    autoComplete="new-password"
                    placeholder="请再次输入新密码"
                    className="h-12 w-full rounded-2xl py-3 pl-12 pr-4 text-sm outline-none transition-all"
                    style={inputStyle}
                  />
                </Field>
                <ErrorLine error={error} />
                <PrimaryButton busy={busy === 'reset'} label="重置密码" />
              </form>
            )}
          </div>
        </section>
        <p
          className="kvm-login-reveal text-center text-xs"
          style={{ color: 'var(--kvm-text-muted)', animationDelay: '140ms' }}
        >
          (c) 2026 {baseConfig.siteName}. Secure virtualization operations console.
        </p>
      </section>
    </main>
  );
}

const inputStyle = {
  background: 'var(--kvm-control-bg)',
  border: '1px solid var(--kvm-border)',
  color: 'var(--kvm-text)',
};

function ThemeToggle({ theme, onToggle }: { theme: KvmTheme; onToggle: () => void }) {
  return (
    <KvmTooltip
      label={theme === 'dark' ? '切换浅色背景' : '切换深色背景'}
      placement="bottom"
      className="absolute right-5 top-5 z-20 sm:right-6 sm:top-6"
    >
      <button
        type="button"
        onClick={onToggle}
        className="kvm-action-button flex h-11 w-11 items-center justify-center rounded-lg border"
        style={{
          background: 'var(--kvm-control-bg)',
          borderColor: 'var(--kvm-border)',
          color: 'var(--kvm-text-muted)',
          boxShadow: 'var(--kvm-menu-shadow)',
        }}
        aria-label={theme === 'dark' ? '切换浅色背景' : '切换深色背景'}
      >
        {theme === 'dark' ? <SunIcon size={18} /> : <MoonIcon size={18} />}
      </button>
    </KvmTooltip>
  );
}

function Field({
  label,
  required,
  icon: Icon,
  action,
  hideIcon,
  children,
}: {
  label: string;
  required?: boolean;
  icon: typeof UserIcon;
  action?: ReactNode;
  hideIcon?: boolean;
  children: ReactNode;
}) {
  return (
    <label className="block text-sm">
      <span className="mb-2 flex items-center justify-between gap-3">
        <span className="block font-medium" style={{ color: 'var(--kvm-text)' }}>
          {label}
          {required && <span style={{ color: '#f87171' }}> *</span>}
        </span>
        {action}
      </span>
      <span className="relative block min-w-0">
        {!hideIcon && (
          <Icon
            className="pointer-events-none absolute left-4 top-1/2 z-10 -translate-y-1/2"
            size={17}
            style={{ color: 'var(--kvm-text-muted)' }}
            aria-hidden="true"
          />
        )}
        {children}
      </span>
    </label>
  );
}

function ErrorLine({ error }: { error: string }) {
  return (
    <div aria-live="polite" className="min-h-6">
      {error && (
        <p
          role="alert"
          className="rounded-xl px-3 py-2 text-xs"
          style={{
            background: 'rgba(239,68,68,0.12)',
            border: '1px solid rgba(239,68,68,0.28)',
            color: '#fca5a5',
          }}
        >
          {error}
        </p>
      )}
    </div>
  );
}

function PrimaryButton({ busy, label }: { busy: boolean; label: string }) {
  return (
    <button
      type="submit"
      disabled={busy}
      className="kvm-login-submit flex h-12 w-full items-center justify-center gap-2 rounded-2xl text-sm font-semibold transition-all disabled:cursor-not-allowed disabled:opacity-60"
      style={{
        background: 'linear-gradient(135deg, #0f766e, #14b8a6)',
        color: '#fff',
        boxShadow: '0 18px 48px rgba(20,184,166,0.32)',
      }}
    >
      {busy ? '处理中' : label}
    </button>
  );
}
