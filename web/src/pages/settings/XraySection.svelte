<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../../lib/api';
  import { errorText } from '../../lib/errors';
  import { i18n, t } from '../../lib/i18n/index.svelte';
  import { onLive } from '../../lib/state.svelte';
  import Alert from '../../lib/ui/Alert.svelte';
  import Badge from '../../lib/ui/Badge.svelte';
  import Button from '../../lib/ui/Button.svelte';
  import Card from '../../lib/ui/Card.svelte';
  import Skeleton from '../../lib/ui/Skeleton.svelte';
  import Spinner from '../../lib/ui/Spinner.svelte';
  import { toast } from '../../lib/ui/toasts.svelte';

  interface Release {
    current: string;
    pinned: string;
    latest: string;
    checkedAt: string;
    available: boolean;
    major: boolean;
    breaksHiddify: boolean;
    updating: boolean;
    backup: boolean;
  }

  let rel = $state<Release | null>(null);
  let checking = $state<'idle' | 'loading' | 'done'>('idle');
  let stage = $state('');
  let error = $state('');

  onMount(() => {
    api<Release>('xray/release').then((r) => (rel = r)).catch((e) => toast(errorText(e)));
    return onLive('xray_update', (data) => {
      const s = (data as { stage: string }).stage;
      stage = ['downloading', 'verifying', 'testing', 'restarting'].includes(s) ? s : '';
    });
  });

  const checked = $derived.by(() => {
    if (!rel || new Date(rel.checkedAt).getFullYear() < 2000) return t('settings.xray_never');
    const d = new Date(rel.checkedAt);
    const loc = i18n.locale === 'ru' ? 'ru-RU' : 'en-US';
    return t('settings.xray_checked', { time: d.toLocaleString(loc, { day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit' }) });
  });

  function explain(e: unknown): string {
    if (e instanceof ApiError && (e.code.startsWith('update_') || e.code === 'check_failed')) {
      return t(`settings.xray_errors.${e.code}`, { detail: String(e.data.detail ?? '').split('\n').pop() ?? '' });
    }
    return errorText(e);
  }

  async function check() {
    checking = 'loading';
    error = '';
    try {
      rel = await api<Release>('xray/check', { method: 'POST' });
      checking = 'done';
      setTimeout(() => (checking = 'idle'), 1600);
    } catch (e) {
      checking = 'idle';
      error = explain(e);
    }
  }

  async function update() {
    if (!rel) return;
    const version = rel.latest;
    error = '';
    stage = 'downloading';
    rel.updating = true;
    try {
      rel = await api<Release>('xray/update', { method: 'POST', body: { version } });
      toast(t('settings.xray_updated', { version }));
    } catch (e) {
      error = explain(e);
      rel = await api<Release>('xray/release').catch(() => rel);
    } finally {
      stage = '';
    }
  }
</script>

<Card>
  <div class="head">
    <h2 class="t-heading">{t('settings.xray')}</h2>
    {#if rel && !rel.available && rel.latest}<Badge>{t('settings.xray_up_to_date')}</Badge>{/if}
  </div>
  {#if !rel}
    <div class="stack"><Skeleton width="40%" height="18px" /><Skeleton height="40px" /></div>
  {:else}
    <div class="stack">
      <dl class="facts">
        <dt class="t-small muted">{t('settings.xray_current')}</dt>
        <dd class="t-code">{rel.current}</dd>
        {#if rel.latest}
          <dt class="t-small muted">{t('settings.xray_latest')}</dt>
          <dd class="t-code">{rel.latest}</dd>
        {/if}
      </dl>
      <p class="t-small muted">{checked}</p>
      {#if rel.available && rel.major}
        <Alert kind="warning">{t('settings.xray_major', { current: rel.current })}</Alert>
      {/if}
      {#if rel.breaksHiddify}
        <Alert kind="warning">{t('settings.xray_breaks_hiddify', { pinned: rel.pinned })}</Alert>
      {/if}
      {#if error}<Alert>{error}</Alert>{/if}
      {#if stage}
        <p class="progress" role="status"><Spinner /> {t(`settings.xray_stage.${stage}`)}</p>
      {/if}
      <div class="actions">
        {#if rel.available}
          <Button variant="primary" status={rel.updating ? 'loading' : 'idle'} disabled={rel.updating} onclick={update}>{t('settings.xray_update', { version: rel.latest })}</Button>
        {/if}
        <Button variant={rel.available ? 'quiet' : 'secondary'} status={checking} disabled={rel.updating} onclick={check}>{t('settings.xray_check')}</Button>
      </div>
      <p class="t-small muted note">{t('settings.xray_note')}</p>
    </div>
  {/if}
</Card>

<style>
  .head { display: flex; align-items: center; justify-content: space-between; gap: var(--s3); margin-bottom: var(--s4); }
  .stack { display: flex; flex-direction: column; gap: var(--s4); }
  .facts { display: grid; grid-template-columns: max-content 1fr; gap: var(--s2) var(--s6); margin: 0; }
  .facts dd { margin: 0; }
  .progress { display: flex; align-items: center; gap: var(--s2); font: var(--t-small); }
  .actions { display: flex; gap: var(--s3); flex-wrap: wrap; }
  .note { max-width: 64ch; }
</style>
