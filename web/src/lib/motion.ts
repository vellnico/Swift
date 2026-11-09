import { cubicOut } from 'svelte/easing';
import type { TransitionConfig } from 'svelte/transition';

/** Durations mirrored from tokens.css (--dur-*); tokens.test.ts keeps them in sync. */
export const dur = { fast: 140, base: 220, slow: 420, gleam: 1100, spin: 900, pulse: 3000 } as const;

/** Spring presets from SPEC §10.6. */
export const soft = { stiffness: 0.12, damping: 0.85 } as const;
export const precise = { stiffness: 0.3, damping: 0.9 } as const;

export function reducedMotion(): boolean {
  return typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;
}

/** Spring options that become instant under prefers-reduced-motion. */
export function spring(preset: typeof soft | typeof precise) {
  return reducedMotion() ? { stiffness: 1, damping: 1 } : preset;
}

/** Section change: crossfade with a 6px shift; plain fade when motion is reduced. */
export function section(_node: Element, { y = 6 } = {}): TransitionConfig {
  if (reducedMotion()) return { duration: dur.fast, css: (t) => `opacity:${t}` };
  return {
    duration: dur.base,
    easing: cubicOut,
    css: (t, u) => `opacity:${t};transform:translateY(${u * y}px)`,
  };
}

/** Rise in from blur, used for QR codes and the dashboard reveal. */
export function unblur(_node: Element, { duration = dur.slow, delay = 0 }: { duration?: number; delay?: number } = {}): TransitionConfig {
  if (reducedMotion()) return { duration: dur.fast, delay, css: (t) => `opacity:${t}` };
  return {
    duration,
    delay,
    easing: cubicOut,
    css: (t, u) => `opacity:${t};filter:blur(${u * 8}px)`,
  };
}

/** Stagger for first appearance of lists: 20ms per item, at most 8 items delayed. */
export function stagger(index: number): number {
  return reducedMotion() ? 0 : Math.min(index, 8) * 20;
}

type Opts = { delay?: number; y?: number; duration?: number };

export function fadeShift(_node: Element, { delay = 0, y = 8, duration = dur.base }: Opts = {}): TransitionConfig {
  if (reducedMotion()) return { duration: dur.fast, delay, css: (t) => `opacity:${t}` };
  return {
    duration,
    delay,
    easing: cubicOut,
    css: (t, u) => `opacity:${t};transform:translateY(${u * y}px)`,
  };
}
