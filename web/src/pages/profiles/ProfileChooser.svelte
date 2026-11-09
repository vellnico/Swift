<script lang="ts">
  import Layers from '@lucide/svelte/icons/layers';
  import Shuffle from '@lucide/svelte/icons/shuffle';
  import Zap from '@lucide/svelte/icons/zap';
  import { errorText } from '../../lib/errors';
  import { t } from '../../lib/i18n/index.svelte';
  import { createProfile, kinds, type Kind } from '../../lib/profiles.svelte';
  import { trackApply, type Stage } from '../../lib/apply';
  import ApplyStatus from '../../lib/ui/ApplyStatus.svelte';
  import Modal from '../../lib/ui/Modal.svelte';
  import { toast } from '../../lib/ui/toasts.svelte';

  let { open = $bindable(false) }: { open?: boolean } = $props();
  let busy = $state<Kind | null>(null);
  let stage = $state<Stage>('idle');
  let error = $state('');

  const icons = { vless_reality_vision: Layers, vless_reality_xhttp: Shuffle, hysteria2: Zap };

  async function pick(kind: Kind) {
    if (busy) return;
    busy = kind;
    error = '';
    stage = 'checking';
    const stop = trackApply((s) => (stage = s));
    try {
      await createProfile(kind);
      stage = 'done';
      toast(t('profiles.created'));
      open = false;
    } catch (e) {
      stage = 'error';
      error = errorText(e);
    } finally {
      stop();
      busy = null;
    }
  }

  $effect(() => {
    if (open) {
      stage = 'idle';
      error = '';
    }
  });
</script>

<Modal bind:open title={t('profiles.choose_title')}>
  <p class="muted hint">{t('profiles.choose_hint')}</p>
  <div class="kinds">
    {#each kinds as kind (kind)}
      {@const Icon = icons[kind]}
      <button type="button" class="kind" class:active={busy === kind} disabled={!!busy} onclick={() => pick(kind)}>
        <span class="icon"><Icon size={20} strokeWidth={1.5} aria-hidden="true" /></span>
        <span class="text">
          <span class="t-heading">{t(`profiles.kind.${kind}.title`)}</span>
          <span class="t-small muted">{t(`profiles.kind.${kind}.desc`)}</span>
        </span>
      </button>
    {/each}
  </div>
  <div class="status"><ApplyStatus {stage} {error} /></div>
</Modal>

<style>
  .hint { margin-bottom: var(--s5); }
  .kinds { display: flex; flex-direction: column; gap: var(--s3); }
  .kind {
    display: flex;
    align-items: flex-start;
    gap: var(--s4);
    width: 100%;
    padding: var(--s5);
    border: 0;
    border-radius: var(--radius-surface);
    background: var(--surface-2);
    box-shadow: inset 0 0 0 1px var(--line);
    text-align: left;
    transition: transform var(--dur-fast) var(--ease-out);
  }
  .kind:hover:not(:disabled) { box-shadow: var(--edge-lit); }
  .kind:active:not(:disabled) { transform: scale(0.98); }
  .kind:focus-visible { box-shadow: inset 0 0 0 1px var(--line-strong), var(--focus-ring); }
  .kind:disabled:not(.active) { opacity: 0.4; cursor: default; }
  .kind.active { box-shadow: var(--edge-lit); }
  .icon {
    display: grid;
    place-items: center;
    width: var(--s10);
    height: var(--s10);
    flex: none;
    border-radius: var(--radius-control);
    box-shadow: inset 0 0 0 1px var(--line-strong);
    color: var(--text-1);
  }
  .text { display: flex; flex-direction: column; gap: var(--s1); }
  .status { margin-top: var(--s4); }
</style>
