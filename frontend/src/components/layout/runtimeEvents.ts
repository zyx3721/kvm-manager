import { toast } from 'sonner';
import { fetchTask, fetchTasks, type Task } from '../../lib/api';
import { hasActiveVMCloneTask } from '../../features/vms/utils/vmCloneTaskRegistry';
import { hasActiveVMCreateTask } from '../../features/vms/utils/vmCreateTaskRegistry';
import { hasActiveVMMigrateTask } from '../../features/vms/utils/vmMigrateTaskRegistry';

export async function currentRefreshTask(): Promise<Task | null> {
  const body = await fetchTasks(5);
  const task = body.items.find(
    item =>
      item.type.startsWith('runtime.refresh') &&
      (item.status === 'queued' || item.status === 'running')
  );
  if (!task) return null;
  const detail = await fetchTask(task.id);
  return detail.task;
}

export function parseStorageEvent(data: string) {
  try {
    const event = JSON.parse(data) as {
      type?: string;
      payload?: { message?: unknown; operation?: unknown };
    };
    return {
      type: event.type?.endsWith('.failed') ? 'failed' : 'completed',
      message: typeof event.payload?.message === 'string' ? event.payload.message : '',
      operation: typeof event.payload?.operation === 'string' ? event.payload.operation : '',
    };
  } catch {
    return { type: 'completed', message: '', operation: '' };
  }
}

export function showVMCloneResult(event: MessageEvent) {
  const payload = parseResultEvent(event.data);
  if (!payload.message) return;
  if (
    hasActiveVMCloneTask() ||
    (payload.eventType.startsWith('vm.create.') && hasActiveVMCreateTask()) ||
    (payload.eventType.startsWith('vm.migrate.') && hasActiveVMMigrateTask())
  ) {
    return;
  }
  if (payload.type === 'failed') toast.error(payload.message);
  else toast.success(payload.message);
}

function parseResultEvent(data: string) {
  try {
    const event = JSON.parse(data) as { type?: string; payload?: { message?: unknown } };
    const eventType = event.type || '';
    return {
      eventType,
      type: eventType.endsWith('.failed') ? 'failed' : 'completed',
      message: typeof event.payload?.message === 'string' ? event.payload.message : '',
    };
  } catch {
    return { eventType: '', type: 'completed', message: '' };
  }
}
