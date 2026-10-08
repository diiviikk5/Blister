<script lang="ts">
  // Progress, in whichever style the rice asks for. "lanes" shows what's on
  // disk and where each connection is writing; the text styles (blocks,
  // ascii, braille, dots) render the same coverage as glyphs.
  import type { Status } from "../lib/types";
  import { store } from "../lib/store.svelte";
  import type { BarStyle } from "../lib/rice";

  let {
    progress = 0,
    map = "",
    heads = [],
    status = "downloading",
    height,
    force,
  }: { progress?: number; map?: string; heads?: number[]; status?: Status; height?: number; force?: BarStyle } = $props();

  const style = $derived(force ?? store.rice.bar.style);
  const showHeads = $derived(store.rice.bar.heads);
  const live = $derived(status === "downloading" || status === "starting");
  const h = $derived(height ?? store.rice.bar.height);
  const pct = $derived(Math.max(0, Math.min(1, progress)) * 100);
  const done = $derived(status === "completed");

  // Runs of equal digits become one gradient stop pair.
  const bg = $derived.by(() => {
    if (!map || style !== "lanes") return "";
    const n = map.length;
    const stops: string[] = [];
    let i = 0;
    while (i < n) {
      let j = i;
      while (j < n && map[j] === map[i]) j++;
      const v = map.charCodeAt(i) - 48;
      const col =
        v === 0
          ? "transparent"
          : v === 9
            ? "var(--fill)"
            : `color-mix(in srgb, var(--fill) ${Math.max(25, Math.round((v / 9) * 100))}%, transparent)`;
      stops.push(`${col} ${((i / n) * 100).toFixed(3)}% ${((j / n) * 100).toFixed(3)}%`);
      i = j;
    }
    return `linear-gradient(90deg, ${stops.join(", ")})`;
  });

  // Coverage resampled to N cells in [0,1].
  const N = 40;
  const cells = $derived.by(() => {
    const out = new Array<number>(N).fill(0);
    if (done) return out.fill(1);
    if (map) {
      for (let i = 0; i < N; i++) {
        const a = Math.floor((i / N) * map.length);
        const b = Math.max(a + 1, Math.floor(((i + 1) / N) * map.length));
        let sum = 0;
        for (let k = a; k < b; k++) sum += map.charCodeAt(k) - 48;
        out[i] = sum / ((b - a) * 9);
      }
      return out;
    }
    const f = (Math.max(0, progress) * N);
    for (let i = 0; i < N; i++) out[i] = Math.max(0, Math.min(1, f - i));
    return out;
  });

  const headCells = $derived(new Set((showHeads ? heads : []).map((x) => Math.min(N - 1, Math.floor(x * N)))));

  const ramps: Record<string, string[]> = {
    braille: ["⠀", "⡀", "⣀", "⣄", "⣤", "⣦", "⣶", "⣷", "⣿"],
    blocks: ["▱", "▱", "▱", "▱", "▰", "▰", "▰", "▰", "▰"],
    dots: ["·", "·", "∘", "∘", "○", "◎", "◉", "●", "●"],
  };

  const text = $derived.by(() => {
    if (style === "ascii") {
      const body = cells
        .map((v, i) => (headCells.has(i) && live ? ">" : v >= 0.95 ? "#" : v >= 0.4 ? "=" : v > 0 ? "-" : "."))
        .join("");
      return `[${body}]`;
    }
    const ramp = ramps[style];
    if (!ramp) return "";
    return cells.map((v, i) => (headCells.has(i) && live && style !== "blocks" ? ramp[8] : ramp[Math.round(v * 8)])).join("");
  });

  const isText = $derived(style === "ascii" || style === "braille" || style === "blocks" || style === "dots");
</script>

<div
  class="seg s-{style}"
  class:live
  class:paused={status === "paused" || status === "queued"}
  class:error={status === "error"}
  class:done
  class:seeding={status === "seeding"}
  style:--h="{style === 'line' ? Math.max(3, Math.round(h / 3)) : h}px"
  role="progressbar"
  aria-valuemin={0}
  aria-valuemax={100}
  aria-valuenow={progress >= 0 ? Math.round(pct) : undefined}
>
  {#if isText}
    <svg viewBox="0 0 400 20" preserveAspectRatio="none" aria-hidden="true">
      <text x="0" y="15" textLength="400" lengthAdjust="spacing">{text}</text>
    </svg>
  {:else if bg && !done}
    <div class="map" style:background-image={bg}></div>
    {#if showHeads}
      {#each heads as hd, i (i)}
        <i class="head" style:left="{hd * 100}%"></i>
      {/each}
    {/if}
  {:else if progress >= 0 || done}
    <div class="fill" class:heat={live && store.rice.bar.stripes} class:heat--live={live && store.rice.bar.stripes} style:width="{done ? 100 : pct}%"></div>
  {:else}
    <div class="fill sweep heat heat--live"></div>
  {/if}
</div>

<style>
  .seg {
    --fill: var(--ib-accent);
    position: relative;
    width: 100%;
    height: var(--h);
    border: var(--bw) solid var(--ib-line);
    border-radius: var(--r-xs);
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
  }
  .fill {
    height: 100%;
    background-color: var(--fill);
    border-right: var(--bw) solid var(--ib-line);
    transition: width calc(0.45s * var(--motion, 1)) cubic-bezier(0.3, 0.7, 0.4, 1);
  }
  .fill[style*="width: 0%"],
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

  /* solid: a plain rounded fill */
  .s-solid {
    border-radius: 999px;
    box-shadow: none;
  }
  .s-solid .fill {
    border-right: 0;
    border-radius: 999px;
  }
  /* line: thin, borderless */
  .s-line {
    border: 0;
    border-radius: 999px;
    box-shadow: none;
    background: color-mix(in srgb, var(--ib-faint) 30%, transparent);
  }
  .s-line .fill {
    border-right: 0;
    border-radius: 999px;
  }
  /* glyph styles */
  .s-ascii,
  .s-braille,
  .s-blocks,
  .s-dots {
    height: calc(var(--h) + 6px);
    border: 0;
    background: none;
    box-shadow: none;
    border-radius: 0;
  }
  svg {
    display: block;
    width: 100%;
    height: 100%;
    overflow: visible;
  }
  text {
    font-family: var(--ib-mono);
    font-size: 17px;
    font-weight: 700;
    fill: var(--fill);
    white-space: pre;
  }
  .s-braille text {
    font-size: 19px;
  }
</style>
