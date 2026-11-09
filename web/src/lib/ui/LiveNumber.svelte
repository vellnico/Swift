<script lang="ts">
  import { Spring } from 'svelte/motion';
  import { soft, spring } from '../motion';

  // A number that flows to its new value instead of jumping (SPEC 10.6).
  let { value, format }: { value: number; format: (v: number) => string } = $props();
  const v = new Spring(0, spring(soft));
  let first = true;
  $effect(() => {
    if (first) {
      v.set(value, { instant: true });
      first = false;
    } else v.target = value;
  });
</script>

<span class="live">{format(v.current)}</span>

<style>
  .live { font-variant-numeric: tabular-nums; }
</style>
