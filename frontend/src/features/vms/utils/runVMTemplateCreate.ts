import { toast } from 'sonner';

import { createVMFromTemplate, fetchTask, type Task, type VMClonePayload } from '../../../lib/api';
import { taskToastDoneOptionsFor, taskToastOptions, taskToastOptionsFor } from './taskToast';
import { registerVMCreateTask, unregisterVMCreateTask } from './vmCreateTaskRegistry';

type RunVMTemplateCreateOptions = {
  templateId: string;
  vmName: string;
  payload: VMClonePayload;
  onQueued?: () => void;
};

export async function runVMTemplateCreate({
  templateId,
  vmName,
  payload,
  onQueued,
}: RunVMTemplateCreateOptions) {
  let toastId: string | number | undefined;
  try {
    const response = await createVMFromTemplate(templateId, payload);
    toastId = toast.loading(`${vmName} 从模板创建任务已排队`, taskToastOptions);
    registerVMCreateTask(response.task.id);
    onQueued?.();
    try {
      await waitForVMTemplateCreateTask(response.task, message =>
        toast.loading(`${vmName} ${message}`, taskToastOptionsFor(toastId))
      );
    } finally {
      unregisterVMCreateTask(response.task.id);
    }
    toast.success(`${vmName} 从模板创建完成`, taskToastDoneOptionsFor(toastId));
    return true;
  } catch (error) {
    const message = error instanceof Error ? error.message : '从模板创建虚拟机失败';
    if (toastId === undefined) toast.error(message);
    else toast.error(message, taskToastDoneOptionsFor(toastId));
    return false;
  }
}

async function waitForVMTemplateCreateTask(task: Task, onProgress: (message: string) => void) {
  let current = task;
  for (;;) {
    const payload = parseTaskPayload(current.payload);
    const message = payload?.message || '正在从模板创建虚拟机';
    if (current.status === 'completed') return true;
    if (current.status === 'failed') {
      throw new Error(current.errorMessage || message || '从模板创建虚拟机失败');
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
