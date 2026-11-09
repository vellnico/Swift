import { describe, expect, it } from 'vitest';
import { dur } from '../motion';
import tokens from './tokens.css?raw';

const sources = import.meta.glob<string>(['/src/**/*.{svelte,ts,css}', '/sub/**/*.{svelte,ts,css}', '!**/*.test.ts', '!**/tokens.css'], {
  query: '?raw',
  import: 'default',
  eager: true,
});

const colorLiteral = /#[0-9a-fA-F]{3,8}(?![\w-])|\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch)\(|\b(?:white|black|red|green|blue|yellow|orange|purple|pink|gray|grey)\b(?=\s*[;}"'])/;

type RGB = [number, number, number];

function hexToRgb(hex: string): RGB {
  const h = hex.replace('#', '');
  const full = h.length === 3 ? [...h].map((c) => c + c).join('') : h.slice(0, 6);
  return [0, 2, 4].map((i) => parseInt(full.slice(i, i + 2), 16)) as RGB;
}

/** Chroma (max−min)/255. HSL saturation explodes near black and white, chroma does not. */
function chroma([r, g, b]: RGB): number {
  return (Math.max(r, g, b) - Math.min(r, g, b)) / 255;
}

function luminance([r, g, b]: RGB): number {
  const lin = (c: number) => {
    const s = c / 255;
    return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

function contrast(a: string, b: string): number {
  const [l1, l2] = [luminance(hexToRgb(a)), luminance(hexToRgb(b))].sort((x, y) => y - x) as [number, number];
  return (l1 + 0.05) / (l2 + 0.05);
}

function token(name: string): string {
  const m = new RegExp(`--${name}:\\s*(#[0-9A-Fa-f]{6})`).exec(tokens);
  if (!m?.[1]) throw new Error(`token ${name} not found`);
  return m[1];
}

describe('design tokens', () => {
  it('scans the sources', () => {
    expect(Object.keys(sources).length).toBeGreaterThan(10);
  });

  it('no color literals outside tokens.css', () => {
    const offenders: string[] = [];
    for (const [file, text] of Object.entries(sources)) {
      text.split('\n').forEach((line, i) => {
        if (colorLiteral.test(line)) offenders.push(`${file}:${i + 1}: ${line.trim()}`);
      });
    }
    expect(offenders).toEqual([]);
  });

  it('every palette color is neutral (chroma ≤ 6%)', () => {
    const hexes = tokens.match(/#[0-9A-Fa-f]{6}\b/g) ?? [];
    expect(hexes.length).toBeGreaterThanOrEqual(8);
    for (const hex of hexes) expect(chroma(hexToRgb(hex)), hex).toBeLessThanOrEqual(0.06);
    for (const m of tokens.matchAll(/rgb\((\d+) (\d+) (\d+)/g)) {
      expect(chroma([Number(m[1]), Number(m[2]), Number(m[3])]), m[0]).toBeLessThanOrEqual(0.06);
    }
    expect(chroma(hexToRgb('#3B82F6'))).toBeGreaterThan(0.06);
  });

  it('text passes WCAG AA on every surface', () => {
    for (const s of ['obsidian', 'graphite-1', 'graphite-2', 'graphite-3']) {
      expect(contrast(token('porcelain'), token(s)), `porcelain on ${s}`).toBeGreaterThanOrEqual(4.5);
      expect(contrast(token('silver'), token(s)), `silver on ${s}`).toBeGreaterThanOrEqual(4.5);
    }
    expect(contrast(token('obsidian'), token('porcelain'))).toBeGreaterThanOrEqual(4.5);
    // ash is only for inactive elements on the app background and cards
    expect(contrast(token('ash'), token('obsidian'))).toBeGreaterThanOrEqual(3);
    expect(contrast(token('ash'), token('graphite-1'))).toBeGreaterThanOrEqual(3);
  });

  it('motion.ts durations match tokens.css', () => {
    for (const [name, ms] of Object.entries(dur)) {
      const m = new RegExp(`--dur-${name}:\\s*(\\d+)ms`).exec(tokens);
      expect(m?.[1], name).toBe(String(ms));
    }
  });

  it('spacing tokens are multiples of 4px', () => {
    expect(tokens).toContain('--space: 4px');
    for (const m of tokens.matchAll(/--s(\d+): calc\(var\(--space\) \* (\d+)\)/g)) expect(m[1]).toBe(m[2]);
  });
});
