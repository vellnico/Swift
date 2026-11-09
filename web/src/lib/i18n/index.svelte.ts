import ru from './ru.json';
import en from './en.json';

export type Locale = 'ru' | 'en';
export type Key = keyof typeof en;
type Dict = Record<string, string>;

export const dictionaries: Record<Locale, Dict> = { ru, en };
export const locales: Locale[] = ['en', 'ru'];
export const defaultLocale: Locale = 'en';

const STORAGE_KEY = 'swift.locale';

function isLocale(v: string | null | undefined): v is Locale {
  return v === 'ru' || v === 'en';
}

/**
 * The browser's own choice wins; otherwise the language picked at install time,
 * which the server writes into <html lang>; otherwise English.
 */
export function detectLocale(stored: string | null, pageLang: string | null | undefined): Locale {
  if (isLocale(stored)) return stored;
  const base = pageLang?.toLowerCase().split('-')[0];
  if (isLocale(base)) return base;
  return defaultLocale;
}

function initial(): Locale {
  if (typeof window === 'undefined') return defaultLocale;
  let stored: string | null;
  try {
    stored = localStorage.getItem(STORAGE_KEY);
  } catch {
    stored = null;
  }
  return detectLocale(stored, document.documentElement.lang);
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

/** Translate with {name} placeholders. Missing keys fall back to English, then to the key. */
export function translate(locale: Locale, key: string, params?: Record<string, string | number>): string {
  let s = dictionaries[locale][key] ?? dictionaries[defaultLocale][key] ?? key;
  if (params) {
    for (const [k, v] of Object.entries(params)) s = s.replaceAll(`{${k}}`, String(v));
  }
  return s;
}

export function t(key: Key | (string & {}), params?: Record<string, string | number>): string {
  return translate(i18n.locale, key, params);
}
