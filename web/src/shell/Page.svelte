<script lang="ts">
  import type { Snippet } from 'svelte';
  let { title, action, children }: { title: string; action?: Snippet; children: Snippet } = $props();
</script>

<section class="page">
  <header class="head">
    <h1 class="t-title">{title}</h1>
    {#if action}<div class="action">{@render action()}</div>{/if}
  </header>
  {@render children()}
</section>

<style>
  .page { display: flex; flex-direction: column; gap: var(--s8); }
  .head { display: flex; align-items: center; justify-content: space-between; gap: var(--s4); min-height: var(--control-h); }
  @media (max-width: 959px) {
    .page { gap: var(--s6); }
  }
</style>
