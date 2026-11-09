<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../../lib/api';
  import { errorText } from '../../lib/errors';
  import { t } from '../../lib/i18n/index.svelte';
  import Card from '../../lib/ui/Card.svelte';
  import Skeleton from '../../lib/ui/Skeleton.svelte';
  import Switch from '../../lib/ui/Switch.svelte';
  import { toast } from '../../lib/ui/toasts.svelte';

  interface Options {
    autoSelect: boolean;
    healthChecks: boolean;
  }

  let opts = $state<Options | null>(null);

  onMount(() => {
    api<Options>('settings/subscriptions').then((o) => (opts = o)).catch((e) => toast(errorText(e)));
  });

  async function save(next: Options) {
    try {
      opts = await api<Options>('settings/subscriptions', { method: 'PUT', body: next });
      toast(t('settings.subs_saved'));
    } catch (e) {
      toast(errorText(e));
      opts = await api<Options>('settings/subscriptions').catch(() => opts);
    }
  }
</script>

<Card>
  <h2 class="t-heading head">{t('settings.subs')}</h2>
  {#if !opts}
    <div class="stack"><Skeleton height="40px" /><Skeleton height="40px" /></div>
  {:else}
    <div class="stack">
      <div class="option">
        <Switch checked={opts.autoSelect} label={t('settings.subs_autoselect')} showLabel onchange={(on) => opts && save({ ...opts, autoSelect: on })} />
        <p class="t-small muted note">{t('settings.subs_autoselect_note')}</p>
      </div>
      <div class="option">
        <Switch checked={opts.healthChecks} label={t('settings.subs_health')} showLabel onchange={(on) => opts && save({ ...opts, healthChecks: on })} />
        <p class="t-small muted note">{t('settings.subs_health_note')}</p>
      </div>
    </div>
  {/if}
</Card>

<style>
  .head { margin-bottom: var(--s4); }
  .stack { display: flex; flex-direction: column; gap: var(--s5); }
  .option { display: flex; flex-direction: column; gap: var(--s2); }
  .note { max-width: 64ch; }
</style>
