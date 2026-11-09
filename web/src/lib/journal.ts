import { dictionaries, i18n, translate, type Locale } from './i18n/index.svelte';

export type Level = 'info' | 'warn' | 'error';

export interface JournalEvent {
  id: number;
  ts: string;
  level: Level;
  source: string;
  key: string;
  params: Record<string, unknown>;
}

/** The journal line for an event; unknown keys still show something readable. */
export function eventText(ev: Pick<JournalEvent, 'key' | 'params'>, locale: Locale = i18n.locale): string {
  const key = `ev.${ev.key}`;
  if (!(key in dictionaries[locale]) && !(key in dictionaries.ru)) return translate(locale, 'log.unknown', { key: ev.key });
  const params: Record<string, string> = {};
  for (const [k, v] of Object.entries(ev.params ?? {})) params[k] = typeof v === 'string' ? v : JSON.stringify(v);
  return translate(locale, key, params);
}
