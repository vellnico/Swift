export type RouteName = 'dashboard' | 'profiles' | 'users' | 'routing' | 'settings' | 'log' | 'more' | 'kit' | 'notfound';

export interface Route {
  name: RouteName;
  params: Record<string, string>;
  path: string;
}

const table: [RegExp, RouteName, string[]][] = [
  [/^\/$/, 'dashboard', []],
  [/^\/profiles(?:\/(\d+))?$/, 'profiles', ['id']],
  [/^\/users(?:\/(\d+))?$/, 'users', ['id']],
  [/^\/routing$/, 'routing', []],
  [/^\/settings$/, 'settings', []],
  [/^\/log$/, 'log', []],
  [/^\/more$/, 'more', []],
  [/^\/_kit$/, 'kit', []],
];

/** Parse a location hash such as "#/users/3" into a route. */
export function parse(hash: string, dev = false): Route {
  const path = (hash.replace(/^#/, '') || '/').replace(/\/+$/, '') || '/';
  for (const [re, name, keys] of table) {
    const m = re.exec(path);
    if (!m) continue;
    if (name === 'kit' && !dev) break;
    const params: Record<string, string> = {};
    keys.forEach((k, i) => {
      const v = m[i + 1];
      if (v !== undefined) params[k] = v;
    });
    return { name, params, path };
  }
  return { name: 'notfound', params: {}, path };
}

export function href(path: string): string {
  return '#' + (path.startsWith('/') ? path : '/' + path);
}

const isBrowser = typeof window !== 'undefined';

export const router = $state<{ route: Route }>({
  route: parse(isBrowser ? location.hash : '', import.meta.env.DEV),
});

if (isBrowser) {
  window.addEventListener('hashchange', () => {
    router.route = parse(location.hash, import.meta.env.DEV);
  });
}

export function navigate(path: string) {
  location.hash = href(path);
}
