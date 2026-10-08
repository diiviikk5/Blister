<script lang="ts">
  import Icon from "./Icon.svelte";
  import Menu, { type MenuItem } from "./Menu.svelte";
  import { api } from "../lib/api";
  import { store } from "../lib/store.svelte";
  import { speed, bytes } from "../lib/format";

  let menu = $state<{ x: number; y: number; items: MenuItem[] } | null>(null);
  const limit = $derived(store.settings?.speedLimit ?? 0);

  const presets = [0, 256 * 1024, 1024 ** 2, 5 * 1024 ** 2, 10 * 1024 ** 2, 25 * 1024 ** 2, 50 * 1024 ** 2];

  function openLimit(e: MouseEvent) {
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
    menu = {
      x: r.left,
      y: r.top - presets.length * 34 - 20,
      items: presets.map((p) => ({
        label: p === 0 ? "Unlimited" : `${bytes(p, 0)}/s`,
        icon: p === limit ? "check" : undefined,
        run: () => store.run(api.setSpeedLimit(p).then((s) => (store.settings = s))),
      })),
    };
  }

  const scheduled = $derived(!!(store.settings?.scheduleStart && store.settings?.scheduleEnd));
</script>

<footer class="status">
  <div class="cell">
    <Icon name="down" size={15} stroke={2.8} />
    <span class="num strong">{speed(store.speed)}</span>
  </div>
  <div class="cell">
    <Icon name="up" size={15} stroke={2.8} />
    <span class="num">{speed(store.upSpeed)}</span>
  </div>
  <div class="cell faint">
    {store.counts.active ?? 0} active · {store.counts.queued ?? 0} queued
  </div>

  <div class="spacer"></div>

  {#if scheduled}
    <div class="cell faint" title="Downloads only run inside this window">
      <Icon name="clock" size={14} /> {store.settings?.scheduleStart}–{store.settings?.scheduleEnd}
    </div>
  {/if}
  <button class="cell btnish" class:on={limit > 0} onclick={openLimit} title="Global speed cap">
    <Icon name={limit > 0 ? "turtle" : "bolt"} size={15} />
    {limit > 0 ? `Capped at ${bytes(limit, 0)}/s` : "Full speed"}
  </button>
  <button class="cell btnish" onclick={() => store.run(api.openDownloads())} title="Open download folder">
    <Icon name="folder" size={14} /> Folder
  </button>
  <div class="cell faint mono ver">v{store.version}</div>
</footer>

{#if menu}<Menu {...menu} onclose={() => (menu = null)} />{/if}

<style>
  .status {
    display: flex;
    align-items: center;
    gap: 4px;
    height: 34px;
    padding: 0 8px;
    border-top: var(--bw-lg) solid var(--ib-line);
    background: var(--ib-paper-2);
    font-size: 12.5px;
    font-weight: 700;
  }
  .cell {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 0 8px;
    height: 26px;
    white-space: nowrap;
  }
  .strong {
    font-weight: 900;
  }
  .spacer {
    flex: 1;
  }
  .btnish {
    border: 2px solid transparent;
    border-radius: var(--r-sm);
    background: none;
    font-weight: 750;
    cursor: pointer;
    color: var(--ib-muted);
  }
  .btnish:hover {
    border-color: var(--ib-line);
    color: var(--ib-text);
    background: var(--ib-card);
  }
  .btnish.on {
    background: var(--ib-hot);
    color: var(--ib-hot-ink);
    border-color: var(--ib-line);
  }
  .ver {
    font-size: 11px;
  }
</style>
