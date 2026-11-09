<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../../lib/api';
  import { errorText } from '../../lib/errors';
  import { i18n, t } from '../../lib/i18n/index.svelte';
  import { onLive } from '../../lib/state.svelte';
  import { href } from '../../lib/router.svelte';
  import Button from '../../lib/ui/Button.svelte';
  import Card from '../../lib/ui/Card.svelte';
  import Modal from '../../lib/ui/Modal.svelte';
  import Skeleton from '../../lib/ui/Skeleton.svelte';
  import StatusDot from '../../lib/ui/StatusDot.svelte';
  import { toast } from '../../lib/ui/toasts.svelte';

  interface Fix {
    action: 'target';
    value: string;
  }
  interface ProfileHealth {
    profileId: number;
    name: string;
    ok: boolean;
    problem?: 'xray_down' | 'target_unreachable' | 'no_traffic';
    latencyMs?: number;
    fix?: Fix;
  }
  interface Report {
    checkedAt: string;
    running: boolean;
    enabled: boolean;
    profiles: ProfileHealth[];
  }

  let report = $state<Report | null>(null);
  let checking = $state<'idle' | 'loading' | 'done'>('idle');
  let pending = $state<{ profile: ProfileHealth; fix: Fix } | null>(null);
  let confirmOpen = $state(false);

  onMount(() => {
    api<Report>('diagnostics').then((r) => (report = r)).catch(() => {});
    return onLive('health', (data) => (report = data as Report));
  });

  const checked = $derived.by(() => {
    if (!report || new Date(report.checkedAt).getFullYear() < 2000) return t('health.never');
    const loc = i18n.locale === 'ru' ? 'ru-RU' : 'en-US';
    return t('health.checked', { time: new Date(report.checkedAt).toLocaleTimeString(loc, { hour: '2-digit', minute: '2-digit' }) });
  });

  async function check() {
    checking = 'loading';
    try {
      report = await api<Report>('diagnostics/check', { method: 'POST' });
      checking = 'done';
      setTimeout(() => (checking = 'idle'), 1600);
    } catch (e) {
      checking = 'idle';
      toast(e instanceof ApiError && e.code === 'check_busy' ? t('health.busy') : errorText(e));
    }
  }

  function ask(profile: ProfileHealth, fix: Fix) {
    pending = { profile, fix };
    confirmOpen = true;
  }

  async function apply() {
    if (!pending) return;
    const { profile, fix } = pending;
    confirmOpen = false;
    try {
      report = await api<Report>('diagnostics/fix', { method: 'POST', body: { profileId: profile.profileId, ...fix } });
      toast(t('health.fixed', { name: profile.name, value: fix.value }));
    } catch (e) {
      toast(errorText(e));
    }
  }
</script>

<Card>
  <div class="head">
    <p class="t-small muted">{t('health.title')}</p>
    <Button variant="quiet" status={report?.running ? 'loading' : checking} disabled={report?.running} onclick={check}>{t('health.check')}</Button>
  </div>
  {#if !report}
    <Skeleton height="40px" />
  {:else}
    {#if report.profiles.length === 0}
      <p class="t-small muted">{report.enabled ? checked : t('health.off')}</p>
    {:else}
      <ul class="list">
        {#each report.profiles as p (p.profileId)}
          <li>
            <div class="line">
              <StatusDot status={p.ok ? 'ok' : 'off'} label={p.name} />
              <span class="t-small muted">{p.ok ? (p.latencyMs ? t('health.ok', { ms: p.latencyMs }) : t('health.ok_plain')) : t(`health.problem.${p.problem}`)}</span>
            </div>
            {#if p.fix}
              <div class="fix">
                <Button variant="secondary" onclick={() => p.fix && ask(p, p.fix)}>{t('health.fix.target', { value: p.fix.value })}</Button>
              </div>
            {/if}
          </li>
        {/each}
      </ul>
      <p class="t-small muted">{checked}{#if !report.enabled}. {t('health.off')}{/if}</p>
    {/if}
    {#if !report.enabled}<a class="t-small" href={href('/settings')}>{t('health.settings')}</a>{/if}
  {/if}
</Card>

<Modal bind:open={confirmOpen} title={t('health.fix_title')}>
  <p class="muted">{pending ? t('health.fix_confirm', { name: pending.profile.name, value: pending.fix.value }) : ''}</p>
  {#snippet footer()}
    <Button variant="quiet" onclick={() => (confirmOpen = false)}>{t('common.cancel')}</Button>
    <Button variant="primary" onclick={apply}>{t('health.fix_apply')}</Button>
  {/snippet}
</Modal>

<style>
  .head { display: flex; align-items: center; justify-content: space-between; gap: var(--s3); }
  .list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--s3); }
  .line { display: flex; align-items: center; justify-content: space-between; gap: var(--s3); flex-wrap: wrap; }
  .fix { margin-top: var(--s2); }
</style>
