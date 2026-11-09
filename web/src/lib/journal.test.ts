import { describe, expect, it } from 'vitest';
import { eventText } from './journal';
import en from './i18n/en.json';
import ru from './i18n/ru.json';

describe('journal', () => {
  it('fills params', () => {
    expect(eventText({ key: 'xray.updated', params: { from: '26.10.31', to: '26.11.5' } }, 'ru')).toBe('Xray обновлён с 26.10.31 до 26.11.5');
    expect(eventText({ key: 'xray.crashed', params: { code: 2, retry_s: 4 } }, 'en')).toBe('Xray exited with code 2, restarting in 4 s');
  });

  it('falls back for unknown keys', () => {
    expect(eventText({ key: 'new.thing', params: {} }, 'en')).toBe('Event new.thing');
  });

  it('every event key exists in both languages', () => {
    const keys = Object.keys(ru).filter((k) => k.startsWith('ev.'));
    expect(keys.length).toBeGreaterThan(20);
    for (const k of keys) expect(en).toHaveProperty([k]);
  });
});
