<script lang="ts">
  import { t } from '../i18n/index.svelte';
  import Badge from './Badge.svelte';

  let {
    id,
    hint,
    isRecommended,
    onreset,
  }: {
    id?: string;
    hint?: string;
    /** undefined: the field has no recommended value */
    isRecommended?: boolean;
    onreset?: () => void;
  } = $props();
</script>

{#if hint || isRecommended !== undefined}
  <div class="field-hint" {id}>
    {#if hint}<p class="text">{hint}</p>{/if}
    {#if isRecommended === true}
      <Badge variant="recommended">{t('common.recommended')}</Badge>
    {:else if isRecommended === false && onreset}
      <button type="button" class="reset" onclick={onreset}>{t('common.reset_recommended')}</button>
    {/if}
  </div>
{/if}

<style>
  .field-hint {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s2) var(--s3);
    margin-top: var(--s2);
  }
  .text { font: var(--t-small); color: var(--text-2); max-width: 64ch; flex: 1 1 200px; }
  .reset {
    border: 0;
    padding: 0;
    background: none;
    font: var(--t-small);
    color: var(--text-2);
  }
  .reset:hover { color: var(--text-1); }
</style>
