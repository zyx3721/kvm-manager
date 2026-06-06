import { getBaseConfigSnapshot } from './branding';
import type { SystemBaseConfig } from './api';

type ThresholdConfig = Pick<
  SystemBaseConfig,
  'resourceWarningThreshold' | 'resourceCriticalThreshold'
>;

export function metricUsageColor(value: number, config: ThresholdConfig = getBaseConfigSnapshot()) {
  const warning = config.resourceWarningThreshold;
  const critical = config.resourceCriticalThreshold;
  if (value >= critical) return '#ef4444';
  if (value >= warning) return '#f59e0b';
  return '#3b82f6';
}
