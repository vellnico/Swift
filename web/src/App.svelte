<script lang="ts">
  import './lib/styles/fonts.css';
  import './lib/styles/tokens.css';
  import './lib/styles/base.css';
  import { MediaQuery } from 'svelte/reactivity';
  import { i18n, t } from './lib/i18n/index.svelte';
  import { router } from './lib/router.svelte';
  import { section } from './lib/motion';
  import ToastHost from './lib/ui/ToastHost.svelte';
  import Sidebar from './shell/Sidebar.svelte';
  import BottomNav from './shell/BottomNav.svelte';
  import Placeholder from './pages/Placeholder.svelte';
  import More from './pages/More.svelte';
  import NotFound from './pages/NotFound.svelte';

  const desktop = new MediaQuery('(min-width: 960px)');
  const route = $derived(router.route);

  $effect(() => {
    document.documentElement.lang = i18n.locale;
  });

  const titles: Record<string, string> = {
    dashboard: 'nav.dashboard',
    profiles: 'nav.profiles',
    users: 'nav.users',
    routing: 'nav.routing',
    settings: 'nav.settings',
    log: 'nav.log',
  };
</script>

<div class="shell" class:desktop={desktop.current}>
  {#if desktop.current}<Sidebar />{/if}
  <main class="content">
    {#key route.name}
      <div class="view" in:section>
        {#if route.name === 'kit' && import.meta.env.DEV}
          {#await import('./pages/Kit.svelte') then Kit}<Kit.default />{/await}
        {:else if route.name === 'more'}
          <More />
        {:else if route.name === 'notfound'}
          <NotFound />
        {:else}
          <Placeholder title={t(titles[route.name] ?? 'nav.dashboard')} />
        {/if}
      </div>
    {/key}
  </main>
  {#if !desktop.current}<BottomNav />{/if}
</div>
<ToastHost />

<style>
  .shell { min-height: 100dvh; }
  .shell.desktop { display: grid; grid-template-columns: var(--sidebar-w) minmax(0, 1fr); }
  .content {
    width: 100%;
    max-width: calc(var(--content-max) + var(--s10) * 2);
    padding: var(--s6) var(--s4) calc(var(--s16) + var(--s10) + env(safe-area-inset-bottom));
  }
  .desktop .content { padding: var(--s10); }
  .view { min-width: 0; }
</style>
