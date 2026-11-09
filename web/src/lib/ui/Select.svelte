<script lang="ts">
  import type { HTMLSelectAttributes } from 'svelte/elements';
  import ChevronDown from '@lucide/svelte/icons/chevron-down';
  import FieldHint from './FieldHint.svelte';

  type Option = { value: string; label: string };
  type Props = Omit<HTMLSelectAttributes, 'value'> & {
    label: string;
    options: Option[];
    value?: string;
    hint?: string;
    recommended?: string;
    hideLabel?: boolean;
  };

  const uid = $props.id();
  let { label, options, value = $bindable(''), hint, recommended, hideLabel = false, id = `sel-${uid}`, ...rest }: Props = $props();
  const isRecommended = $derived(recommended === undefined ? undefined : value === recommended);
</script>

<div class="field">
  <label class="label" class:sr-only={hideLabel} for={id}>{label}</label>
  <div class="wrap">
    <select {id} class="control" bind:value aria-describedby={hint ? `${id}-hint` : undefined} {...rest}>
      {#each options as o (o.value)}
        <option value={o.value}>{o.label}</option>
      {/each}
    </select>
    <span class="chevron" aria-hidden="true"><ChevronDown size={18} strokeWidth={1.5} /></span>
  </div>
  <FieldHint id="{id}-hint" {hint} {isRecommended} onreset={() => (value = recommended ?? value)} />
</div>

<style>
  .field { display: flex; flex-direction: column; min-width: 0; }
  .label { font: var(--t-small); font-weight: 500; margin-bottom: var(--s2); }
  .wrap { position: relative; }
  .control {
    appearance: none;
    width: 100%;
    height: var(--control-h);
    padding: 0 var(--s10) 0 var(--s3);
    border: 0;
    border-radius: var(--radius-control);
    background: var(--surface-2);
    box-shadow: inset 0 0 0 1px var(--line);
    font: var(--t-body);
  }
  .control:hover { box-shadow: inset 0 0 0 1px var(--line-strong); }
  .control:focus { background: var(--surface-3); box-shadow: inset 0 0 0 1px var(--line-strong); }
  .control:focus-visible { box-shadow: inset 0 0 0 1px var(--line-strong), var(--focus-ring); }
  .control option { background: var(--surface-2); color: var(--text-1); }
  .chevron {
    position: absolute;
    right: var(--s3);
    top: 50%;
    transform: translateY(-50%);
    color: var(--text-2);
    pointer-events: none;
  }
</style>
