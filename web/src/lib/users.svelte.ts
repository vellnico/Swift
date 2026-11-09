import { api, apiURL } from './api';

export interface User {
  id: number;
  name: string;
  note: string;
  enabled: boolean;
  createdAt: string;
  subUrl: string;
}

export interface ProfileLink {
  profileId: number;
  name: string;
  kind: string;
  url: string;
}

export const users = $state<{ list: User[]; loaded: boolean; hidden: number[] }>({ list: [], loaded: false, hidden: [] });

export async function loadUsers() {
  users.list = await api<User[]>('users');
  users.loaded = true;
}

function replace(u: User) {
  users.list = users.list.map((x) => (x.id === u.id ? u : x));
}

export async function createUser(name: string): Promise<User> {
  const u = await api<User>('users', { method: 'POST', body: { name } });
  users.list = [u, ...users.list];
  return u;
}

export async function setEnabled(id: number, enabled: boolean): Promise<User> {
  const u = await api<User>(`users/${id}`, { method: 'PUT', body: { enabled } });
  replace(u);
  return u;
}

export async function renameUser(id: number, name: string): Promise<User> {
  const u = await api<User>(`users/${id}`, { method: 'PUT', body: { name } });
  replace(u);
  return u;
}

export async function reissueUser(id: number): Promise<User> {
  const u = await api<User>(`users/${id}/reissue`, { method: 'POST' });
  replace(u);
  return u;
}

export async function deleteUser(id: number) {
  await api(`users/${id}`, { method: 'DELETE' });
  users.list = users.list.filter((x) => x.id !== id);
  users.hidden = users.hidden.filter((x) => x !== id);
}

export function profileLinks(id: number): Promise<ProfileLink[]> {
  return api<ProfileLink[]>(`users/${id}/links`);
}

/** Short non-secret hash so QR image URLs change after a reissue without exposing the token. */
export function version(s: string): string {
  let h = 5381;
  for (let i = 0; i < s.length; i++) h = ((h << 5) + h + s.charCodeAt(i)) >>> 0;
  return h.toString(36);
}

export function qrURL(u: User): string {
  return apiURL(`users/${u.id}/qr.svg`) + `?v=${version(u.subUrl)}`;
}

export function linkQRURL(userId: number, profileId: number, url: string): string {
  return apiURL(`users/${userId}/links/${profileId}/qr.svg`) + `?v=${version(url)}`;
}

export function matches(u: User, query: string): boolean {
  const q = query.trim().toLocaleLowerCase();
  return !q || u.name.toLocaleLowerCase().includes(q) || u.note.toLocaleLowerCase().includes(q);
}
