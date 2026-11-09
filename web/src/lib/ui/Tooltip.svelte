<script lang="ts">
  import type { Snippet } from 'svelte';
  import { fadeShift } from '../motion';

  let { text, children }: { text: string; children: Snippet<[{ describedby: string }]> } = $props();
  const uid = $props.id();
  let open = $state(false);
  let timer: ReturnType<typeof setTimeout> | undefined;

  function show() {
    clearTimeout(timer);
    timer = setTimeout(() => (open = true), 300);
  }
  function hide() {
    clearTimeout(timer);
    open = false;
  }
</script>

<span
  class="tip-wrap"
  role="presentation"
  onmouseenter={show}
  onmouseleave={hide}
  onfocusin={() => (open = true)}
  onfocusout={hide}
  onkeydown={(e) => e.key === 'Escape' && hide()}
>
  {@render children({ describedby: `tip-${uid}` })}
  {#if open}
    <span class="tip" role="tooltip" id="tip-{uid}" transition:fadeShift={{ y: 4 }}>{text}</span>
  {:else}
    <span class="sr-only" id="tip-{uid}">{text}</span>
  {/if}
</span>

<style>
  .tip-wrap { position: relative; display: inline-flex; }
  .tip {
    position: absolute;
    bottom: calc(100% + var(--s2));
    left: 50%;
    translate: -50% 0;
    z-index: 30;
    width: max-content;
    max-width: 260px;
    padding: var(--s2) var(--s3);
    border-radius: var(--radius-control);
    background: var(--surface-3);
    box-shadow: var(--edge-lit);
    font: var(--t-small);
    color: var(--text-1);
    pointer-events: none;
  }
</style>
