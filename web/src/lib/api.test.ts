import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, ApiError, apiURL, panelBase, readCookie } from './api';

describe('panelBase', () => {
  it('takes the first path segment', () => {
    expect(panelBase('/abc123/')).toBe('/abc123/');
    expect(panelBase('/abc123/users')).toBe('/abc123/');
    expect(panelBase('/')).toBe('/');
  });
  it('builds api urls', () => {
    expect(apiURL('/health', '/abc/')).toBe('/abc/api/health');
    expect(apiURL('users/1', '/abc/x')).toBe('/abc/api/users/1');
  });
});

describe('api', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('reads cookies', () => {
    expect(readCookie('a=1; swift_csrf=tok%3D; b=2', 'swift_csrf')).toBe('tok=');
    expect(readCookie('', 'x')).toBe('');
  });

  it('sends the CSRF header on unsafe methods only', async () => {
    document.cookie = 'swift_csrf=secret-token';
    const fetchMock = vi.fn(async () => new Response('{"ok":true}', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    await api('auth/logout', { method: 'POST' });
    await api('auth/session');
    const calls = fetchMock.mock.calls as unknown as [string, RequestInit][];
    expect((calls[0]![1].headers as Record<string, string>)['X-Swift-CSRF']).toBe('secret-token');
    expect((calls[1]![1].headers as Record<string, string>)['X-Swift-CSRF']).toBeUndefined();
  });

  it('turns error responses into ApiError with the server code', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('{"error":"rate_limited","retry_after":90}', { status: 429 })));
    const err = (await api('auth/login', { method: 'POST', body: {} }).catch((e) => e)) as ApiError;
    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe('rate_limited');
    expect(err.data.retry_after).toBe(90);
  });

  it('reports network failures', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => Promise.reject(new TypeError('offline'))));
    const err = (await api('auth/session').catch((e) => e)) as ApiError;
    expect(err.code).toBe('network');
  });
});
