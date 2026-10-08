<script lang="ts">
  // The whole app as a 52px strip, Polybar-style: speed, a sparkline and
  // the active downloads. Drag it anywhere; it stays on top.
  import Icon from "./Icon.svelte";
  import Logo from "./Logo.svelte";
  import { store, prog } from "../lib/store.svelte";
  import { speed, percent, eta } from "../lib/format";
  import { iconFor } from "../lib/kinds";
  import { readClipboard } from "../lib/api";

  const active = $derived(store.all.filter((t) => t.status === "downloading" || t.status === "starting").slice(0, 3));
  const queued = $derived(store.counts.queued ?? 0);

  const W = 90;
  const H = 26;
  const spark = $derived.by(() => {
    const h = store.history.slice(-40);
    const peak = Math.max(1, ...h);
    const step = W / (h.length - 1);
    return `M0 ${H} ` + h.map((v, i) => `L${(i * step).toFixed(1)} ${(H - (v / peak) * (H - 3)).toFixed(1)}`).join(" ") + ` L${W} ${H} Z`;
  });

  async function quickAdd() {
    store.setMini(false);
    const text = await readClipboard();
    store.openAdd(text ? { urls: [text], request: { url: "" }, source: "clipboard" } : null);
  }
</script>

<div class="mini" style="--wails-draggable: drag">
  <span class="logo"><Logo size={22} /></span>
  <div class="rate">
    <b class="num">{speed(store.speed)}</b>
    <svg viewBox="0 0 {W} {H}" preserveAspectRatio="none" aria-hidden="true"><path d={spark} /></svg>
  </div>

  <div class="tasks">
    {#each active as t (t.id)}
      {@const look = iconFor(t)}
      <div class="task" title={t.name}>
        <i class="dot" style:background={look.color}></i>
        <span class="nm ell">{t.name}</span>
        <span class="bar"><i style:width="{Math.round(prog(t) * 100)}%"></i></span>
        <span class="pc num">{percent(t.done, t.size)}{t.eta >= 0 ? ` · ${eta(t.eta)}` : ""}</span>
      </div>
    {:else}
      <span class="idle faint">{queued ? `${queued} waiting` : "Idle. Copy a link and hit +"}</span>
    {/each}
  </div>

  <div class="btns" style="--wails-draggable: no-drag">
    <button onclick={quickAdd} title="Add from clipboard" aria-label="Add from clipboard"><Icon name="plus" size={16} stroke={3} /></button>
    <button onclick={() => store.setMini(false)} title="Back to the full window (Ctrl M)" aria-label="Expand"><Icon name="max" size={14} /></button>
  </div>
</div>

<style>
  .mini {
    height: 100vh;
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 0 6px 0 12px;
    border: var(--bw) solid var(--ib-line);
    border-radius: var(--r);
    background: var(--ib-paper);
    overflow: hidden;
    font-size: 12.5px;
    font-weight: 700;
  }
  .logo {
    display: grid;
    flex: none;
  }
  .rate {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: none;
  }
  .rate b {
    min-width: 76px;
    font-size: 14px;
    font-weight: 900;
    letter-spacing: -0.02em;
  }
  svg {
    width: 90px;
    height: 26px;
  }
  path {
    fill: color-mix(in srgb, var(--ib-accent) 80%, transparent);
    stroke: var(--ib-line);
    stroke-width: 1;
    vector-effect: non-scaling-stroke;
  }
  .tasks {
    flex: 1;
    min-width: 0;
    display: flex;
    gap: 12px;
  }
  .task {
    flex: 1;
    min-width: 0;
    display: grid;
    grid-template-columns: 8px minmax(0, 1fr) auto;
    grid-template-rows: auto auto;
    column-gap: 7px;
    row-gap: 3px;
    align-items: center;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 2px;
    border: 1.5px solid var(--ib-line);
  }
  .nm {
    font-size: 11.5px;
  }
  .bar {
    grid-column: 1 / span 2;
    height: 6px;
    border-radius: 3px;
    background: var(--ib-paper-2);
    overflow: hidden;
  }
  .bar i {
    display: block;
    height: 100%;
    background: var(--ib-accent);
    transition: width 0.4s linear;
  }
  .pc {
    grid-row: 1 / span 2;
    grid-column: 3;
    font-size: 11px;
    color: var(--ib-muted);
  }
  .idle {
    font-size: 12px;
  }
  .btns {
    display: flex;
    gap: 4px;
    flex: none;
  }
  .btns button {
    width: 34px;
    height: 34px;
    display: grid;
    place-items: center;
    border: var(--bw) solid var(--ib-line);
    border-radius: var(--r-sm);
    background: var(--ib-card);
    cursor: pointer;
  }
  .btns button:first-child {
    background: var(--ib-accent);
    color: var(--ib-accent-ink);
  }
</style>
