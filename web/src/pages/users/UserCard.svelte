<script lang="ts">
  import { t } from '../../lib/i18n/index.svelte';
  import { qrURL, type User } from '../../lib/users.svelte';
  import Badge from '../../lib/ui/Badge.svelte';
  import Card from '../../lib/ui/Card.svelte';
  import QR from '../../lib/ui/QR.svelte';
  import Switch from '../../lib/ui/Switch.svelte';

  let { user, busy = false, onopen, ontoggle }: { user: User; busy?: boolean; onopen: () => void; ontoggle: (on: boolean) => void } = $props();
</script>

<Card interactive class="user-card">
  <button type="button" class="open" onclick={onopen} aria-label={t('users.open', { name: user.name })}></button>
  <div class="qr" class:paused={!user.enabled}><QR src={qrURL(user)} size={64} label={user.name} /></div>
  <div class="body">
    <div class="top">
      <p class="t-heading name">{user.name}</p>
      <div class="switch"><Switch checked={user.enabled} disabled={busy} label={t('users.enabled')} onchange={ontoggle} /></div>
    </div>
    {#if user.note}<p class="t-small muted note">{user.note}</p>{/if}
    {#if !user.enabled}<div><Badge>{t('users.paused')}</Badge></div>{/if}
  </div>
</Card>

<style>
  :global(.user-card) { display: flex; gap: var(--s4); align-items: center; }
  .open { position: absolute; inset: 0; border: 0; border-radius: inherit; background: transparent; cursor: pointer; }
  .open:focus-visible { box-shadow: var(--focus-ring); }
  .qr { pointer-events: none; transition: opacity var(--dur-base) var(--ease-out); }
  .qr.paused { opacity: 0.35; }
  .body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: var(--s2); pointer-events: none; }
  .top { display: flex; align-items: center; justify-content: space-between; gap: var(--s3); }
  .name, .note { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .switch { position: relative; z-index: 1; pointer-events: auto; }
</style>
