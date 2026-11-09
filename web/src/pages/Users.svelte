<script lang="ts">
  import { onMount } from 'svelte';
  import { flip } from 'svelte/animate';
  import Plus from '@lucide/svelte/icons/plus';
  import Search from '@lucide/svelte/icons/search';
  import { errorText } from '../lib/errors';
  import { hotkey } from '../lib/hotkeys';
  import { t } from '../lib/i18n/index.svelte';
  import { dur, fadeShift, stagger } from '../lib/motion';
  import { deleteUser, loadUsers, matches, reissueUser, setEnabled, users, type User } from '../lib/users.svelte';
  import Button from '../lib/ui/Button.svelte';
  import Card from '../lib/ui/Card.svelte';
  import EmptyState from '../lib/ui/EmptyState.svelte';
  import Skeleton from '../lib/ui/Skeleton.svelte';
  import { toast } from '../lib/ui/toasts.svelte';
  import Page from '../shell/Page.svelte';
  import UserAdd from './users/UserAdd.svelte';
  import UserCard from './users/UserCard.svelte';
  import UserDetail from './users/UserDetail.svelte';

  let adding = $state(false);
  let openId = $state<number | null>(null);
  let query = $state('');
  let busy = $state<number[]>([]);
  let firstLoad = $state(!users.loaded);
  let loadError = $state('');
  let search = $state<HTMLInputElement>();

  onMount(() => {
    loadUsers()
      .catch((e) => (loadError = errorText(e)))
      .finally(() => setTimeout(() => (firstLoad = false), 600));
    const offN = hotkey('n', () => (adding = true));
    const offSlash = hotkey('/', () => search?.focus());
    return () => (offN(), offSlash());
  });

  const visible = $derived(users.list.filter((u) => !users.hidden.includes(u.id) && matches(u, query)));
  const total = $derived(users.list.filter((u) => !users.hidden.includes(u.id)).length);

  async function toggle(u: User, on: boolean) {
    busy = [...busy, u.id];
    try {
      await setEnabled(u.id, on);
    } catch (e) {
      toast(errorText(e));
      users.list = [...users.list];
    } finally {
      busy = busy.filter((x) => x !== u.id);
    }
  }

  function remove(u: User) {
    users.hidden = [...users.hidden, u.id];
    toast(t('users.deleted'), {
      action: { label: t('common.undo'), run: () => (users.hidden = users.hidden.filter((x) => x !== u.id)) },
      onexpire: () => {
        deleteUser(u.id).catch((e) => {
          users.hidden = users.hidden.filter((x) => x !== u.id);
          toast(errorText(e));
        });
      },
    });
  }

  function reissue(u: User) {
    toast(t('users.reissue_pending'), {
      action: { label: t('common.undo'), run: () => {} },
      onexpire: () => {
        reissueUser(u.id)
          .then(() => toast(t('users.reissued')))
          .catch((e) => toast(errorText(e)));
      },
    });
  }
</script>

<Page title={t('nav.users')}>
  {#snippet action()}
    <Button variant="primary" onclick={() => (adding = true)}>
      {#snippet icon()}<Plus size={18} strokeWidth={1.5} />{/snippet}
      {t('users.add')}
    </Button>
  {/snippet}

  {#if total > 0}
    <label class="search">
      <Search size={18} strokeWidth={1.5} aria-hidden="true" />
      <span class="sr-only">{t('users.search')}</span>
      <input bind:this={search} type="search" placeholder={t('users.search')} bind:value={query} autocomplete="off" />
      <kbd aria-hidden="true">/</kbd>
    </label>
  {/if}

  {#if !users.loaded && !loadError}
    <div class="grid">
      {#each [0, 1, 2] as i (i)}
        <Card><div class="sk"><Skeleton width="64px" height="64px" /><div class="sk-lines"><Skeleton width="60%" height="18px" /><Skeleton width="40%" height="14px" /></div></div></Card>
      {/each}
    </div>
  {:else if loadError}
    <Card><EmptyState text={loadError} /></Card>
  {:else if total === 0}
    <Card>
      <EmptyState text={t('users.empty')}>
        {#snippet action()}
          <Button variant="secondary" onclick={() => (adding = true)}>{t('users.add')}</Button>
        {/snippet}
      </EmptyState>
    </Card>
  {:else if visible.length === 0}
    <p class="muted">{t('users.nothing_found')}</p>
  {:else}
    <div class="grid">
      {#each visible as u, i (u.id)}
        <div animate:flip={{ duration: dur.base }} in:fadeShift={{ delay: firstLoad ? stagger(i) : 0 }}>
          <UserCard user={u} busy={busy.includes(u.id)} onopen={() => (openId = u.id)} ontoggle={(on) => toggle(u, on)} />
        </div>
      {/each}
    </div>
  {/if}
</Page>

<UserAdd bind:open={adding} oncreated={(u) => (toast(t('users.created')), (openId = u.id))} />
<UserDetail bind:userId={openId} ondelete={remove} onreissue={reissue} />

<style>
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: var(--s4); }
  @media (max-width: 400px) {
    .grid { grid-template-columns: 1fr; }
  }
  .sk { display: flex; gap: var(--s4); align-items: center; }
  .sk-lines { flex: 1; display: flex; flex-direction: column; gap: var(--s2); }
  .search {
    display: flex;
    align-items: center;
    gap: var(--s3);
    height: var(--control-h);
    padding: 0 var(--s3);
    border-radius: var(--radius-control);
    background: var(--surface-2);
    box-shadow: inset 0 0 0 1px var(--line);
    color: var(--text-2);
    max-width: 420px;
  }
  .search:focus-within { box-shadow: inset 0 0 0 1px var(--line-strong); background: var(--surface-3); }
  .search input {
    flex: 1;
    min-width: 0;
    height: 100%;
    border: 0;
    background: transparent;
    color: var(--text-1);
    font: var(--t-body);
  }
  .search input::placeholder { color: var(--text-2); }
  .search input::-webkit-search-cancel-button { display: none; }
  kbd {
    font: var(--t-micro);
    padding: 0 var(--s2);
    border-radius: var(--s1);
    box-shadow: inset 0 0 0 1px var(--line-strong);
  }
</style>
