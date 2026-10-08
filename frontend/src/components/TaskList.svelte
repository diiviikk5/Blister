<script lang="ts">
  import TaskRow from "./TaskRow.svelte";
  import Menu, { type MenuItem } from "./Menu.svelte";
  import Icon from "./Icon.svelte";
  import { api, copyText } from "../lib/api";
  import { store } from "../lib/store.svelte";
  import type { Task } from "../lib/types";

  let { onremove, onrename }: { onremove: (ids: string[]) => void; onrename: (t: Task) => void } = $props();

  let menu = $state<{ x: number; y: number; items: MenuItem[] } | null>(null);

  function context(e: MouseEvent, t: Task) {
    const ids = store.targets();
    const many = ids.length > 1;
    const running = t.status === "downloading" || t.status === "starting" || t.status === "queued" || t.status === "seeding";
    menu = {
      x: e.clientX,
      y: e.clientY,
      items: [
        { label: "Open", icon: "open", hint: "Enter", disabled: many || t.status !== "completed", run: () => store.run(api.open(t.id)) },
        { label: "Show in folder", icon: "folder", disabled: many, run: () => store.run(api.reveal(t.id)) },
        { sep: true },
        running
          ? { label: many ? `Pause ${ids.length}` : "Pause", icon: "pause", hint: "Space", run: () => store.run(api.pause(ids)) }
          : { label: many ? `Resume ${ids.length}` : "Resume", icon: "play", hint: "Space", disabled: t.status === "completed" && !many, run: () => store.run(api.resume(ids)) },
        { label: "Restart from zero", icon: "restart", run: () => store.run(api.restart(ids)) },
        { label: "Rename…", icon: "doc", hint: "F2", disabled: many, run: () => onrename(t) },
        { label: "Replace link…", icon: "link", disabled: many || t.status === "completed", run: () => (store.relink = t) },
        { sep: true },
        { label: many ? `Copy ${ids.length} links` : "Copy link", icon: "link", run: () => copyText(ids.map((i) => store.tasks[i]?.url).join("\n")) },
        { label: "Download again", icon: "down", disabled: many, run: () => store.openAdd({ urls: [t.url], request: t.request, source: "launch" }) },
        { sep: true },
        { label: many ? `Remove ${ids.length}` : "Remove", icon: "trash", hint: "Del", danger: true, run: () => onremove(ids) },
      ],
    };
  }
</script>

<div class="list" role="listbox" aria-multiselectable="true" aria-label="Downloads">
  {#each store.visible as task (task.id)}
    <TaskRow {task} oncontext={context} />
  {:else}
    <div class="empty">
      {#if store.query}
        <p class="big">Nothing matches “{store.query}”.</p>
        <button class="btn" onclick={() => (store.query = "")}>Clear search</button>
      {:else if store.all.length}
        <p class="big">Nothing here right now.</p>
        <button class="btn" onclick={() => (store.filter = "all")}>Show all downloads</button>
      {:else}
        <div class="drop">
          <div class="keys" aria-hidden="true">
            <span class="ib-keycap">Ctrl</span><span class="plus">+</span><span class="ib-keycap ib-keycap--accent">V</span>
          </div>
          <p class="big">Paste a link to start.</p>
          <p class="muted">Files, magnets, .torrent files, YouTube and 1,800 other sites. Drop them here or copy one anywhere and Blister will offer to grab it.</p>
          <button class="btn btn--accent btn--lg" onclick={() => store.openAdd()}><Icon name="plus" size={18} stroke={3} /> Add download</button>
        </div>
      {/if}
    </div>
  {/each}
</div>

{#if menu}
  <Menu {...menu} onclose={() => (menu = null)} />
{/if}

<style>
  .list {
    flex: 1;
    overflow-y: auto;
    outline: none;
  }
  .empty {
    height: 100%;
    display: grid;
    place-content: center;
    justify-items: center;
    gap: 14px;
    padding: 40px;
    text-align: center;
  }
  .drop {
    display: grid;
    justify-items: center;
    gap: 14px;
    max-width: 440px;
  }
  .big {
    margin: 0;
    font-size: 30px;
    font-weight: 900;
    letter-spacing: -0.05em;
  }
  .muted {
    margin: 0 0 8px;
    line-height: 1.5;
  }
  .keys {
    display: flex;
    align-items: center;
    margin-bottom: 8px;
  }
  .keys :global(.ib-keycap) {
    height: 56px;
    min-width: 56px;
    font-size: 18px;
  }
  .plus {
    font-weight: 900;
    font-size: 22px;
    margin: 0 10px;
  }
</style>
