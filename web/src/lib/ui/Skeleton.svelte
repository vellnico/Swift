<script lang="ts">
  let { width = '100%', height = '16px', radius = 'control' }: { width?: string; height?: string; radius?: 'control' | 'surface' | 'pill' } = $props();
</script>

<span class="skeleton r-{radius}" style:width style:height aria-hidden="true"></span>

<style>
  .skeleton {
    display: block;
    background: var(--surface-2);
    animation: breathe var(--dur-pulse) var(--ease-in-out) infinite;
  }
  .r-control { border-radius: var(--radius-control); }
  .r-surface { border-radius: var(--radius-surface); }
  .r-pill { border-radius: var(--radius-pill); }
  @keyframes breathe {
    50% { opacity: 0.55; }
  }
  @media (prefers-reduced-motion: reduce) {
    .skeleton { animation: none; }
  }
</style>
