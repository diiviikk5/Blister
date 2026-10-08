<script lang="ts">
  import Icon from "./Icon.svelte";
  import { api } from "../lib/api";
  import { store, type SortKey } from "../lib/store.svelte";

  let { onremove }: { onremove: () => void } = $props();

  const titles: Record<string, string> = {
    all: "All downloads",
    active: "Active",
    queued: "Queued",
    paused: "Paused",
    completed: "Finished",
    failed: "Failed",
    "cat:video": "Video",
    "cat:audio": "Music",
    "cat:torrent": "Torrents",
    "cat:archive": "Archives",
    "cat:program": "Programs",
    "cat:document": "Documents",
    "cat:image": "Images",
    "cat:other": "Other",
  };

  const t = $derived(store.targets());
  const any = $derived(t.length > 0);
  const hasDone = $derived((store.counts.completed ?? 0) > 0);
</script>

<div class="tools">
  <h1>
    {titles[store.filter] ?? "Downloads"}
    <span class="n num">{store.visible.length}</span>
  </h1>

  <div class="group">
    <button class="btn" disabled={!any} onclick={() => store.run(api.resume(t))} title="Resume (Space)">
      <Icon name="play" size={15} /> Resume
    </button>
    <button class="btn" disabled={!any} onclick={() => store.run(api.pause(t))} title="Pause (Space)">
      <Icon name="pause" size={15} /> Pause
    </button>
    <button class="btn btn--icon" disabled={!any} onclick={onremove} title="Remove (Delete)" aria-label="Remove">
      <Icon name="trash" size={16} />
    </button>
  </div>

  <div class="spacer"></div>

  <select class="field sort" bind:value={store.sort} aria-label="Sort by">
    {#each [["smart", "Smart"], ["added", "Newest"], ["name", "Name"], ["size", "Size"], ["progress", "Progress"], ["speed", "Speed"]] as [k, l] (k)}
      <option value={k as SortKey}>{l}</option>
    {/each}
  </select>

  <div class="group bulk">
    <button class="btn btn--ghost" onclick={() => store.run(api.resumeAll())} title="Resume everything">Resume all</button>
    <button class="btn btn--ghost" onclick={() => store.run(api.pauseAll())} title="Pause everything">Pause all</button>
    {#if hasDone}
      <button class="btn btn--ghost" onclick={() => store.run(api.clearCompleted())} title="Remove finished downloads from the list; files stay">
        Clear finished
      </button>
    {/if}
  </div>
</div>

<style>
  .tools {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 16px 18px 14px;
    border-bottom: 3px solid var(--ib-line);
  }
  h1 {
    margin: 0 8px 0 0;
    display: flex;
    align-items: baseline;
    gap: 9px;
    font-size: 26px;
    font-weight: 900;
    letter-spacing: -0.05em;
    white-space: nowrap;
  }
  .n {
    font-size: 14px;
    font-weight: 800;
    letter-spacing: 0;
    color: var(--ib-faint);
  }
  .group {
    display: flex;
    gap: 8px;
  }
  .spacer {
    flex: 1;
  }
  .sort {
    width: 118px;
    height: 36px;
  }
  .tools {
    container-type: inline-size;
    overflow: hidden;
  }
  @container (max-width: 860px) {
    .bulk {
      display: none;
    }
  }
  @container (max-width: 560px) {
    .sort,
    .label-txt {
      display: none;
    }
  }
</style>
