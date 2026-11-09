import ru from './ru.json';
import en from './en.json';

export type Locale = 'ru' | 'en';
export type Key = keyof typeof ru;
type Dict = Record<string, string>;

export const dictionaries: Record<Locale, Dict> = { ru, en };
export const locales: Locale[] = ['ru', 'en'];

const STORAGE_KEY = 'swift.locale';

export function detectLocale(stored: string | null, languages: readonly string[]): Locale {
  if (stored === 'ru' || stored === 'en') return stored;
  for (const lang of languages) {
    const base = lang.toLowerCase().split('-')[0];
    if (base === 'ru' || base === 'en') return base;
  }
  return 'ru';
}

function initial(): Locale {
  if (typeof window === 'undefined') return 'ru';
  let stored: string | null;
  try {
    stored = localStorage.getItem(STORAGE_KEY);
  } catch {
    stored = null;
  }
  return detectLocale(stored, navigator.languages ?? [navigator.language]);
}

export const i18n = $state({ locale: initial() });

export function setLocale(locale: Locale) {
  i18n.locale = locale;
  try {
    localStorage.setItem(STORAGE_KEY, locale);
  } catch {
    // storage may be unavailable; the choice still applies to this tab
  }
  if (typeof document !== 'undefined') document.documentElement.lang = locale;
}

/** Translate with {name} placeholders. Missing keys fall back to Russian, then to the key. */
export function translate(locale: Locale, key: string, params?: Record<string, string | number>): string {
  let s = dictionaries[locale][key] ?? dictionaries.ru[key] ?? key;
  if (params) {
    for (const [k, v] of Object.entries(params)) s = s.replaceAll(`{${k}}`, String(v));
  }
  return s;
}

export function t(key: Key | (string & {}), params?: Record<string, string | number>): string {
  return translate(i18n.locale, key, params);
}
