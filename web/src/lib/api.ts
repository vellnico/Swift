/** Panel root (`/<panel_path>/`) derived from where index.html was served. */
export function panelBase(pathname: string): string {
  const parts = pathname.split('/').filter(Boolean);
  return parts.length ? `/${parts[0]}/` : '/';
}

export function apiURL(path: string, pathname = location.pathname): string {
  return panelBase(pathname) + 'api/' + path.replace(/^\/+/, '');
}

export function readCookie(cookies: string, name: string): string {
  for (const part of cookies.split(';')) {
    const [k, ...v] = part.trim().split('=');
    if (k === name) return decodeURIComponent(v.join('='));
  }
  return '';
}

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    public data: Record<string, unknown> = {},
  ) {
    super(code);
  }
}

let onUnauthorized: (() => void) | undefined;
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn;
}

export async function api<T = unknown>(path: string, opts: { method?: string; body?: unknown; signal?: AbortSignal } = {}): Promise<T> {
  const method = opts.method ?? 'GET';
  const headers: Record<string, string> = {};
  if (opts.body !== undefined) headers['Content-Type'] = 'application/json';
  if (method !== 'GET' && method !== 'HEAD') headers['X-Swift-CSRF'] = readCookie(document.cookie, 'swift_csrf');
  let res: Response;
  try {
    res = await fetch(apiURL(path), {
      method,
      headers,
      body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
      credentials: 'same-origin',
      signal: opts.signal,
    });
  } catch (e) {
    if ((e as Error).name === 'AbortError') throw e;
    throw new ApiError(0, 'network');
  }
  let data: Record<string, unknown> = {};
  const text = await res.text();
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = {};
    }
  }
  if (!res.ok) {
    if (res.status === 401 && !path.startsWith('auth/')) onUnauthorized?.();
    throw new ApiError(res.status, typeof data.error === 'string' ? data.error : 'internal', data);
  }
  return data as T;
}
