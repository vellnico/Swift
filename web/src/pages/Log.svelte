<script lang="ts">
  import { onMount } from 'svelte';
  import { flip } from 'svelte/animate';
  import Info from '@lucide/svelte/icons/info';
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
  import CircleAlert from '@lucide/svelte/icons/circle-alert';
  import { api } from '../lib/api';
  import { errorText } from '../lib/errors';
  import { i18n, t } from '../lib/i18n/index.svelte';
  import { dur, fadeShift, reducedMotion } from '../lib/motion';
  import { radioKeys, radioTabIndex } from '../lib/a11y';
  import { onLive } from '../lib/state.svelte';
  import { eventText, type JournalEvent, type Level } from '../lib/journal';
  import Button from '../lib/ui/Button.svelte';
  import Card from '../lib/ui/Card.svelte';
  import EmptyState from '../lib/ui/EmptyState.svelte';
  import Skeleton from '../lib/ui/Skeleton.svelte';
  import { toast } from '../lib/ui/toasts.svelte';
  import Page from '../shell/Page.svelte';

  const PAGE = 50;
  const levels: ('' | Level)[] = ['', 'info', 'warn', 'error'];
  const icons = { info: Info, warn: TriangleAlert, error: CircleAlert };

  let level = $state<'' | Level>('');
  let events = $state<JournalEvent[] | null>(null);
  let more = $state(false);
  let loading = $state<'idle' | 'loading' | 'done'>('idle');
  let request = 0;

  async function load(append: boolean) {
    const id = ++request;
    const before = append && events?.length ? events[events.length - 1]!.id : 0;
    const q = [`limit=${PAGE}`, before ? `before=${before}` : '', level ? `level=${level}` : ''].filter(Boolean).join('&');
    if (append) loading = 'loading';
    try {
      const page = await api<JournalEvent[]>(`journal?${q}`);
      if (id !== request) return;
      events = append && events ? [...events, ...page] : page;
      more = page.length === PAGE;
    } catch (e) {
      toast(errorText(e));
      if (!append) events = [];
    } finally {
      loading = 'idle';
    }
  }

  function setLevel(l: '' | Level) {
    if (l === level) return;
    level = l;
    events = null;
    load(false);
  }

  onMount(() => {
    load(false);
    return onLive('event', (data) => {
      const ev = data as JournalEvent;
      if (!events || (level && ev.level !== level) || events.some((e) => e.id === ev.id)) return;
      events = [ev, ...events];
    });
  });

  function when(ts: string): { time: string; date: string } {
    const d = new Date(ts);
    const loc = i18n.locale === 'ru' ? 'ru-RU' : 'en-US';
    const today = new Date().toDateString() === d.toDateString();
    return {
      time: d.toLocaleTimeString(loc, { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
      date: today ? '' : d.toLocaleDateString(loc, { day: 'numeric', month: 'short' }),
    };
  }
</script>

<Page title={t('nav.log')}>
  <div class="filter" role="radiogroup" aria-label={t('log.filter')} tabindex="-1" onkeydown={radioKeys}>
    {#each levels as l (l)}
      <button type="button" role="radio" aria-checked={level === l} tabindex={radioTabIndex(level === l)} class:active={level === l} onclick={() => setLevel(l)}>
        {t(`log.level.${l || 'all'}`)}
      </button>
    {/each}
  </div>

  {#if events === null}
    <Card class="feed">
      {#each [0, 1, 2, 3, 4] as i (i)}
        <div class="row skel"><Skeleton width="56px" height="14px" /><Skeleton width={`${70 - i * 8}%`} height="14px" /></div>
      {/each}
    </Card>
  {:else if events.length === 0}
    <Card><EmptyState text={level ? t('log.empty_filtered') : t('log.empty')} /></Card>
  {:else}
    <Card class="feed">
      <ol aria-live="polite">
        {#each events as ev (ev.id)}
          {@const w = when(ev.ts)}
          {@const Icon = icons[ev.level]}
          <li
            class="row {ev.level}"
            in:fadeShift={{ y: -6, duration: dur.base }}
            animate:flip={{ duration: reducedMotion() ? 0 : dur.base }}
          >
            <time class="t-code muted" datetime={ev.ts}>
              {#if w.date}<span class="date">{w.date}</span>{/if}
              {w.time}
            </time>
            <span class="icon" aria-label={t(`log.level.${ev.level}`)}><Icon size={16} strokeWidth={1.5} /></span>
            <span class="text">{eventText(ev)}</span>
          </li>
        {/each}
      </ol>
    </Card>
    {#if more}
      <div class="more"><Button variant="quiet" status={loading} onclick={() => load(true)}>{t('log.more')}</Button></div>
    {/if}
  {/if}
</Page>

<style>
  .filter {
    display: inline-flex;
    align-self: flex-start;
    margin-top: calc(var(--s4) * -1);
    padding: 2px;
    border-radius: var(--radius-pill);
    box-shadow: inset 0 0 0 1px var(--line);
    max-width: 100%;
    overflow-x: auto;
    scrollbar-width: none;
  }
  .filter button {
    height: calc(var(--space) * 8);
    padding: 0 var(--s3);
    border: 0;
    border-radius: var(--radius-pill);
    background: transparent;
    color: var(--text-2);
    font: var(--t-small);
    white-space: nowrap;
    transition: background var(--dur-fast) var(--ease-out), color var(--dur-fast) var(--ease-out);
  }
  .filter button:hover { color: var(--text-1); }
  .filter .active { background: var(--surface-3); color: var(--text-1); }
  :global(.feed) { padding: var(--s2) !important; }
  ol { list-style: none; margin: 0; padding: 0; }
  .row {
    display: grid;
    grid-template-columns: max-content max-content minmax(0, 1fr);
    align-items: baseline;
    gap: var(--s3);
    padding: var(--s3);
  }
  .row + .row { box-shadow: inset 0 1px 0 var(--line); }
  .skel { grid-template-columns: max-content 1fr; align-items: center; }
  time { white-space: nowrap; font-variant-numeric: tabular-nums; }
  .date { margin-right: var(--s1); }
  .icon { display: inline-flex; align-self: center; color: var(--text-3); }
  .warn .icon { color: var(--text-2); }
  .error .icon { color: var(--text-1); }
  .error .text { color: var(--text-1); font-weight: 500; }
  .text { min-width: 0; overflow-wrap: anywhere; color: var(--text-2); }
  .info .text { color: var(--text-1); }
  .more { display: flex; justify-content: center; }
  @media (max-width: 959px) {
    .row { grid-template-columns: max-content minmax(0, 1fr); row-gap: var(--s1); }
    .row time { grid-column: 1 / -1; order: -1; }
  }
</style>
