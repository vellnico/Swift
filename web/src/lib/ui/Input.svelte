<script lang="ts">
  import type { HTMLInputAttributes } from 'svelte/elements';
  import CircleAlert from '@lucide/svelte/icons/circle-alert';
  import FieldHint from './FieldHint.svelte';

  type Props = Omit<HTMLInputAttributes, 'value'> & {
    label: string;
    value?: string | number;
    hint?: string;
    error?: string;
    mono?: boolean;
    hideLabel?: boolean;
    recommended?: string | number;
    input?: HTMLInputElement;
  };

  const uid = $props.id();
  let {
    label,
    value = $bindable(''),
    hint,
    error,
    mono = false,
    hideLabel = false,
    recommended,
    id = `in-${uid}`,
    input = $bindable(),
    ...rest
  }: Props = $props();

  const describedBy = $derived([hint || recommended !== undefined ? `${id}-hint` : '', error ? `${id}-err` : ''].filter(Boolean).join(' ') || undefined);
  const isRecommended = $derived(recommended === undefined ? undefined : String(value) === String(recommended));
</script>

<div class="field">
  <label class="label" class:sr-only={hideLabel} for={id}>{label}</label>
  <input
    bind:this={input}
    {id}
    class="control"
    class:mono
    class:invalid={!!error}
    bind:value
    aria-invalid={error ? 'true' : undefined}
    aria-describedby={describedBy}
    {...rest}
  />
  {#if error}
    <p class="error" id="{id}-err" role="alert">
      <CircleAlert size={14} strokeWidth={1.5} aria-hidden="true" />
      {error}
    </p>
  {/if}
  <FieldHint id="{id}-hint" {hint} {isRecommended} onreset={() => (value = recommended ?? value)} />
</div>

<style>
  .field { display: flex; flex-direction: column; min-width: 0; }
  .label { font: var(--t-small); font-weight: 500; color: var(--text-1); margin-bottom: var(--s2); }
  .control {
    height: var(--control-h);
    width: 100%;
    padding: 0 var(--s3);
    border: 0;
    border-radius: var(--radius-control);
    background: var(--surface-2);
    box-shadow: inset 0 0 0 1px var(--line);
    font: var(--t-body);
    color: var(--text-1);
  }
  .control::placeholder { color: var(--text-2); }
  .control:hover { box-shadow: inset 0 0 0 1px var(--line-strong); }
  .control:focus { background: var(--surface-3); box-shadow: inset 0 0 0 1px var(--line-strong); }
  .control:focus-visible { box-shadow: inset 0 0 0 1px var(--line-strong), var(--focus-ring); }
  .control.invalid { box-shadow: inset 0 0 0 1px var(--text-1); }
  .control:disabled { opacity: 0.4; }
  .mono { font: var(--t-code); }
  .error {
    display: flex;
    align-items: center;
    gap: var(--s2);
    margin-top: var(--s2);
    font: var(--t-small);
    color: var(--text-1);
  }
</style>
