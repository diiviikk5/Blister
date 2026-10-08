<script lang="ts">
  import Icon from "./Icon.svelte";
  import SegBar from "./SegBar.svelte";
  import IsoLanes from "./IsoLanes.svelte";
  import { api, copyText } from "../lib/api";
  import { store, prog } from "../lib/store.svelte";
  import { bytes, speed, eta, percent, ago, parseRate } from "../lib/format";
  import { iconFor, statusLabel, kindLabel } from "../lib/kinds";
  import type { Task } from "../lib/types";

  let { task, dock = "right" }: { task: Task; dock?: "right" | "bottom" } = $props();

  const look = $derived(iconFor(task));
  const live = $derived(store.live[task.id]);
  const running = $derived(task.status === "downloading" || task.status === "starting");
  const map = $derived(live?.map || task.torrent?.pieceMap || "");
  const sep = $derived(task.dir.includes("\\") ? "\\" : "/");

  // 3D lanes or just the flat bar; remembered for the session.
  let viz = $state<"iso" | "flat">((sessionStorage.getItem("blister-viz") as "iso" | "flat") || "iso");
  $effect(() => sessionStorage.setItem("blister-viz", viz));

  // Local edits for tuning; resynced when another task is focused.
  let conns = $state(0);
  let limit = $state("");
  let lastId = "";
  $effect(() => {
    if (task.id !== lastId) {
      lastId = task.id;
      conns = task.connections || store.settings?.connections || 16;
      limit = task.speedLimit > 0 ? bytes(task.speedLimit).replace(" ", "") : "";
    }
  });

  function tune() {
    store.run(api.tune(task.id, conns, parseRate(limit)));
  }

  function toggleFile(i: number) {
    const files = task.torrent?.files ?? [];
    const sel = files.map((f, k) => (k === i ? !f.selected : f.selected));
    store.run(api.selectFiles(task.id, sel));
  }
  function allFiles(on: boolean) {
    store.run(api.selectFiles(task.id, (task.torrent?.files ?? []).map(() => on)));
  }
</script>

<aside class="detail" class:bottom={dock === "bottom"} aria-label="Download details">
  <div class="head">
    <div class="tile" style:--c={look.color}><Icon name={look.icon} size={22} stroke={2.6} /></div>
    <div class="title">
      <h2 title={task.name}>{task.name}</h2>
      <div class="pills">
        <span class="pill">{kindLabel[task.kind]}</span>
        <span class="pill" class:pill--accent={running} class:pill--ok={task.status === "completed"} class:pill--bad={task.status === "error"} class:pill--alt={task.status === "seeding"}>
          {statusLabel[task.status]}
        </span>
        {#if task.verified === true}<span class="pill pill--ok"><Icon name="shield" size={11} stroke={3} /> Verified</span>{/if}
      </div>
    </div>
    <button class="btn btn--ghost btn--icon x" onclick={() => (store.focus = null)} aria-label="Close details"><Icon name="x" size={16} /></button>
  </div>

  <div class="body">
    <div class="barwrap">
      {#if viz === "iso" && dock === "right"}
        <IsoLanes {map} heads={task.kind === "http" ? (live?.heads ?? []) : []} progress={prog(task)} status={task.status} />
      {/if}
      <SegBar progress={task.size > 0 || task.status === "completed" ? prog(task) : running ? -1 : 0} {map} heads={task.kind === "http" ? (live?.heads ?? []) : []} status={task.status} height={30} />
      <div class="legend num">
        {#if dock === "right"}
          <button class="viz" onclick={() => (viz = viz === "iso" ? "flat" : "iso")}>{viz === "iso" ? "Flat" : "3D"}</button>
        {/if}
        <span>{percent(task.done, task.size) || (task.status === "completed" ? "100%" : "–")}</span>
        {#if task.kind === "http" && running && live?.heads?.length}
          <span class="faint"><i class="sw"></i>{live.heads.length} connections writing</span>
        {:else if task.kind === "torrent" && task.torrent?.pieces}
          <span class="faint">{task.torrent.pieces.toLocaleString()} pieces</span>
        {/if}
      </div>
    </div>

    {#if task.status === "error" && task.error}
      <div class="ib-alert ib-alert--hot err">
        <span class="ib-alert__icon">!</span>
        <div>
          <div>{task.error}</div>
          <div class="erracts">
            <button class="btn btn--sm" onclick={() => store.run(api.resume([task.id]))}><Icon name="restart" size={13} /> Try again</button>
            <button class="btn btn--sm" onclick={() => (store.relink = task)}><Icon name="link" size={13} /> Use a new link</button>
          </div>
        </div>
      </div>
    {/if}

    <div class="stats">
      <div class="stat">
        <div class="label">{task.status === "seeding" ? "Upload" : "Speed"}</div>
        <div class="v num">{task.status === "seeding" ? speed(task.upSpeed) : running ? speed(task.speed) : "–"}</div>
      </div>
      <div class="stat">
        <div class="label">Time left</div>
        <div class="v num">{running && task.eta >= 0 ? eta(task.eta) : "–"}</div>
      </div>
      <div class="stat">
        <div class="label">Downloaded</div>
        <div class="v num">{bytes(task.done)}</div>
        <div class="s faint num">of {bytes(task.size)}</div>
      </div>
      <div class="stat">
        <div class="label">{task.kind === "torrent" ? "Peers" : "Connections"}</div>
        <div class="v num">{running || task.status === "seeding" ? task.conns : "–"}</div>
        {#if task.kind === "torrent"}<div class="s faint num">{task.seeds} seeds</div>{/if}
      </div>
    </div>

    {#if task.status !== "completed"}
      <section>
        <div class="label">Tuning</div>
        {#if task.kind === "http" || task.kind === "media" || task.kind === "hls"}
          <div class="tune">
            <span class="tl">Connections</span>
            <input type="range" min="1" max="64" bind:value={conns} onchange={tune} aria-label="Connections" />
            <span class="tv num">{conns}</span>
          </div>
        {/if}
        <div class="tune">
          <span class="tl">Speed cap</span>
          <input class="field" placeholder="Unlimited, or e.g. 2 MB" bind:value={limit} onchange={tune} onkeydown={(e) => e.key === "Enter" && tune()} />
        </div>
      </section>
    {/if}

    {#if task.torrent?.files?.length}
      <section>
        <div class="row-h">
          <div class="label">Files <span class="faint">{task.torrent.files.length}</span></div>
          <div class="mini">
            <button onclick={() => allFiles(true)}>All</button>
            <button onclick={() => allFiles(false)}>None</button>
          </div>
        </div>
        <div class="files">
          {#each task.torrent.files as f, i (f.path)}
            <label class="file">
              <span class="ib-check"><input type="checkbox" checked={f.selected} onchange={() => toggleFile(i)} /></span>
              <span class="fp ell" title={f.path}>{f.path.split("/").pop()}</span>
              <span class="fs num faint">{f.selected && f.size ? `${Math.floor((f.done / f.size) * 100)}% · ` : ""}{bytes(f.size)}</span>
            </label>
          {/each}
        </div>
      </section>
    {/if}

    <section class="info">
      <div class="label">Details</div>
      <dl>
        <dt>Saved to</dt>
        <dd>
          <button class="linkish ell" title="Show in folder" onclick={() => store.run(api.reveal(task.id))}>{task.dir}{sep}{task.name}</button>
        </dd>
        <dt>Link</dt>
        <dd class="linkrow">
          <span class="ell mono" title={task.url}>{task.url}</span>
          <button class="btn btn--sm btn--icon" onclick={() => (copyText(task.url), store.toast("Link copied"))} aria-label="Copy link"><Icon name="copy" size={13} /></button>
        </dd>
        <dt>Added</dt>
        <dd>{ago(task.createdAt)}</dd>
        {#if task.completedAt && !task.completedAt.startsWith("0001")}
          <dt>Finished</dt>
          <dd>{ago(task.completedAt)}</dd>
        {/if}
        {#if task.kind === "http"}
          <dt>Resume</dt>
          <dd>{task.resumable ? "Supported" : task.status === "completed" ? "–" : "Not supported by server"}</dd>
        {/if}
        {#if task.checksum}
          <dt>Checksum</dt>
          <dd class="ell mono" title={task.checksum}>{task.checksum}</dd>
        {/if}
        {#if task.torrent?.infoHash}
          <dt>Info hash</dt>
          <dd class="ell mono">{task.torrent.infoHash}</dd>
          <dt>Ratio</dt>
          <dd class="num">{task.torrent.ratio.toFixed(2)} · {bytes(task.uploaded)} up</dd>
        {/if}
      </dl>
    </section>
  </div>

  <div class="foot">
    {#if task.status === "completed"}
      <button class="btn btn--accent" onclick={() => store.run(api.open(task.id))}><Icon name="open" size={15} /> Open</button>
    {:else if running || task.status === "queued" || task.status === "seeding"}
      <button class="btn" onclick={() => store.run(api.pause([task.id]))}><Icon name="pause" size={15} /> Pause</button>
    {:else}
      <button class="btn btn--accent" onclick={() => store.run(api.resume([task.id]))}><Icon name="play" size={15} /> Resume</button>
    {/if}
    <button class="btn" onclick={() => store.run(api.reveal(task.id))}><Icon name="folder" size={15} /> Folder</button>
  </div>
</aside>

<style>
  .detail {
    width: var(--detail-w);
    display: flex;
    flex-direction: column;
    border-left: var(--bw-lg) solid var(--ib-line);
    background: transparent; /* the app shell paints the one translucent layer */
    min-height: 0;
  }
  /* docked under the list: wide and short, sections side by side */
  .bottom {
    width: auto;
    height: 300px;
    flex: none;
    border-left: 0;
    border-top: var(--bw-lg) solid var(--ib-line);
    display: grid;
    grid-template-columns: 300px 1fr;
    grid-template-rows: auto 1fr;
  }
  .bottom .head {
    grid-column: 1;
    grid-row: 1;
    border-bottom: 0;
  }
  .bottom .foot {
    grid-column: 1;
    grid-row: 2;
    align-self: end;
    border-top: 0;
  }
  .bottom .body {
    grid-column: 2;
    grid-row: 1 / span 2;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
    align-content: start;
    border-left: calc(var(--bw) * 0.8) solid var(--ib-paper-2);
  }
  .head {
    display: flex;
    gap: 12px;
    align-items: flex-start;
    padding: 18px 16px 14px;
    border-bottom: 2px solid var(--ib-paper-2);
  }
  .tile {
    width: 48px;
    height: 48px;
    flex: none;
    display: grid;
    place-items: center;
    border: var(--bw) solid var(--ib-line);
    border-radius: var(--r);
    background: var(--c);
    color: #140d1f;
    box-shadow: var(--ib-ex4);
  }
  .title {
    flex: 1;
    min-width: 0;
  }
  h2 {
    margin: 2px 0 8px;
    font-size: 17px;
    font-weight: var(--h-weight);
    letter-spacing: var(--h-track);
    line-height: 1.2;
    word-break: break-word;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .pills {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .x {
    margin: -4px -6px 0 0;
  }
  .body {
    flex: 1;
    overflow-y: auto;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 18px;
  }
  .viz {
    padding: 1px 7px;
    border: calc(var(--bw) * 0.7) solid var(--ib-line);
    border-radius: var(--r-xs);
    background: none;
    font-size: 10.5px;
    font-weight: 900;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    cursor: pointer;
    color: var(--ib-muted);
  }
  .viz:hover {
    background: var(--ib-accent);
    color: var(--ib-accent-ink);
  }
  .legend {
    display: flex;
    justify-content: space-between;
    margin-top: 8px;
    font-size: 12px;
    font-weight: 800;
  }
  .sw {
    display: inline-block;
    width: 4px;
    height: 10px;
    margin-right: 6px;
    background: var(--ib-hot);
    border: calc(var(--bw) * 0.6) solid var(--ib-line);
    vertical-align: -1px;
  }
  .err {
    font-size: 13px;
  }
  .erracts {
    display: flex;
    gap: 8px;
    margin-top: 10px;
  }
  .stats {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;
  }
  .stat {
    padding: 11px 12px;
    border: var(--bw) solid var(--ib-line);
    border-radius: var(--r);
    background: var(--ib-card);
    box-shadow: var(--ib-ex3);
  }
  .stat .label {
    margin: 0 0 4px;
    font-size: 10.5px;
    color: var(--ib-faint);
  }
  .v {
    font-size: 21px;
    font-weight: var(--h-weight);
    letter-spacing: var(--h-track);
  }
  .s {
    font-size: 11.5px;
    font-weight: 700;
  }
  section .label {
    margin-bottom: 10px;
  }
  .tune {
    display: grid;
    grid-template-columns: 100px 1fr auto;
    align-items: center;
    gap: 10px;
    margin-bottom: 10px;
  }
  .tune .field {
    grid-column: span 2;
    height: 34px;
  }
  .tl {
    font-size: 13px;
    font-weight: 750;
    color: var(--ib-muted);
  }
  .tv {
    width: 26px;
    text-align: right;
    font-weight: 900;
  }
  input[type="range"] {
    width: 100%;
    accent-color: var(--ib-accent);
  }
  .row-h {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
  }
  .mini button {
    border: 0;
    background: none;
    font-weight: 800;
    font-size: 12px;
    color: var(--ib-muted);
    cursor: pointer;
    padding: 2px 4px;
  }
  .mini button:hover {
    color: var(--ib-text);
    text-decoration: underline;
  }
  .files {
    border: var(--bw) solid var(--ib-line);
    border-radius: var(--r);
    background: var(--ib-card);
    max-height: 260px;
    overflow-y: auto;
  }
  .file {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 10px;
    border-bottom: 2px solid var(--ib-paper-2);
    font-size: 13px;
    font-weight: 700;
    cursor: pointer;
  }
  .file:last-child {
    border-bottom: 0;
  }
  .file :global(.ib-check input) {
    width: 20px;
    height: 20px;
    border-width: var(--bw);
    border-radius: var(--r-sm);
  }
  .fp {
    flex: 1;
  }
  .fs {
    font-size: 11.5px;
    white-space: nowrap;
  }
  dl {
    display: grid;
    grid-template-columns: 82px minmax(0, 1fr);
    gap: 9px 10px;
    margin: 0;
    font-size: 12.5px;
  }
  dt {
    font-weight: 800;
    color: var(--ib-faint);
  }
  dd {
    margin: 0;
    font-weight: 650;
    min-width: 0;
  }
  .linkrow {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .linkrow .ell {
    flex: 1;
    font-size: 11.5px;
  }
  .linkish {
    display: block;
    width: 100%;
    padding: 0;
    border: 0;
    background: none;
    text-align: left;
    font-weight: 650;
    cursor: pointer;
    text-decoration: underline;
    text-decoration-color: var(--ib-accent);
    text-decoration-thickness: 2px;
    text-underline-offset: 3px;
  }
  .foot {
    display: flex;
    gap: 10px;
    padding: 14px 16px;
    border-top: var(--bw-lg) solid var(--ib-line);
  }
  .foot .btn {
    flex: 1;
  }
</style>
