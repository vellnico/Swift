import { api, setUnauthorizedHandler } from './api';

type Status = 'unknown' | 'anon' | 'authed';

export const session = $state<{ status: Status; username: string }>({ status: 'unknown', username: '' });

setUnauthorizedHandler(() => {
  session.status = 'anon';
});

interface SessionResponse {
  authenticated: boolean;
  username?: string;
}

export async function checkSession() {
  try {
    const r = await api<SessionResponse>('auth/session');
    session.username = r.username ?? '';
    session.status = r.authenticated ? 'authed' : 'anon';
  } catch {
    session.status = 'anon';
  }
}

export async function login(username: string, password: string) {
  const r = await api<SessionResponse>('auth/login', { method: 'POST', body: { username, password } });
  session.username = r.username ?? username;
}

export async function logout() {
  try {
    await api('auth/logout', { method: 'POST' });
  } finally {
    session.status = 'anon';
    session.username = '';
  }
}
