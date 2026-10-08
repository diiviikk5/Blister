<script lang="ts">
  // Stepped area chart of total speed: hard edges, no smoothing, like the
  // rest of the UI.
  import { store } from "../lib/store.svelte";
  import { speed } from "../lib/format";

  const W = 200;
  const H = 54;
  const peak = $derived(Math.max(1, ...store.history));
  const path = $derived.by(() => {
    const n = store.history.length;
    const step = W / (n - 1);
    let d = `M0 ${H}`;
    store.history.forEach((v, i) => {
      const y = H - (v / peak) * (H - 6);
      d += ` L${(i * step).toFixed(1)} ${y.toFixed(1)}`;
    });
    return d + ` L${W} ${H} Z`;
  });
</script>

<div class="graph">
  <div class="top">
    <span class="label">Speed</span>
    <span class="now num">{speed(store.speed)}</span>
  </div>
  <svg viewBox="0 0 {W} {H}" preserveAspectRatio="none" aria-hidden="true">
    <path d={path} class="area" />
  </svg>
  <div class="foot num faint">peak {speed(peak)}</div>
</div>

<style>
  .graph {
    padding: 10px 12px 8px;
    border: 2.5px solid var(--ib-line);
    border-radius: 12px;
    background: var(--ib-card);
    box-shadow: var(--ib-ex4);
  }
  .top {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
  }
  .label {
    margin: 0;
  }
  .now {
    font-weight: 900;
    font-size: 15px;
    letter-spacing: -0.03em;
  }
  svg {
    display: block;
    width: 100%;
    height: 54px;
    margin-top: 6px;
    border-bottom: 2px solid var(--ib-line);
  }
  .area {
    fill: color-mix(in srgb, var(--ib-accent) 85%, transparent);
    stroke: var(--ib-line);
    stroke-width: 1.5;
    vector-effect: non-scaling-stroke;
  }
  .foot {
    margin-top: 5px;
    font-size: 11px;
    font-weight: 700;
  }
</style>
