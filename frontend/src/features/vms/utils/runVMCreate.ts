import { toast } from 'sonner';

import { ApiError, createVM, fetchTask, type Task, type VMCreatePayload } from '../../../lib/api';
import { taskToastDoneOptionsFor, taskToastOptions, taskToastOptionsFor } from './taskToast';
import { registerVMCreateTask, unregisterVMCreateTask } from './vmCreateTaskRegistry';

type RunVMCreateOptions = {
  vmName: string;
  payload: VMCreatePayload;
  onQueued?: () => void;
};

export async function runVMCreate({ vmName, payload, onQueued }: RunVMCreateOptions) {
  let toastId: string | number | undefined;
  try {
    const response = await createVM(payload);
    toastId = toast.loading(`${vmName} 创建任务已排队`, taskToastOptions);
    registerVMCreateTask(response.task.id);
    onQueued?.();
    try {
      await waitForVMCreateTask(response.task, message =>
        toast.loading(`${vmName} ${message}`, taskToastOptionsFor(toastId))
      );
    } finally {
      unregisterVMCreateTask(response.task.id);
    }
    toast.success(`${vmName} 创建完成`, taskToastDoneOptionsFor(toastId));
    return true;
  } catch (error) {
    const message = error instanceof Error ? error.message : '创建虚拟机失败';
    if (toastId === undefined && error instanceof ApiError && error.status === 400) {
      toast.warning(message);
    } else if (toastId === undefined) toast.error(message);
    else toast.error(message, taskToastDoneOptionsFor(toastId));
    return false;
  }
}

async function waitForVMCreateTask(task: Task, onProgress: (message: string) => void) {
  let current = task;
  for (;;) {
    const payload = parseTaskPayload(current.payload);
    const message = payload?.message || '正在创建虚拟机';
    if (current.status === 'completed') return true;
    if (current.status === 'failed') {
      throw new Error(current.errorMessage || message || '创建虚拟机失败');
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
