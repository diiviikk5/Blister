<script lang="ts">
  // The segmented bar: shows which parts of the file are already on disk and
  // where each live connection is writing. Torrents use the same bar with
  // their piece map. Falls back to a plain fill when there's no map.
  import type { Status } from "../lib/types";

  let {
    progress = 0,
    map = "",
    heads = [],
    status = "downloading",
    height = 16,
  }: { progress?: number; map?: string; heads?: number[]; status?: Status; height?: number } = $props();

  const live = $derived(status === "downloading" || status === "starting");

  // Runs of equal digits become one gradient stop pair.
  const bg = $derived.by(() => {
    if (!map) return "";
    const n = map.length;
    const stops: string[] = [];
    let i = 0;
    while (i < n) {
      let j = i;
      while (j < n && map[j] === map[i]) j++;
      const v = map.charCodeAt(i) - 48;
      const pct = Math.round((v / 9) * 100);
      const col =
        v === 0
          ? "transparent"
          : v === 9
            ? "var(--fill)"
            : `color-mix(in srgb, var(--fill) ${Math.max(25, pct)}%, transparent)`;
      stops.push(`${col} ${((i / n) * 100).toFixed(3)}% ${((j / n) * 100).toFixed(3)}%`);
      i = j;
    }
    return `linear-gradient(90deg, ${stops.join(", ")})`;
  });

  const pct = $derived(Math.max(0, Math.min(1, progress)) * 100);
</script>

<div
  class="seg"
  class:live
  class:paused={status === "paused" || status === "queued"}
  class:error={status === "error"}
  class:done={status === "completed"}
  class:seeding={status === "seeding"}
  class:indeterminate={progress < 0 && live}
  style:height="{height}px"
  role="progressbar"
  aria-valuemin={0}
  aria-valuemax={100}
  aria-valuenow={progress >= 0 ? Math.round(pct) : undefined}
>
  {#if bg && status !== "completed"}
    <div class="map" style:background-image={bg}></div>
    {#each heads as h, i (i)}
      <i class="head" style:left="{h * 100}%"></i>
    {/each}
  {:else if progress >= 0}
    <div class="fill" class:heat={live} class:heat--live={live} style:width="{pct}%"></div>
  {:else}
    <div class="fill sweep heat heat--live"></div>
  {/if}
</div>

<style>
  .seg {
    --fill: var(--ib-accent);
    position: relative;
    width: 100%;
    border: var(--bw) solid var(--ib-line);
    border-radius: var(--r-sm);
    background: var(--ib-paper-2);
    overflow: hidden;
    box-shadow: inset 2px 2px 0 rgba(0, 0, 0, 0.2);
  }
  .paused {
    --fill: var(--ib-faint);
  }
  .error {
    --fill: var(--bad);
  }
  .done {
    --fill: var(--ok);
  }
  .seeding {
    --fill: var(--ib-alt);
  }
  .map {
    position: absolute;
    inset: 0;
    transition: background-image 0.4s;
  }
  .fill {
    height: 100%;
    background-color: var(--fill);
    border-right: var(--bw) solid var(--ib-line);
    transition: width 0.45s cubic-bezier(0.3, 0.7, 0.4, 1);
  }
  .fill[style*="width: 0%"] {
    border-right: 0;
  }
  .done .fill {
    border-right: 0;
  }
  .head {
    position: absolute;
    top: -1px;
    bottom: -1px;
    width: 4px;
    margin-left: -2px;
    background: var(--ib-hot);
    border-left: calc(var(--bw) * 0.6) solid var(--ib-line);
    border-right: calc(var(--bw) * 0.6) solid var(--ib-line);
    transition: left 0.5s linear;
  }
  .sweep {
    width: 35%;
    animation: sweep 1.3s ease-in-out infinite alternate;
    border-left: var(--bw) solid var(--ib-line);
  }
  @keyframes sweep {
    from {
      transform: translateX(-10%);
    }
    to {
      transform: translateX(200%);
    }
  }
</style>
