import { metricUsageColor } from '../../../lib/resourceThresholds';

export function storageUsageColor(usage: number) {
  return metricUsageColor(usage);
}
