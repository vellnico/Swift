/**
 * Arrow-key navigation for a role="radiogroup" of buttons: arrows move focus
 * and select, as in a native radio group; Tab enters and leaves the group.
 */
export function radioKeys(e: KeyboardEvent) {
  const keys: Record<string, number> = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 };
  const step = keys[e.key];
  if (!step) return;
  const group = e.currentTarget as HTMLElement;
  const radios = [...group.querySelectorAll<HTMLElement>('[role="radio"]:not([disabled])')];
  const i = radios.indexOf(document.activeElement as HTMLElement);
  if (i < 0) return;
  e.preventDefault();
  const next = radios[(i + step + radios.length) % radios.length]!;
  next.focus();
  next.click();
}

/** Only the checked radio is in the tab order (roving tabindex). */
export function radioTabIndex(checked: boolean): number {
  return checked ? 0 : -1;
}
