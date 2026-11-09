import { describe, expect, it } from 'vitest';
import { apiURL, panelBase } from './api';

describe('panelBase', () => {
  it('takes the first path segment', () => {
    expect(panelBase('/abc123/')).toBe('/abc123/');
    expect(panelBase('/abc123/users')).toBe('/abc123/');
    expect(panelBase('/')).toBe('/');
  });
  it('builds api urls', () => {
    expect(apiURL('/health', '/abc/')).toBe('/abc/api/health');
    expect(apiURL('users/1', '/abc/x')).toBe('/abc/api/users/1');
  });
});
