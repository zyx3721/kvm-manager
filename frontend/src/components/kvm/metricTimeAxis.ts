export type MetricRangeKey = '1h' | '24h' | '7d' | '30d' | 'custom';

export type TimeWindow = {
  start: Date;
  end: Date;
  range: MetricRangeKey;
};

export function getChartWindow(
  range: MetricRangeKey,
  customStart: string,
  customEnd: string,
  now: Date
): TimeWindow {
  if (
    range === 'custom' &&
    customStart &&
    customEnd &&
    new Date(customStart) < new Date(customEnd)
  ) {
    return { start: new Date(customStart), end: new Date(customEnd), range };
  }
  const end = now;
  return { start: new Date(end.getTime() - rangeDurationMs(range)), end, range };
}

export function buildTimeTicks(window: TimeWindow, wide = false) {
  switch (window.range) {
    case '1h':
      return buildIntervalTicks(window.start, window.end, (wide ? 2 : 4) * 60 * 1000);
    case '24h':
      return buildIntervalTicks(window.start, window.end, (wide ? 60 : 90) * 60 * 1000);
    case '7d':
      return wide
        ? buildDailyHourTicks(window.start, window.end, [3, 7, 11, 15, 19, 23])
        : buildDailyHourTicks(window.start, window.end, [7, 15, 23]);
    case '30d':
      return buildIntervalTicks(
        startOfDay(window.start),
        window.end,
        (wide ? 2 : 4) * 24 * 60 * 60 * 1000
      );
    default:
      return buildCustomTicks(window.start, window.end);
  }
}

export function formatTimeTick(value: number | string, window: TimeWindow) {
  const date = new Date(Number(value));
  if (Number.isNaN(date.getTime())) return '';
  if (window.range === '24h' || window.range === '7d')
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  if (window.range === '30d')
    return date.toLocaleDateString([], { month: '2-digit', day: '2-digit' });
  const sameDay = window.start.toDateString() === window.end.toDateString();
  if (sameDay) return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  return date.toLocaleString([], {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function formatTooltipTime(value: unknown) {
  const date = new Date(Number(value));
  if (Number.isNaN(date.getTime())) return String(value);
  return date.toLocaleString([], {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function rangeDurationMs(range: MetricRangeKey) {
  switch (range) {
    case '24h':
      return 24 * 60 * 60 * 1000;
    case '7d':
      return 7 * 24 * 60 * 60 * 1000;
    case '30d':
      return 30 * 24 * 60 * 60 * 1000;
    default:
      return 60 * 60 * 1000;
  }
}

function buildIntervalTicks(start: Date, end: Date, intervalMs: number) {
  const startMs = Math.ceil(start.getTime() / intervalMs) * intervalMs;
  return buildTicks(startMs, end.getTime(), intervalMs);
}

function buildDailyHourTicks(start: Date, end: Date, hours: number[]) {
  const ticks: number[] = [];
  const cursor = startOfDay(start);
  while (cursor <= end) {
    for (const hour of hours) {
      const tick = new Date(cursor);
      tick.setHours(hour, 0, 0, 0);
      if (tick >= start && tick <= end) ticks.push(tick.getTime());
    }
    cursor.setDate(cursor.getDate() + 1);
  }
  return ticks;
}

function buildCustomTicks(start: Date, end: Date) {
  const span = end.getTime() - start.getTime();
  if (span <= 0) return [start.getTime(), end.getTime()];
  const interval = niceInterval(span / 8);
  return buildIntervalTicks(start, end, interval);
}

function buildTicks(startMs: number, endMs: number, intervalMs: number) {
  const ticks: number[] = [];
  for (let value = startMs; value <= endMs; value += intervalMs) {
    ticks.push(value);
  }
  if (ticks.length === 0) return [startMs, endMs];
  return ticks;
}

function niceInterval(targetMs: number) {
  const candidates = [5, 15, 30, 60, 2 * 60, 6 * 60, 12 * 60, 24 * 60].map(
    minutes => minutes * 60 * 1000
  );
  return candidates.find(candidate => candidate >= targetMs) ?? 24 * 60 * 60 * 1000;
}

function startOfDay(date: Date) {
  const value = new Date(date);
  value.setHours(0, 0, 0, 0);
  return value;
}
