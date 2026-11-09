<script lang="ts">
  import ChevronRight from '@lucide/svelte/icons/chevron-right';
  import { t } from '../lib/i18n/index.svelte';
  import { href } from '../lib/router.svelte';
  import Card from '../lib/ui/Card.svelte';
  import LangSwitch from '../shell/LangSwitch.svelte';
  import Page from '../shell/Page.svelte';
  import { sidebarItems } from '../shell/nav';

  const items = sidebarItems.filter((i) => ['routing', 'settings', 'log'].includes(i.route));
</script>

<Page title={t('nav.more')}>
  <Card class="list">
    <ul>
      {#each items as item (item.route)}
        <li>
          <a href={href(item.path)}>
            <item.icon size={18} strokeWidth={1.5} aria-hidden="true" />
            <span>{t(item.label)}</span>
            <ChevronRight size={18} strokeWidth={1.5} aria-hidden="true" class="chev" />
          </a>
        </li>
      {/each}
    </ul>
  </Card>
  <div class="lang">
    <span class="muted">{t('lang.label')}</span>
    <LangSwitch />
  </div>
</Page>

<style>
  :global(.list) { padding: var(--s2) !important; }
  ul { list-style: none; margin: 0; padding: 0; }
  a {
    display: flex;
    align-items: center;
    gap: var(--s3);
    height: calc(var(--space) * 13);
    padding: 0 var(--s3);
    border-radius: var(--radius-control);
    color: var(--text-1);
    text-decoration: none;
  }
  a:hover { background: var(--surface-2); }
  a :global(svg) { color: var(--text-2); }
  a span { flex: 1; }
  .lang { display: flex; align-items: center; justify-content: space-between; padding: 0 var(--s2); }
</style>
