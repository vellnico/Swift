import { api } from './api';
import { onLive } from './state.svelte';

export interface Sample {
  ts: number;
  cpu: number;
  memUsed: number;
  memTotal: number;
  diskUsed: number;
  diskTotal: number;
  rxBps: number;
  txBps: number;
  uptime: number;
}

export const MAX_SAMPLES = 150; // 5 minutes at 2 s

export const stats = $state<{ samples: Sample[]; loaded: boolean }>({ samples: [], loaded: false });

export function push(list: Sample[], s: Sample): Sample[] {
  const next = list.length >= MAX_SAMPLES ? list.slice(list.length - MAX_SAMPLES + 1) : list.slice();
  next.push(s);
  return next;
}

let stop: (() => void) | null = null;

/** Load the history once and append live samples pushed over SSE. */
export async function watchStats(): Promise<() => void> {
  if (!stop) stop = onLive('sysstat', (d) => (stats.samples = push(stats.samples, d as Sample)));
  try {
    const history = await api<Sample[]>('sysstat');
    // live samples may already have arrived; keep the newest of both
    const last = stats.samples[0]?.ts ?? Infinity;
    stats.samples = [...history.filter((s) => s.ts < last), ...stats.samples].slice(-MAX_SAMPLES);
  } finally {
    stats.loaded = true;
  }
  return () => {
    stop?.();
    stop = null;
  };
}
