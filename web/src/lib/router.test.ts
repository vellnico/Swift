import { describe, expect, it } from 'vitest';
import { href, parse } from './router.svelte';

describe('router', () => {
  it('parses known routes', () => {
    expect(parse('').name).toBe('dashboard');
    expect(parse('#/').name).toBe('dashboard');
    expect(parse('#/profiles').name).toBe('profiles');
    expect(parse('#/users/42')).toEqual({ name: 'users', params: { id: '42' }, path: '/users/42' });
    expect(parse('#/settings/').name).toBe('settings');
  });

  it('hides the kit outside dev builds', () => {
    expect(parse('#/_kit', false).name).toBe('notfound');
    expect(parse('#/_kit', true).name).toBe('kit');
  });

  it('returns notfound for unknown paths', () => {
    expect(parse('#/nope').name).toBe('notfound');
    expect(parse('#/users/abc').name).toBe('notfound');
  });

  it('builds hrefs', () => {
    expect(href('/users')).toBe('#/users');
    expect(href('log')).toBe('#/log');
  });
});
