<script lang="ts">
  import { onMount } from 'svelte';
  import Globe from '@lucide/svelte/icons/globe';
  import Split from '@lucide/svelte/icons/split';
  import { radioKeys, radioTabIndex } from '../lib/a11y';
  import { api } from '../lib/api';
  import { errorText } from '../lib/errors';
  import { t } from '../lib/i18n/index.svelte';
  import { fadeShift, stagger } from '../lib/motion';
  import Badge from '../lib/ui/Badge.svelte';
  import Card from '../lib/ui/Card.svelte';
  import Skeleton from '../lib/ui/Skeleton.svelte';
  import Spinner from '../lib/ui/Spinner.svelte';
  import { toast } from '../lib/ui/toasts.svelte';
  import Page from '../shell/Page.svelte';

  type Preset = 'ru_direct' | 'none';
  let preset = $state<Preset | null>(null);
  let saving = $state<Preset | null>(null);

  onMount(() => {
    api<{ preset: Preset }>('routing').then((r) => (preset = r.preset)).catch((e) => toast(errorText(e)));
  });

  const options: { id: Preset; icon: typeof Split }[] = [
    { id: 'ru_direct', icon: Split },
    { id: 'none', icon: Globe },
  ];
  const apps = ['hiddify', 'v2rayng', 'happ', 'v2raytun'] as const;
  const appNames = { hiddify: 'Hiddify', v2rayng: 'v2rayNG', happ: 'Happ', v2raytun: 'v2RayTun' };

  async function choose(p: Preset) {
    if (p === preset || saving) return;
    saving = p;
    try {
      await api('routing', { method: 'PUT', body: { preset: p } });
      preset = p;
      toast(t('routing.saved'));
    } catch (e) {
      toast(errorText(e));
    } finally {
      saving = null;
    }
  }
</script>

<Page title={t('nav.routing')}>
  <div class="options" role="radiogroup" aria-label={t('nav.routing')} tabindex="-1" onkeydown={radioKeys}>
    {#if preset === null}
      {#each options as o (o.id)}<Card><Skeleton width="50%" height="22px" /><div class="gap"></div><Skeleton height="14px" /></Card>{/each}
    {:else}
      {#each options as o, i (o.id)}
        <button
          type="button"
          role="radio"
          aria-checked={preset === o.id}
          tabindex={radioTabIndex(preset === o.id)}
          class="option"
          class:active={preset === o.id}
          onclick={() => choose(o.id)}
          in:fadeShift={{ delay: stagger(i) }}
        >
          <span class="icon"><o.icon size={22} strokeWidth={1.5} aria-hidden="true" /></span>
          <span class="t-heading">{t(`routing.${o.id}.title`)}</span>
          <span class="t-small muted desc">{t(`routing.${o.id}.desc`)}</span>
          <span class="state">
            {#if saving === o.id}<Spinner />{:else if preset === o.id}<Badge variant="solid">{t('routing.active')}</Badge>{/if}
          </span>
        </button>
      {/each}
    {/if}
  </div>

  <section class="apps">
    <h2 class="t-heading">{t('routing.apps')}</h2>
    <Card>
      <dl>
        {#each apps as a (a)}
          <dt>{appNames[a]}</dt>
          <dd class="t-small muted">{t(`routing.app.${a}`)}</dd>
        {/each}
      </dl>
    </Card>
  </section>
</Page>

<style>
  .options { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: var(--s4); }
  .gap { height: var(--s3); }
  .option {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--s3);
    min-height: calc(var(--space) * 52);
    padding: var(--s6);
    border: 0;
    border-radius: var(--radius-surface);
    background: var(--surface-1);
    box-shadow: var(--edge);
    text-align: left;
    color: var(--text-1);
    transition: transform var(--dur-fast) var(--ease-out);
  }
  .option:hover { box-shadow: var(--edge-lit); }
  .option:active { transform: scale(0.98); }
  .option:focus-visible { box-shadow: var(--edge-lit), var(--focus-ring); }
  .option.active { box-shadow: inset 0 0 0 1px var(--porcelain); }
  .icon {
    display: grid;
    place-items: center;
    width: var(--s12);
    height: var(--s12);
    border-radius: var(--radius-control);
    box-shadow: inset 0 0 0 1px var(--line-strong);
    margin-bottom: var(--s2);
  }
  .active .icon { background: var(--accent); color: var(--accent-fg); box-shadow: none; }
  .desc { max-width: 44ch; }
  .state { position: absolute; top: var(--s6); right: var(--s6); }
  .apps { display: flex; flex-direction: column; gap: var(--s4); }
  dl { display: grid; grid-template-columns: max-content 1fr; gap: var(--s3) var(--s6); margin: 0; }
  dt { font-weight: 500; }
  dd { margin: 0; }
  @media (max-width: 480px) {
    dl { grid-template-columns: 1fr; gap: var(--s1); }
    dd { margin-bottom: var(--s3); }
  }
</style>
