import { describe, expect, it, vi } from 'vitest';
import { radioKeys } from './a11y';

describe('radioKeys', () => {
  it('moves and selects with arrows, wrapping around', () => {
    document.body.innerHTML = '<div role="radiogroup"><button role="radio">a</button><button role="radio">b</button><button role="radio">c</button></div>';
    const group = document.querySelector<HTMLElement>('[role="radiogroup"]')!;
    const [a, , c] = [...group.querySelectorAll<HTMLButtonElement>('button')];
    const clicked = vi.fn();
    c!.addEventListener('click', clicked);
    group.addEventListener('keydown', radioKeys);
    a!.focus();
    group.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true }));
    expect(document.activeElement).toBe(c);
    expect(clicked).toHaveBeenCalledOnce();
  });
});
