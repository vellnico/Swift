<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../lib/api';
  import { bytes, duration, mbit, percent, plural } from '../lib/format';
  import { t } from '../lib/i18n/index.svelte';
  import { fadeShift, stagger } from '../lib/motion';
  import { live } from '../lib/state.svelte';
  import { href } from '../lib/router.svelte';
  import { stats, watchStats } from '../lib/stats.svelte';
  import Alert from '../lib/ui/Alert.svelte';
  import Badge from '../lib/ui/Badge.svelte';
  import Card from '../lib/ui/Card.svelte';
  import LiveNumber from '../lib/ui/LiveNumber.svelte';
  import Skeleton from '../lib/ui/Skeleton.svelte';
  import Sparkline from '../lib/ui/Sparkline.svelte';
  import StatusDot from '../lib/ui/StatusDot.svelte';
  import Page from '../shell/Page.svelte';

  interface Overview {
    profiles: number;
    profilesEnabled: number;
    users: number;
    usersEnabled: number;
    cert?: { kind: string; lastError: string };
    xrayUpdate?: string;
  }

  let overview = $state<Overview | null>(null);
  let now = $state(Date.now());

  onMount(() => {
    let stop: (() => void) | undefined;
    watchStats().then((s) => (stop = s));
    api<Overview>('overview').then((o) => (overview = o)).catch(() => {});
    const clock = setInterval(() => (now = Date.now()), 30_000);
    return () => {
      stop?.();
      clearInterval(clock);
    };
  });

  const samples = $derived(stats.samples);
  const last = $derived(samples[samples.length - 1]);
  const mem = $derived(last ? bytes(last.memUsed) : null);
  const netMax = $derived(Math.max(1, ...samples.map((s) => Math.max(s.rxBps, s.txBps))));
  const xraySince = $derived(live.xraySince ? Math.max(0, (now - Date.parse(live.xraySince)) / 1000) : 0);
  const hasMetrics = $derived(stats.loaded && samples.length > 0);
</script>

<Page title={t('nav.dashboard')}>
  {#if overview?.cert?.lastError}
    <div in:fadeShift><Alert kind="warning">{t('dash.cert_error', { error: overview.cert.lastError })}</Alert></div>
  {/if}
  {#if overview?.xrayUpdate}
    <div in:fadeShift>
      <Alert kind="warning">{t('dash.xray_update', { version: overview.xrayUpdate })} <a href={href('/settings')}>{t('dash.xray_update_action')}</a></Alert>
    </div>
  {/if}

  {#if !stats.loaded}
    <div class="hero">
      {#each [0, 1, 2] as i (i)}<div class="metric"><Skeleton width="60%" height="56px" /><Skeleton width="40%" height="14px" /><Skeleton height="48px" /></div>{/each}
    </div>
  {:else if hasMetrics && last}
    <div class="hero">
      <section class="metric" in:fadeShift={{ delay: stagger(0) }}>
        <p class="t-hero num"><LiveNumber value={last.cpu} format={(v) => percent(v)} /><span class="unit">%</span></p>
        <p class="t-small muted">{t('dash.cpu')}</p>
        <Sparkline values={samples.map((s) => s.cpu)} max={100} label={t('dash.cpu')} />
      </section>
      <section class="metric" in:fadeShift={{ delay: stagger(1) }}>
        <p class="t-hero num"><LiveNumber value={last.memUsed} format={(v) => bytes(v).value} /><span class="unit">{mem?.unit}</span></p>
        <p class="t-small muted">{t('dash.memory')}</p>
        <Sparkline values={samples.map((s) => s.memUsed)} max={last.memTotal} label={t('dash.memory')} />
      </section>
      <section class="metric" in:fadeShift={{ delay: stagger(2) }}>
        <p class="t-hero num net">
          <span class="dir" aria-label={t('dash.up')}>↑</span><LiveNumber value={last.txBps} format={(v) => mbit(v)} />
          <span class="dir" aria-label={t('dash.down')}>↓</span><LiveNumber value={last.rxBps} format={(v) => mbit(v)} />
        </p>
        <p class="t-small muted">{t('dash.network')}</p>
        <div class="net-sparks">
          <Sparkline values={samples.map((s) => s.rxBps)} max={netMax} label={t('dash.down')} />
          <div class="tx"><Sparkline values={samples.map((s) => s.txBps)} max={netMax} label={t('dash.up')} /></div>
        </div>
      </section>
    </div>
  {:else}
    <p class="muted">{t('dash.no_metrics')}</p>
  {/if}

  <div class="cards">
    <div in:fadeShift={{ delay: stagger(3) }}>
      <Card>
        <p class="t-small muted label">{t('dash.xray')}</p>
        <StatusDot status={live.xray} label={t(`xray.status.${live.xray}`)} />
        <div class="row">
          {#if live.xrayVersion}<Badge><span class="t-code">{live.xrayVersion}</span></Badge>{/if}
          {#if live.xray === 'ok' && xraySince}<span class="t-small muted">{t('dash.xray_since', { time: duration(xraySince) })}</span>{/if}
        </div>
      </Card>
    </div>
    <div in:fadeShift={{ delay: stagger(4) }}>
      <Card>
        <p class="t-small muted label">{t('dash.access')}</p>
        {#if overview}
          <p class="t-heading">{plural('dash.profiles', overview.profilesEnabled)}</p>
          <p class="t-heading">{plural('dash.people', overview.usersEnabled)}</p>
        {:else}
          <Skeleton width="50%" height="20px" />
        {/if}
      </Card>
    </div>
    {#if last && last.diskTotal > 0}
      <div in:fadeShift={{ delay: stagger(5) }}>
        <Card>
          <p class="t-small muted label">{t('dash.disk')}</p>
          <p class="t-heading">{t('dash.disk_used', { used: `${bytes(last.diskUsed).value} ${bytes(last.diskUsed).unit}`, total: `${bytes(last.diskTotal).value} ${bytes(last.diskTotal).unit}` })}</p>
          <div class="bar" role="presentation"><span style:transform="scaleX({last.diskUsed / last.diskTotal})"></span></div>
        </Card>
      </div>
    {/if}
    {#if last && last.uptime > 0}
      <div in:fadeShift={{ delay: stagger(6) }}>
        <Card>
          <p class="t-small muted label">{t('dash.server')}</p>
          <p class="t-heading">{t('dash.server_uptime', { time: duration(last.uptime) })}</p>
        </Card>
      </div>
    {/if}
  </div>
</Page>

<style>
  .hero {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: var(--s8);
  }
  @media (max-width: 1100px) {
    .hero { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  }
  @media (max-width: 640px) {
    .hero { grid-template-columns: 1fr; gap: var(--s6); }
  }
  .metric { display: flex; flex-direction: column; gap: var(--s2); min-width: 0; }
  .num { display: flex; align-items: baseline; gap: var(--s1); white-space: nowrap; }
  .unit { font: var(--t-title); letter-spacing: var(--t-title-track); color: var(--text-2); margin-left: var(--s1); }
  .net { gap: var(--s2); }
  .dir { font: var(--t-title); color: var(--text-2); }
  .dir:not(:first-child) { margin-left: var(--s3); }
  .metric :global(.spark) { margin-top: var(--s3); }
  .net-sparks { position: relative; }
  .tx { position: absolute; inset: 0; opacity: 0.55; }
  .cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: var(--s4); }
  .cards :global(.card) { display: flex; flex-direction: column; gap: var(--s2); height: 100%; }
  .label { margin-bottom: var(--s1); }
  .row { display: flex; align-items: center; gap: var(--s3); flex-wrap: wrap; }
  .bar { height: 4px; border-radius: var(--radius-pill); background: var(--surface-3); overflow: hidden; margin-top: var(--s2); }
  .bar span { display: block; height: 100%; background: var(--accent); transform-origin: left; transition: transform var(--dur-slow) var(--ease-out); }
  @media (max-width: 400px) {
    .cards { grid-template-columns: 1fr; }
  }
</style>
