<script lang="ts">
  import { unblur } from '../motion';

  let { src, size = 200, label }: { src: string; size?: number; label: string } = $props();
</script>

<!-- QR codes stay pure black on pure white on their own plate (SPEC 10.2) -->
<div class="plate" style:--qr={`${size}px`}>
  {#key src}
    <img {src} alt={label} width={size} height={size} in:unblur />
  {/key}
</div>

<style>
  .plate {
    display: inline-grid;
    place-items: center;
    padding: calc(var(--qr) * 0.06);
    border-radius: calc(var(--radius-control) * 0.8);
    background: var(--qr-light);
    flex: none;
  }
  img {
    width: var(--qr);
    height: var(--qr);
    display: block;
    image-rendering: pixelated;
  }
</style>
