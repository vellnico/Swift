<script lang="ts">
  import { onMount } from 'svelte';
  import { flip } from 'svelte/animate';
  import Plus from '@lucide/svelte/icons/plus';
  import { errorText } from '../lib/errors';
  import { hotkey } from '../lib/hotkeys';
  import { t } from '../lib/i18n/index.svelte';
  import { dur, fadeShift, stagger } from '../lib/motion';
  import { deleteProfile, loadProfiles, profiles, updateProfile, type Profile } from '../lib/profiles.svelte';
  import Button from '../lib/ui/Button.svelte';
  import Card from '../lib/ui/Card.svelte';
  import EmptyState from '../lib/ui/EmptyState.svelte';
  import Skeleton from '../lib/ui/Skeleton.svelte';
  import { toast } from '../lib/ui/toasts.svelte';
  import Page from '../shell/Page.svelte';
  import ProfileCard from './profiles/ProfileCard.svelte';
  import ProfileChooser from './profiles/ProfileChooser.svelte';
  import ProfileEditor from './profiles/ProfileEditor.svelte';

  let choosing = $state(false);
  let editing = $state<Profile | null>(null);
  let busy = $state<number[]>([]);
  let firstLoad = $state(!profiles.loaded);
  let loadError = $state('');

  onMount(() => {
    loadProfiles()
      .catch((e) => (loadError = errorText(e)))
      .finally(() => setTimeout(() => (firstLoad = false), 600));
    return hotkey('n', () => (choosing = true));
  });

  const visible = $derived(profiles.list.filter((p) => !profiles.hidden.includes(p.id)));

  async function toggle(p: Profile, on: boolean) {
    busy = [...busy, p.id];
    try {
      await updateProfile(p.id, { enabled: on });
    } catch (e) {
      toast(errorText(e));
      profiles.list = [...profiles.list];
    } finally {
      busy = busy.filter((x) => x !== p.id);
    }
  }

  function remove(p: Profile) {
    profiles.hidden = [...profiles.hidden, p.id];
    toast(t('profiles.deleted'), {
      action: { label: t('common.undo'), run: () => (profiles.hidden = profiles.hidden.filter((x) => x !== p.id)) },
      onexpire: () => {
        deleteProfile(p.id).catch((e) => {
          profiles.hidden = profiles.hidden.filter((x) => x !== p.id);
          toast(errorText(e));
        });
      },
    });
  }
</script>

<Page title={t('nav.profiles')}>
  {#snippet action()}
    <Button variant="primary" onclick={() => (choosing = true)}>
      {#snippet icon()}<Plus size={18} strokeWidth={1.5} />{/snippet}
      {t('profiles.create')}
    </Button>
  {/snippet}

  {#if !profiles.loaded && !loadError}
    <div class="grid">
      {#each [0, 1] as i (i)}
        <Card><div class="sk"><Skeleton width="50%" height="20px" /><Skeleton width="30%" height="14px" /><Skeleton width="60%" height="24px" radius="pill" /></div></Card>
      {/each}
    </div>
  {:else if loadError}
    <Card><EmptyState text={loadError} /></Card>
  {:else if visible.length === 0}
    <Card>
      <EmptyState text={t('profiles.empty')}>
        {#snippet action()}
          <Button variant="secondary" onclick={() => (choosing = true)}>{t('profiles.create')}</Button>
        {/snippet}
      </EmptyState>
    </Card>
  {:else}
    <div class="grid">
      {#each visible as p, i (p.id)}
        <div animate:flip={{ duration: dur.base }} in:fadeShift={{ delay: firstLoad ? stagger(i) : 0 }}>
          <ProfileCard profile={p} busy={busy.includes(p.id)} onopen={() => (editing = p)} ontoggle={(on) => toggle(p, on)} />
        </div>
      {/each}
    </div>
  {/if}
</Page>

<ProfileChooser bind:open={choosing} />
<ProfileEditor bind:profile={editing} ondelete={remove} />

<style>
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: var(--s4); }
  @media (max-width: 400px) {
    .grid { grid-template-columns: 1fr; }
  }
  .sk { display: flex; flex-direction: column; gap: var(--s3); }
</style>
