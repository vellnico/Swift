import { describe, expect, it } from 'vitest';
import { MAX_SAMPLES, push, type Sample } from './stats.svelte';
import { sparkPath } from './ui/sparkline';

const s = (ts: number, cpu = 0): Sample => ({ ts, cpu, memUsed: 0, memTotal: 0, diskUsed: 0, diskTotal: 0, rxBps: 0, txBps: 0, uptime: 0 });

describe('stats buffer', () => {
  it('keeps at most five minutes', () => {
    let list: Sample[] = [];
    for (let i = 0; i < MAX_SAMPLES + 10; i++) list = push(list, s(i));
    expect(list).toHaveLength(MAX_SAMPLES);
    expect(list[0]!.ts).toBe(10);
  });
});

describe('sparkline path', () => {
  it('maps values into the box, newest at the right edge', () => {
    const { line, area } = sparkPath([0, 50, 100], 100, 40, 100, 3);
    expect(line).toBe('M0 40L50 20L100 0');
    expect(area).toBe('M0 40L50 20L100 0L100 40L0 40Z');
  });
  it('grows in from the right and auto-scales', () => {
    expect(sparkPath([], 100, 40, 0, 3).line).toBe('');
    expect(sparkPath([5, 5], 100, 40, 0, 3).line).toBe('M50 0L100 0');
  });
});
