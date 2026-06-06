const activeMigrateTaskIds = new Set<string>();
let suppressMigrateUntil = 0;
const suppressAfterFinishMs = 15000;

export function registerVMMigrateTask(taskId: string) {
  if (!taskId) return;
  activeMigrateTaskIds.add(taskId);
}

export function unregisterVMMigrateTask(taskId: string) {
  if (!taskId) return;
  activeMigrateTaskIds.delete(taskId);
  suppressMigrateUntil = Date.now() + suppressAfterFinishMs;
}

export function hasActiveVMMigrateTask() {
  return activeMigrateTaskIds.size > 0 || Date.now() < suppressMigrateUntil;
}
