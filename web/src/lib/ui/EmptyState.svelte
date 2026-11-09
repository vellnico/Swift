<script lang="ts">
  import type { Snippet } from 'svelte';
  let { text, action }: { text: string; action?: Snippet } = $props();
</script>

<div class="empty">
  <svg class="bird" width="132" height="72" viewBox="0 0 132 72" aria-hidden="true">
    <path
      d="M66 40C56 26 34 13 6 10C30 18 50 28 66 40ZM66 40C76 26 98 13 126 10C102 18 82 28 66 40ZM66 40L60 62M66 40L72 62"
      fill="none"
      stroke="currentColor"
      stroke-width="1.5"
      stroke-linecap="round"
      stroke-linejoin="round"
    />
    <path d="M14 52C26 50 34 48 42 44" fill="none" stroke="currentColor" stroke-width="1" stroke-linecap="round" opacity="0.35" />
    <path d="M22 60C30 58 38 56 46 52" fill="none" stroke="currentColor" stroke-width="1" stroke-linecap="round" opacity="0.2" />
  </svg>
  <p class="text">{text}</p>
  {#if action}<div class="action">{@render action()}</div>{/if}
</div>

<style>
  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: var(--s5);
    padding: var(--s12) var(--s6);
  }
  .bird { color: var(--text-2); }
  .text { max-width: 40ch; color: var(--text-2); font: var(--t-body); }
</style>
