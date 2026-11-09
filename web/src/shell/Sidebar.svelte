<script lang="ts">
  import { Spring } from 'svelte/motion';
  import { t } from '../lib/i18n/index.svelte';
  import { href, router } from '../lib/router.svelte';
  import { live } from '../lib/state.svelte';
  import { soft, spring } from '../lib/motion';
  import Logo from '../lib/ui/Logo.svelte';
  import StatusDot from '../lib/ui/StatusDot.svelte';
  import LangSwitch from './LangSwitch.svelte';
  import { sidebarItems } from './nav';
  import LogOut from '@lucide/svelte/icons/log-out';
  import IconButton from '../lib/ui/IconButton.svelte';
  import { logout } from '../lib/session.svelte';

  let { hideLogo = false }: { hideLogo?: boolean } = $props();

  const links: HTMLElement[] = $state([]);
  const bar = new Spring({ y: 0, h: 0 }, spring(soft));
  const activeIndex = $derived(sidebarItems.findIndex((i) => i.route === router.route.name));
  let placed = false;

  $effect(() => {
    const el = links[activeIndex];
    if (!el) return;
    const target = { y: el.offsetTop, h: el.offsetHeight };
    if (!placed) {
      bar.set(target, { instant: true });
      placed = true;
    } else bar.target = target;
  });

  const statusLabel = $derived(t(`xray.status.${live.xray}`));
</script>

<aside class="sidebar">
  <a class="brand" href={href('/')} aria-label="Swift">
    <span data-logo-target style:visibility={hideLogo ? 'hidden' : undefined}><Logo size={22} /></span>
  </a>
  <nav aria-label={t('nav.main')}>
    <ul>
      {#each sidebarItems as item, i (item.route)}
        <li>
          <a
            bind:this={links[i]}
            href={href(item.path)}
            class="link"
            class:active={i === activeIndex}
            aria-current={i === activeIndex ? 'page' : undefined}
          >
            <item.icon size={18} strokeWidth={1.5} aria-hidden="true" />
            <span>{t(item.label)}</span>
          </a>
        </li>
      {/each}
    </ul>
    {#if activeIndex >= 0}
      <span class="bar" style:transform="translateY({bar.current.y}px)" style:height="{bar.current.h}px" aria-hidden="true"></span>
    {/if}
  </nav>
  <div class="foot">
    <StatusDot status={live.xray} label={statusLabel} />
    <div class="tools">
      <LangSwitch />
      <IconButton label={t('nav.logout')} onclick={logout}><LogOut size={18} strokeWidth={1.5} /></IconButton>
    </div>
  </div>
</aside>

<style>
  .sidebar {
    position: sticky;
    top: 0;
    display: flex;
    flex-direction: column;
    width: var(--sidebar-w);
    height: 100dvh;
    padding: var(--s6) var(--s4);
    background: var(--surface-1);
    box-shadow: inset -1px 0 0 var(--line);
  }
  .brand {
    display: inline-flex;
    align-self: flex-start;
    padding: var(--s2);
    margin-bottom: var(--s8);
    text-decoration: none;
  }
  nav { position: relative; }
  ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--s1); }
  .link {
    display: flex;
    align-items: center;
    gap: var(--s3);
    height: var(--control-h);
    padding: 0 var(--s3);
    border-radius: var(--radius-control);
    color: var(--text-2);
    text-decoration: none;
  }
  .link:hover { color: var(--text-1); background: var(--surface-2); }
  .link.active { color: var(--text-1); }
  .bar {
    position: absolute;
    left: calc(var(--s4) * -1);
    top: 0;
    width: 2px;
    border-radius: 0 2px 2px 0;
    background: var(--accent);
    pointer-events: none;
  }
  .foot {
    margin-top: auto;
    display: flex;
    flex-direction: column;
    align-items: stretch;
    gap: var(--s4);
    padding: var(--s2);
  }
  .tools { display: flex; align-items: center; justify-content: space-between; }
</style>
