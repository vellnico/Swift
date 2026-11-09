type Handler = (e: KeyboardEvent) => void;

const handlers: Record<string, Handler[]> = {};

/** Register a single-key shortcut ("/", "n", "Escape") for as long as the caller lives. */
export function hotkey(key: string, fn: Handler): () => void {
  (handlers[key] ??= []).push(fn);
  return () => {
    handlers[key] = (handlers[key] ?? []).filter((h) => h !== fn);
  };
}

function typing(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null;
  if (!el) return false;
  return el.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(el.tagName);
}

export function onGlobalKey(e: KeyboardEvent) {
  if (e.ctrlKey || e.metaKey || e.altKey) return;
  if (e.key !== 'Escape' && typing(e.target)) return;
  if (document.querySelector('[role="dialog"]') && e.key !== 'Escape') return;
  const list = handlers[e.key];
  const last = list?.[list.length - 1];
  if (last) {
    e.preventDefault();
    last(e);
  }
}
