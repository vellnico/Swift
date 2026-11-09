<script lang="ts">
  import { Spring } from 'svelte/motion';
  import { precise, spring } from '../motion';

  let {
    checked = $bindable(false),
    label,
    showLabel = false,
    disabled = false,
    onchange,
  }: {
    checked?: boolean;
    label: string;
    showLabel?: boolean;
    disabled?: boolean;
    onchange?: (checked: boolean) => void;
  } = $props();

  const pos = new Spring(0, spring(precise));
  $effect(() => {
    pos.target = checked ? 1 : 0;
  });

  function toggle() {
    if (disabled) return;
    checked = !checked;
    onchange?.(checked);
  }
</script>

<button
  type="button"
  role="switch"
  class="switch"
  class:with-label={showLabel}
  aria-checked={checked}
  aria-label={showLabel ? undefined : label}
  {disabled}
  onclick={toggle}
>
  <span class="track">
    <span class="fill" style:opacity={pos.current}></span>
    <span class="thumb" class:on={checked} style:transform="translateX({pos.current * 16}px)"></span>
  </span>
  {#if showLabel}<span class="text">{label}</span>{/if}
</button>

<style>
  .switch {
    display: inline-flex;
    align-items: center;
    gap: var(--s3);
    padding: 0;
    border: 0;
    background: none;
    border-radius: var(--radius-pill);
    -webkit-tap-highlight-color: transparent;
  }
  .with-label { border-radius: var(--radius-control); }
  .track {
    position: relative;
    display: block;
    width: calc(var(--space) * 10);
    height: calc(var(--space) * 6);
    border-radius: var(--radius-pill);
    box-shadow: inset 0 0 0 1px var(--line-strong);
    flex: none;
  }
  .fill {
    position: absolute;
    inset: 0;
    border-radius: inherit;
    background: var(--accent);
  }
  .thumb {
    position: absolute;
    top: var(--s1);
    left: var(--s1);
    width: var(--s4);
    height: var(--s4);
    border-radius: 50%;
    background: var(--text-2);
  }
  .thumb.on { background: var(--accent-fg); }
  .text { font: var(--t-body); color: var(--text-1); }
  .switch:disabled { opacity: 0.4; cursor: not-allowed; }
</style>
