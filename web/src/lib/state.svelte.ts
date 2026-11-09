/* eslint-disable svelte/prefer-svelte-reactivity -- the listener registry is plain bookkeeping, not UI state */
import { apiURL } from './api';

export type XrayStatus = 'ok' | 'busy' | 'off';

interface XrayStatusMessage {
  status: XrayStatus;
  pid?: number;
  since: string;
  restarts: number;
  version: string;
}

/** Live state pushed by the server over SSE; the UI never polls. */
export const live = $state<{ xray: XrayStatus; xrayVersion: string; xraySince: string; connected: boolean }>({
  xray: 'off',
  xrayVersion: '',
  xraySince: '',
  connected: false,
});

type Listener = (data: unknown) => void;
const listeners = new Map<string, Set<Listener>>();

/** Subscribe to a server event type; returns an unsubscribe function. */
export function onLive(type: string, fn: Listener): () => void {
  let set = listeners.get(type);
  if (!set) {
    set = new Set();
    listeners.set(type, set);
    source?.addEventListener(type, dispatch);
  }
  set.add(fn);
  return () => set.delete(fn);
}

let source: EventSource | null = null;

function dispatch(e: Event) {
  const me = e as MessageEvent<string>;
  let data: unknown;
  try {
    data = JSON.parse(me.data);
  } catch {
    return;
  }
  for (const fn of listeners.get(me.type) ?? []) fn(data);
}

export function applyXrayStatus(m: XrayStatusMessage) {
  live.xray = m.status;
  live.xrayVersion = m.version;
  live.xraySince = m.since;
}

export function connectLive() {
  if (source) return;
  source = new EventSource(apiURL('events'));
  source.onopen = () => (live.connected = true);
  source.onerror = () => (live.connected = false);
  source.addEventListener('xray_status', (e) => applyXrayStatus(JSON.parse((e as MessageEvent<string>).data)));
  for (const type of listeners.keys()) source.addEventListener(type, dispatch);
}

export function disconnectLive() {
  source?.close();
  source = null;
  live.connected = false;
}
