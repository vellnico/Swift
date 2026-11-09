<script lang="ts">
  let { size = 16, label }: { size?: number; label?: string } = $props();
</script>

<span
  class="spinner"
  style:--size="{size}px"
  role={label ? 'status' : undefined}
  aria-label={label}
  aria-hidden={label ? undefined : 'true'}
></span>

<style>
  .spinner {
    display: inline-block;
    width: var(--size);
    height: var(--size);
    border-radius: 50%;
    border: var(--stroke) solid var(--line-strong);
    border-top-color: currentColor;
    animation: spin var(--dur-spin) linear infinite;
  }
  @keyframes spin {
    to { transform: rotate(1turn); }
  }
  @media (prefers-reduced-motion: reduce) {
    .spinner { animation-duration: calc(var(--dur-spin) * 3); }
  }
</style>
