import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { dismiss, runAction, toast, toasts } from './toasts.svelte';

describe('toasts', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => {
    for (const t of toasts.list) dismiss(t.id);
    vi.useRealTimers();
  });

  it('expires after the timeout and commits', () => {
    const commit = vi.fn();
    toast('Removed', { onexpire: commit });
    expect(toasts.list).toHaveLength(1);
    vi.advanceTimersByTime(5000);
    expect(toasts.list).toHaveLength(0);
    expect(commit).toHaveBeenCalledOnce();
  });

  it('undo runs the action and skips the commit', () => {
    const commit = vi.fn();
    const undo = vi.fn();
    const id = toast('Removed', { onexpire: commit, action: { label: 'Undo', run: undo } });
    runAction(id);
    vi.advanceTimersByTime(6000);
    expect(undo).toHaveBeenCalledOnce();
    expect(commit).not.toHaveBeenCalled();
  });

  it('keeps at most three toasts', () => {
    for (let i = 0; i < 5; i++) toast(`t${i}`);
    expect(toasts.list.map((t) => t.message)).toEqual(['t2', 't3', 't4']);
  });
});
