export interface PageData {
  lang: 'ru' | 'en';
  name: string;
  enabled: boolean;
  url: string;
  links: { name: string; url: string }[];
  strings: Record<string, string>;
}

export type Platform = 'android' | 'ios' | 'windows' | 'macos' | 'linux';

export function platformOf(ua: string): Platform {
  const s = ua.toLowerCase();
  if (s.includes('android')) return 'android';
  if (/iphone|ipad|ipod/.test(s)) return 'ios';
  if (s.includes('windows')) return 'windows';
  if (s.includes('mac os') || s.includes('macintosh')) return 'macos';
  return 'linux';
}

export interface App {
  id: 'hiddify' | 'happ' | 'v2rayng';
  name: string;
  /** deeplink that imports the subscription */
  link: (url: string, name: string) => string;
  download: string;
  platforms: Platform[];
}

// Deeplink schemes as documented by the apps; verify on release (SPEC 7.4).
export const apps: App[] = [
  {
    id: 'hiddify',
    name: 'Hiddify',
    link: (url, name) => `hiddify://import/${url}#${encodeURIComponent(name)}`,
    download: 'https://github.com/hiddify/hiddify-app/releases/latest',
    platforms: ['android', 'ios', 'windows', 'macos', 'linux'],
  },
  {
    id: 'happ',
    name: 'Happ',
    link: (url) => `happ://add/${url}`,
    download: 'https://www.happ.su/',
    platforms: ['android', 'ios', 'windows', 'macos'],
  },
  {
    id: 'v2rayng',
    name: 'v2rayNG',
    link: (url) => `v2rayng://install-config?url=${encodeURIComponent(url)}`,
    download: 'https://github.com/2dust/v2rayNG/releases/latest',
    platforms: ['android'],
  },
];

export function appsFor(p: Platform): App[] {
  return apps.filter((a) => a.platforms.includes(p));
}

export function tr(strings: Record<string, string>, key: string, params: Record<string, string> = {}): string {
  let s = strings[key] ?? key;
  for (const [k, v] of Object.entries(params)) s = s.replaceAll(`{${k}}`, v);
  return s;
}
