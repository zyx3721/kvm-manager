import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIcon, CheckIcon, ChevronDownIcon, RefreshCwIcon, XIcon } from 'lucide-react';
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { fetchHostMetricSeries, type Host, type MetricPoint } from '../../lib/api';
import {
  buildTimeTicks,
  formatTimeTick,
  formatTooltipTime,
  getChartWindow,
  type TimeWindow,
} from './metricTimeAxis';
import { DialogPortal } from './DialogPortal';

type RangeKey = '1h' | '24h' | '7d' | '30d' | 'custom';

type ChartPoint = MetricPoint & {
  label: string;
  timestamp: number;
  diskReadKB: number;
  diskWriteKB: number;
  networkRxKB: number;
  networkTxKB: number;
  networkAvgKB: number;
};

type MonitorCard = {
  title: string;
  unit: string;
  domain?: [number, number];
  lines: Array<{ key: keyof ChartPoint; name: string; color: string }>;
};

const ranges: Array<{ value: RangeKey; label: string }> = [
  { value: '1h', label: '1 小时' },
  { value: '24h', label: '24 小时' },
  { value: '7d', label: '7 天' },
  { value: '30d', label: '30 天' },
  { value: 'custom', label: '自定义' },
];

const cards: MonitorCard[] = [
  {
    title: 'CPU 占用率',
    unit: '%',
    domain: [0, 100],
    lines: [{ key: 'cpu', name: 'CPU', color: '#2dd4bf' }],
  },
  {
    title: '内存占用率',
    unit: '%',
    domain: [0, 100],
    lines: [{ key: 'memory', name: '内存', color: '#60a5fa' }],
  },
  {
    title: '逻辑磁盘占用率',
    unit: '%',
    domain: [0, 100],
    lines: [{ key: 'storage', name: '逻辑磁盘', color: '#34d399' }],
  },
  {
    title: '磁盘 I/O',
    unit: 'KB/s',
    lines: [
      { key: 'diskReadKB', name: '读取', color: '#2dd4bf' },
      { key: 'diskWriteKB', name: '写入', color: '#facc15' },
    ],
  },
  {
    title: '网络吞吐量',
    unit: 'KB/s',
    lines: [
      { key: 'networkRxKB', name: '流入', color: '#22d3ee' },
      { key: 'networkTxKB', name: '流出', color: '#facc15' },
      { key: 'networkAvgKB', name: '平均带宽', color: '#2dd4bf' },
    ],
  },
];

export function HostTrendDialog({ host, onClose }: { host: Host; onClose: () => void }) {
  const [range, setRange] = useState<RangeKey>('1h');
  const [customStart, setCustomStart] = useState(() =>
    toLocalInputValue(new Date(Date.now() - 60 * 60 * 1000))
  );
  const [customEnd, setCustomEnd] = useState(() => toLocalInputValue(new Date()));
  const [windowEnd, setWindowEnd] = useState(() => new Date());
  const [items, setItems] = useState<MetricPoint[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      setWindowEnd(new Date());
      if (
        range === 'custom' &&
        (!customStart || !customEnd || new Date(customStart) >= new Date(customEnd))
      ) {
        setItems([]);
        setError('请选择有效的开始和结束时间');
        return;
      }
      const response = await fetchHostMetricSeries(
        host.id,
        range,
        range === 'custom'
          ? { start: toISOFromLocalInput(customStart), end: toISOFromLocalInput(customEnd) }
          : undefined
      );
      setItems(response.items);
    } catch (err) {
      setError(err instanceof Error ? err.message : '读取宿主机监控数据失败');
    } finally {
      setLoading(false);
    }
  }, [customEnd, customStart, host.id, range]);

  useEffect(() => {
    void load();
  }, [load]);

  const data = useMemo(
    () =>
      items.map(item => ({
        ...item,
        label: formatLabel(item.time, range),
        timestamp: new Date(item.time).getTime(),
        diskReadKB: bytesToKB(item.diskReadBytesPerSecond),
        diskWriteKB: bytesToKB(item.diskWriteBytesPerSecond),
        networkRxKB: bytesToKB(item.networkRxBytesPerSecond),
        networkTxKB: bytesToKB(item.networkTxBytesPerSecond),
        networkAvgKB: totalKB(item.networkRxBytesPerSecond, item.networkTxBytesPerSecond),
      })),
    [items, range]
  );
  const chartWindow = useMemo(
    () => getChartWindow(range, customStart, customEnd, windowEnd),
    [customEnd, customStart, range, windowEnd]
  );

  return (
    <DialogPortal>
      <div className="kvm-dialog-backdrop fixed inset-0 z-50 flex items-center justify-center px-4 py-6">
        <div className="kvm-dialog-panel flex max-h-[92vh] w-[96vw] max-w-[1680px] flex-col overflow-hidden rounded-xl shadow-2xl">
          <div
            className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b px-5 py-4"
            style={{ borderColor: 'var(--kvm-border)' }}
          >
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <ActivityIcon size={17} style={{ color: '#2dd4bf' }} />
                <h2 className="text-base font-semibold" style={{ color: 'var(--kvm-text)' }}>
                  {host.name} 监控
                </h2>
              </div>
              <p className="mt-1 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
                {host.hostname || host.address}
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <RangePicker value={range} onChange={setRange} />
              {range === 'custom' && (
                <CustomRangeInputs
                  start={customStart}
                  end={customEnd}
                  onStartChange={setCustomStart}
                  onEndChange={setCustomEnd}
                />
              )}
              <button
                type="button"
                onClick={() => void load()}
                disabled={loading}
                className="kvm-action-button flex h-10 w-10 items-center justify-center rounded-lg border disabled:opacity-50"
                style={{
                  background: 'var(--kvm-control-bg)',
                  borderColor: 'var(--kvm-border)',
                  color: 'var(--kvm-text-muted)',
                }}
                aria-label="刷新监控"
              >
                <RefreshCwIcon size={15} className={loading ? 'animate-spin' : ''} />
              </button>
              <button
                type="button"
                onClick={onClose}
                className="kvm-action-button flex h-10 w-10 items-center justify-center rounded-lg border"
                style={{
                  background: 'var(--kvm-control-bg)',
                  borderColor: 'var(--kvm-border)',
                  color: 'var(--kvm-text-muted)',
                }}
                aria-label="关闭监控"
              >
                <XIcon size={15} />
              </button>
            </div>
          </div>
          <div className="kvm-hidden-scrollbar overflow-y-auto p-5">
            {error && (
              <div
                className="mb-4 rounded-lg p-3 text-sm"
                style={{
                  background: 'rgba(245,158,11,0.1)',
                  border: '1px solid rgba(245,158,11,0.25)',
                  color: '#f59e0b',
                }}
              >
                {error}
              </div>
            )}
            <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
              {cards.map((card, index) => (
                <MonitorChartCard
                  key={card.title}
                  card={card}
                  data={data}
                  window={chartWindow}
                  wide={index === cards.length - 1}
                />
              ))}
            </div>
          </div>
        </div>
      </div>
    </DialogPortal>
  );
}

function RangePicker({
  value,
  onChange,
}: {
  value: RangeKey;
  onChange: (value: RangeKey) => void;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);
  const current = ranges.find(item => item.value === value) ?? ranges[0];

  useEffect(() => {
    const close = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    window.addEventListener('mousedown', close);
    return () => window.removeEventListener('mousedown', close);
  }, []);

  return (
    <div ref={rootRef} className="relative flex items-center gap-2">
      <span className="shrink-0 text-xs" style={{ color: 'var(--kvm-text-muted)' }}>
        时间范围
      </span>
      <button
        type="button"
        onClick={() => setOpen(next => !next)}
        className="kvm-action-button flex h-10 w-36 cursor-pointer items-center justify-between gap-3 rounded-lg border px-3 text-left"
        style={{
          background: 'rgba(255,255,255,0.045)',
          borderColor: open ? 'rgba(45,212,191,0.45)' : 'var(--kvm-border)',
          color: 'var(--kvm-text)',
          boxShadow: open ? '0 0 0 3px rgba(45,212,191,0.08)' : 'none',
        }}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className="truncate text-sm font-semibold">{current.label}</span>
        <ChevronDownIcon
          size={15}
          className={open ? 'rotate-180 transition-transform' : 'transition-transform'}
          style={{ color: 'var(--kvm-text-muted)' }}
        />
      </button>
      {open && (
        <div
          className="absolute left-[calc(100%-9rem)] top-[calc(100%+8px)] z-[60] w-36 overflow-hidden rounded-lg border p-1 shadow-2xl"
          role="listbox"
          style={{
            background: 'var(--kvm-menu-bg)',
            borderColor: 'var(--kvm-popover-border)',
            boxShadow: 'var(--kvm-menu-shadow)',
          }}
        >
          {ranges.map(item => {
            const active = item.value === value;
            return (
              <button
                key={item.value}
                type="button"
                role="option"
                aria-selected={active}
                onClick={() => {
                  onChange(item.value);
                  setOpen(false);
                }}
                className="group flex h-10 w-full cursor-pointer items-center justify-between gap-3 rounded-md px-3 text-left text-sm font-semibold transition-colors hover:bg-[rgba(45,212,191,0.1)]"
                style={{
                  background: active ? 'rgba(45,212,191,0.14)' : undefined,
                  color: active ? '#99f6e4' : 'var(--kvm-text)',
                }}
              >
                <span>{item.label}</span>
                {active && <CheckIcon size={15} />}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

function CustomRangeInputs({
  start,
  end,
  onStartChange,
  onEndChange,
}: {
  start: string;
  end: string;
  onStartChange: (value: string) => void;
  onEndChange: (value: string) => void;
}) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <label
        className="flex h-10 items-center gap-2 rounded-lg border px-3 text-xs"
        style={{
          background: 'var(--kvm-control-bg)',
          borderColor: 'var(--kvm-border)',
          color: 'var(--kvm-text-muted)',
        }}
      >
        开始
        <input
          type="datetime-local"
          value={start}
          onChange={event => onStartChange(event.target.value)}
          className="w-[180px] bg-transparent text-sm outline-none"
          style={{ color: 'var(--kvm-text)', colorScheme: 'var(--kvm-color-scheme)' }}
        />
      </label>
      <label
        className="flex h-10 items-center gap-2 rounded-lg border px-3 text-xs"
        style={{
          background: 'var(--kvm-control-bg)',
          borderColor: 'var(--kvm-border)',
          color: 'var(--kvm-text-muted)',
        }}
      >
        结束
        <input
          type="datetime-local"
          value={end}
          onChange={event => onEndChange(event.target.value)}
          className="w-[180px] bg-transparent text-sm outline-none"
          style={{ color: 'var(--kvm-text)', colorScheme: 'var(--kvm-color-scheme)' }}
        />
      </label>
    </div>
  );
}

function MonitorChartCard({
  card,
  data,
  window,
  wide,
}: {
  card: MonitorCard;
  data: ChartPoint[];
  window: TimeWindow;
  wide?: boolean;
}) {
  const ticks = useMemo(() => buildTimeTicks(window, wide), [wide, window]);
  return (
    <section className={(wide ? 'xl:col-span-2 ' : '') + 'kvm-dialog-card rounded-lg p-4'}>
      <div className="mb-3 flex items-center justify-between gap-3">
        <h3 className="text-sm font-semibold" style={{ color: 'var(--kvm-text)' }}>
          {card.title}
        </h3>
        <div
          className="flex flex-wrap justify-end gap-3 text-xs"
          style={{ color: 'var(--kvm-text-muted)' }}
        >
          {card.lines.map(line => (
            <span key={String(line.key)} className="flex items-center gap-1">
              <span className="h-2.5 w-2.5 rounded-full" style={{ background: line.color }} />
              {line.name}
            </span>
          ))}
        </div>
      </div>
      <div className="mb-1 text-xs font-semibold" style={{ color: 'var(--kvm-text-muted)' }}>
        单位 {card.unit}
      </div>
      <div className="h-56 w-full">
        {data.length === 0 ? (
          <div
            className="flex h-full items-center justify-center text-sm"
            style={{ color: 'var(--kvm-text-muted)' }}
          >
            暂无监控数据
          </div>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={data} margin={{ left: 0, right: 8, top: 8, bottom: 0 }}>
              <CartesianGrid vertical={false} stroke="var(--kvm-border)" strokeOpacity={0.42} />
              <XAxis
                dataKey="timestamp"
                type="number"
                domain={[window.start.getTime(), window.end.getTime()]}
                ticks={ticks}
                tickFormatter={value => formatTimeTick(value, window)}
                tick={{ fill: 'var(--kvm-text-muted)', fontSize: 12 }}
                axisLine={false}
                tickLine={{ stroke: 'var(--kvm-border)', strokeOpacity: 0.7 }}
                interval={0}
                minTickGap={0}
                tickMargin={8}
                allowDataOverflow
              />
              <YAxis
                domain={card.domain}
                tick={{ fill: 'var(--kvm-text-muted)', fontSize: 12 }}
                axisLine={false}
                tickLine={false}
                width={42}
              />
              <Tooltip
                labelFormatter={value => formatTooltipTime(value)}
                formatter={(value, name) => [`${formatMetricValue(value)} ${card.unit}`, name]}
                contentStyle={{
                  background: 'var(--kvm-menu-bg)',
                  border: '1px solid var(--kvm-border)',
                  borderRadius: 8,
                  color: 'var(--kvm-text)',
                }}
                labelStyle={{ color: 'var(--kvm-text-muted)' }}
              />
              {card.lines.map(line => (
                <Line
                  key={String(line.key)}
                  type="monotone"
                  dataKey={line.key as string}
                  name={line.name}
                  stroke={line.color}
                  strokeWidth={2}
                  dot={false}
                />
              ))}
            </LineChart>
          </ResponsiveContainer>
        )}
      </div>
    </section>
  );
}

function formatLabel(value: string, range: RangeKey) {
  const date = new Date(value);
  if (range === '1h' || range === '24h' || range === 'custom')
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  return date.toLocaleDateString([], { month: '2-digit', day: '2-digit' });
}

function toISOFromLocalInput(value: string) {
  return new Date(value).toISOString();
}

function toLocalInputValue(date: Date) {
  const offset = date.getTimezoneOffset() * 60000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

function bytesToKB(value?: number) {
  if (!value || value < 0) return 0;
  return Number((value / 1024).toFixed(2));
}

function totalKB(rx?: number, tx?: number) {
  return Number((bytesToKB(rx) + bytesToKB(tx)).toFixed(2));
}

function formatMetricValue(value: unknown) {
  if (typeof value !== 'number') return value;
  if (!Number.isFinite(value)) return 0;
  return Number.isInteger(value) ? value : Number(value.toFixed(2));
}
