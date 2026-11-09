import { api } from './api';

export type Kind = 'vless_reality_vision' | 'vless_reality_xhttp' | 'hysteria2';
export const kinds: Kind[] = ['vless_reality_vision', 'vless_reality_xhttp', 'hysteria2'];

export interface Params {
  privateKey?: string;
  publicKey?: string;
  shortIds?: string[];
  target?: string;
  serverNames?: string[];
  fingerprint?: string;
  path?: string;
  mode?: string;
  obfsPassword?: string;
}

export interface Profile {
  id: number;
  name: string;
  kind: Kind;
  port: number;
  enabled: boolean;
  manual: boolean;
  params: Params;
  position: number;
}

export interface Field {
  name: string;
  type: 'text' | 'number' | 'select' | 'list' | 'secret' | 'switch';
  recommended?: string | number | boolean;
  options?: string[];
  advanced: boolean;
  min?: number;
  max?: number;
  pattern?: string;
  regenerate?: boolean;
}

export interface ProfileUpdate {
  name?: string;
  port?: number;
  enabled?: boolean;
  params?: Params;
  regenerate?: string[];
}

export const profiles = $state<{ list: Profile[]; schema: Record<string, Field[]>; loaded: boolean; hidden: number[] }>({
  list: [],
  schema: {},
  loaded: false,
  hidden: [],
});

export async function loadProfiles() {
  const [list, schema] = await Promise.all([api<Profile[]>('profiles'), api<Record<string, Field[]>>('profiles/schema')]);
  profiles.list = list;
  profiles.schema = schema;
  profiles.loaded = true;
}

export async function createProfile(kind: Kind): Promise<Profile> {
  const p = await api<Profile>('profiles', { method: 'POST', body: { kind } });
  profiles.list = [...profiles.list, p];
  return p;
}

export async function updateProfile(id: number, upd: ProfileUpdate): Promise<Profile> {
  const p = await api<Profile>(`profiles/${id}`, { method: 'PUT', body: upd });
  profiles.list = profiles.list.map((x) => (x.id === id ? p : x));
  return p;
}

export async function deleteProfile(id: number) {
  await api(`profiles/${id}`, { method: 'DELETE' });
  profiles.list = profiles.list.filter((x) => x.id !== id);
  profiles.hidden = profiles.hidden.filter((x) => x !== id);
}

/** The SNI shown on cards: the first disguise domain. */
export function sniOf(p: Profile): string {
  return p.params.serverNames?.[0] ?? '';
}

export function networkOf(kind: Kind): 'TCP' | 'UDP' {
  return kind === 'hysteria2' ? 'UDP' : 'TCP';
}
