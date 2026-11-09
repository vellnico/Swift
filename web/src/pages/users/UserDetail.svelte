<script lang="ts">
  import ChevronDown from '@lucide/svelte/icons/chevron-down';
  import Share2 from '@lucide/svelte/icons/share-2';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import { errorText } from '../../lib/errors';
  import { t } from '../../lib/i18n/index.svelte';
  import { fadeShift, stagger } from '../../lib/motion';
  import { linkQRURL, profileLinks, qrURL, setEnabled, users, type ProfileLink, type User } from '../../lib/users.svelte';
  import Button from '../../lib/ui/Button.svelte';
  import CopyField from '../../lib/ui/CopyField.svelte';
  import Modal from '../../lib/ui/Modal.svelte';
  import QR from '../../lib/ui/QR.svelte';
  import Switch from '../../lib/ui/Switch.svelte';
  import { toast } from '../../lib/ui/toasts.svelte';

  let {
    userId = $bindable(null),
    ondelete,
    onreissue,
  }: { userId?: number | null; ondelete: (u: User) => void; onreissue: (u: User) => void } = $props();

  let open = $state(false);
  let showLinks = $state(false);
  let linkList = $state<ProfileLink[] | null>(null);
  let busy = $state(false);
  const user = $derived(users.list.find((u) => u.id === userId) ?? null);
  const canShare = typeof navigator !== 'undefined' && typeof navigator.share === 'function';

  $effect(() => {
    if (userId !== null) {
      open = true;
      showLinks = false;
      linkList = null;
    }
  });
  $effect(() => {
    if (!open) userId = null;
  });

  async function toggleLinks() {
    showLinks = !showLinks;
    if (showLinks && user && linkList === null) {
      try {
        linkList = await profileLinks(user.id);
      } catch (e) {
        toast(errorText(e));
        linkList = [];
      }
    }
  }

  async function toggle(on: boolean) {
    if (!user) return;
    busy = true;
    try {
      await setEnabled(user.id, on);
    } catch (e) {
      toast(errorText(e));
    } finally {
      busy = false;
    }
  }

  async function share() {
    if (!user) return;
    try {
      await navigator.share({ title: 'Swift', text: user.name, url: user.subUrl });
    } catch {
      // the share sheet was dismissed
    }
  }

  function remove() {
    if (!user) return;
    const u = user;
    open = false;
    ondelete(u);
  }

  function reissue() {
    if (!user) return;
    onreissue(user);
  }
</script>

<Modal bind:open title={user?.name ?? ''}>
  {#if user}
    <div class="detail">
      <div class="qr" in:fadeShift><QR src={qrURL(user)} size={220} label={t('users.subscription')} /></div>
      <CopyField label={t('users.subscription')} value={user.subUrl} />
      <div class="actions">
        {#if canShare}
          <Button variant="secondary" onclick={share}>
            {#snippet icon()}<Share2 size={18} strokeWidth={1.5} />{/snippet}
            {t('users.share')}
          </Button>
        {/if}
        <Switch checked={user.enabled} disabled={busy} label={t('users.enabled')} showLabel onchange={toggle} />
      </div>

      <button type="button" class="disclosure" aria-expanded={showLinks} onclick={toggleLinks}>
        <span>{t('users.profile_links')}</span>
        <span class="chev" class:open={showLinks}><ChevronDown size={18} strokeWidth={1.5} aria-hidden="true" /></span>
      </button>
      {#if showLinks}
        <p class="t-small muted">{t('users.profile_links_hint')}</p>
        {#if linkList && linkList.length === 0}
          <p class="t-small muted">{t('users.no_profiles')}</p>
        {:else if linkList}
          <ul class="links">
            {#each linkList as l, i (l.profileId)}
              <li in:fadeShift={{ delay: stagger(i) }}>
                <QR src={linkQRURL(user.id, l.profileId, l.url)} size={96} label={l.name} />
                <div class="link-body"><CopyField label={l.name} value={l.url} /></div>
              </li>
            {/each}
          </ul>
        {/if}
      {/if}

      <div class="danger">
        <div>
          <Button variant="quiet" onclick={reissue}>
            {#snippet icon()}<RefreshCw size={16} strokeWidth={1.5} />{/snippet}
            {t('users.reissue')}
          </Button>
          <p class="t-small muted">{t('users.reissue_hint')}</p>
        </div>
      </div>
    </div>
  {/if}
  {#snippet footer()}
    <div class="foot">
      <Button variant="quiet" onclick={remove}>{t('users.delete')}</Button>
      <Button variant="secondary" onclick={() => (open = false)}>{t('common.done')}</Button>
    </div>
  {/snippet}
</Modal>

<style>
  .detail { display: flex; flex-direction: column; gap: var(--s5); }
  .qr { display: flex; justify-content: center; }
  .actions { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--s3); }
  .disclosure {
    display: flex;
    justify-content: space-between;
    align-items: center;
    height: var(--control-h);
    padding: 0;
    border: 0;
    border-top: 1px solid var(--line);
    background: none;
    color: var(--text-2);
    font-weight: 500;
  }
  .disclosure:hover { color: var(--text-1); }
  .chev { display: grid; transition: transform var(--dur-base) var(--ease-out); }
  .chev.open { transform: rotate(180deg); }
  .links { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: var(--s4); }
  .links li { display: flex; gap: var(--s4); align-items: center; }
  .link-body { flex: 1; min-width: 0; }
  .danger { border-top: 1px solid var(--line); padding-top: var(--s3); margin-left: calc(var(--s2) * -1); }
  .danger p { padding-left: var(--s2); max-width: 48ch; }
  .foot { display: flex; justify-content: space-between; width: 100%; }
  @media (max-width: 420px) {
    .links li { flex-direction: column; align-items: stretch; }
  }
</style>
