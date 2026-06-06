import { toast } from 'sonner';

import { cloneVM, fetchTask, type Task, type VMClonePayload } from '../../../lib/api';
import { taskToastDoneOptionsFor, taskToastOptions, taskToastOptionsFor } from './taskToast';
import { registerVMCloneTask, unregisterVMCloneTask } from './vmCloneTaskRegistry';

type RunVMCloneOptions = {
  vmId: string;
  cloneName: string;
  payload: VMClonePayload;
  onQueued?: () => void;
};

const VM_CLONE_TASK_POLL_INTERVAL_MS = 3000;

export async function runVMClone({ vmId, cloneName, payload, onQueued }: RunVMCloneOptions) {
  let toastId: string | number | undefined;
  try {
    const response = await cloneVM(vmId, payload);
    toastId = toast.loading(`${cloneName} 克隆虚拟机排队中`, taskToastOptions);
    registerVMCloneTask(response.task.id);
    onQueued?.();
    try {
      await waitForVMCloneTask(response.task, message =>
        toast.loading(`${cloneName} ${message}`, taskToastOptionsFor(toastId))
      );
    } finally {
      unregisterVMCloneTask(response.task.id);
    }
    toast.success(`${cloneName} 克隆完成`, taskToastDoneOptionsFor(toastId));
    return true;
  } catch (error) {
    const message = error instanceof Error ? error.message : '虚拟机克隆失败';
    if (toastId === undefined) toast.error(message);
    else toast.error(message, taskToastDoneOptionsFor(toastId));
    return false;
  }
}

async function waitForVMCloneTask(task: Task, onProgress: (message: string) => void) {
  let current = task;
  for (;;) {
    const payload = parseTaskPayload(current.payload);
    const message = payload?.message || '正在克隆虚拟机';
    if (current.status === 'completed') return true;
    if (current.status === 'failed') {
      throw new Error(current.errorMessage || message || '虚拟机克隆失败');
    }
    onProgress(message);
    await delay(VM_CLONE_TASK_POLL_INTERVAL_MS);
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
