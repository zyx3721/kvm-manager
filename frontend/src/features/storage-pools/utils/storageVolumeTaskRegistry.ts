type StorageVolumeOperation = 'upload' | 'clone';

const activeTaskIds = new Set<string>();
const activeOperationCounts = new Map<StorageVolumeOperation, number>();
const suppressUntil = new Map<StorageVolumeOperation, number>();
const suppressAfterFinishMs = 15000;

export function registerStorageVolumeTask(taskId: string, operation: StorageVolumeOperation) {
  if (!taskId) return;
  activeTaskIds.add(taskId);
  activeOperationCounts.set(operation, (activeOperationCounts.get(operation) ?? 0) + 1);
}

export function unregisterStorageVolumeTask(taskId: string, operation: StorageVolumeOperation) {
  if (!taskId) return;
  activeTaskIds.delete(taskId);
  const nextCount = Math.max(0, (activeOperationCounts.get(operation) ?? 0) - 1);
  if (nextCount === 0) activeOperationCounts.delete(operation);
  else activeOperationCounts.set(operation, nextCount);
  suppressUntil.set(operation, Date.now() + suppressAfterFinishMs);
}

export function hasActiveStorageVolumeTask(operation?: string) {
  if (!operation) return activeTaskIds.size > 0;
  const target = operation as StorageVolumeOperation;
  return (
    (activeOperationCounts.get(target) ?? 0) > 0 || Date.now() < (suppressUntil.get(target) ?? 0)
  );
}
