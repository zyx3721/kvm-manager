export function sumBy<T>(items: T[], selector: (item: T) => number) {
  return items.reduce((total, item) => total + selector(item), 0);
}

export function fallbackPercent(realtime: number, used: number, total: number) {
  if (realtime > 0 || total <= 0) return clampPercent(realtime);
  return clampPercent(Math.round((used / total) * 100));
}

function clampPercent(value: number) {
  if (!Number.isFinite(value)) return 0;
  return Math.min(100, Math.max(0, value));
}
