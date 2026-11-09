<script lang="ts">
  import Check from '@lucide/svelte/icons/check';
  import CircleAlert from '@lucide/svelte/icons/circle-alert';
  import { t } from '../i18n/index.svelte';
  import { fadeShift } from '../motion';
  import Spinner from './Spinner.svelte';
  import type { Stage } from '../apply';

  let { stage, error = '' }: { stage: Stage; error?: string } = $props();
</script>

<div class="apply" aria-live="polite">
  {#key stage}
    {#if stage !== 'idle'}
      <div class="row {stage}" in:fadeShift={{ y: 4 }}>
        <span class="icon" aria-hidden="true">
          {#if stage === 'done'}
            <Check size={16} strokeWidth={1.5} />
          {:else if stage === 'error'}
            <CircleAlert size={16} strokeWidth={1.5} />
          {:else}
            <Spinner size={14} />
          {/if}
        </span>
        <span>{t(`apply.${stage}`)}</span>
      </div>
      {#if stage === 'error' && error}
        <p class="detail">{error}</p>
      {/if}
    {/if}
  {/key}
</div>

<style>
  .apply { min-height: calc(var(--space) * 6); }
  .row { display: flex; align-items: center; gap: var(--s2); font: var(--t-small); color: var(--text-2); }
  .row.done, .row.error { color: var(--text-1); }
  .icon { display: grid; place-items: center; width: var(--s4); }
  .detail { margin-top: var(--s2); font: var(--t-small); color: var(--text-2); max-width: 64ch; }
</style>
