<script lang="ts">
  import { Spring } from 'svelte/motion';
  import { t } from '../lib/i18n/index.svelte';
  import { href, router } from '../lib/router.svelte';
  import { soft, spring } from '../lib/motion';
  import { bottomItems, moreRoutes } from './nav';

  const activeIndex = $derived.by(() => {
    const name = router.route.name;
    const i = bottomItems.findIndex((it) => it.route === name);
    if (i >= 0) return i;
    return moreRoutes.includes(name) ? bottomItems.length - 1 : -1;
  });
  const x = new Spring(0, spring(soft));
  $effect(() => {
    if (activeIndex >= 0) x.target = activeIndex;
  });
</script>

<nav class="bottom" aria-label={t('nav.main')}>
  {#if activeIndex >= 0}
    <span class="bar" style:transform="translateX({x.current * 100}%)" aria-hidden="true"><span></span></span>
  {/if}
  {#each bottomItems as item, i (item.route)}
    <a href={href(item.path)} class:active={i === activeIndex} aria-current={i === activeIndex ? 'page' : undefined}>
      <item.icon size={20} strokeWidth={1.5} aria-hidden="true" />
      <span>{t(item.label)}</span>
    </a>
  {/each}
</nav>

<style>
  .bottom {
    position: fixed;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 40;
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    padding-bottom: env(safe-area-inset-bottom);
    background: var(--glass);
    backdrop-filter: blur(20px) saturate(1.2);
    -webkit-backdrop-filter: blur(20px) saturate(1.2);
    box-shadow: inset 0 1px 0 var(--line);
  }
  a {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--s1);
    height: calc(var(--space) * 16);
    color: var(--text-2);
    text-decoration: none;
    font: var(--t-micro);
    letter-spacing: var(--t-micro-track);
    -webkit-tap-highlight-color: transparent;
  }
  a.active { color: var(--text-1); }
  .bar {
    position: absolute;
    top: 0;
    left: 0;
    width: 25%;
    display: flex;
    justify-content: center;
    pointer-events: none;
  }
  .bar span { width: var(--s6); height: 2px; border-radius: 0 0 2px 2px; background: var(--accent); }
</style>
