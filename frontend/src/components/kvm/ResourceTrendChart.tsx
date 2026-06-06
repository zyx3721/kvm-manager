import { useMemo } from "react";
import { ActivityIcon } from "lucide-react";
import { Area, AreaChart, CartesianGrid, Line, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { MetricPoint } from "../../lib/api";

type TrendLine = {
  key: "cpu" | "memory" | "storage" | "disk";
  name: string;
  color: string;
};

type TrendTooltipPayload = {
  color?: string;
  dataKey?: string;
  name?: string;
  value?: unknown;
};

export function ResourceTrendChart({ title, subtitle, items, lines }: { title: string; subtitle: string; items: MetricPoint[]; lines: TrendLine[] }) {
  const end = Date.now();
  const start = end - 60 * 60 * 1000;
  const ticks = buildHourTicks(start, end);
  const data = useMemo(() => items.map((item) => ({
    ...item,
    timestamp: new Date(item.time).getTime(),
  })), [items]);

  return (
    <section className="rounded-xl p-5" style={{ background: "var(--kvm-card)", border: "1px solid var(--kvm-border)" }}>
      <div className="mb-5 flex items-center justify-between">
        <div>
          <h2 className="text-sm font-semibold" style={{ color: "var(--kvm-text)" }}>{title}</h2>
          <p className="mt-1 text-xs" style={{ color: "var(--kvm-text-muted)" }}>{subtitle}</p>
        </div>
        <ActivityIcon size={18} style={{ color: "#06b6d4" }} />
      </div>
      <div className="h-64 w-full">
        {data.length === 0 ? (
          <div className="flex h-full items-center justify-center text-sm" style={{ color: "var(--kvm-text-muted)" }}>暂无趋势数据</div>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={data} margin={{ left: 0, right: 8, top: 8, bottom: 0 }}>
              <defs>
                {lines.map((line) => (
                  <linearGradient key={`${line.key}-area`} id={`trend-area-${line.key}`} x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor={line.color} stopOpacity={0.16} />
                    <stop offset="100%" stopColor={line.color} stopOpacity={0.04} />
                  </linearGradient>
                ))}
                {lines.map((line) => (
                  <filter key={`${line.key}-shadow`} id={`trend-shadow-${line.key}`} x="-20%" y="-20%" width="140%" height="140%">
                    <feDropShadow dx="0" dy="3" stdDeviation="2.2" floodColor={line.color} floodOpacity="0.28" />
                  </filter>
                ))}
              </defs>
              <CartesianGrid vertical={false} stroke="var(--kvm-border)" strokeOpacity={0.38} />
              <XAxis dataKey="timestamp" type="number" domain={[start, end]} ticks={ticks} tickFormatter={formatTick} tick={{ fill: "var(--kvm-text-muted)", fontSize: 12 }} axisLine={false} tickLine={{ stroke: "var(--kvm-border)", strokeOpacity: 0.7 }} interval={0} minTickGap={0} tickMargin={8} allowDataOverflow />
              <YAxis domain={[0, 100]} tick={{ fill: "var(--kvm-text-muted)", fontSize: 12 }} axisLine={false} tickLine={false} width={30} />
              <Tooltip content={<TrendTooltip lines={lines} />} />
              {lines.map((line) => (
                <Area
                  key={`${line.key}-area`}
                  type="monotone"
                  dataKey={line.key}
                  name={line.name}
                  stroke="none"
                  fill={`url(#trend-area-${line.key})`}
                  fillOpacity={1}
                  dot={false}
                  activeDot={false}
                  connectNulls
                  isAnimationActive={false}
                />
              ))}
              {lines.map((line) => (
                <Line
                  key={`${line.key}-line`}
                  type="monotone"
                  dataKey={line.key}
                  name={line.name}
                  stroke={line.color}
                  strokeWidth={2}
                  dot={false}
                  activeDot={{ r: 3.5, stroke: line.color, strokeWidth: 2, fill: "var(--kvm-card)" }}
                  connectNulls
                  style={{ filter: `url(#trend-shadow-${line.key})` }}
                />
              ))}
            </AreaChart>
          </ResponsiveContainer>
        )}
      </div>
    </section>
  );
}

function TrendTooltip({ active, label, payload, lines }: { active?: boolean; label?: unknown; payload?: TrendTooltipPayload[]; lines: TrendLine[] }) {
  if (!active || !payload?.length) return null;
  const seen = new Set<string>();
  const items = payload.filter((item) => {
    const key = String(item.dataKey ?? item.name ?? "");
    if (!key || seen.has(key)) return false;
    seen.add(key);
    return true;
  });

  return (
    <div className="rounded-lg px-3 py-2 text-sm shadow-2xl" style={{ background: "var(--kvm-popover-bg)", border: "1px solid var(--kvm-popover-border)", color: "var(--kvm-text)", boxShadow: "var(--kvm-menu-shadow)" }}>
      <div className="mb-2 text-xs" style={{ color: "var(--kvm-text-muted)" }}>{formatTooltipLabel(label)}</div>
      <div className="space-y-1.5">
        {items.map((item) => (
          <div key={String(item.dataKey ?? item.name)} className="flex items-center justify-between gap-5">
            <span className="flex items-center gap-2">
              <span className="h-2.5 w-2.5 rounded-full" style={{ background: lineColor(lines, item), boxShadow: `0 0 10px ${lineColor(lines, item)}66` }} />
              <span>{item.name}</span>
            </span>
            <span className="font-mono tabular-nums">{formatPercentValue(item.value)}%</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function lineColor(lines: TrendLine[], item: TrendTooltipPayload) {
  const key = String(item.dataKey ?? "");
  return lines.find((line) => line.key === key)?.color || item.color || "var(--kvm-accent)";
}

function buildHourTicks(start: number, end: number) {
  const interval = 5 * 60 * 1000;
  const first = Math.ceil(start / interval) * interval;
  const ticks: number[] = [];
  for (let value = first; value <= end; value += interval) ticks.push(value);
  return ticks;
}

function formatTick(value: number | string) {
  const date = new Date(Number(value));
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

function formatTooltipLabel(value: unknown) {
  const date = new Date(Number(value));
  if (Number.isNaN(date.getTime())) return String(value);
  return date.toLocaleString([], { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" });
}

function formatPercentValue(value: unknown) {
  if (typeof value !== "number") return "-";
  if (!Number.isFinite(value)) return 0;
  return Number.isInteger(value) ? value : Number(value.toFixed(2));
}
