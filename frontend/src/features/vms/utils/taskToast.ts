export const taskToastOptions = {
  position: 'top-right' as const,
  duration: Infinity,
};

export const taskToastDoneOptions = {
  duration: 5000,
};

export function taskToastOptionsFor(id?: string | number, options?: { position?: 'top-left' | 'top-right' }) {
  return {
    ...taskToastOptions,
    ...options,
    ...(id !== undefined ? { id } : {}),
  };
}

export function taskToastDoneOptionsFor(id?: string | number, options?: { position?: 'top-left' | 'top-right' }) {
  return {
    ...taskToastOptions,
    ...options,
    ...taskToastDoneOptions,
    ...(id !== undefined ? { id } : {}),
  };
}
