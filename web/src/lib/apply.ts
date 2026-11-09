import { onLive } from './state.svelte';

export type Stage = 'idle' | 'checking' | 'restarting' | 'done' | 'error';

/** Mirror server-side apply progress (SSE "apply") into set() while a request runs. */
export function trackApply(set: (s: Stage) => void): () => void {
  return onLive('apply', (d) => {
    const s = (d as { stage: Stage }).stage;
    if (s === 'checking' || s === 'restarting') set(s);
  });
}
