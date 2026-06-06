import { toast } from 'sonner';

import { ApiError, fetchTask, migrateVM, type Task, type VMMigratePayload } from '../../../lib/api';
import { taskToastDoneOptionsFor, taskToastOptions, taskToastOptionsFor } from './taskToast';
import { registerVMMigrateTask, unregisterVMMigrateTask } from './vmMigrateTaskRegistry';

type RunVMMigrateOptions = {
  vmId: string;
  vmName: string;
  payload: VMMigratePayload;
  onQueued?: () => void;
  onSSHPasswordRequired?: (message: string) => void;
};

export async function runVMMigrate({
  vmId,
  vmName,
  payload,
  onQueued,
  onSSHPasswordRequired,
}: RunVMMigrateOptions) {
  let toastId: string | number | undefined;
  try {
    const response = await migrateVM(vmId, payload);
    toastId = toast.loading(`${vmName} 迁移任务已排队`, taskToastOptions);
    registerVMMigrateTask(response.task.id);
    onQueued?.();
    try {
      await waitForVMMigrateTask(response.task, message =>
        toast.loading(`${vmName} ${message}`, taskToastOptionsFor(toastId))
      );
    } finally {
      unregisterVMMigrateTask(response.task.id);
    }
    toast.success(`${vmName} 迁移完成`, taskToastDoneOptionsFor(toastId));
    return true;
  } catch (error) {
    const message = error instanceof Error ? error.message : '迁移虚拟机失败';
    if (toastId === undefined && error instanceof ApiError && error.code === 'vm_migrate_ssh_password_required') {
      onSSHPasswordRequired?.(message);
      return false;
    }
    if (toastId === undefined && error instanceof ApiError && error.status === 400) {
      toast.warning(message);
    } else if (toastId === undefined) toast.error(message);
    else toast.error(message, taskToastDoneOptionsFor(toastId));
    return false;
  }
}

async function waitForVMMigrateTask(task: Task, onProgress: (message: string) => void) {
  let current = task;
  for (;;) {
    const payload = parseTaskPayload(current.payload);
    const message = payload?.message || '正在迁移虚拟机';
    if (current.status === 'completed') return true;
    if (current.status === 'failed') {
      throw new Error(current.errorMessage || message || '迁移虚拟机失败');
    }
    onProgress(message);
    await delay(10000);
    current = (await fetchTask(current.id)).task;
  }
}

function parseTaskPayload(payload: unknown) {
  if (!payload || typeof payload !== 'object') return null;
  const message = (payload as { message?: unknown }).message;
  return { message: typeof message === 'string' ? message : '' };
}

function delay(ms: number) {
  return new Promise(resolve => window.setTimeout(resolve, ms));
}
