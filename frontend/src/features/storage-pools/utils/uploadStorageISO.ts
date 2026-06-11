import { toast } from 'sonner';

import {
  ApiError,
  cloneStorageVolume,
  fetchTask,
  type AsyncTaskResponse,
  type StorageVolumeClonePayload,
  type Task,
} from '../../../lib/api';
import { clearSession, getAuthToken } from '../../../lib/auth';
import { formatBytesAuto } from '../../../lib/format';
import {
  taskToastDoneOptionsFor,
  taskToastOptions,
  taskToastOptionsFor,
} from '../../vms/utils/taskToast';
import { friendlyVolumeError } from '../components/storagePoolErrors';
import {
  registerStorageVolumeTask,
  unregisterStorageVolumeTask,
} from './storageVolumeTaskRegistry';

type UploadProgress = {
  loaded: number;
  total: number;
  percent: number;
};

type UploadStorageISOOptions = {
  agentId: string;
  poolName: string;
  file: File;
  volumeName: string;
  onProgress?: (progress: UploadProgress) => void;
};

type RunUploadOptions = {
  agentId: string;
  poolName: string;
  file: File;
  targetName: string;
};

type RunCloneOptions = {
  agentId: string;
  poolName: string;
  payload: StorageVolumeClonePayload;
};

type ApiErrorPayload = {
  error?: string;
  message?: string;
};

const businessUnauthorizedCodes = new Set(['invalid_agent_token', 'invalid_old_password']);

export const storageVolumeToastOptions = {
  ...taskToastOptions,
  position: 'top-left' as const,
};

export function uploadStorageISOWithProgress({
  agentId,
  poolName,
  file,
  volumeName,
  onProgress,
}: UploadStorageISOOptions) {
  const form = new FormData();
  form.set('file', file);
  form.set('name', volumeName);

  return xhrUpload<AsyncTaskResponse>(
    `/api/storage-pools/${encodeURIComponent(agentId)}/volumes/${encodeURIComponent(poolName)}/upload`,
    form,
    onProgress
  );
}

export async function runStorageISOUpload({
  agentId,
  poolName,
  file,
  targetName,
}: RunUploadOptions) {
  const toastId = toast.loading(`${targetName} 正在上传到后端 0%`, storageVolumeToastOptions);
  try {
    const response = await uploadStorageISOWithProgress({
      agentId,
      poolName,
      file,
      volumeName: targetName,
      onProgress: progress => {
        toast.loading(`${targetName} 正在上传到后端 ${progress.percent}%`, {
          ...storageVolumeToastOptionsFor(
            toastId,
            `${formatBytesAuto(progress.loaded)} / ${formatBytesAuto(progress.total)}`
          ),
        });
      },
    });
    registerStorageVolumeTask(response.task.id, 'upload');
    try {
      toast.loading(
        `${targetName} 已提交后台，正在写入存储池`,
        storageVolumeToastOptionsFor(toastId)
      );
      await waitForStorageUploadTask(response.task, message =>
        toast.loading(`${targetName} ${message}`, storageVolumeToastOptionsFor(toastId))
      );
    } finally {
      unregisterStorageVolumeTask(response.task.id, 'upload');
    }
    toast.success(`${targetName} 上传完成`, storageVolumeToastDoneOptionsFor(toastId));
    return true;
  } catch (error) {
    toast.error(
      error instanceof Error ? error.message : '上传 ISO 失败',
      storageVolumeToastDoneOptionsFor(toastId)
    );
    return false;
  }
}

export async function runStorageVolumeClone({ agentId, poolName, payload }: RunCloneOptions) {
  const toastId = toast.loading(`${payload.name} 正在克隆镜像`, storageVolumeToastOptions);
  try {
    const response = await cloneStorageVolume(agentId, poolName, payload);
    registerStorageVolumeTask(response.task.id, 'clone');
    try {
      await waitForStorageVolumeTask(
        response.task,
        message =>
          toast.loading(`${payload.name} ${message}`, storageVolumeToastOptionsFor(toastId)),
        '克隆镜像失败'
      );
    } finally {
      unregisterStorageVolumeTask(response.task.id, 'clone');
    }
    toast.success(`${payload.name} 克隆完成`, storageVolumeToastDoneOptionsFor(toastId));
    return true;
  } catch (error) {
    toast.error(
      friendlyVolumeError(error, '克隆镜像失败'),
      storageVolumeToastDoneOptionsFor(toastId)
    );
    return false;
  }
}

export async function waitForStorageUploadTask(task: Task, onProgress: (message: string) => void) {
  return waitForStorageVolumeTask(task, onProgress, '上传 ISO 失败');
}

export async function waitForStorageVolumeTask(
  task: Task,
  onProgress: (message: string) => void,
  failureMessage: string
) {
  let current = task;
  for (;;) {
    const payload = parseTaskPayload(current.payload);
    const message = payload?.message || '正在写入存储池';
    if (current.status === 'completed') {
      return true;
    }
    if (current.status === 'failed') {
      throw new Error(current.errorMessage || message || failureMessage);
    }
    onProgress(message);
    await delay(10000);
    current = (await fetchTask(current.id)).task;
  }
}

export function storageVolumeToastOptionsFor(id?: string | number, description?: string) {
  return {
    ...taskToastOptionsFor(id, { position: 'top-left' }),
    ...(description ? { description } : {}),
  };
}

function storageVolumeToastDoneOptionsFor(id?: string | number) {
  return taskToastDoneOptionsFor(id, { position: 'top-left' });
}

function xhrUpload<T>(
  url: string,
  body: FormData,
  onProgress?: (progress: UploadProgress) => void
) {
  return new Promise<T>((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('POST', url);

    const token = getAuthToken();
    if (token) {
      xhr.setRequestHeader('Authorization', `Bearer ${token}`);
    }

    xhr.upload.onprogress = event => {
      if (!event.lengthComputable || event.total <= 0) return;
      onProgress?.({
        loaded: event.loaded,
        total: event.total,
        percent: Math.min(100, Math.round((event.loaded * 100) / event.total)),
      });
    };

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(parseSuccess<T>(xhr.responseText));
        return;
      }
      const error = parseError(xhr.status, xhr.responseText);
      if (xhr.status === 401 && !businessUnauthorizedCodes.has(error.code)) {
        clearSession();
        if (window.location.pathname !== '/login') {
          window.location.assign('/login');
        }
      }
      reject(error);
    };
    xhr.onerror = () => reject(new ApiError(0, 'network_error', '网络连接失败'));
    xhr.onabort = () => reject(new ApiError(0, 'request_aborted', '上传已取消'));
    xhr.send(body);
  });
}

function parseSuccess<T>(text: string): T {
  if (!text) return undefined as T;
  return JSON.parse(text) as T;
}

function parseError(status: number, text: string) {
  try {
    const body = JSON.parse(text) as ApiErrorPayload;
    return new ApiError(
      status,
      body.error || 'request_failed',
      body.message || `请求失败：${status}`
    );
  } catch {
    return new ApiError(status, 'request_failed', `请求失败：${status}`);
  }
}

function parseTaskPayload(payload: unknown) {
  if (!payload) return null;
  if (typeof payload === 'string') {
    try {
      return JSON.parse(payload) as { message?: string };
    } catch {
      return null;
    }
  }
  return payload as { message?: string };
}

function delay(ms: number) {
  return new Promise(resolve => window.setTimeout(resolve, ms));
}
