<script lang="ts">
  import './lib/styles/fonts.css';
  import './lib/styles/tokens.css';
  import './lib/styles/base.css';
  import { onMount } from 'svelte';
  import { MediaQuery } from 'svelte/reactivity';
  import { i18n, t } from './lib/i18n/index.svelte';
  import { navigate, router } from './lib/router.svelte';
  import { dur, fadeShift, section } from './lib/motion';
  import { checkSession, session } from './lib/session.svelte';
  import { connectLive, disconnectLive } from './lib/state.svelte';
  import ToastHost from './lib/ui/ToastHost.svelte';
  import Sidebar from './shell/Sidebar.svelte';
  import BottomNav from './shell/BottomNav.svelte';
  import LogoFlight from './shell/LogoFlight.svelte';
  import Login from './pages/Login.svelte';
  import Placeholder from './pages/Placeholder.svelte';
  import More from './pages/More.svelte';
  import Profiles from './pages/Profiles.svelte';
  import { onGlobalKey } from './lib/hotkeys';
  import NotFound from './pages/NotFound.svelte';

  const desktop = new MediaQuery('(min-width: 960px)');
  const route = $derived(router.route);
  let flightFrom = $state<DOMRect | null>(null);
  let entering = $state(false);

  onMount(checkSession);

  $effect(() => {
    document.documentElement.lang = i18n.locale;
  });

  $effect(() => {
    if (session.status === 'authed') connectLive();
    else disconnectLive();
  });

  function signedIn(rect: DOMRect | null) {
    flightFrom = rect;
    entering = true;
    navigate('/');
    session.status = 'authed';
  }

  const titles: Record<string, string> = {
    dashboard: 'nav.dashboard',
    profiles: 'nav.profiles',
    users: 'nav.users',
    routing: 'nav.routing',
    settings: 'nav.settings',
    log: 'nav.log',
  };
</script>

<svelte:window onkeydown={(e) => session.status === 'authed' && onGlobalKey(e)} />

{#if session.status === 'anon'}
  <Login onsuccess={signedIn} />
{:else if session.status === 'authed'}
  <div class="shell" class:desktop={desktop.current} in:fadeShift={{ duration: entering ? dur.slow : dur.base, delay: entering ? dur.base : 0, y: entering ? 8 : 0 }}>
    {#if desktop.current}<Sidebar hideLogo={!!flightFrom} />{/if}
    <main class="content">
      {#key route.name}
        <div class="view" in:section>
          {#if route.name === 'kit' && import.meta.env.DEV}
            {#await import('./pages/Kit.svelte') then Kit}<Kit.default />{/await}
          {:else if route.name === 'more'}
            <More />
          {:else if route.name === 'profiles'}
            <Profiles />
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
  {#if flightFrom}
    <LogoFlight from={flightFrom} ondone={() => ((flightFrom = null), (entering = false))} />
  {/if}
{/if}
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
