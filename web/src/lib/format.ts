import { i18n, translate, type Locale } from './i18n/index.svelte';

function nf(locale: Locale, digits: number): Intl.NumberFormat {
  return new Intl.NumberFormat(locale === 'ru' ? 'ru-RU' : 'en-US', { maximumFractionDigits: digits, minimumFractionDigits: digits });
}

/** One significant decimal below 10, none above: 1.2 / 18 / 240. */
function smart(locale: Locale, v: number): string {
  return nf(locale, v < 10 && v !== 0 ? 1 : 0).format(v);
}

export interface Parts {
  value: string;
  unit: string;
}

export function bytes(n: number, locale: Locale = i18n.locale): Parts {
  const units = ['units.b', 'units.kb', 'units.mb', 'units.gb', 'units.tb'];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return { value: smart(locale, v), unit: translate(locale, units[i]!) };
}

/** Network rate in bits per second → Mbit/s (kbit/s below 0.1 Mbit/s). */
export function rate(bps: number, locale: Locale = i18n.locale): Parts {
  if (bps < 100_000) return { value: smart(locale, bps / 1000), unit: translate(locale, 'units.kbit') };
  return { value: smart(locale, bps / 1_000_000), unit: translate(locale, 'units.mbit') };
}

export function mbit(bps: number, locale: Locale = i18n.locale): string {
  return smart(locale, bps / 1_000_000);
}

export function percent(v: number, locale: Locale = i18n.locale): string {
  return nf(locale, 0).format(Math.round(v));
}

/** "4 d 3 h", "3 h 12 min", "45 min". */
export function duration(seconds: number, locale: Locale = i18n.locale): string {
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const u = (k: string) => translate(locale, `units.${k}`);
  // a number never wraps away from its unit
  const nb = ' ';
  if (d > 0) return `${d}${nb}${u('day')} ${h}${nb}${u('hour')}`;
  if (h > 0) return `${h}${nb}${u('hour')} ${m}${nb}${u('min')}`;
  return `${Math.max(m, 0)}${nb}${u('min')}`;
}

/** Pluralised count: key.one / key.few / key.many / key.other with {n}. */
export function plural(key: string, n: number, locale: Locale = i18n.locale): string {
  const form = new Intl.PluralRules(locale === 'ru' ? 'ru-RU' : 'en-US').select(n);
  const s = translate(locale, `${key}.${form}`);
  return (s === `${key}.${form}` ? translate(locale, `${key}.other`) : s).replace('{n}', String(n));
}
