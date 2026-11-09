import { describe, expect, it } from 'vitest';
import { bytes, duration, mbit, percent, plural, rate } from './format';

describe('format', () => {
  it('bytes', () => {
    expect(bytes(1.2 * 1024 ** 3, 'en')).toEqual({ value: '1.2', unit: 'GB' });
    expect(bytes(512 * 1024 ** 2, 'ru')).toEqual({ value: '512', unit: 'МБ' });
    expect(bytes(0, 'en')).toEqual({ value: '0', unit: 'B' });
  });

  it('rates in bits', () => {
    expect(rate(18_000_000, 'en')).toEqual({ value: '18', unit: 'Mbit/s' });
    expect(rate(3_400_000, 'ru')).toEqual({ value: '3,4', unit: 'Мбит/с' });
    expect(rate(50_000, 'en')).toEqual({ value: '50', unit: 'kbit/s' });
    expect(mbit(1_250_000, 'en')).toBe('1.3');
  });

  it('percent and duration', () => {
    expect(percent(23.6, 'en')).toBe('24');
    expect(duration(4 * 86400 + 3 * 3600 + 59, 'ru')).toBe('4 д 3 ч');
    expect(duration(3 * 3600 + 12 * 60, 'en')).toBe('3 h 12 min');
    expect(duration(45 * 60, 'en')).toBe('45 min');
  });

  it('plurals', () => {
    expect(plural('dash.profiles', 1, 'ru')).toBe('1 профиль');
    expect(plural('dash.profiles', 3, 'ru')).toBe('3 профиля');
    expect(plural('dash.profiles', 7, 'ru')).toBe('7 профилей');
    expect(plural('dash.people', 21, 'ru')).toBe('21 человек');
    expect(plural('dash.profiles', 1, 'en')).toBe('1 profile');
    expect(plural('dash.profiles', 2, 'en')).toBe('2 profiles');
  });
});
