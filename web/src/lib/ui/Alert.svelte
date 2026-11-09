<script lang="ts">
  import type { Snippet } from 'svelte';
  import CircleAlert from '@lucide/svelte/icons/circle-alert';
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert';

  let { kind = 'error', title, children }: { kind?: 'error' | 'warning'; title?: string; children: Snippet } = $props();
</script>

<div class="alert {kind}" role={kind === 'error' ? 'alert' : 'status'}>
  <span class="icon" aria-hidden="true">
    {#if kind === 'error'}<CircleAlert size={18} strokeWidth={1.5} />{:else}<TriangleAlert size={18} strokeWidth={1.5} />{/if}
  </span>
  <div class="text">
    {#if title}<p class="title">{title}</p>{/if}
    <div class="body">{@render children()}</div>
  </div>
</div>

<style>
  .alert {
    display: flex;
    gap: var(--s3);
    padding: var(--s4);
    border-radius: var(--radius-control);
  }
  .error { background: var(--state-error-bg); color: var(--state-error-fg); }
  .warning { box-shadow: inset 0 0 0 1px var(--line-strong); color: var(--text-1); }
  .icon { flex: none; padding-top: 2px; }
  .title { font-weight: 600; margin-bottom: var(--s1); }
  .body { font: var(--t-small); }
  .warning .body { color: var(--text-2); }
</style>
