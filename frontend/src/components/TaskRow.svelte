<script lang="ts">
  import Icon from "./Icon.svelte";
  import SegBar from "./SegBar.svelte";
  import { api } from "../lib/api";
  import { store, prog } from "../lib/store.svelte";
  import { bytes, speed, eta, percent, host, ago } from "../lib/format";
  import { iconFor, statusLabel } from "../lib/kinds";
  import type { Task } from "../lib/types";

  let { task, oncontext }: { task: Task; oncontext: (e: MouseEvent, t: Task) => void } = $props();

  const look = $derived(iconFor(task));
  const live = $derived(store.live[task.id]);
  const sel = $derived(store.selected.includes(task.id));
  const running = $derived(task.status === "downloading" || task.status === "starting");
  const stoppable = $derived(running || task.status === "queued" || task.status === "seeding");
  const map = $derived(task.kind === "torrent" ? (task.torrent?.pieceMap ?? "") : (live?.map ?? ""));
  const progress = $derived(task.size > 0 || task.status === "completed" ? prog(task) : running ? -1 : 0);

  function click(e: MouseEvent) {
    store.select(task.id, e.shiftKey ? "range" : e.ctrlKey || e.metaKey ? "toggle" : "one");
  }
  function dbl() {
    if (task.status === "completed") store.run(api.open(task.id));
    else if (task.status === "paused" || task.status === "error") store.run(api.resume([task.id]));
  }
  function toggle(e: MouseEvent) {
    e.stopPropagation();
    store.run(stoppable ? api.pause([task.id]) : api.resume([task.id]));
  }
  function context(e: MouseEvent) {
    e.preventDefault();
    if (!sel) store.select(task.id);
    oncontext(e, task);
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
  class="row"
  class:sel
  role="option"
  aria-selected={sel}
  tabindex="-1"
  onclick={click}
  ondblclick={dbl}
  oncontextmenu={context}
>
  <div class="tile" style:--c={look.color}>
    {#if task.media?.thumbnail}
      <img src={task.media.thumbnail} alt="" loading="lazy" />
    {:else}
      <Icon name={look.icon} size={20} stroke={2.6} />
    {/if}
  </div>

  <div class="main">
    <div class="name ell" title={task.name}>{task.name}</div>
    <div class="bar">
      <SegBar {progress} {map} heads={task.kind === "http" ? (live?.heads ?? []) : []} status={task.status} height={14} />
    </div>
    <div class="meta">
      <span class="state" data-s={task.status}>{task.media?.stage && running ? task.media.stage : statusLabel[task.status]}</span>
      {#if task.status === "error"}
        <span class="err ell" title={task.error}>{task.error}</span>
      {:else}
        <span class="num">
          {#if task.status === "completed"}
            {bytes(task.size)}
          {:else}
            {bytes(task.done)}{task.size > 0 ? ` of ${bytes(task.size)}` : ""}
          {/if}
        </span>
        <span class="dot"></span>
        <span class="ell faint">{task.status === "completed" ? ago(task.completedAt) : host(task.url)}</span>
      {/if}
    </div>
  </div>

  <div class="nums num">
    {#if running}
      <div class="big">{speed(task.speed)}</div>
      <div class="small">
        {percent(task.done, task.size)}{task.eta >= 0 ? ` · ${eta(task.eta)}` : ""}
      </div>
      <div class="small faint">
        {task.conns}
        {task.kind === "torrent" ? "peers" : task.conns === 1 ? "connection" : "connections"}
      </div>
    {:else if task.status === "seeding"}
      <div class="big">↑ {speed(task.upSpeed)}</div>
      <div class="small">ratio {(task.torrent?.ratio ?? 0).toFixed(2)}</div>
    {:else if task.status !== "completed" && task.size > 0}
      <div class="big muted">{percent(task.done, task.size)}</div>
    {/if}
  </div>

  <div class="acts">
    {#if task.status === "completed"}
      <button class="btn btn--sm btn--icon" title="Open" onclick={(e) => (e.stopPropagation(), store.run(api.open(task.id)))}>
        <Icon name="open" size={14} />
      </button>
    {:else}
      <button class="btn btn--sm btn--icon" title={stoppable ? "Pause" : "Resume"} onclick={toggle}>
        <Icon name={stoppable ? "pause" : task.status === "error" ? "restart" : "play"} size={14} />
      </button>
    {/if}
    <button class="btn btn--sm btn--icon" title="Show in folder" onclick={(e) => (e.stopPropagation(), store.run(api.reveal(task.id)))}>
      <Icon name="folder" size={14} />
    </button>
  </div>
</div>

<style>
  .row {
    display: grid;
    grid-template-columns: 44px minmax(0, 1fr) 128px 70px;
    align-items: center;
    gap: 14px;
    height: var(--row-h);
    padding: 0 16px 0 14px;
    border-bottom: 2px solid var(--ib-paper-2);
    position: relative;
  }
  .row:hover {
    background: color-mix(in srgb, var(--ib-paper-2) 60%, transparent);
  }
  .row.sel {
    background: color-mix(in srgb, var(--ib-accent) 13%, var(--ib-paper));
  }
  .row.sel::before {
    content: "";
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    width: 5px;
    background: var(--ib-accent);
    border-right: 2px solid var(--ib-line);
  }
  .tile {
    width: 44px;
    height: 44px;
    display: grid;
    place-items: center;
    border: 2.5px solid var(--ib-line);
    border-radius: 11px;
    background: var(--c);
    color: #1a110d;
    box-shadow: 3px 3px 0 var(--ib-ex);
    overflow: hidden;
  }
  .tile img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  :global([data-density="compact"]) .tile {
    width: 36px;
    height: 36px;
    border-radius: 9px;
  }
  .main {
    display: flex;
    flex-direction: column;
    gap: 5px;
    min-width: 0;
  }
  .name {
    font-weight: 800;
    font-size: 14.5px;
    letter-spacing: -0.015em;
  }
  .meta {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    font-weight: 650;
    color: var(--ib-muted);
    min-width: 0;
  }
  .state {
    font-weight: 900;
    font-size: 10.5px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--ib-text);
    flex: none;
  }
  .state[data-s="error"],
  .err {
    color: var(--bad);
  }
  .state[data-s="completed"] {
    color: var(--ok);
  }
  .state[data-s="paused"],
  .state[data-s="queued"] {
    color: var(--ib-faint);
  }
  .state[data-s="seeding"] {
    color: var(--ib-alt);
  }
  .dot {
    width: 4px;
    height: 4px;
    border-radius: 1px;
    background: var(--ib-faint);
    flex: none;
  }
  :global([data-density="compact"]) .meta {
    display: none;
  }
  .nums {
    text-align: right;
    line-height: 1.3;
  }
  .big {
    font-weight: 900;
    font-size: 15px;
    letter-spacing: -0.03em;
  }
  .small {
    font-size: 11.5px;
    font-weight: 700;
    color: var(--ib-muted);
  }
  .acts {
    display: flex;
    gap: 6px;
    opacity: 0;
    transition: opacity 0.1s;
  }
  .row:hover .acts,
  .row.sel .acts {
    opacity: 1;
  }
</style>
