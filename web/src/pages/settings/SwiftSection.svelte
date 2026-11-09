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
  import Switch from '../../lib/ui/Switch.svelte';
  import { toast } from '../../lib/ui/toasts.svelte';

  interface Release {
    current: string;
    latest: string;
    checkedAt: string;
    nextCheck: string;
    available: boolean;
    updating: boolean;
    auto: boolean;
    supported: boolean;
  }

  const stages = ['downloading', 'verifying', 'testing', 'backup', 'xray', 'restarting'];

  let rel = $state<Release | null>(null);
  let checking = $state<'idle' | 'loading' | 'done'>('idle');
  let stage = $state('');
  let error = $state('');

  onMount(() => {
    api<Release>('swift/release').then((r) => (rel = r)).catch((e) => toast(errorText(e)));
    return onLive('swift_update', (data) => {
      const s = (data as { stage: string }).stage;
      if (stages.includes(s)) stage = s;
      if (s === 'restarting') waitForRestart();
    });
  });

  const loc = $derived(i18n.locale === 'ru' ? 'ru-RU' : 'en-US');

  const checked = $derived.by(() => {
    if (!rel || new Date(rel.checkedAt).getFullYear() < 2000) return t('settings.xray_never');
    const d = new Date(rel.checkedAt);
    return t('settings.xray_checked', { time: d.toLocaleString(loc, { day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit' }) });
  });

  const next = $derived(rel ? new Date(rel.nextCheck).toLocaleTimeString(loc, { hour: '2-digit', minute: '2-digit' }) : '');

  function explain(e: unknown): string {
    if (e instanceof ApiError && (e.code.startsWith('update_') || e.code === 'check_failed')) {
      return t(`settings.swift_errors.${e.code}`, { detail: String(e.data.detail ?? '').split('\n').pop() ?? '' });
    }
    return errorText(e);
  }

  async function check() {
    checking = 'loading';
    error = '';
    try {
      rel = await api<Release>('swift/check', { method: 'POST' });
      checking = 'done';
      setTimeout(() => (checking = 'idle'), 1600);
    } catch (e) {
      checking = 'idle';
      error = explain(e);
    }
  }

  async function setAuto(on: boolean) {
    try {
      rel = await api<Release>('swift/auto', { method: 'PUT', body: { enabled: on } });
      toast(on ? t('settings.swift_auto_on', { time: next }) : t('settings.swift_auto_off'));
    } catch (e) {
      toast(errorText(e));
      rel = await api<Release>('swift/release').catch(() => rel);
    }
  }

  async function update() {
    if (!rel) return;
    error = '';
    stage = 'downloading';
    rel.updating = true;
    try {
      await api('swift/update', { method: 'POST', body: { version: rel.latest } });
      waitForRestart();
    } catch (e) {
      stage = '';
      error = explain(e);
      rel = await api<Release>('swift/release').catch(() => rel);
    }
  }

  let waiting = false;

  // The panel exits and systemd starts the new version; reload once it answers again.
  async function waitForRestart() {
    if (waiting) return;
    waiting = true;
    stage = 'restarting';
    await new Promise((r) => setTimeout(r, 4000));
    for (let i = 0; i < 60; i++) {
      try {
        await api('health');
        location.reload();
        return;
      } catch {
        await new Promise((r) => setTimeout(r, 1500));
      }
    }
    waiting = false;
    stage = '';
    error = t('settings.swift_errors.no_restart');
  }
</script>

<Card>
  <div class="head">
    <h2 class="t-heading">{t('settings.swift')}</h2>
    {#if rel && rel.supported && !rel.available && rel.latest}<Badge>{t('settings.xray_up_to_date')}</Badge>{/if}
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
      {#if !rel.supported}
        <p class="t-small muted note">{t('settings.swift_dev')}</p>
      {:else}
        <p class="t-small muted">{checked}. {t('settings.swift_next', { time: next })}</p>
        {#if error}<Alert>{error}</Alert>{/if}
        {#if stage}
          <p class="progress" role="status"><Spinner /> {t(`settings.swift_stage.${stage}`)}</p>
        {/if}
        <div class="actions">
          {#if rel.available}
            <Button variant="primary" status={rel.updating ? 'loading' : 'idle'} disabled={rel.updating} onclick={update}>{t('settings.xray_update', { version: rel.latest })}</Button>
          {/if}
          <Button variant={rel.available ? 'quiet' : 'secondary'} status={checking} disabled={rel.updating} onclick={check}>{t('settings.xray_check')}</Button>
        </div>
        <div class="auto">
          <Switch checked={rel.auto} label={t('settings.swift_auto')} showLabel disabled={rel.updating} onchange={setAuto} />
          <p class="t-small muted note">{t('settings.swift_auto_note', { time: next })}</p>
        </div>
        <p class="t-small muted note">{t('settings.swift_note')}</p>
      {/if}
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
  .auto { display: flex; flex-direction: column; gap: var(--s2); padding-top: var(--s4); border-top: 1px solid var(--line); }
  .note { max-width: 64ch; }
</style>
