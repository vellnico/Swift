<script lang="ts">
  import { t } from '../../lib/i18n/index.svelte';
  import { networkOf, sniOf, type Profile } from '../../lib/profiles.svelte';
  import Badge from '../../lib/ui/Badge.svelte';
  import Card from '../../lib/ui/Card.svelte';
  import Switch from '../../lib/ui/Switch.svelte';

  let {
    profile,
    busy = false,
    onopen,
    ontoggle,
  }: { profile: Profile; busy?: boolean; onopen: () => void; ontoggle: (on: boolean) => void } = $props();
</script>

<Card interactive class="profile-card">
  <button type="button" class="open" onclick={onopen} aria-label={t('profiles.edit', { name: profile.name })}></button>
  <div class="head">
    <div class="titles">
      <p class="t-heading name">{profile.name}</p>
      <p class="t-small muted">{t(`profiles.kind.${profile.kind}.title`)}</p>
    </div>
    <div class="switch">
      <Switch checked={profile.enabled} disabled={busy} label={t('profiles.enable')} onchange={ontoggle} />
    </div>
  </div>
  <div class="meta">
    <Badge><span class="t-code">{networkOf(profile.kind)} {profile.port}</span></Badge>
    {#if sniOf(profile)}<Badge><span class="t-code">{sniOf(profile)}</span></Badge>{/if}
    {#if !profile.enabled}<Badge>{t('profiles.off')}</Badge>{/if}
  </div>
</Card>

<style>
  :global(.profile-card) { display: flex; flex-direction: column; gap: var(--s5); }
  .open {
    position: absolute;
    inset: 0;
    border: 0;
    border-radius: inherit;
    background: transparent;
    cursor: pointer;
  }
  .open:focus-visible { box-shadow: var(--focus-ring); }
  .head { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--s3); pointer-events: none; }
  .titles { min-width: 0; }
  .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .switch { position: relative; z-index: 1; pointer-events: auto; }
  .meta { display: flex; flex-wrap: wrap; gap: var(--s2); pointer-events: none; }
  .meta :global(.badge) { max-width: 100%; overflow: hidden; }
</style>
