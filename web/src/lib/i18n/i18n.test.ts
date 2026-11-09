import { describe, expect, it } from 'vitest';
import ru from './ru.json';
import en from './en.json';
import { detectLocale, translate } from './index.svelte';

describe('i18n', () => {
  it('has identical key sets in every language', () => {
    expect(Object.keys(en).sort()).toEqual(Object.keys(ru).sort());
  });

  it('has no empty strings', () => {
    for (const [k, v] of [...Object.entries(ru), ...Object.entries(en)]) {
      expect(v.trim(), k).not.toBe('');
    }
  });

  it('interpolates params and falls back', () => {
    expect(translate('en', 'nav.users')).toBe('People');
    expect(translate('en', 'no.such.key')).toBe('no.such.key');
    expect(translate('ru', 'x {n}' as never, { n: 3 })).toBe('x 3');
  });

  it('detects the locale', () => {
    expect(detectLocale('en', ['ru-RU'])).toBe('en');
    expect(detectLocale(null, ['de-DE', 'en-US'])).toBe('en');
    expect(detectLocale(null, ['de'])).toBe('ru');
    expect(detectLocale('xx', ['ru'])).toBe('ru');
  });

  it('never shouts in caps', () => {
    for (const [k, v] of Object.entries(ru)) {
      const letters = v.replace(/[^A-Za-zА-Яа-яЁё]/g, '');
      if (letters.length > 4 && !/^(TCP|UDP|QR|URL|VLESS|SNI|TLS|Xray)/.test(v)) {
        expect(letters, k).not.toBe(letters.toUpperCase());
      }
    }
  });
});
