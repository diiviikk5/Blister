<script lang="ts">
  // Dense, sortable table: the qBittorrent-style view for big queues.
  import Icon from "./Icon.svelte";
  import SegBar from "./SegBar.svelte";
  import { api } from "../lib/api";
  import { store, prog, type SortKey } from "../lib/store.svelte";
  import { bytes, speed, eta, percent, ago, host } from "../lib/format";
  import { iconFor, statusLabel } from "../lib/kinds";
  import type { Task } from "../lib/types";

  let { oncontext }: { oncontext: (e: MouseEvent, t: Task) => void } = $props();

  const cols: { key: SortKey | null; label: string; cls: string }[] = [
    { key: "name", label: "Name", cls: "c-name" },
    { key: "size", label: "Size", cls: "c-size" },
    { key: "progress", label: "Progress", cls: "c-prog" },
    { key: "speed", label: "Speed", cls: "c-speed" },
    { key: null, label: "ETA", cls: "c-eta" },
    { key: null, label: "Status", cls: "c-status" },
    { key: null, label: "Conns", cls: "c-conns" },
    { key: "added", label: "Source", cls: "c-src" },
  ];

  function click(e: MouseEvent, t: Task) {
    store.select(t.id, e.shiftKey ? "range" : e.ctrlKey || e.metaKey ? "toggle" : "one");
  }
  function dbl(t: Task) {
    if (t.status === "completed") store.run(api.open(t.id));
    else if (t.status === "paused" || t.status === "error") store.run(api.resume([t.id]));
  }
  function ctx(e: MouseEvent, t: Task) {
    e.preventDefault();
    if (!store.selected.includes(t.id)) store.select(t.id);
    oncontext(e, t);
  }
</script>

<div class="wrap" role="grid" aria-label="Downloads" aria-multiselectable="true">
  <div class="thead" role="row">
    <span class="c-icon"></span>
    {#each cols as c (c.label)}
      <button
        role="columnheader"
        class={c.cls}
        class:on={c.key && store.sort === c.key}
        disabled={!c.key}
        onclick={() => c.key && (store.sort = store.sort === c.key ? "smart" : c.key)}
      >
        {c.label}
        {#if c.key && store.sort === c.key}<Icon name="down" size={11} stroke={3} />{/if}
      </button>
    {/each}
  </div>
  <div class="tbody">
    {#each store.visible as t (t.id)}
      {@const look = iconFor(t)}
      {@const live = store.live[t.id]}
      {@const running = t.status === "downloading" || t.status === "starting"}
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <div
        class="tr"
        class:sel={store.selected.includes(t.id)}
        role="row"
        tabindex="-1"
        aria-selected={store.selected.includes(t.id)}
        onclick={(e) => click(e, t)}
        ondblclick={() => dbl(t)}
        oncontextmenu={(e) => ctx(e, t)}
      >
        <span class="c-icon"><i style:background={look.color}><Icon name={look.icon} size={12} stroke={2.8} /></i></span>
        <span class="c-name ell" title={t.name}>{t.name}</span>
        <span class="c-size num">{bytes(t.size)}</span>
        <span class="c-prog">
          <SegBar progress={t.size > 0 || t.status === "completed" ? prog(t) : running ? -1 : 0} map={live?.map || t.torrent?.pieceMap || ""} heads={t.kind === "http" ? (live?.heads ?? []) : []} status={t.status} height={10} />
          <b class="num">{t.status === "completed" ? "100%" : percent(t.done, t.size)}</b>
        </span>
        <span class="c-speed num">{running ? speed(t.speed) : t.status === "seeding" ? "↑ " + speed(t.upSpeed) : ""}</span>
        <span class="c-eta num">{running && t.eta >= 0 ? eta(t.eta) : ""}</span>
        <span class="c-status" data-s={t.status}>{statusLabel[t.status]}</span>
        <span class="c-conns num">{running || t.status === "seeding" ? t.conns : ""}</span>
        <span class="c-src ell faint">{t.status === "completed" ? ago(t.completedAt) : host(t.url)}</span>
      </div>
    {:else}
      <p class="none faint">Nothing to show.</p>
    {/each}
  </div>
</div>

<style>
  .wrap {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    font-size: 12.5px;
  }
  .thead,
  .tr {
    display: grid;
    grid-template-columns: 30px minmax(160px, 3fr) 80px minmax(140px, 2fr) 86px 70px 92px 54px minmax(90px, 1fr);
    align-items: center;
    gap: 10px;
    padding: 0 12px;
  }
  .thead {
    height: 32px;
    border-bottom: var(--bw) solid var(--ib-line);
    background: var(--ib-paper-2);
  }
  .thead button {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 0;
    border: 0;
    background: none;
    font-size: 10.5px;
    font-weight: 900;
    letter-spacing: 0.07em;
    text-transform: uppercase;
    color: var(--ib-faint);
    cursor: pointer;
    text-align: left;
  }
  .thead button:disabled {
    cursor: default;
  }
  .thead button.on {
    color: var(--ib-text);
  }
  .tbody {
    flex: 1;
    overflow-y: auto;
  }
  .tr {
    height: 34px;
    border-bottom: 1px solid color-mix(in srgb, var(--ib-line) 12%, transparent);
    font-weight: 650;
    position: relative;
  }
  .tr:nth-child(even) {
    background: color-mix(in srgb, var(--ib-paper-2) 45%, transparent);
  }
  .tr:hover {
    background: color-mix(in srgb, var(--ib-accent) 8%, transparent);
  }
  .tr.sel {
    background: color-mix(in srgb, var(--ib-accent) 18%, transparent);
    box-shadow: inset 4px 0 0 var(--ib-accent);
  }
  .c-icon i {
    width: 22px;
    height: 22px;
    display: grid;
    place-items: center;
    border: calc(var(--bw) * 0.7) solid var(--ib-line);
    border-radius: var(--r-xs);
    color: #140d1f;
  }
  .c-name {
    font-weight: 750;
  }
  .c-size,
  .c-speed,
  .c-eta,
  .c-conns {
    text-align: right;
  }
  .thead .c-size,
  .thead .c-speed,
  .thead .c-eta,
  .thead .c-conns {
    justify-content: flex-end;
  }
  .c-prog {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .c-prog b {
    width: 40px;
    text-align: right;
    font-size: 11.5px;
  }
  .c-status {
    font-size: 10.5px;
    font-weight: 900;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }
  .c-status[data-s="error"] {
    color: var(--bad);
  }
  .c-status[data-s="completed"] {
    color: var(--ok);
  }
  .c-status[data-s="paused"],
  .c-status[data-s="queued"] {
    color: var(--ib-faint);
  }
  .c-status[data-s="seeding"] {
    color: var(--ib-alt);
  }
  .none {
    padding: 40px;
    text-align: center;
  }
</style>
