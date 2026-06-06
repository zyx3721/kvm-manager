const activeCreateTaskIds = new Set<string>();
let suppressCreateUntil = 0;
const suppressAfterFinishMs = 15000;

export function registerVMCreateTask(taskId: string) {
  if (!taskId) return;
  activeCreateTaskIds.add(taskId);
}

export function unregisterVMCreateTask(taskId: string) {
  if (!taskId) return;
  activeCreateTaskIds.delete(taskId);
  suppressCreateUntil = Date.now() + suppressAfterFinishMs;
}

export function hasActiveVMCreateTask() {
  return activeCreateTaskIds.size > 0 || Date.now() < suppressCreateUntil;
}
