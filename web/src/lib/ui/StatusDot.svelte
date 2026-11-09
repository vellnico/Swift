<script lang="ts">
  let { status, label }: { status: 'ok' | 'busy' | 'off'; label?: string } = $props();
</script>

<span class="status {status}" role="status">
  <span class="mark" aria-hidden="true">
    {#if status === 'off'}
      <svg width="12" height="12" viewBox="0 0 12 12"><circle cx="6" cy="6" r="4.75" fill="none" stroke="currentColor" stroke-width="1.5" /><path d="M2.6 9.4L9.4 2.6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" /></svg>
    {/if}
  </span>
  {#if label}<span class="label">{label}</span>{/if}
</span>

<style>
  .status { display: inline-flex; align-items: center; gap: var(--s2); font: var(--t-small); }
  .mark { position: relative; display: grid; place-items: center; width: 12px; height: 12px; flex: none; }

  .ok .mark::before {
    content: "";
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--state-ok);
  }
  .ok .mark::after {
    content: "";
    position: absolute;
    inset: 2px;
    border-radius: 50%;
    background: var(--state-ok);
    opacity: 0;
    animation: pulse var(--dur-pulse) var(--ease-out) infinite;
  }
  @keyframes pulse {
    0% { opacity: 0.5; transform: scale(1); }
    40%, 100% { opacity: 0; transform: scale(2.6); }
  }

  .busy .mark::before {
    content: "";
    width: 10px;
    height: 10px;
    border-radius: 50%;
    border: 1.5px solid var(--state-busy);
    border-right-color: transparent;
    animation: spin var(--dur-spin) linear infinite;
  }
  @keyframes spin { to { transform: rotate(1turn); } }

  .off .mark { color: var(--state-off); }
  .ok .label { color: var(--text-1); }
  .busy .label, .off .label { color: var(--text-2); }

  @media (prefers-reduced-motion: reduce) {
    .ok .mark::after { animation: none; }
    .busy .mark::before { animation-duration: calc(var(--dur-spin) * 3); }
  }
</style>
