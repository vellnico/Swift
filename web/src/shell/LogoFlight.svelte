<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { Spring } from 'svelte/motion';
  import { reducedMotion, soft } from '../lib/motion';
  import Logo from '../lib/ui/Logo.svelte';

  // The login logo flies into the sidebar after a successful sign-in (SPEC 10.6).
  let { from, ondone }: { from: DOMRect; ondone: () => void } = $props();

  const pos = new Spring({ x: 0, y: 0, s: 1, o: 1 }, soft);
  let ready = $state(false);

  onMount(() => {
    pos.set({ x: from.left, y: from.top, s: 1, o: 1 }, { instant: true });
    ready = true;
    tick().then(() => {
      const target = document.querySelector<HTMLElement>('[data-logo-target]');
      if (!target || reducedMotion()) {
        pos.set({ x: from.left, y: from.top, s: 1, o: 0 }).then(ondone);
        return;
      }
      const to = target.getBoundingClientRect();
      pos.set({ x: to.left, y: to.top, s: to.height / from.height, o: 1 }).then(ondone);
    });
  });
</script>

{#if ready}
  <div
    class="flight"
    aria-hidden="true"
    style:transform="translate({pos.current.x}px, {pos.current.y}px) scale({pos.current.s})"
    style:opacity={pos.current.o}
  >
    <Logo size={40} />
  </div>
{/if}

<style>
  .flight {
    position: fixed;
    left: 0;
    top: 0;
    z-index: 70;
    transform-origin: 0 0;
    pointer-events: none;
  }
</style>
