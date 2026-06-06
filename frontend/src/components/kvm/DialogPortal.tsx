import type { ReactNode } from 'react';
import { createPortal } from 'react-dom';

export function DialogPortal({ children }: { children: ReactNode }) {
  return typeof document === 'undefined' ? children : createPortal(children, document.body);
}
