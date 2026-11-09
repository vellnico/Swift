<script lang="ts">
  import { onMount } from 'svelte';
  import { dur } from '../motion';

  let { size = 28, word = true, gleam = false }: { size?: number; word?: boolean; gleam?: boolean } = $props();
  let running = $state(false);

  onMount(() => {
    if (!gleam) return;
    const start = setTimeout(() => (running = true), dur.base);
    const stop = setTimeout(() => (running = false), dur.base + dur.gleam + dur.fast);
    return () => {
      clearTimeout(start);
      clearTimeout(stop);
    };
  });
</script>

<span class="logo" class:gleam class:gleam-run={running} style:--logo={`${size}px`}>
  <svg class="mark" viewBox="0 0 32 20" aria-hidden="true">
    <path
      d="M16 10.5C12.6 6.4 7.6 3.4 1.2 2.6C6.8 4.6 11.6 7.2 16 10.5ZM16 10.5C19.4 6.4 24.4 3.4 30.8 2.6C25.2 4.6 20.4 7.2 16 10.5ZM16 10.5L14.4 17.4M16 10.5L17.6 17.4"
      fill="none"
      stroke="currentColor"
      stroke-width="1.5"
      stroke-linecap="round"
      stroke-linejoin="round"
      vector-effect="non-scaling-stroke"
    />
  </svg>
  {#if word}<span class="word">Swift</span>{/if}
</span>

<style>
  .logo {
    display: inline-flex;
    align-items: center;
    gap: calc(var(--logo) * 0.36);
    color: var(--text-1);
    border-radius: var(--radius-control);
  }
  .mark { width: calc(var(--logo) * 1.6); height: var(--logo); flex: none; }
  .word {
    font-family: var(--font-display);
    font-weight: 400;
    font-size: calc(var(--logo) * 0.72);
    line-height: 1;
    letter-spacing: -0.02em;
  }
</style>
