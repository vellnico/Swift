<script lang="ts">
  import { radioKeys, radioTabIndex } from '../lib/a11y';
  import { i18n, locales, setLocale, t } from '../lib/i18n/index.svelte';
</script>

<div class="lang" role="radiogroup" aria-label={t('lang.label')} tabindex="-1" onkeydown={radioKeys}>
  {#each locales as l (l)}
    <button
      type="button"
      role="radio"
      aria-checked={i18n.locale === l}
      tabindex={radioTabIndex(i18n.locale === l)}
      class:active={i18n.locale === l}
      onclick={() => setLocale(l)}
      lang={l}
      title={t(`lang.${l}`)}
    >
      {l.toUpperCase()}
    </button>
  {/each}
</div>

<style>
  .lang {
    display: inline-flex;
    padding: 2px;
    border-radius: var(--radius-pill);
    box-shadow: inset 0 0 0 1px var(--line);
  }
  button {
    height: calc(var(--space) * 7);
    min-width: calc(var(--space) * 10);
    padding: 0 var(--s2);
    border: 0;
    border-radius: var(--radius-pill);
    background: transparent;
    color: var(--text-2);
    font: var(--t-micro);
    letter-spacing: var(--t-micro-track);
  }
  button:hover { color: var(--text-1); }
  .active { background: var(--surface-3); color: var(--text-1); }
</style>
