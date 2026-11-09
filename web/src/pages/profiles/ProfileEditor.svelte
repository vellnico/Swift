<script lang="ts">
  import ChevronDown from '@lucide/svelte/icons/chevron-down';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import { trackApply, type Stage } from '../../lib/apply';
  import { errorText, fieldErrors } from '../../lib/errors';
  import { t } from '../../lib/i18n/index.svelte';
  import { profiles, updateProfile, type Field, type Params, type Profile } from '../../lib/profiles.svelte';
  import ApplyStatus from '../../lib/ui/ApplyStatus.svelte';
  import Badge from '../../lib/ui/Badge.svelte';
  import Button from '../../lib/ui/Button.svelte';
  import FieldHint from '../../lib/ui/FieldHint.svelte';
  import Input from '../../lib/ui/Input.svelte';
  import Modal from '../../lib/ui/Modal.svelte';
  import Select from '../../lib/ui/Select.svelte';
  import Switch from '../../lib/ui/Switch.svelte';
  import { toast } from '../../lib/ui/toasts.svelte';

  let {
    profile = $bindable(null),
    ondelete,
  }: { profile?: Profile | null; ondelete: (p: Profile) => void } = $props();

  let open = $state(false);
  let draft = $state<Record<string, string | number | boolean>>({});
  let regen = $state<string[]>([]);
  let errors = $state<Record<string, string>>({});
  let stage = $state<Stage>('idle');
  let error = $state('');
  let showAdvanced = $state(false);
  let status = $state<'idle' | 'loading' | 'done'>('idle');

  const fields = $derived<Field[]>(profile ? (profiles.schema[profile.kind] ?? []) : []);
  const basic = $derived(fields.filter((f) => !f.advanced));
  const advanced = $derived(fields.filter((f) => f.advanced));

  function initial(p: Profile): Record<string, string | number | boolean> {
    const pr = p.params;
    return {
      name: p.name,
      port: p.port,
      serverNames: (pr.serverNames ?? []).join(', '),
      target: pr.target ?? '',
      fingerprint: pr.fingerprint ?? 'chrome',
      shortIds: (pr.shortIds ?? []).join(', '),
      privateKey: pr.privateKey ?? '',
      path: pr.path ?? '',
      mode: pr.mode ?? 'auto',
      obfs: !!pr.obfsPassword,
      obfsPassword: pr.obfsPassword ?? '',
    };
  }

  $effect(() => {
    if (profile) {
      draft = initial(profile);
      regen = [];
      errors = {};
      stage = 'idle';
      error = '';
      showAdvanced = false;
      status = 'idle';
      open = true;
    }
  });

  $effect(() => {
    if (!open) profile = null;
  });

  const list = (v: unknown) =>
    String(v ?? '')
      .split(/[\s,]+/)
      .map((s) => s.trim())
      .filter(Boolean);

  function toParams(p: Profile): Params {
    const d = draft;
    if (p.kind === 'hysteria2') {
      return { obfsPassword: d.obfs ? String(d.obfsPassword) : '' };
    }
    const out: Params = {
      ...p.params,
      serverNames: list(d.serverNames),
      target: String(d.target).trim(),
      fingerprint: String(d.fingerprint),
      shortIds: list(d.shortIds),
      privateKey: String(d.privateKey).trim(),
    };
    if (p.kind === 'vless_reality_xhttp') {
      out.path = String(d.path).trim();
      out.mode = String(d.mode);
    }
    return out;
  }

  async function apply() {
    if (!profile || status === 'loading') return;
    const p = profile;
    errors = {};
    error = '';
    status = 'loading';
    stage = 'checking';
    const regenerate = [...regen];
    if (p.kind === 'hysteria2' && draft.obfs && !draft.obfsPassword) regenerate.push('obfsPassword');
    const stop = trackApply((s) => (stage = s));
    try {
      const saved = await updateProfile(p.id, {
        name: String(draft.name),
        port: Number(draft.port),
        params: toParams(p),
        regenerate,
      });
      stage = 'done';
      status = 'done';
      draft = initial(saved);
      regen = [];
      toast(t('profiles.saved'));
      setTimeout(() => (status = 'idle'), 1600);
    } catch (e) {
      errors = fieldErrors(e, 'profiles');
      stage = 'error';
      status = 'idle';
      error = Object.keys(errors).length ? '' : errorText(e);
      if (Object.keys(errors).some((f) => advanced.some((a) => a.name === f))) showAdvanced = true;
    } finally {
      stop();
    }
  }

  function toggleRegen(name: string) {
    regen = regen.includes(name) ? regen.filter((x) => x !== name) : [...regen, name];
  }

  function remove() {
    if (!profile) return;
    const p = profile;
    open = false;
    ondelete(p);
  }
</script>

{#snippet field(f: Field)}
  {@const label = t(`profiles.fields.${f.name}.label`)}
  {@const hint = t(`profiles.fields.${f.name}.hint`)}
  <div class="field">
    {#if f.type === 'select'}
      <Select
        {label}
        {hint}
        options={(f.options ?? []).map((o) => ({ value: o, label: o }))}
        bind:value={draft[f.name] as string}
        recommended={f.recommended as string | undefined}
      />
    {:else if f.type === 'switch'}
      <div class="switch-row">
        <Switch bind:checked={draft[f.name] as boolean} {label} showLabel />
      </div>
      <FieldHint
        {hint}
        isRecommended={f.recommended === undefined ? undefined : draft[f.name] === f.recommended}
        onreset={() => (draft[f.name] = f.recommended as boolean)}
      />
    {:else if f.type === 'secret'}
      <Input
        {label}
        {hint}
        mono
        autocomplete="off"
        spellcheck="false"
        bind:value={draft[f.name] as string}
        error={errors[f.name]}
        disabled={regen.includes(f.name) || (f.name === 'obfsPassword' && !draft.obfs)}
      />
      {#if f.regenerate}
        <div class="regen">
          <Button variant="quiet" onclick={() => toggleRegen(f.name)} aria-pressed={regen.includes(f.name)}>
            {#snippet icon()}<RefreshCw size={16} strokeWidth={1.5} />{/snippet}
            {t('profiles.regenerate')}
          </Button>
          {#if regen.includes(f.name)}<Badge variant="recommended">{t('profiles.regen_pending')}</Badge>{/if}
        </div>
      {/if}
    {:else}
      <Input
        {label}
        {hint}
        type={f.type === 'number' ? 'number' : 'text'}
        mono={f.name !== 'name'}
        inputmode={f.type === 'number' ? 'numeric' : undefined}
        min={f.min}
        max={f.max}
        autocomplete="off"
        spellcheck="false"
        bind:value={draft[f.name] as string | number}
        recommended={f.recommended as string | number | undefined}
        error={errors[f.name]}
        disabled={regen.includes(f.name)}
      />
      {#if f.regenerate && f.type !== 'list'}
        <div class="regen">
          <Button variant="quiet" onclick={() => toggleRegen(f.name)} aria-pressed={regen.includes(f.name)}>
            {#snippet icon()}<RefreshCw size={16} strokeWidth={1.5} />{/snippet}
            {t('profiles.regenerate')}
          </Button>
        </div>
      {/if}
    {/if}
  </div>
{/snippet}

<Modal bind:open title={profile ? profile.name : ''}>
  {#if profile}
    <form class="form" onsubmit={(e) => (e.preventDefault(), apply())} novalidate>
      {#each basic as f (f.name)}{@render field(f)}{/each}
      {#if advanced.length}
        <button type="button" class="disclosure" aria-expanded={showAdvanced} onclick={() => (showAdvanced = !showAdvanced)}>
          <span>{t('profiles.advanced')}</span>
          <span class="chev" class:open={showAdvanced}><ChevronDown size={18} strokeWidth={1.5} aria-hidden="true" /></span>
        </button>
        {#if showAdvanced}
          {#each advanced as f (f.name)}{@render field(f)}{/each}
        {/if}
      {/if}
      <ApplyStatus {stage} {error} />
      <!-- keeps Enter-to-apply working; the visible button lives in the footer -->
      <button type="submit" class="sr-only" aria-hidden="true" tabindex="-1"></button>
    </form>
  {/if}
  {#snippet footer()}
    <div class="foot">
      <Button variant="quiet" onclick={remove}>{t('profiles.delete')}</Button>
      <Button variant="primary" status={status} onclick={apply}>{t('profiles.apply')}</Button>
    </div>
  {/snippet}
</Modal>

<style>
  .form { display: flex; flex-direction: column; gap: var(--s5); }
  .field { display: flex; flex-direction: column; }
  .switch-row { display: flex; align-items: center; min-height: var(--control-h); }
  .regen { display: flex; align-items: center; gap: var(--s2); margin-top: var(--s1); margin-left: calc(var(--s2) * -1); }
  .disclosure {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    height: var(--control-h);
    padding: 0;
    border: 0;
    border-top: 1px solid var(--line);
    background: none;
    color: var(--text-2);
    font: var(--t-body);
    font-weight: 500;
  }
  .disclosure:hover { color: var(--text-1); }
  .chev { display: grid; transition: transform var(--dur-base) var(--ease-out); }
  .chev.open { transform: rotate(180deg); }
  .foot { display: flex; justify-content: space-between; width: 100%; gap: var(--s3); }
</style>
