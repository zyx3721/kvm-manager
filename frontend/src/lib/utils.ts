import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export type KvmTheme = 'dark' | 'light';

export const kvmThemeStorageKey = 'kvm-theme';

export function getInitialKvmTheme(): KvmTheme {
  if (typeof window === 'undefined') return 'dark';
  return window.localStorage.getItem(kvmThemeStorageKey) === 'light' ? 'light' : 'dark';
}

export function applyKvmTheme(theme: KvmTheme) {
  if (typeof document === 'undefined') return;
  document.documentElement.dataset.kvmTheme = theme;
  document.documentElement.style.colorScheme = theme;
}

export function persistKvmTheme(theme: KvmTheme) {
  if (typeof window === 'undefined') return;
  window.localStorage.setItem(kvmThemeStorageKey, theme);
}

export function toggleKvmTheme(theme: KvmTheme): KvmTheme {
  return theme === 'dark' ? 'light' : 'dark';
}
