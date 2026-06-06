const activeCloneTaskIds = new Set<string>();
let suppressCloneUntil = 0;
const suppressAfterFinishMs = 15000;

export function registerVMCloneTask(taskId: string) {
  if (!taskId) return;
  activeCloneTaskIds.add(taskId);
}

export function unregisterVMCloneTask(taskId: string) {
  if (!taskId) return;
  activeCloneTaskIds.delete(taskId);
  suppressCloneUntil = Date.now() + suppressAfterFinishMs;
}

export function hasActiveVMCloneTask() {
  return activeCloneTaskIds.size > 0 || Date.now() < suppressCloneUntil;
}
