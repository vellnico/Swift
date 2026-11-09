<script lang="ts">
  import Copy from '@lucide/svelte/icons/copy';
  import Check from '@lucide/svelte/icons/check';
  import { buzz, copyText } from '../clipboard';
  import { t } from '../i18n/index.svelte';

  let { value, label, mono = true }: { value: string; label?: string; mono?: boolean } = $props();
  let copied = $state(false);
  let timer: ReturnType<typeof setTimeout> | undefined;

  async function copy() {
    if (!(await copyText(value))) return;
    buzz();
    copied = true;
    clearTimeout(timer);
    timer = setTimeout(() => (copied = false), 1600);
  }
</script>

<div class="copy-field">
  {#if label}<span class="label">{label}</span>{/if}
  <button type="button" class="field" class:copied onclick={copy} aria-label="{t('common.copy')}: {label ?? value}">
    <span class="value" class:mono>{value}</span>
    <span class="icon" aria-hidden="true">
      {#if copied}<Check size={18} strokeWidth={1.5} />{:else}<Copy size={18} strokeWidth={1.5} />{/if}
    </span>
    <span class="flash" aria-hidden="true"></span>
  </button>
  <span class="sr-only" aria-live="polite">{copied ? t('common.copied') : ''}</span>
</div>

<style>
  .copy-field { display: flex; flex-direction: column; min-width: 0; }
  .label { font: var(--t-small); font-weight: 500; margin-bottom: var(--s2); }
  .field {
    position: relative;
    display: flex;
    align-items: center;
    gap: var(--s3);
    width: 100%;
    min-height: var(--control-h);
    padding: var(--s2) var(--s2) var(--s2) var(--s3);
    border: 0;
    border-radius: var(--radius-control);
    background: var(--surface-2);
    box-shadow: inset 0 0 0 1px var(--line);
    text-align: left;
  }
  .field:hover { box-shadow: inset 0 0 0 1px var(--line-strong); }
  .field:focus-visible { box-shadow: inset 0 0 0 1px var(--line-strong), var(--focus-ring); }
  .value { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text-1); }
  .mono { font: var(--t-code); }
  .icon { flex: none; display: grid; place-items: center; width: var(--s8); color: var(--text-2); }
  .copied .icon { color: var(--text-1); }
  .flash {
    position: absolute;
    inset: 0;
    border-radius: inherit;
    box-shadow: var(--edge-lit);
    opacity: 0;
    pointer-events: none;
    transition: opacity var(--dur-slow) var(--ease-out);
  }
  .copied .flash { opacity: 1; transition-duration: var(--dur-fast); }
</style>
