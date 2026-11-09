export interface Toast {
  id: number;
  message: string;
  action?: { label: string; run: () => void };
  onexpire?: () => void;
}

let next = 1;
const timers: Record<number, ReturnType<typeof setTimeout>> = {};

export const toasts = $state<{ list: Toast[] }>({ list: [] });

/**
 * Show a toast for `timeout` ms. onexpire runs when the toast times out
 * without its action being used — this is how "delete with undo" commits.
 */
export function toast(message: string, opts: { action?: Toast['action']; onexpire?: () => void; timeout?: number } = {}): number {
  const id = next++;
  toasts.list = [...toasts.list.slice(-2), { id, message, action: opts.action, onexpire: opts.onexpire }];
  timers[id] = setTimeout(() => expire(id), opts.timeout ?? 5000);
  return id;
}

function expire(id: number) {
  const item = toasts.list.find((x) => x.id === id);
  dismiss(id);
  item?.onexpire?.();
}

export function dismiss(id: number) {
  clearTimeout(timers[id]);
  delete timers[id];
  toasts.list = toasts.list.filter((x) => x.id !== id);
}

export function runAction(id: number) {
  const item = toasts.list.find((x) => x.id === id);
  dismiss(id);
  item?.action?.run();
}
