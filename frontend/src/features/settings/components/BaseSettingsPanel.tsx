import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent } from 'react';
import { toast } from 'sonner';
import {
  BellRingIcon,
  ImageIcon,
  KeyRoundIcon,
  MonitorCogIcon,
  RadioTowerIcon,
  SaveIcon,
  UploadCloudIcon,
} from 'lucide-react';

import {
  fetchSystemBaseConfig,
  updateSystemBaseConfig,
  type SystemBaseConfig,
} from '../../../lib/api';
import {
  defaultBaseConfig,
  normalizeBaseConfig,
  setBaseConfigSnapshot,
} from '../../../lib/branding';
import { MetricBar } from '../../../components/kvm/StatusBadge';

type BaseConfigTab = 'brand' | 'security' | 'thresholds' | 'agent' | 'notifications';
type BaseForm = SystemBaseConfig;

const configCards: Array<{
  id: BaseConfigTab;
  title: string;
  description: string;
  icon: React.ElementType;
  color: string;
}> = [
  {
    id: 'brand',
    title: '品牌标识',
    description: '网站名称、登录展示和图标',
    icon: ImageIcon,
    color: '#38bdf8',
  },
  {
    id: 'security',
    title: '安全时效',
    description: '找回密码验证码、发送冷却与限流窗口',
    icon: KeyRoundIcon,
    color: '#22c55e',
  },
  {
    id: 'thresholds',
    title: '资源阈值',
    description: 'CPU、内存、磁盘百分比条颜色阈值',
    icon: MonitorCogIcon,
    color: '#f59e0b',
  },
  {
    id: 'agent',
    title: 'Agent 判定',
    description: '离线失败次数和资源告警连续次数',
    icon: RadioTowerIcon,
    color: '#8b5cf6',
  },
  {
    id: 'notifications',
    title: '通知策略',
    description: '告警通知超时、重试节奏和处理批量',
    icon: BellRingIcon,
    color: '#14b8a6',
  },
];

export function BaseSettingsPanel({ canManage = true }: { canManage?: boolean }) {
  const [active, setActive] = useState<BaseConfigTab>('brand');
  const [form, setForm] = useState<BaseForm>(defaultBaseConfig);
  const [savedForm, setSavedForm] = useState<BaseForm>(defaultBaseConfig);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [dragging, setDragging] = useState(false);
  const inputRef = useRef<HTMLInputElement | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const config = normalizeBaseConfig(await fetchSystemBaseConfig());
      setForm(config);
      setSavedForm(config);
      setBaseConfigSnapshot(config);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '读取基础配置失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const preview = useMemo(() => normalizeBaseConfig(savedForm), [savedForm]);
  const selectedCard = configCards.find(card => card.id === active) ?? configCards[0];
  const ActiveIcon = selectedCard.icon;

  const save = async () => {
    const payload = mergeBaseConfigTab(savedForm, form, active);
    const validationError = validateBaseConfigTab(active, payload);
    if (validationError) {
      toast.error(validationError);
      return;
    }
    setBusy(true);
    try {
      const saved = normalizeBaseConfig(await updateSystemBaseConfig(stripReadonlyFields(payload)));
      setForm(saved);
      setSavedForm(saved);
      setBaseConfigSnapshot(saved);
      toast.success('基础配置已保存');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '保存基础配置失败');
    } finally {
      setBusy(false);
    }
  };

  const update = (patch: Partial<BaseForm>) => setForm(current => ({ ...current, ...patch }));
  const selectTab = (tab: BaseConfigTab) => {
    setActive(tab);
    setForm(savedForm);
  };

  const updateFile = async (file: File | undefined) => {
    if (!file) return;
    if (!file.type.startsWith('image/')) {
      toast.error('请选择图片文件');
      return;
    }
    if (file.size > 256 * 1024) {
      toast.error('图标文件不能超过 256KB');
      return;
    }
    update({ iconData: await readFileAsDataURL(file) });
  };

  const onDrop = (event: DragEvent<HTMLButtonElement>) => {
    event.preventDefault();
    setDragging(false);
    void updateFile(event.dataTransfer.files[0]);
  };

  return (
    <div className="grid min-h-0 flex-1 grid-cols-1 items-stretch gap-4 overflow-hidden xl:grid-cols-[430px_minmax(0,1fr)]">
      <nav
        className="kvm-hidden-scrollbar max-h-64 min-h-0 space-y-2 overflow-y-auto rounded-lg p-2 xl:max-h-none"
        style={{ background: 'rgba(255,255,255,0.035)', border: '1px solid var(--kvm-border)' }}
        aria-label="基础配置"
      >
        {configCards.map(card => (
          <BaseConfigCard
            key={card.id}
            card={card}
            active={active === card.id}
            value={cardValue(card.id, preview)}
            onClick={() => selectTab(card.id)}
          />
        ))}
      </nav>
      <aside
        className="flex min-h-0 flex-col overflow-hidden rounded-lg p-4"
        style={{ background: 'rgba(255,255,255,0.035)', border: '1px solid var(--kvm-border)' }}
      >
        <div className="mb-4 flex items-center gap-3">
          <div
            className="flex h-10 w-10 items-center justify-center rounded-lg"
            style={{
              color: selectedCard.color,
              background: 'rgba(255,255,255,0.05)',
              border: '1px solid rgba(255,255,255,0.08)',
            }}
          >
            <ActiveIcon size={19} />
          </div>
          <div className="min-w-0">
            <div className="truncate text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
              {selectedCard.title}
            </div>
            <div className="mt-0.5 truncate text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
              {selectedCard.description}
            </div>
          </div>
        </div>
        {loading ? (
          <div className="text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
            正在加载基础配置...
          </div>
        ) : (
          <div className="kvm-hidden-scrollbar min-h-0 flex-1 overflow-y-auto pr-2">
            {active === 'brand' && (
              <BrandPanel
                form={form}
                dragging={dragging}
                canManage={canManage}
                inputRef={inputRef}
                setDragging={setDragging}
                onDrop={onDrop}
                onFile={updateFile}
                onUpdate={update}
              />
            )}
            {active === 'security' && (
              <SecurityPanel form={form} canManage={canManage} onUpdate={update} />
            )}
            {active === 'thresholds' && (
              <ThresholdPanel form={form} canManage={canManage} onUpdate={update} />
            )}
            {active === 'agent' && (
              <AgentPanel form={form} canManage={canManage} onUpdate={update} />
            )}
            {active === 'notifications' && (
              <NotificationPolicyPanel form={form} canManage={canManage} onUpdate={update} />
            )}
          </div>
        )}
        {canManage && (
          <div
            className="mt-5 flex shrink-0 justify-end gap-2 border-t pt-4"
            style={{ borderColor: 'var(--kvm-border)' }}
          >
            <button
              type="button"
              onClick={() => setForm(defaultBaseConfig)}
              disabled={busy}
              className="kvm-action-button rounded-lg border px-3 py-2 text-sm disabled:opacity-50"
              style={{
                borderColor: 'var(--kvm-border)',
                color: 'var(--kvm-text-muted)',
                background: 'rgba(255,255,255,0.035)',
              }}
            >
              恢复默认
            </button>
            <button
              type="button"
              onClick={() => void save()}
              disabled={busy || loading}
              className="kvm-action-button flex items-center gap-2 rounded-lg border px-3 py-2 text-sm disabled:opacity-50"
              style={{
                borderColor: 'rgba(59,130,246,0.38)',
                color: 'var(--kvm-accent-text)',
                background: 'rgba(59,130,246,0.1)',
              }}
            >
              <SaveIcon size={14} />
              {busy ? '保存中' : '保存'}
            </button>
          </div>
        )}
      </aside>
    </div>
  );
}

function BaseConfigCard({
  card,
  active,
  value,
  onClick,
}: {
  card: (typeof configCards)[number];
  active: boolean;
  value: string;
  onClick: () => void;
}) {
  const Icon = card.icon;
  return (
    <button
      type="button"
      onClick={onClick}
      className="kvm-action-button flex w-full items-start gap-3 rounded-lg p-3 text-left"
      style={{
        background: active ? 'rgba(59,130,246,0.12)' : 'transparent',
        border: active ? '1px solid rgba(96,165,250,0.56)' : '1px solid transparent',
        color: 'var(--kvm-text)',
      }}
    >
      <div
        className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg"
        style={{
          color: card.color,
          background: 'rgba(255,255,255,0.05)',
          border: '1px solid rgba(255,255,255,0.08)',
        }}
      >
        <Icon size={19} />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center justify-between gap-3">
          <span className="truncate text-sm font-semibold">{card.title}</span>
          <span className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
            {value}
          </span>
        </div>
        <p
          className="mt-1 line-clamp-2 text-xs leading-5"
          style={{ color: 'var(--kvm-text-muted)' }}
        >
          {card.description}
        </p>
      </div>
    </button>
  );
}

function BrandPanel({
  form,
  dragging,
  canManage,
  inputRef,
  setDragging,
  onDrop,
  onFile,
  onUpdate,
}: {
  form: BaseForm;
  dragging: boolean;
  canManage: boolean;
  inputRef: React.RefObject<HTMLInputElement | null>;
  setDragging: (value: boolean) => void;
  onDrop: (event: DragEvent<HTMLButtonElement>) => void;
  onFile: (file: File | undefined) => void;
  onUpdate: (patch: Partial<BaseForm>) => void;
}) {
  return (
    <div className="space-y-5">
      <div className="grid gap-4 lg:grid-cols-2">
        <TextField
          label="网站名称"
          value={form.siteName}
          disabled={!canManage}
          placeholder="KVM Manager"
          onChange={value => onUpdate({ siteName: value })}
        />
        <TextField
          label="认证页品牌名称"
          value={form.loginName}
          disabled={!canManage}
          placeholder="KVM Manager"
          onChange={value => onUpdate({ loginName: value })}
        />
        <TextField
          label="控制台品牌名称"
          value={form.appName}
          disabled={!canManage}
          placeholder="KVM Manager"
          onChange={value => onUpdate({ appName: value })}
        />
        <TextField
          label="控制台品牌副标题"
          value={form.appSubtitle}
          disabled={!canManage}
          placeholder="VIRTUALIZATION OPS"
          onChange={value => onUpdate({ appSubtitle: value })}
        />
      </div>
      <div className="grid gap-4 lg:grid-cols-[260px_minmax(0,1fr)]">
        <button
          type="button"
          disabled={!canManage}
          onClick={() => inputRef.current?.click()}
          onDragOver={event => {
            event.preventDefault();
            setDragging(true);
          }}
          onDragLeave={() => setDragging(false)}
          onDrop={onDrop}
          className="kvm-action-button flex min-h-44 flex-col items-center justify-center gap-3 rounded-xl border border-dashed p-4 disabled:cursor-not-allowed disabled:opacity-60"
          style={{
            borderColor: dragging ? 'rgba(96,165,250,0.78)' : 'var(--kvm-border)',
            background: dragging ? 'rgba(59,130,246,0.12)' : 'rgba(255,255,255,0.026)',
            color: 'var(--kvm-text-muted)',
          }}
        >
          <img
            src={form.iconData || defaultBaseConfig.iconData}
            alt="系统图标预览"
            className="h-16 w-16 rounded-xl object-contain"
          />
          <span className="flex items-center gap-2 text-sm">
            <UploadCloudIcon size={15} />
            拖动或点击上传
          </span>
          <span className="text-xs">支持图片，建议 256KB 内</span>
        </button>
        <BrandPreview config={form} />
      </div>
      <input
        ref={inputRef}
        type="file"
        accept="image/*"
        className="hidden"
        onChange={event => void onFile(event.target.files?.[0])}
      />
    </div>
  );
}

function SecurityPanel({
  form,
  canManage,
  onUpdate,
}: {
  form: BaseForm;
  canManage: boolean;
  onUpdate: (patch: Partial<BaseForm>) => void;
}) {
  return (
    <div className="grid gap-3 lg:grid-cols-2">
      <NumberControl
        label="找回密码验证码有效期"
        unit="分钟"
        value={form.passwordResetCodeTtlMinutes}
        min={1}
        max={60}
        disabled={!canManage}
        onChange={value => onUpdate({ passwordResetCodeTtlMinutes: value })}
      />
      <NumberControl
        label="图形验证码有效期"
        unit="分钟"
        value={form.passwordResetCaptchaTtlMinutes}
        min={1}
        max={10}
        disabled={!canManage}
        onChange={value => onUpdate({ passwordResetCaptchaTtlMinutes: value })}
      />
      <NumberControl
        label="发送冷却时间"
        description={`验证码发送后 ${form.passwordResetSendCooldownMinutes} 分钟内不可重复请求`}
        unit="分钟"
        value={form.passwordResetSendCooldownMinutes}
        min={0.5}
        max={10}
        step={0.5}
        disabled={!canManage}
        onChange={value => onUpdate({ passwordResetSendCooldownMinutes: value })}
      />
      <NumberControl
        label="频率限制统计窗口"
        description={`验证码在 ${form.passwordResetRateLimitMinutes} 分钟内最多请求 5 次`}
        unit="分钟"
        value={form.passwordResetRateLimitMinutes}
        min={5}
        max={10}
        disabled={!canManage}
        onChange={value => onUpdate({ passwordResetRateLimitMinutes: value })}
      />
    </div>
  );
}

function ThresholdPanel({
  form,
  canManage,
  onUpdate,
}: {
  form: BaseForm;
  canManage: boolean;
  onUpdate: (patch: Partial<BaseForm>) => void;
}) {
  const thresholdConfig = {
    resourceWarningThreshold: form.resourceWarningThreshold,
    resourceCriticalThreshold: form.resourceCriticalThreshold,
  };
  return (
    <div className="space-y-4">
      <div className="grid gap-3 lg:grid-cols-2">
        <NumberControl
          label="警告阈值"
          unit="%"
          value={form.resourceWarningThreshold}
          min={1}
          max={99}
          disabled={!canManage}
          onChange={value => onUpdate({ resourceWarningThreshold: value })}
        />
        <NumberControl
          label="严重阈值"
          unit="%"
          value={form.resourceCriticalThreshold}
          min={2}
          max={100}
          disabled={!canManage}
          onChange={value => onUpdate({ resourceCriticalThreshold: value })}
        />
      </div>
      <div
        className="rounded-xl border p-4"
        style={{ borderColor: 'var(--kvm-border)', background: 'rgba(255,255,255,0.026)' }}
      >
        <div className="mb-3 text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
          百分比条颜色预览
        </div>
        <div className="grid gap-4 lg:grid-cols-3">
          <MetricBar
            value={Math.max(1, form.resourceWarningThreshold - 8)}
            label="正常区间"
            thresholdConfig={thresholdConfig}
          />
          <MetricBar
            value={form.resourceWarningThreshold}
            label="警告区间"
            thresholdConfig={thresholdConfig}
          />
          <MetricBar
            value={form.resourceCriticalThreshold}
            label="严重区间"
            thresholdConfig={thresholdConfig}
          />
        </div>
        <p className="mt-3 text-xs leading-5" style={{ color: 'var(--kvm-text-muted)' }}>
          保存后，总览、虚拟机、宿主机等页面的 CPU、内存、磁盘百分比条会按这里的阈值切换颜色。
        </p>
      </div>
    </div>
  );
}

function AgentPanel({
  form,
  canManage,
  onUpdate,
}: {
  form: BaseForm;
  canManage: boolean;
  onUpdate: (patch: Partial<BaseForm>) => void;
}) {
  return (
    <div className="grid gap-3 lg:grid-cols-2">
      <NumberControl
        label="资源告警连续次数"
        unit="次"
        value={form.resourceAlertConsecutiveCount}
        min={1}
        max={20}
        disabled={!canManage}
        onChange={value => onUpdate({ resourceAlertConsecutiveCount: value })}
      />
      <NumberControl
        label="Agent 离线失败次数"
        unit="次"
        value={form.agentOfflineFailureCount}
        min={1}
        max={20}
        disabled={!canManage}
        onChange={value => onUpdate({ agentOfflineFailureCount: value })}
      />
    </div>
  );
}

function NotificationPolicyPanel({
  form,
  canManage,
  onUpdate,
}: {
  form: BaseForm;
  canManage: boolean;
  onUpdate: (patch: Partial<BaseForm>) => void;
}) {
  return (
    <div className="grid gap-3 lg:grid-cols-2">
      <NumberControl
        label="通知发送超时"
        description={`外部通知请求超过 ${form.alertNotificationTimeoutSeconds} 秒视为失败`}
        unit="秒"
        value={form.alertNotificationTimeoutSeconds}
        min={3}
        max={60}
        disabled={!canManage}
        onChange={value => onUpdate({ alertNotificationTimeoutSeconds: value })}
      />
      <NumberControl
        label="最大重试次数"
        description="设置为 0 时发送失败后不再重试"
        unit="次"
        value={form.alertNotificationMaxRetryCount}
        min={0}
        max={10}
        disabled={!canManage}
        onChange={value => onUpdate({ alertNotificationMaxRetryCount: value })}
      />
      <NumberControl
        label="重试基础间隔"
        description="失败后按基础间隔指数退避重试"
        unit="秒"
        value={form.alertNotificationRetryBaseSeconds}
        min={10}
        max={300}
        step={10}
        disabled={!canManage}
        onChange={value => onUpdate({ alertNotificationRetryBaseSeconds: value })}
      />
      <NumberControl
        label="重试最大间隔"
        description="指数退避不会超过该间隔"
        unit="分钟"
        value={form.alertNotificationRetryMaxMinutes}
        min={1}
        max={120}
        disabled={!canManage}
        onChange={value => onUpdate({ alertNotificationRetryMaxMinutes: value })}
      />
      <NumberControl
        label="单轮处理批量"
        description="每次扫描待通知告警和待重试投递的数量"
        unit="条"
        value={form.alertNotificationBatchSize}
        min={10}
        max={100}
        step={5}
        disabled={!canManage}
        onChange={value => onUpdate({ alertNotificationBatchSize: value })}
      />
    </div>
  );
}

function BrandPreview({ config }: { config: BaseForm }) {
  return (
    <div
      className="flex min-h-44 flex-col rounded-xl border p-5"
      style={{ borderColor: 'var(--kvm-border)', background: 'rgba(255,255,255,0.026)' }}
    >
      <div className="text-sm font-semibold" style={{ color: 'var(--kvm-text-muted)' }}>
        实时预览
      </div>
      <div className="flex flex-1 items-center">
        <div className="flex min-w-0 items-center gap-4">
          <img
            src={config.iconData}
            alt={config.appName}
            className="h-16 w-16 shrink-0 rounded-xl object-contain"
          />
          <div className="min-w-0">
            <div className="kvm-gradient-text truncate text-xl font-bold">{config.appName}</div>
            <div
              className="mt-1 truncate text-sm tracking-widest"
              style={{ color: 'var(--kvm-text-muted)' }}
            >
              {config.appSubtitle}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function TextField({
  label,
  value,
  placeholder,
  disabled,
  onChange,
}: {
  label: string;
  value: string;
  placeholder: string;
  disabled: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <label className="block min-w-0 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
      <span className="mb-1.5 block w-full leading-5">{label}</span>
      <input
        value={value}
        disabled={disabled}
        maxLength={60}
        onChange={event => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-10 w-full min-w-0 rounded-lg px-3 text-sm outline-none disabled:opacity-60"
        style={{
          background: 'var(--kvm-control-bg)',
          border: '1px solid var(--kvm-border)',
          color: 'var(--kvm-text)',
        }}
      />
    </label>
  );
}

function NumberControl({
  label,
  description,
  unit,
  value,
  min,
  max,
  step = 1,
  disabled,
  onChange,
}: {
  label: string;
  description?: string;
  unit: string;
  value: number;
  min: number;
  max: number;
  step?: number;
  disabled: boolean;
  onChange: (value: number) => void;
}) {
  const normalized = Math.min(max, Math.max(min, Number(value) || min));
  const progress = max === min ? 100 : ((normalized - min) / (max - min)) * 100;
  return (
    <label
      className="rounded-xl border p-4"
      style={{
        borderColor: 'var(--kvm-border)',
        background: 'rgba(255,255,255,0.026)',
        color: 'var(--kvm-text-muted)',
      }}
    >
      <span
        className="flex items-center justify-between gap-3 text-sm font-semibold"
        style={{ color: 'var(--kvm-text)' }}
      >
        <span>{label}</span>
        <span className="flex min-w-0 items-center gap-2">
          <span
            className="hidden truncate text-xs font-normal lg:inline"
            style={{ color: 'var(--kvm-text-muted)' }}
          >
            {description}
          </span>
          <span
            className="shrink-0 rounded-md border px-2 py-0.5 text-xs"
            style={{
              borderColor: 'rgba(59,130,246,0.28)',
              color: 'var(--kvm-accent-text)',
              background: 'rgba(59,130,246,0.08)',
            }}
          >
            {normalized} {unit}
          </span>
        </span>
      </span>
      {description && (
        <span className="mt-2 block text-xs lg:hidden" style={{ color: 'var(--kvm-text-muted)' }}>
          {description}
        </span>
      )}
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={normalized}
        disabled={disabled}
        onChange={event => onChange(Number(event.target.value))}
        className="kvm-flow-range mt-4 w-full disabled:opacity-60"
        style={{ '--kvm-range-progress': `${progress}%` } as React.CSSProperties}
      />
      <div className="mt-3 flex items-center gap-3">
        <input
          type="number"
          min={min}
          max={max}
          step={step}
          value={normalized}
          disabled={disabled}
          onChange={event =>
            onChange(Math.min(max, Math.max(min, Number(event.target.value) || min)))
          }
          className="h-9 w-24 rounded-lg px-3 text-sm outline-none disabled:opacity-60"
          style={{
            background: 'var(--kvm-control-bg)',
            border: '1px solid var(--kvm-border)',
            color: 'var(--kvm-text)',
          }}
        />
        <span className="text-xs">
          范围 {min}-{max} {unit}
        </span>
      </div>
    </label>
  );
}

function cardValue(tab: BaseConfigTab, config: BaseForm) {
  if (tab === 'brand') return config.appName;
  if (tab === 'security') return `${config.passwordResetCodeTtlMinutes} 分钟`;
  if (tab === 'thresholds')
    return `${config.resourceWarningThreshold}/${config.resourceCriticalThreshold}%`;
  if (tab === 'notifications') return `${config.alertNotificationMaxRetryCount} 次`;
  return `${config.agentOfflineFailureCount} 次`;
}

function stripReadonlyFields(config: SystemBaseConfig): SystemBaseConfig {
  const { created_at, updated_at, ...payload } = config;
  return payload;
}

function mergeBaseConfigTab(saved: BaseForm, form: BaseForm, tab: BaseConfigTab) {
  const payload = { ...saved };
  for (const key of baseConfigTabKeys(tab)) {
    payload[key] = form[key] as never;
  }
  return normalizeBaseConfig(payload);
}

function baseConfigTabKeys(tab: BaseConfigTab): Array<keyof BaseForm> {
  switch (tab) {
    case 'brand':
      return ['siteName', 'loginName', 'appName', 'appSubtitle', 'iconData'];
    case 'security':
      return [
        'passwordResetCodeTtlMinutes',
        'passwordResetCaptchaTtlMinutes',
        'passwordResetSendCooldownMinutes',
        'passwordResetRateLimitMinutes',
      ];
    case 'thresholds':
      return ['resourceWarningThreshold', 'resourceCriticalThreshold'];
    case 'agent':
      return ['resourceAlertConsecutiveCount', 'agentOfflineFailureCount'];
    case 'notifications':
      return [
        'alertNotificationTimeoutSeconds',
        'alertNotificationMaxRetryCount',
        'alertNotificationRetryBaseSeconds',
        'alertNotificationRetryMaxMinutes',
        'alertNotificationBatchSize',
      ];
  }
}

function validateBaseConfigTab(tab: BaseConfigTab, config: SystemBaseConfig) {
  if (
    tab === 'brand' &&
    (!config.siteName.trim() ||
      !config.loginName.trim() ||
      !config.appName.trim() ||
      !config.appSubtitle.trim())
  )
    return '名称配置不能为空';
  if (tab === 'thresholds' && config.resourceWarningThreshold >= config.resourceCriticalThreshold)
    return '警告阈值必须小于严重阈值';
  if (
    tab === 'notifications' &&
    config.alertNotificationRetryMaxMinutes * 60 < config.alertNotificationRetryBaseSeconds
  )
    return '告警通知重试最大间隔不能小于基础间隔';
  return '';
}

function readFileAsDataURL(file: File) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ''));
    reader.onerror = () => reject(new Error('读取图标失败'));
    reader.readAsDataURL(file);
  });
}
