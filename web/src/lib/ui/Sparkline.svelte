<script lang="ts">
  import { Spring } from 'svelte/motion';
  import { precise, spring } from '../motion';
  import { sparkPath } from './sparkline';

  let {
    values,
    max = 0,
    slots = 150,
    height = 48,
    label,
  }: { values: number[]; max?: number; slots?: number; height?: number; label: string } = $props();

  const width = 300;
  const uid = $props.id();
  const step = $derived(width / (slots - 1));
  // each new point starts one step to the right and slides into place
  const shift = new Spring(0, spring(precise));
  let lastLen = 0;
  let lastTail: number | undefined;

  $effect(() => {
    const tail = values[values.length - 1];
    if (values.length > 0 && (values.length !== lastLen || tail !== lastTail)) {
      if (lastLen > 0) {
        shift.set(step, { instant: true });
        shift.target = 0;
      }
      lastLen = values.length;
      lastTail = tail;
    }
  });

  const paths = $derived(sparkPath(values, width, height, max, slots));
</script>

<svg class="spark" viewBox="0 0 {width} {height}" preserveAspectRatio="none" role="img" aria-label={label}>
  <defs>
    <linearGradient id="fill-{uid}" x1="0" x2="0" y1="0" y2="1">
      <stop offset="0" class="stop-top" />
      <stop offset="1" class="stop-bottom" />
    </linearGradient>
    <clipPath id="clip-{uid}"><rect {width} height={height + 2} y="-1" /></clipPath>
  </defs>
  <g clip-path="url(#clip-{uid})">
    <g style:transform="translateX({shift.current}px)">
      <path d={paths.area} fill="url(#fill-{uid})" />
      <path d={paths.line} class="line" vector-effect="non-scaling-stroke" />
    </g>
  </g>
</svg>

<style>
  .spark { width: 100%; height: var(--spark-h, 48px); display: block; overflow: visible; }
  .line { fill: none; stroke: var(--porcelain); stroke-width: 1.5; stroke-linejoin: round; stroke-linecap: round; }
  .stop-top { stop-color: var(--spark-fill-top); }
  .stop-bottom { stop-color: var(--spark-fill-bottom); }
</style>
