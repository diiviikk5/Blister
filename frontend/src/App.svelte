<script lang="ts">
  import TitleBar from "./components/TitleBar.svelte";
  import Sidebar from "./components/Sidebar.svelte";
  import Toolbar from "./components/Toolbar.svelte";
  import TaskList from "./components/TaskList.svelte";
  import Detail from "./components/Detail.svelte";
  import StatusBar from "./components/StatusBar.svelte";
  import AddDialog from "./components/AddDialog.svelte";
  import Settings from "./components/Settings.svelte";
  import RiceStudio from "./components/RiceStudio.svelte";
  import Palette from "./components/Palette.svelte";
  import Toasts from "./components/Toasts.svelte";
  import Modal from "./components/Modal.svelte";
  import { api, onFileDrop, readClipboard } from "./lib/api";
  import { store } from "./lib/store.svelte";
  import type { Task } from "./lib/types";

  let ready = $state(false);
  let failed = $state("");
  let search = $state<HTMLInputElement>();
  let removing = $state<string[] | null>(null);
  let withFiles = $state(false);
  let renaming = $state<Task | null>(null);
  let newName = $state("");
  let dragging = $state(false);
  let relinkURL = $state("");

  // Prefill a replacement link from the clipboard when it looks like one.
  $effect(() => {
    const t = store.relink;
    if (!t) return;
    relinkURL = "";
    readClipboard().then((c) => {
      const v = c.trim();
      if (/^(https?:\/\/|magnet:)/i.test(v) && v !== t.url) relinkURL = v;
    });
  });

  function doRelink() {
    const t = store.relink!;
    store.relink = null;
    store.run(api.setURL(t.id, relinkURL.trim()).then(() => store.toast("Link replaced. Resuming where it left off.", "ok")));
  }

  store
    .init()
    .then(() => (ready = true))
    .catch((e) => (failed = String(e)));


  function askRemove(ids: string[]) {
    if (!ids.length) return;
    if (store.settings?.confirmDelete === false) {
      store.run(api.remove(ids, false));
      return;
    }
    withFiles = false;
    removing = ids;
  }

  function doRemove() {
    const ids = removing!;
    removing = null;
    store.run(api.remove(ids, withFiles));
  }

  function askRename(t: Task) {
    newName = t.name;
    renaming = t;
  }

  function doRename() {
    const t = renaming!;
    renaming = null;
    if (newName.trim() && newName !== t.name) store.run(api.rename(t.id, newName.trim()));
  }

  function typing(e: KeyboardEvent) {
    const el = e.target as HTMLElement;
    return el.closest("input, textarea, select, [contenteditable]") !== null;
  }

  async function keys(e: KeyboardEvent) {
    const mod = e.ctrlKey || e.metaKey;
    if (mod && e.key.toLowerCase() === "k") {
      e.preventDefault();
      store.paletteOpen = !store.paletteOpen;
      return;
    }
    if (store.addOpen || removing || renaming || store.relink || store.paletteOpen) return;
    if (mod && e.key.toLowerCase() === "r" && e.shiftKey) {
      e.preventDefault();
      store.view = store.view === "rice" ? "list" : "rice";
      return;
    }
    if (mod && e.key.toLowerCase() === "f") {
      e.preventDefault();
      store.view = "list";
      search?.focus();
      search?.select();
      return;
    }
    if (mod && e.key === ",") {
      e.preventDefault();
      store.view = store.view === "settings" ? "list" : "settings";
      return;
    }
    if (mod && e.key.toLowerCase() === "n") {
      e.preventDefault();
      store.openAdd();
      return;
    }
    if (typing(e)) return;
    if (mod && e.key.toLowerCase() === "v") {
      e.preventDefault();
      const text = await readClipboard();
      store.openAdd(text ? { urls: [text], request: { url: "" }, source: "clipboard" } : null);
      return;
    }
    if (store.view !== "list") return;
    const ids = store.targets();
    if (mod && e.key.toLowerCase() === "a") {
      e.preventDefault();
      store.selected = store.visible.map((t) => t.id);
      return;
    }
    if (e.key === "Delete" && ids.length) {
      askRemove(ids);
    } else if (e.key === " " && ids.length) {
      e.preventDefault();
      const anyRunning = ids.some((id) => ["downloading", "starting", "queued", "seeding"].includes(store.tasks[id]?.status));
      store.run(anyRunning ? api.pause(ids) : api.resume(ids));
    } else if (e.key === "Enter" && store.focused?.status === "completed") {
      store.run(api.open(store.focused.id));
    } else if (e.key === "F2" && store.focused) {
      askRename(store.focused);
    } else if (e.key === "Escape") {
      store.selected = [];
      store.focus = null;
    } else if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      const list = store.visible;
      if (!list.length) return;
      const i = list.findIndex((t) => t.id === store.focus);
      const next = list[Math.max(0, Math.min(list.length - 1, i + (e.key === "ArrowDown" ? 1 : -1)))];
      store.select(next.id, e.shiftKey ? "range" : "one");
      document.querySelector(`[aria-selected="true"]`)?.scrollIntoView({ block: "nearest" });
    }
  }

  // Native file drops (.torrent) arrive through Wails; text/links through HTML5.
  onFileDrop((paths) => {
    const torrents = paths.filter((p) => p.toLowerCase().endsWith(".torrent"));
    if (torrents.length) store.openAdd({ urls: torrents, request: { url: "" }, source: "launch" });
    else store.toast("Drop .torrent files or links", "info");
  });

  function drop(e: DragEvent) {
    e.preventDefault();
    dragging = false;
    const text = e.dataTransfer?.getData("text/uri-list") || e.dataTransfer?.getData("text/plain") || "";
    if (text.trim()) store.openAdd({ urls: [text.trim()], request: { url: "" }, source: "launch" });
  }
</script>

<svelte:window onkeydown={keys} />

{#if store.rice.fx.wallpaper}<div class="wall" aria-hidden="true"></div>{/if}
<div class="fx" aria-hidden="true"></div>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="app"
  ondragover={(e) => {
    e.preventDefault();
    dragging = true;
  }}
  ondragleave={(e) => e.relatedTarget === null && (dragging = false)}
  ondrop={drop}
>
  <TitleBar bind:search />
  {#if failed}
    <div class="boot-err">
      <h2>Blister couldn't start</h2>
      <p class="mono">{failed}</p>
    </div>
  {:else if ready}
    {@const L = store.rice.layout}
    <div class="body" data-side={L.sidebar}>
      {#if L.sidebar !== "hidden"}<Sidebar />{/if}
      <main class="main">
        {#if store.view === "settings"}
          <Settings />
        {:else if store.view === "rice"}
          <RiceStudio />
        {:else}
          <Toolbar onremove={() => askRemove(store.targets())} />
          <TaskList onremove={askRemove} onrename={askRename} />
          {#if L.detail === "bottom" && store.focused}
            {#key store.focused.id}
              <Detail task={store.focused} dock="bottom" />
            {/key}
          {/if}
        {/if}
      </main>
      {#if store.view === "list" && L.detail === "right" && store.focused}
        {#key store.focused.id}
          <Detail task={store.focused} />
        {/key}
      {/if}
    </div>
    <StatusBar />
  {/if}

  {#if dragging}
    <div class="dropzone" aria-hidden="true">
      <div class="dz">Drop to download</div>
    </div>
  {/if}
</div>

{#if store.addOpen}<AddDialog />{/if}
{#if store.paletteOpen}<Palette />{/if}

{#if removing}
  <Modal title={removing.length === 1 ? "Remove this download?" : `Remove ${removing.length} downloads?`} onclose={() => (removing = null)}>
    <p>
      {#if removing.length === 1}
        <strong>{store.tasks[removing[0]]?.name}</strong> will leave the list.
      {:else}
        They will leave the list.
      {/if}
      Unfinished parts are always cleaned up.
    </p>
    <label class="ib-check del"><input type="checkbox" bind:checked={withFiles} /> Also delete the downloaded files</label>
    {#snippet actions()}
      <button class="btn" onclick={() => (removing = null)}>Cancel</button>
      <button class="btn btn--danger" onclick={doRemove}>{withFiles ? "Remove and delete" : "Remove"}</button>
    {/snippet}
  </Modal>
{/if}

{#if renaming}
  <Modal title="Rename" onclose={() => (renaming = null)}>
    <!-- svelte-ignore a11y_autofocus -->
    <input class="field" bind:value={newName} autofocus onkeydown={(e) => e.key === "Enter" && doRename()} />
    {#snippet actions()}
      <button class="btn" onclick={() => (renaming = null)}>Cancel</button>
      <button class="btn btn--accent" onclick={doRename}>Rename</button>
    {/snippet}
  </Modal>
{/if}

{#if store.relink}
  <Modal title="Replace link" onclose={() => (store.relink = null)}>
    <p>
      Paste a fresh link for <strong>{store.relink.name}</strong>. Progress is kept if it's the same file. Links that expire
      (signed or session URLs) are the usual reason to do this.
    </p>
    <!-- svelte-ignore a11y_autofocus -->
    <input class="field mono" bind:value={relinkURL} placeholder="https://…" autofocus onkeydown={(e) => e.key === "Enter" && relinkURL.trim() && doRelink()} />
    {#snippet actions()}
      <button class="btn" onclick={() => (store.relink = null)}>Cancel</button>
      <button class="btn btn--accent" disabled={!relinkURL.trim()} onclick={doRelink}>Replace and resume</button>
    {/snippet}
  </Modal>
{/if}

<Toasts />

<style>
  .app {
    height: 100%;
    display: flex;
    flex-direction: column;
    background: var(--ib-paper);
    color: var(--ib-text);
  }
  .body {
    flex: 1;
    display: flex;
    min-height: 0;
  }
  .body[data-side="right"] :global(.side) {
    order: 3;
    border-right: 0;
    border-left: var(--bw-lg) solid var(--ib-line);
  }
  .main {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    position: relative;
  }
  .boot-err {
    margin: auto;
    max-width: 560px;
    text-align: center;
  }
  .dropzone {
    position: fixed;
    inset: 0;
    z-index: 70;
    display: grid;
    place-items: center;
    background: color-mix(in srgb, var(--ib-accent) 18%, transparent);
    pointer-events: none;
  }
  .dz {
    padding: 28px 40px;
    border: var(--bw-lg) dashed var(--ib-line);
    border-radius: var(--r-lg);
    background: var(--ib-card);
    box-shadow: var(--ib-ex10);
    font-size: 30px;
    font-weight: var(--h-weight);
    letter-spacing: var(--h-track);
  }
  .del {
    margin-top: 14px;
    font-size: 13.5px;
    color: var(--ib-text);
  }
  .del input {
    width: 20px;
    height: 20px;
    border-width: var(--bw);
    border-radius: var(--r-sm);
  }
</style>
