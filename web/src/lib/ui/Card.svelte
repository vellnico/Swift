<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { HTMLAttributes } from 'svelte/elements';

  type Props = HTMLAttributes<HTMLElement> & {
    as?: 'div' | 'article' | 'section' | 'li';
    interactive?: boolean;
    children: Snippet;
  };
  let { as = 'div', interactive = false, children, class: cls = '', ...rest }: Props = $props();
</script>

<svelte:element this={as} class="card {cls}" class:interactive {...rest}>
  {@render children()}
</svelte:element>

<style>
  .card {
    position: relative;
    background: var(--surface-1);
    box-shadow: var(--edge);
    border-radius: var(--radius-surface);
    padding: var(--s5);
    min-width: 0;
  }
  .card:hover,
  .card.interactive:focus-within { box-shadow: var(--edge-lit); }
</style>
