<script lang="ts">
  import type { Snippet } from 'svelte';
  let { variant = 'default', children }: { variant?: 'default' | 'recommended' | 'solid'; children: Snippet } = $props();
</script>

<span class="badge {variant}">
  {#if variant === 'recommended'}
    <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden="true">
      <path d="M5 0.5V9.5M0.5 5H9.5M2 2L8 8M8 2L2 8" stroke="currentColor" stroke-width="1" stroke-linecap="round" opacity="0.9" />
    </svg>
  {/if}
  {@render children()}
</span>

<style>
  .badge {
    display: inline-flex;
    align-items: center;
    gap: var(--s1);
    height: calc(var(--space) * 6);
    padding: 0 var(--s2);
    border-radius: var(--radius-pill);
    font: var(--t-micro);
    letter-spacing: var(--t-micro-track);
    color: var(--text-2);
    box-shadow: inset 0 0 0 1px var(--line);
    white-space: nowrap;
  }
  .recommended { color: var(--text-1); box-shadow: inset 0 0 0 1px var(--line-strong); }
  .solid { background: var(--accent); color: var(--accent-fg); box-shadow: none; }
</style>
