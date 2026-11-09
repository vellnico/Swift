import { describe, expect, it } from 'vitest';
import { apps, appsFor, platformOf, tr } from './data';

describe('subscription page data', () => {
  it('detects platforms', () => {
    expect(platformOf('Mozilla/5.0 (Linux; Android 14; Pixel 8)')).toBe('android');
    expect(platformOf('Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X)')).toBe('ios');
    expect(platformOf('Mozilla/5.0 (Windows NT 10.0; Win64; x64)')).toBe('windows');
    expect(platformOf('Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5)')).toBe('macos');
  });

  it('offers v2rayNG only on Android and Hiddify everywhere', () => {
    expect(appsFor('ios').map((a) => a.id)).toEqual(['hiddify', 'happ']);
    expect(appsFor('android').map((a) => a.id)).toEqual(['hiddify', 'happ', 'v2rayng']);
    expect(appsFor('linux').map((a) => a.id)).toEqual(['hiddify']);
  });

  it('builds deeplinks', () => {
    const url = 'https://1.2.3.4:31000/s/tok';
    const [hiddify, happ, ng] = apps;
    expect(hiddify!.link(url, 'Swift · Мама')).toBe('hiddify://import/https://1.2.3.4:31000/s/tok#Swift%20%C2%B7%20%D0%9C%D0%B0%D0%BC%D0%B0');
    expect(happ!.link(url, '')).toBe('happ://add/https://1.2.3.4:31000/s/tok');
    expect(ng!.link(url, '')).toBe('v2rayng://install-config?url=https%3A%2F%2F1.2.3.4%3A31000%2Fs%2Ftok');
  });

  it('translates with params', () => {
    expect(tr({ 'sub.add': 'Добавить в {app}' }, 'sub.add', { app: 'Happ' })).toBe('Добавить в Happ');
    expect(tr({}, 'missing')).toBe('missing');
  });
});
