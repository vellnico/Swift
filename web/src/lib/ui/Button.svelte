<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { HTMLButtonAttributes } from 'svelte/elements';
  import Check from '@lucide/svelte/icons/check';
  import Spinner from './Spinner.svelte';
  import { dur } from '../motion';

  type Props = HTMLButtonAttributes & {
    variant?: 'primary' | 'secondary' | 'quiet';
    /** loading shows a spinner, done a check mark; width stays the same */
    status?: 'idle' | 'loading' | 'done';
    wide?: boolean;
    children: Snippet;
    icon?: Snippet;
  };

  let {
    variant = 'secondary',
    status = 'idle',
    wide = false,
    type = 'button',
    children,
    icon,
    class: cls = '',
    ...rest
  }: Props = $props();

  let gleaming = $state(false);
  let previous: string = 'idle';

  $effect(() => {
    const now = status;
    if (now === 'done' && previous !== 'done' && variant === 'primary') {
      gleaming = false;
      requestAnimationFrame(() => (gleaming = true));
      const id = setTimeout(() => (gleaming = false), dur.gleam + dur.fast);
      previous = now;
      return () => clearTimeout(id);
    }
    previous = now;
  });
</script>

<button
  {type}
  class="btn {variant} {cls}"
  class:wide
  class:gleam={variant === 'primary'}
  class:gleam-run={gleaming}
  aria-busy={status === 'loading' ? 'true' : undefined}
  {...rest}
>
  <span class="content" class:hidden={status !== 'idle'}>
    {#if icon}{@render icon()}{/if}
    {@render children()}
  </span>
  {#if status === 'loading'}
    <span class="overlay"><Spinner /></span>
  {:else if status === 'done'}
    <span class="overlay"><Check size={18} strokeWidth={1.5} aria-hidden="true" /></span>
  {/if}
</button>

<style>
  .btn {
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    height: var(--control-h);
    padding: 0 var(--s4);
    border: 0;
    border-radius: var(--radius-control);
    font: var(--t-body);
    font-weight: 600;
    white-space: nowrap;
    -webkit-tap-highlight-color: transparent;
    transition: transform var(--dur-fast) var(--ease-out);
  }
  .btn:active:not(:disabled) { transform: scale(0.97); }
  .wide { width: 100%; }
  .content { display: inline-flex; align-items: center; gap: var(--s2); }
  .content.hidden { visibility: hidden; }
  .overlay { position: absolute; inset: 0; display: grid; place-items: center; }

  .primary { background: var(--accent); color: var(--accent-fg); }
  .primary:hover:not(:disabled) { background: var(--gleam); }

  .secondary { background: transparent; color: var(--text-1); box-shadow: inset 0 0 0 1px var(--line-strong); }
  .secondary:hover:not(:disabled) { background: var(--surface-3); }
  .secondary:focus-visible { box-shadow: inset 0 0 0 1px var(--line-strong), var(--focus-ring); }

  .quiet { background: transparent; color: var(--text-2); padding: 0 var(--s2); }
  .quiet:hover:not(:disabled) { color: var(--text-1); }

  .btn:disabled { opacity: 0.4; cursor: not-allowed; }
</style>
