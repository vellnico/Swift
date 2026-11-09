<script lang="ts">
  import Shuffle from '@lucide/svelte/icons/shuffle';
  import { api } from '../../lib/api';
  import { errorText, fieldErrors } from '../../lib/errors';
  import { t } from '../../lib/i18n/index.svelte';
  import Button from '../../lib/ui/Button.svelte';
  import Card from '../../lib/ui/Card.svelte';
  import CopyField from '../../lib/ui/CopyField.svelte';
  import Input from '../../lib/ui/Input.svelte';
  import Modal from '../../lib/ui/Modal.svelte';
  import Skeleton from '../../lib/ui/Skeleton.svelte';
  import { toast } from '../../lib/ui/toasts.svelte';
  import { panelURL, randomPath, type PanelInfo } from './panel';

  let { info = $bindable() }: { info: PanelInfo | null } = $props();
  let port = $state('');
  let path = $state('');
  let errors = $state<Record<string, string>>({});
  let confirmOpen = $state(false);
  let status = $state<'idle' | 'loading' | 'done'>('idle');
  let loaded = false;

  $effect(() => {
    if (info && !loaded) {
      port = String(info.port);
      path = info.path;
      loaded = true;
    }
  });

  const changed = $derived(!!info && (port.trim() !== String(info.port) || path.trim() !== info.path));
  const target = $derived(panelURL(location, Number(port) || 0, path.trim()));

  function ask(e: SubmitEvent) {
    e.preventDefault();
    errors = {};
    confirmOpen = true;
  }

  async function move() {
    confirmOpen = false;
    status = 'loading';
    const url = target;
    try {
      info = await api<PanelInfo>('settings/panel', { method: 'PUT', body: { port: Number(port), path: path.trim() } });
      status = 'done';
      toast(t('settings.panel_moved'));
      setTimeout(() => location.assign(url), 1500);
    } catch (err) {
      status = 'idle';
      errors = fieldErrors(err, 'settings.panel');
      if (!Object.keys(errors).length) toast(errorText(err));
    }
  }
</script>

<Card>
  <h2 class="t-heading">{t('settings.panel')}</h2>
  {#if !info}
    <Skeleton height="40px" />
  {:else}
    <form onsubmit={ask} novalidate>
      {#if info.pinned}<p class="t-small muted">{t('settings.panel_pinned')}</p>{/if}
      <div class="grid">
        <Input label={t('settings.panel_port')} hint={t('settings.panel_port_hint')} bind:value={port} mono inputmode="numeric" disabled={info.pinned} error={errors.port} />
        <Input label={t('settings.panel_path')} hint={t('settings.panel_path_hint')} bind:value={path} mono autocomplete="off" spellcheck="false" disabled={info.pinned} error={errors.path} />
      </div>
      {#if !info.pinned}
        <div class="actions">
          <Button type="submit" variant="secondary" {status} disabled={!changed}>{t('settings.panel_change')}</Button>
          <Button variant="quiet" onclick={() => (path = randomPath())}>
            {#snippet icon()}<Shuffle size={18} strokeWidth={1.5} />{/snippet}
            {t('settings.panel_generate')}
          </Button>
        </div>
      {/if}
    </form>
  {/if}
</Card>

<Modal bind:open={confirmOpen} title={t('settings.panel_confirm_title')}>
  <div class="confirm">
    <p class="muted">{t('settings.panel_confirm')}</p>
    <CopyField label={t('settings.panel')} value={target} />
  </div>
  {#snippet footer()}
    <Button variant="quiet" onclick={() => (confirmOpen = false)}>{t('common.cancel')}</Button>
    <Button variant="primary" onclick={move}>{t('settings.panel_go')}</Button>
  {/snippet}
</Modal>

<style>
  h2 { margin-bottom: var(--s4); }
  form { display: flex; flex-direction: column; gap: var(--s4); }
  .grid { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 2fr); gap: var(--s4); }
  .actions { display: flex; gap: var(--s3); flex-wrap: wrap; }
  .confirm { display: flex; flex-direction: column; gap: var(--s4); }
  @media (max-width: 599px) {
    .grid { grid-template-columns: 1fr; }
  }
</style>
