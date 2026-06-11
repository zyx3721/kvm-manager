import { useCallback, useEffect, useId, useMemo, useRef, useState, type ElementType } from 'react';
import {
  AlertTriangleIcon,
  BoxesIcon,
  CheckIcon,
  ChevronDownIcon,
  CpuIcon,
  HardDriveIcon,
  MemoryStickIcon,
  ServerIcon,
} from 'lucide-react';
import { Cell, Pie, PieChart, ResponsiveContainer, Sector, Tooltip } from 'recharts';
import {
  fetchDashboardSummary,
  fetchHostMetricSeries,
  fetchHosts,
  fetchVMMetricSeries,
  fetchVMs,
  type DashboardSummary,
  type Host,
  type MetricPoint,
  type VirtualMachine,
} from '../../lib/api';
import { formatBytes } from '../../lib/format';
import { MetricBar } from '../../components/kvm/StatusBadge';
import { ResourceTrendChart } from '../../components/kvm/ResourceTrendChart';
import { storageUsageColor } from '../storage-pools/utils/storageUsage';
import { onKvmRefresh } from '../../lib/refresh';
import { can } from '../../lib/permissions';
import { toast } from 'sonner';

type ChartTooltipPayload = {
  color?: string;
  name?: string;
  value?: number;
  payload?: StatusChartItem;
};

function KpiCard({
  icon: Icon,
  label,
  value,
  hint,
  color,
}: {
  icon: ElementType;
  label: string;
  value: string;
  hint: string;
  color: string;
}) {
  return (
    <div
      className="rounded-xl p-5 kvm-card-hover"
      style={{ background: 'var(--kvm-card)', border: '1px solid var(--kvm-border)' }}
    >
      <div className="mb-3 flex items-center justify-between">
        <span
          className="text-xs uppercase tracking-widest"
          style={{ color: 'var(--kvm-text-muted)' }}
        >
          {label}
        </span>
        <Icon size={18} style={{ color }} />
      </div>
      <div
        className="font-mono text-3xl font-semibold tabular-nums"
        style={{ color: 'var(--kvm-text)' }}
      >
        {value}
      </div>
      <div className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
        {hint}
      </div>
    </div>
  );
}

type StatusChartItem = {
  label: string;
  count: number;
  color: string;
};

function StatusDonutChart({ total, items }: { total: number; items: StatusChartItem[] }) {
  const chartId = useId().replace(/:/g, '');
  const [activeIndex, setActiveIndex] = useState<number | undefined>();
  const [tooltipPosition, setTooltipPosition] = useState<{ x: number; y: number } | undefined>();
  const chartItems = items.filter(item => item.count > 0);
  const hasData = total > 0 && chartItems.length > 0;
  const shadowId = `${chartId}-shadow`;

  return (
    <div className="grid grid-cols-1 gap-4 md:grid-cols-[12rem_1fr] xl:grid-cols-1 2xl:grid-cols-[12rem_1fr]">
      <div className="relative mx-auto h-48 w-full max-w-48">
        <div
          className="pointer-events-none absolute inset-x-7 bottom-3 h-8 rounded-full blur-xl"
          style={{ background: 'rgba(84, 116, 196, 0.34)' }}
        />
        {hasData ? (
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <defs>
                <filter id={shadowId} x="-45%" y="-45%" width="190%" height="190%">
                  <feDropShadow
                    dx="0"
                    dy="9"
                    stdDeviation="6"
                    floodColor="#020617"
                    floodOpacity="0.32"
                  />
                </filter>
                {chartItems.map((item, index) => (
                  <linearGradient
                    key={item.label}
                    id={`${chartId}-slice-${index}`}
                    x1="0"
                    y1="0"
                    x2="1"
                    y2="1"
                  >
                    <stop offset="0%" stopColor={adjustHexColor(item.color, 48)} />
                    <stop offset="42%" stopColor={item.color} />
                    <stop offset="100%" stopColor={adjustHexColor(item.color, -32)} />
                  </linearGradient>
                ))}
              </defs>
              <Pie
                data={chartItems}
                dataKey="count"
                nameKey="label"
                cx="50%"
                cy="50%"
                innerRadius="58%"
                outerRadius="86%"
                paddingAngle={1}
                stroke="var(--kvm-card)"
                strokeWidth={3}
                labelLine={false}
                label={renderStatusSliceLabel}
                activeIndex={activeIndex}
                activeShape={renderStatusSlice(chartId, shadowId, { active: true, dimmed: false })}
                inactiveShape={renderStatusSlice(chartId, shadowId, {
                  active: false,
                  dimmed: true,
                })}
                onMouseEnter={(entry, index) => {
                  setActiveIndex(index);
                  setTooltipPosition(getStatusTooltipPosition(entry));
                }}
                onMouseLeave={() => {
                  setActiveIndex(undefined);
                  setTooltipPosition(undefined);
                }}
                isAnimationActive
                animationDuration={420}
              >
                {chartItems.map((item, index) => (
                  <Cell
                    key={item.label}
                    fill={`url(#${chartId}-slice-${index})`}
                    tabIndex={-1}
                    style={{ cursor: 'pointer', outline: 'none' }}
                  />
                ))}
              </Pie>
              <Tooltip
                content={<StatusChartTooltip />}
                cursor={false}
                allowEscapeViewBox={{ x: true, y: true }}
                position={tooltipPosition}
                wrapperStyle={{ outline: 'none', pointerEvents: 'none', zIndex: 20 }}
              />
            </PieChart>
          </ResponsiveContainer>
        ) : (
          <div
            className="absolute inset-5 rounded-full border"
            style={{ borderColor: 'rgba(148,163,184,0.28)', background: 'rgba(148,163,184,0.06)' }}
          />
        )}
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
          <div className="text-center">
            <div
              className="font-mono text-2xl font-semibold tabular-nums"
              style={{ color: 'var(--kvm-text)' }}
            >
              {total}
            </div>
            <div className="text-[11px]" style={{ color: 'var(--kvm-text-muted)' }}>
              总数
            </div>
          </div>
        </div>
      </div>
      <div className="flex min-w-0 flex-col justify-center gap-2">
        {items.map(item => {
          const pct = total ? Math.round((item.count / total) * 100) : 0;
          return (
            <div
              key={item.label}
              className="flex items-center justify-between gap-3 rounded-lg px-2.5 py-2"
              style={{
                background: item.count > 0 ? 'rgba(255,255,255,0.035)' : 'rgba(148,163,184,0.035)',
                border: '1px solid rgba(148,163,184,0.12)',
              }}
            >
              <div className="flex min-w-0 items-center gap-2">
                <span
                  className="h-2.5 w-2.5 shrink-0 rounded-full"
                  style={{ background: item.color, boxShadow: `0 0 14px ${item.color}66` }}
                />
                <span
                  className="truncate text-sm"
                  style={{ color: item.count > 0 ? 'var(--kvm-text)' : 'var(--kvm-text-muted)' }}
                >
                  {item.label}
                </span>
              </div>
              <div className="flex shrink-0 items-baseline gap-2 font-mono tabular-nums">
                <span className="text-sm" style={{ color: 'var(--kvm-text)' }}>
                  {item.count}
                </span>
                <span className="text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                  {pct}%
                </span>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function getStatusTooltipPosition(entry: unknown) {
  const sector = entry as { cx?: number; cy?: number; outerRadius?: number; midAngle?: number };
  const cx = Number(sector.cx ?? 96);
  const cy = Number(sector.cy ?? 96);
  const outerRadius = Number(sector.outerRadius ?? 82);
  const midAngle = Number(sector.midAngle ?? 0);
  const angle = -midAngle * (Math.PI / 180);
  const anchorX = cx + Math.cos(angle) * (outerRadius + 12);
  const anchorY = cy + Math.sin(angle) * (outerRadius + 10);
  const onRight = Math.cos(angle) >= 0;
  const tooltipWidth = 160;
  const tooltipHeight = 44;
  const rawX = onRight ? anchorX + 8 : anchorX - tooltipWidth + 18;
  const rawY = anchorY - tooltipHeight / 2;

  return {
    x: clamp(rawX, -28, 192 - tooltipWidth + 28),
    y: clamp(rawY, -8, 192 - tooltipHeight + 8),
  };
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

function renderStatusSlice(
  chartId: string,
  filterId: string,
  state: { active: boolean; dimmed: boolean }
) {
  return (props: {
    cx?: number;
    cy?: number;
    innerRadius?: number;
    outerRadius?: number;
    startAngle?: number;
    endAngle?: number;
    fill?: string;
    payload?: StatusChartItem;
    midAngle?: number;
    index?: number;
  }) => {
    const {
      cx = 0,
      cy = 0,
      innerRadius = 0,
      outerRadius = 0,
      startAngle = 0,
      endAngle = 0,
      fill,
      payload,
      midAngle = 0,
      index = 0,
    } = props;
    const angle = -midAngle * (Math.PI / 180);
    const lift = state.active ? 4 : 0;
    const offsetX = Math.cos(angle) * lift;
    const offsetY = Math.sin(angle) * lift;
    const topFill = fill || `url(#${chartId}-slice-${index})`;
    const opacity = state.dimmed ? 0.22 : 1;
    const translateTransform = state.active ? `translate(${offsetX} ${offsetY})` : 'translate(0 0)';

    return (
      <g
        tabIndex={-1}
        transform={translateTransform}
        filter={state.active ? `url(#${filterId})` : undefined}
        opacity={opacity}
        style={{
          cursor: 'pointer',
          outline: 'none',
          transition: 'opacity 360ms ease, filter 360ms ease',
        }}
      >
        {state.active && (
          <animateTransform
            attributeName="transform"
            type="translate"
            from="0 0"
            to={`${offsetX} ${offsetY}`}
            dur="2000ms"
            begin="0s"
            fill="freeze"
            calcMode="spline"
            keySplines="0.1 0 0.2 1"
          />
        )}
        <g transform={`translate(${cx} ${cy})`}>
          <g transform="scale(1)">
            <g transform={`translate(${-cx} ${-cy})`}>
              <Sector
                cx={cx}
                cy={cy}
                innerRadius={innerRadius}
                outerRadius={outerRadius}
                startAngle={startAngle}
                endAngle={endAngle}
                fill={topFill}
                stroke={state.active ? 'rgba(255,255,255,0.58)' : 'var(--kvm-card)'}
                strokeWidth={3}
              />
              {state.active && (
                <Sector
                  cx={cx}
                  cy={cy}
                  innerRadius={innerRadius + 1}
                  outerRadius={outerRadius}
                  startAngle={startAngle}
                  endAngle={endAngle}
                  fill="rgba(255,255,255,0.12)"
                  stroke="none"
                />
              )}
            </g>
          </g>
        </g>
      </g>
    );
  };
}

function StatusChartTooltip({
  active,
  payload,
}: {
  active?: boolean;
  payload?: ChartTooltipPayload[];
}) {
  const item = payload?.[0];
  if (!active || !item) return null;
  const status = item.payload;
  const color = status?.color || item.color || 'var(--kvm-accent)';
  const label = status?.label || item.name || '';
  const value = status?.count ?? item.value ?? 0;

  return (
    <div
      className="rounded-lg px-3 py-2 text-sm shadow-2xl"
      style={{
        background: 'var(--kvm-popover-bg)',
        border: '1px solid var(--kvm-popover-border)',
        color: 'var(--kvm-text)',
        boxShadow: 'var(--kvm-menu-shadow)',
      }}
    >
      <div className="flex items-center gap-2">
        <span
          className="h-2.5 w-2.5 shrink-0 rounded-full"
          style={{ background: color, boxShadow: `0 0 12px ${color}66` }}
        />
        <span className="font-semibold">{label}</span>
        <span className="font-mono font-semibold tabular-nums">{value} 台</span>
      </div>
    </div>
  );
}

function renderStatusSliceLabel(props: {
  cx?: number;
  cy?: number;
  midAngle?: number;
  innerRadius?: number;
  outerRadius?: number;
  percent?: number;
  value?: number;
}) {
  const {
    cx = 0,
    cy = 0,
    midAngle = 0,
    innerRadius = 0,
    outerRadius = 0,
    percent = 0,
    value = 0,
  } = props;
  if (percent < 0.08) return null;
  const radius = innerRadius + (outerRadius - innerRadius) * 0.58;
  const angle = -midAngle * (Math.PI / 180);
  const x = cx + radius * Math.cos(angle);
  const y = cy + radius * Math.sin(angle);

  return (
    <text
      x={x}
      y={y}
      textAnchor="middle"
      dominantBaseline="central"
      className="fill-current font-mono text-xs font-semibold"
      style={{ color: 'var(--kvm-text)' }}
    >
      {value}
    </text>
  );
}

function adjustHexColor(color: string, amount: number) {
  const normalized = color.replace('#', '');
  if (!/^[0-9a-fA-F]{6}$/.test(normalized)) return color;
  const channels = [0, 2, 4].map(start => {
    const value = parseInt(normalized.slice(start, start + 2), 16);
    return Math.max(0, Math.min(255, value + amount))
      .toString(16)
      .padStart(2, '0');
  });
  return `#${channels.join('')}`;
}

export default function Dashboard() {
  const [summary, setSummary] = useState<DashboardSummary | null>(null);
  const [hosts, setHosts] = useState<Host[]>([]);
  const [vms, setVMs] = useState<VirtualMachine[]>([]);
  const [selectedVMId, setSelectedVMId] = useState('');
  const [selectedHostId, setSelectedHostId] = useState('');
  const [vmMetrics, setVMMetrics] = useState<MetricPoint[]>([]);
  const [hostMetrics, setHostMetrics] = useState<MetricPoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const canReadHosts = can('hosts.read');
  const canReadVMs = can('vms.read');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [summaryData, hostData, vmData] = await Promise.all([
        fetchDashboardSummary(),
        canReadHosts ? fetchHosts() : Promise.resolve({ items: [] }),
        canReadVMs ? fetchVMs() : Promise.resolve({ items: [] }),
      ]);
      setSummary(summaryData);
      setHosts(hostData.items);
      setVMs(vmData.items);
      const runningVMs = vmData.items.filter(vm => vm.status === 'running');
      const onlineHosts = hostData.items.filter(host => host.status === 'online');
      setSelectedVMId(current =>
        current && runningVMs.some(vm => vm.id === current) ? current : (runningVMs[0]?.id ?? '')
      );
      setSelectedHostId(current =>
        current && onlineHosts.some(host => host.id === current)
          ? current
          : (onlineHosts[0]?.id ?? '')
      );
    } catch (err) {
      const message = err instanceof Error ? err.message : '加载总览数据失败';
      toast.error(message);
      setError(isPermissionMessage(message) ? '' : message);
    } finally {
      setLoading(false);
    }
  }, [canReadHosts, canReadVMs]);

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0);
    const unsubscribe = onKvmRefresh(() => void load());
    return () => {
      window.clearTimeout(timer);
      unsubscribe();
    };
  }, [load]);

  useEffect(() => {
    if (!selectedVMId) {
      setVMMetrics([]);
      return;
    }
    void fetchVMMetricSeries(selectedVMId, '1h')
      .then(response => setVMMetrics(response.items))
      .catch(() => setVMMetrics([]));
  }, [selectedVMId]);

  useEffect(() => {
    if (!selectedHostId) {
      setHostMetrics([]);
      return;
    }
    void fetchHostMetricSeries(selectedHostId, '1h')
      .then(response => setHostMetrics(response.items))
      .catch(() => setHostMetrics([]));
  }, [selectedHostId]);

  const totals = useMemo(() => {
    const onlineHosts = hosts.filter(host => host.status === 'online');
    const runningVMs = vms.filter(vm => vm.status === 'running');
    return { onlineHosts, runningVMs };
  }, [hosts, vms]);

  if (loading && !summary) {
    return (
      <div className="p-6 text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
        正在加载 KVM 总览...
      </div>
    );
  }

  if (error && !summary) {
    return (
      <div className="p-6">
        <div
          className="flex items-center justify-between rounded-xl p-5"
          style={{ background: 'var(--kvm-card)', border: '1px solid var(--kvm-border)' }}
        >
          <div className="flex items-center gap-2 text-sm" style={{ color: '#f59e0b' }}>
            <AlertTriangleIcon size={16} />
            {error}
          </div>
          <button
            onClick={load}
            className="kvm-action-button rounded-lg px-4 py-2 text-sm"
            style={{
              background: 'rgba(59,130,246,0.12)',
              color: '#3b82f6',
              border: '1px solid rgba(59,130,246,0.28)',
            }}
          >
            重试
          </button>
        </div>
      </div>
    );
  }

  const data = summary!;
  const selectedVM = totals.runningVMs.find(vm => vm.id === selectedVMId);
  const selectedHost = totals.onlineHosts.find(host => host.id === selectedHostId);
  const vmStateData = canReadVMs
    ? data
    : {
        ...data,
        totalVMs: 0,
        runningVMs: 0,
        stoppedVMs: 0,
        pausedVMs: 0,
        errorVMs: 0,
      };

  return (
    <div data-cmp="Dashboard" className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold" style={{ color: 'var(--kvm-text)' }}>
            总览
          </h1>
          <p className="mt-1 text-sm" style={{ color: 'var(--kvm-text-muted)' }}>
            查看在线资源状态与最近 1 小时趋势
          </p>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
        <KpiCard
          icon={ServerIcon}
          label="宿主机"
          value={`${data.onlineHosts}/${data.totalHosts}`}
          hint="在线 / 总数"
          color="#3b82f6"
        />
        <KpiCard
          icon={BoxesIcon}
          label="虚拟机"
          value={`${data.runningVMs}/${data.totalVMs}`}
          hint="运行 / 总数"
          color="#10b981"
        />
        <KpiCard
          icon={CpuIcon}
          label="vCPU 分配"
          value={`${data.usedVCPUs}/${data.totalVCPUs}`}
          hint="已分配 / 总核心"
          color="#06b6d4"
        />
        <KpiCard
          icon={MemoryStickIcon}
          label="内存使用"
          value={formatBytes(data.usedMemoryBytes, 'GB')}
          hint={`共 ${formatBytes(data.totalMemoryBytes, 'GB')}`}
          color="#f59e0b"
        />
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-3">
        <VMResourceCard
          vms={totals.runningVMs}
          selectedId={selectedVMId}
          onSelect={setSelectedVMId}
        />
        <VMStateCard data={vmStateData} />
      </div>
      <ResourceTrendChart
        title="虚拟机趋势"
        subtitle={selectedVM ? `最近 1 小时 · ${selectedVM.name}` : '暂无在线虚拟机可查看'}
        items={vmMetrics}
        lines={[
          { key: 'cpu', name: 'CPU', color: '#3b82f6' },
          { key: 'memory', name: '内存', color: '#f59e0b' },
          { key: 'disk', name: '磁盘', color: '#10b981' },
        ]}
      />

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-3">
        <HostResourceCard
          hosts={totals.onlineHosts}
          selectedId={selectedHostId}
          onSelect={setSelectedHostId}
        />
        <HostStateCard hosts={hosts} />
      </div>
      <ResourceTrendChart
        title="宿主机趋势"
        subtitle={selectedHost ? `最近 1 小时 · ${selectedHost.name}` : '暂无在线宿主机可查看'}
        items={hostMetrics}
        lines={[
          { key: 'cpu', name: 'CPU', color: '#3b82f6' },
          { key: 'memory', name: '内存', color: '#f59e0b' },
          { key: 'storage', name: '存储', color: '#10b981' },
        ]}
      />

      <section
        className="rounded-xl p-5"
        style={{ background: 'var(--kvm-card)', border: '1px solid var(--kvm-border)' }}
      >
        <div className="mb-5 flex items-center justify-between">
          <h2 className="text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
            存储池
          </h2>
          <HardDriveIcon size={18} style={{ color: '#3b82f6' }} />
        </div>
        <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
          {hosts.length === 0 && (
            <div
              className="kvm-empty-state rounded-xl p-6 text-center text-sm md:col-span-3"
              style={{ color: 'var(--kvm-text-muted)' }}
            >
              暂无宿主机数据
            </div>
          )}
          {hosts.map(host => (
            <div
              key={host.id}
              className="rounded-lg p-4"
              style={{
                background: 'rgba(255,255,255,0.04)',
                border: '1px solid var(--kvm-border)',
              }}
            >
              <div className="mb-3 flex items-baseline justify-between">
                <span className="text-sm font-medium" style={{ color: 'var(--kvm-text)' }}>
                  {host.name}
                </span>
                <span className="font-mono text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                  {formatBytes(host.storageBytes, 'GB')}
                </span>
              </div>
              <MetricBar value={host.storageUsage} color={storageUsageColor(host.storageUsage)} />
              <div className="mt-2 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                {host.storageUsage}% 已用
              </div>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}

function isPermissionMessage(message: string) {
  return message.includes('当前用户无权执行此操作');
}

function VMStateCard({ data }: { data: DashboardSummary }) {
  const items = [
    { label: '运行中', count: data.runningVMs, color: '#10b981' },
    { label: '已停止', count: data.stoppedVMs, color: '#94a3b8' },
    { label: '已暂停', count: data.pausedVMs, color: '#f59e0b' },
    { label: '异常', count: data.errorVMs, color: '#ef4444' },
  ];

  return (
    <section
      className="rounded-xl p-5"
      style={{ background: 'var(--kvm-card)', border: '1px solid var(--kvm-border)' }}
    >
      <div className="mb-5 flex items-center justify-between">
        <div>
          <h2 className="text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
            虚拟机状态分布
          </h2>
          <p className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
            共 {data.totalVMs} 台
          </p>
        </div>
        <BoxesIcon size={18} style={{ color: '#10b981' }} />
      </div>
      <StatusDonutChart total={data.totalVMs} items={items} />
    </section>
  );
}

function VMResourceCard({
  vms,
  selectedId,
  onSelect,
}: {
  vms: VirtualMachine[];
  selectedId: string;
  onSelect: (id: string) => void;
}) {
  return (
    <section
      className="rounded-xl p-5 xl:col-span-2"
      style={{ background: 'var(--kvm-card)', border: '1px solid var(--kvm-border)' }}
    >
      <ResourceHeader
        title="在线虚拟机资源利用率"
        subtitle="选择虚拟机查看最近 1 小时趋势"
        value={selectedId}
        onChange={onSelect}
        items={vms.map(vm => ({ id: vm.id, name: vm.name }))}
      />
      <div className="kvm-hidden-scrollbar max-h-[12.5rem] space-y-4 overflow-y-auto pr-1">
        {vms.length === 0 && (
          <div
            className="kvm-empty-state rounded-xl p-6 text-center text-sm"
            style={{ color: 'var(--kvm-text-muted)' }}
          >
            暂无运行中的虚拟机
          </div>
        )}
        {vms.map(vm => (
          <ResourceUsageRow
            key={vm.id}
            name={vm.name}
            meta={vm.primaryIp || vm.hostName}
            cpu={vm.cpuUsage}
            memory={vm.memoryUsage}
            storage={vm.diskUsage}
            active={vm.id === selectedId}
            onClick={() => onSelect(vm.id)}
          />
        ))}
      </div>
    </section>
  );
}

function HostResourceCard({
  hosts,
  selectedId,
  onSelect,
}: {
  hosts: Host[];
  selectedId: string;
  onSelect: (id: string) => void;
}) {
  return (
    <section
      className="rounded-xl p-5 xl:col-span-2"
      style={{ background: 'var(--kvm-card)', border: '1px solid var(--kvm-border)' }}
    >
      <ResourceHeader
        title="在线宿主机资源利用率"
        subtitle="选择宿主机查看最近 1 小时趋势"
        value={selectedId}
        onChange={onSelect}
        items={hosts.map(host => ({ id: host.id, name: host.name }))}
      />
      <div className="kvm-hidden-scrollbar max-h-[12.5rem] space-y-4 overflow-y-auto pr-1">
        {hosts.length === 0 && (
          <div
            className="kvm-empty-state rounded-xl p-6 text-center text-sm"
            style={{ color: 'var(--kvm-text-muted)' }}
          >
            暂无在线宿主机
          </div>
        )}
        {hosts.map(host => (
          <ResourceUsageRow
            key={host.id}
            name={host.name}
            meta={host.hostname || host.address}
            cpu={host.cpuUsage}
            memory={host.memoryUsage}
            storage={host.storageUsage}
            active={host.id === selectedId}
            onClick={() => onSelect(host.id)}
          />
        ))}
      </div>
    </section>
  );
}

function HostStateCard({ hosts }: { hosts: Host[] }) {
  const total = hosts.length;
  const counts = {
    online: hosts.filter(host => host.status === 'online').length,
    maintenance: hosts.filter(host => host.status === 'maintenance').length,
    degraded: hosts.filter(host => host.status === 'degraded').length,
    offline: hosts.filter(host => host.status === 'offline').length,
  };
  const items = [
    { label: '在线', count: counts.online, color: '#10b981' },
    { label: '维护中', count: counts.maintenance, color: '#f59e0b' },
    { label: '降级', count: counts.degraded, color: '#f97316' },
    { label: '离线', count: counts.offline, color: '#94a3b8' },
  ];

  return (
    <section
      className="rounded-xl p-5"
      style={{ background: 'var(--kvm-card)', border: '1px solid var(--kvm-border)' }}
    >
      <div className="mb-5 flex items-center justify-between">
        <div>
          <h2 className="text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
            宿主机状态分布
          </h2>
          <p className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
            共 {total} 台
          </p>
        </div>
        <ServerIcon size={18} style={{ color: '#3b82f6' }} />
      </div>
      <StatusDonutChart total={total} items={items} />
    </section>
  );
}

function ResourceHeader({
  title,
  subtitle,
  value,
  onChange,
  items,
}: {
  title: string;
  subtitle: string;
  value: string;
  onChange: (id: string) => void;
  items: Array<{ id: string; name: string }>;
}) {
  return (
    <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 className="text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
          {title}
        </h2>
        <p className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
          {subtitle}
        </p>
      </div>
      <ResourceSelect value={value} onChange={onChange} items={items} />
    </div>
  );
}

function ResourceSelect({
  value,
  onChange,
  items,
}: {
  value: string;
  onChange: (id: string) => void;
  items: Array<{ id: string; name: string }>;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);
  const current = items.find(item => item.id === value);
  const disabled = items.length === 0;

  useEffect(() => {
    const close = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    window.addEventListener('mousedown', close);
    return () => window.removeEventListener('mousedown', close);
  }, []);

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        disabled={disabled}
        onClick={() => setOpen(next => !next)}
        className="kvm-action-button flex h-10 w-44 cursor-pointer items-center justify-between gap-3 rounded-lg border px-3 text-left text-sm transition disabled:cursor-not-allowed disabled:opacity-60"
        style={{
          background: 'rgba(255,255,255,0.045)',
          borderColor: open ? 'rgba(45,212,191,0.45)' : 'var(--kvm-border)',
          color: 'var(--kvm-text)',
          boxShadow: open ? '0 0 0 3px rgba(45,212,191,0.08)' : 'none',
        }}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className="truncate font-semibold">{current?.name ?? '暂无可选资源'}</span>
        <ChevronDownIcon
          size={15}
          className={
            open ? 'shrink-0 rotate-180 transition-transform' : 'shrink-0 transition-transform'
          }
          style={{ color: 'var(--kvm-text-muted)' }}
        />
      </button>
      {open && !disabled && (
        <div
          className="kvm-hidden-scrollbar absolute right-0 top-[calc(100%+8px)] z-30 max-h-50 w-44 overflow-y-auto rounded-lg border p-1 shadow-2xl"
          role="listbox"
          style={{
            background: 'var(--kvm-menu-bg)',
            borderColor: 'var(--kvm-popover-border)',
            boxShadow: 'var(--kvm-menu-shadow)',
          }}
        >
          {items.map(item => {
            const active = item.id === value;
            return (
              <button
                key={item.id}
                type="button"
                role="option"
                aria-selected={active}
                onClick={() => {
                  onChange(item.id);
                  setOpen(false);
                }}
                className="group flex h-10 w-full cursor-pointer items-center justify-between gap-3 rounded-md px-3 text-left text-sm font-semibold transition-colors hover:bg-[rgba(45,212,191,0.1)]"
                style={{
                  background: active ? 'rgba(45,212,191,0.14)' : undefined,
                  color: active ? '#99f6e4' : 'var(--kvm-text)',
                }}
              >
                <span className="truncate">{item.name}</span>
                {active && <CheckIcon size={15} className="shrink-0" />}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

function ResourceUsageRow({
  name,
  meta,
  cpu,
  memory,
  storage,
  active,
  onClick,
}: {
  name: string;
  meta: string;
  cpu: number;
  memory: number;
  storage: number;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="kvm-action-button block w-full rounded-lg border p-3 text-left"
      style={{
        background: active ? 'rgba(59,130,246,0.08)' : 'rgba(255,255,255,0.02)',
        borderColor: active ? 'rgba(59,130,246,0.35)' : 'transparent',
      }}
    >
      <div className="mb-3 flex items-center justify-between text-sm">
        <span className="font-medium" style={{ color: 'var(--kvm-text)' }}>
          {name}
        </span>
        <span className="font-mono text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
          {meta}
        </span>
      </div>
      <div className="grid grid-cols-3 gap-3">
        <MetricBar value={cpu} label="CPU" />
        <MetricBar value={memory} label="内存" />
        <MetricBar value={storage} label="磁盘" />
      </div>
    </button>
  );
}
