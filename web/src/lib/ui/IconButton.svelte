<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { HTMLButtonAttributes } from 'svelte/elements';

  type Props = HTMLButtonAttributes & {
    label: string;
    variant?: 'quiet' | 'secondary';
    children: Snippet;
  };
  let { label, variant = 'quiet', type = 'button', children, class: cls = '', ...rest }: Props = $props();
</script>

<button {type} class="icon-btn {variant} {cls}" aria-label={label} title={label} {...rest}>
  {@render children()}
</button>

<style>
  .icon-btn {
    display: inline-grid;
    place-items: center;
    width: var(--control-h);
    height: var(--control-h);
    border: 0;
    border-radius: var(--radius-control);
    background: transparent;
    color: var(--text-2);
    -webkit-tap-highlight-color: transparent;
    transition: transform var(--dur-fast) var(--ease-out);
  }
  .icon-btn:hover:not(:disabled) { color: var(--text-1); background: var(--surface-3); }
  .icon-btn:active:not(:disabled) { transform: scale(0.94); }
  .secondary { box-shadow: inset 0 0 0 1px var(--line-strong); color: var(--text-1); }
  .secondary:focus-visible { box-shadow: inset 0 0 0 1px var(--line-strong), var(--focus-ring); }
  .icon-btn:disabled { opacity: 0.4; cursor: not-allowed; }
</style>
