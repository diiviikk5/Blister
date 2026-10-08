<script lang="ts">
  import Icon from "./Icon.svelte";
  import SpeedGraph from "./SpeedGraph.svelte";
  import { store, type Filter } from "../lib/store.svelte";

  const states: { id: Filter; label: string; icon: string }[] = [
    { id: "all", label: "All downloads", icon: "list" },
    { id: "active", label: "Active", icon: "down" },
    { id: "queued", label: "Queued", icon: "clock" },
    { id: "paused", label: "Paused", icon: "pause" },
    { id: "completed", label: "Finished", icon: "check" },
    { id: "failed", label: "Failed", icon: "alert" },
  ];
  const cats: { id: string; label: string; icon: string }[] = [
    { id: "video", label: "Video", icon: "film" },
    { id: "audio", label: "Music", icon: "music" },
    { id: "torrent", label: "Torrents", icon: "magnet" },
    { id: "archive", label: "Archives", icon: "box" },
    { id: "program", label: "Programs", icon: "app" },
    { id: "document", label: "Documents", icon: "doc" },
    { id: "image", label: "Images", icon: "image" },
    { id: "other", label: "Other", icon: "file" },
  ];

  function pick(f: Filter) {
    store.filter = f;
    store.view = "list";
  }
</script>

<nav class="side" aria-label="Filters">
  <button class="btn btn--accent btn--lg add" onclick={() => store.openAdd()}>
    <Icon name="plus" size={20} stroke={3} />
    Add download
  </button>

  <div class="group">
    {#each states as s (s.id)}
      {@const n = store.counts[s.id] ?? 0}
      {#if s.id !== "failed" || n > 0}
        <button
          class="link"
          class:on={store.view === "list" && store.filter === s.id}
          class:bad={s.id === "failed"}
          onclick={() => pick(s.id)}
        >
          <Icon name={s.icon} size={17} />
          <span class="ell">{s.label}</span>
          <span class="count num">{n}</span>
        </button>
      {/if}
    {/each}
  </div>

  <div class="group">
    <div class="label head">Types</div>
    {#each cats as c (c.id)}
      {@const n = store.counts[`cat:${c.id}`] ?? 0}
      {#if n > 0}
        <button class="link" class:on={store.view === "list" && store.filter === `cat:${c.id}`} onclick={() => pick(`cat:${c.id}`)}>
          <Icon name={c.icon} size={17} />
          <span class="ell">{c.label}</span>
          <span class="count num">{n}</span>
        </button>
      {/if}
    {/each}
  </div>

  <div class="bottom">
    <SpeedGraph />
    <button class="link" class:on={store.view === "rice"} onclick={() => (store.view = "rice")}>
      <Icon name="sliders" size={17} />
      <span>Rice Studio</span>
    </button>
    <button class="link" class:on={store.view === "settings"} onclick={() => (store.view = "settings")}>
      <Icon name="gear" size={17} />
      <span>Settings</span>
      <kbd class="count">Ctrl ,</kbd>
    </button>
  </div>
</nav>

<style>
  .side {
    display: flex;
    flex-direction: column;
    gap: 18px;
    width: var(--side-w);
    padding: 18px 14px 14px;
    border-right: var(--bw-lg) solid var(--ib-line);
    background: var(--ib-paper);
    overflow-y: auto;
  }
  .add {
    width: calc(100% - 6px);
  }
  .group {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .head {
    margin: 0 10px 6px;
    color: var(--ib-faint);
  }
  .link {
    display: flex;
    align-items: center;
    gap: 10px;
    height: 36px;
    padding: 0 10px;
    border: var(--bw) solid transparent;
    border-radius: var(--r);
    background: none;
    font-weight: 750;
    font-size: 14px;
    color: var(--ib-muted);
    cursor: pointer;
    text-align: left;
  }
  .link:hover {
    background: var(--ib-paper-2);
    color: var(--ib-text);
  }
  .link.on {
    background: var(--ib-accent);
    color: var(--ib-accent-ink);
    border-color: var(--ib-line);
    box-shadow: var(--ib-ex3);
  }
  .link.bad:not(.on) {
    color: var(--bad);
  }
  .link span:nth-child(2) {
    flex: 1;
  }
  .count {
    font-size: 12px;
    font-weight: 800;
    opacity: 0.75;
  }
  kbd.count {
    font-family: var(--ib-mono);
    font-size: 10.5px;
  }
  .bottom {
    margin-top: auto;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
</style>
