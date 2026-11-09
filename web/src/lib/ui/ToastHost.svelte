<script lang="ts">
  import { flip } from 'svelte/animate';
  import { dur, fadeShift } from '../motion';
  import { runAction, toasts } from './toasts.svelte';
</script>

<div class="host" aria-live="polite" aria-atomic="false">
  {#each toasts.list as item (item.id)}
    <div class="toast" role="status" transition:fadeShift={{ y: 12 }} animate:flip={{ duration: dur.base }}>
      <span class="msg">{item.message}</span>
      {#if item.action}
        <button type="button" class="action" onclick={() => runAction(item.id)}>{item.action.label}</button>
      {/if}
    </div>
  {/each}
</div>

<style>
  .host {
    position: fixed;
    left: 50%;
    bottom: calc(var(--s6) + env(safe-area-inset-bottom));
    translate: -50% 0;
    z-index: 60;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--s2);
    width: min(440px, calc(100vw - var(--s8)));
    pointer-events: none;
  }
  @media (max-width: 959px) {
    .host { bottom: calc(var(--s16) + var(--s8) + env(safe-area-inset-bottom)); }
  }
  .toast {
    pointer-events: auto;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s4);
    width: 100%;
    min-height: calc(var(--space) * 12);
    padding: var(--s2) var(--s2) var(--s2) var(--s4);
    border-radius: var(--radius-control);
    background: var(--surface-3);
    box-shadow: var(--edge-lit);
  }
  .msg { font: var(--t-body); }
  .action {
    flex: none;
    height: calc(var(--space) * 8);
    padding: 0 var(--s3);
    border: 0;
    border-radius: var(--radius-control);
    background: transparent;
    color: var(--text-1);
    font-weight: 600;
  }
  .action:hover { background: var(--surface-2); }
</style>
