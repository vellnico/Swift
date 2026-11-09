<script lang="ts">
  import { errorText, fieldErrors } from '../../lib/errors';
  import { t } from '../../lib/i18n/index.svelte';
  import Button from '../../lib/ui/Button.svelte';
  import Card from '../../lib/ui/Card.svelte';
  import Input from '../../lib/ui/Input.svelte';
  import Skeleton from '../../lib/ui/Skeleton.svelte';
  import { toast } from '../../lib/ui/toasts.svelte';
  import { api } from '../../lib/api';
  import type { PanelInfo } from './panel';

  let { info = $bindable() }: { info: PanelInfo | null } = $props();
  let address = $state('');
  let error = $state('');
  let status = $state<'idle' | 'loading' | 'done'>('idle');
  let loaded = false;

  $effect(() => {
    if (info && !loaded) {
      address = info.address;
      loaded = true;
    }
  });

  async function save(e: SubmitEvent) {
    e.preventDefault();
    error = '';
    status = 'loading';
    try {
      info = await api<PanelInfo>('settings/address', { method: 'PUT', body: { address: address.trim() } });
      address = info.address;
      status = 'done';
      toast(t('settings.address_saved'));
      setTimeout(() => (status = 'idle'), 1600);
    } catch (err) {
      status = 'idle';
      error = fieldErrors(err, 'settings.address').address ?? '';
      if (!error) toast(errorText(err));
    }
  }
</script>

<Card>
  <h2 class="t-heading">{t('settings.server')}</h2>
  {#if !info}
    <Skeleton height="40px" />
  {:else}
    <form onsubmit={save} novalidate>
      <Input label={t('settings.address')} hint={t('settings.address_hint')} bind:value={address} mono inputmode="decimal" autocomplete="off" spellcheck="false" error={error || undefined} />
      <div><Button type="submit" variant="secondary" {status} disabled={address.trim() === info.address}>{t('common.save')}</Button></div>
    </form>
  {/if}
</Card>

<style>
  h2 { margin-bottom: var(--s4); }
  form { display: flex; flex-direction: column; gap: var(--s3); }
</style>
