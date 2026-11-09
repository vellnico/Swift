<script lang="ts">
  import type { Snippet } from 'svelte';
  import { tick } from 'svelte';
  import { Spring } from 'svelte/motion';
  import { MediaQuery } from 'svelte/reactivity';
  import X from '@lucide/svelte/icons/x';
  import { t } from '../i18n/index.svelte';
  import { soft, spring } from '../motion';
  import IconButton from './IconButton.svelte';

  let {
    open = $bindable(false),
    title,
    children,
    footer,
    onclose,
  }: {
    open?: boolean;
    title: string;
    children: Snippet;
    footer?: Snippet;
    onclose?: () => void;
  } = $props();

  const uid = $props.id();
  const mobile = new MediaQuery('(max-width: 959px)');
  // 1 = hidden, 0 = in place
  const pos = new Spring(1, spring(soft));
  let mounted = $state(false);
  let panel = $state<HTMLElement>();
  let restoreFocus: HTMLElement | null = null;

  $effect(() => {
    if (open && !mounted) {
      restoreFocus = document.activeElement as HTMLElement | null;
      mounted = true;
      pos.set(1, { instant: true });
      tick().then(() => {
        pos.target = 0;
        const first = panel?.querySelector<HTMLElement>('input, select, textarea, button:not(.close), [href], [tabindex]:not([tabindex="-1"])');
        (first ?? panel)?.focus();
      });
    } else if (!open && mounted) {
      pos.set(1).then(() => {
        if (!open) {
          mounted = false;
          restoreFocus?.focus();
        }
      });
    }
  });

  function close() {
    open = false;
    onclose?.();
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.stopPropagation();
      close();
    } else if (e.key === 'Tab' && panel) {
      const items = [...panel.querySelectorAll<HTMLElement>('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])')].filter((el) => !el.hasAttribute('disabled'));
      const first = items[0];
      const last = items[items.length - 1];
      if (!first || !last) return;
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    }
  }

  // swipe down to close the bottom sheet
  let dragStart: number | null = null;
  let dragHeight = 1;
  function dragDown(e: PointerEvent) {
    if (!mobile.current || !panel) return;
    dragStart = e.clientY;
    dragHeight = panel.offsetHeight || 1;
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
  }
  function dragMove(e: PointerEvent) {
    if (dragStart === null) return;
    pos.set(Math.max(0, (e.clientY - dragStart) / dragHeight), { instant: true });
  }
  function dragEnd(e: PointerEvent) {
    if (dragStart === null) return;
    const moved = e.clientY - dragStart;
    dragStart = null;
    if (moved > Math.min(120, dragHeight * 0.3)) close();
    else pos.target = 0;
  }

  const style = $derived(
    mobile.current
      ? `transform: translateY(${pos.current * 100}%)`
      : `transform: translateY(${pos.current * 24}px) scale(${1 - pos.current * 0.03}); opacity: ${1 - pos.current}`,
  );
</script>

{#if mounted}
  <div class="layer" role="presentation" {onkeydown}>
    <div class="scrim" style:opacity={1 - pos.current} role="presentation" onclick={close}></div>
    <div
      bind:this={panel}
      class="panel"
      class:sheet={mobile.current}
      role="dialog"
      aria-modal="true"
      aria-labelledby="dlg-{uid}"
      tabindex="-1"
      {style}
    >
      {#if mobile.current}
        <div
          class="grabber"
          role="presentation"
          onpointerdown={dragDown}
          onpointermove={dragMove}
          onpointerup={dragEnd}
          onpointercancel={dragEnd}
        >
          <span></span>
        </div>
      {/if}
      <header class="head">
        <h2 class="t-heading" id="dlg-{uid}">{title}</h2>
        <IconButton label={t('common.close')} class="close" onclick={close}><X size={18} strokeWidth={1.5} /></IconButton>
      </header>
      <div class="body">{@render children()}</div>
      {#if footer}<footer class="foot">{@render footer()}</footer>{/if}
    </div>
  </div>
{/if}

<style>
  .layer {
    position: fixed;
    inset: 0;
    z-index: 50;
    display: grid;
    place-items: center;
    padding: var(--s6);
  }
  .scrim {
    position: absolute;
    inset: 0;
    background: var(--scrim);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
  }
  .panel {
    position: relative;
    width: min(520px, 100%);
    max-height: calc(100dvh - var(--s12));
    overflow: auto;
    border-radius: var(--radius-sheet);
    background: var(--surface-1);
    box-shadow: var(--edge-lit);
    padding: var(--s6);
    outline: none;
  }
  .sheet {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 0;
    width: 100%;
    max-height: 88dvh;
    border-radius: var(--radius-sheet) var(--radius-sheet) 0 0;
    padding: var(--s2) var(--s5) calc(var(--s6) + env(safe-area-inset-bottom));
  }
  .grabber {
    display: grid;
    place-items: center;
    height: var(--s6);
    margin: 0 calc(var(--s5) * -1) var(--s2);
    touch-action: none;
    cursor: grab;
  }
  .grabber span {
    width: var(--s10);
    height: var(--s1);
    border-radius: var(--radius-pill);
    background: var(--line-strong);
  }
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s4);
    margin-bottom: var(--s4);
  }
  .head :global(.close) { margin-right: calc(var(--s2) * -1); }
  .foot {
    display: flex;
    justify-content: flex-end;
    gap: var(--s3);
    margin-top: var(--s6);
  }
</style>
