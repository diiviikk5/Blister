<script lang="ts">
  import Icon from "./Icon.svelte";
  import Logo from "./Logo.svelte";
  import { store } from "../lib/store.svelte";
  import { win, native } from "../lib/api";

  let { search = $bindable() }: { search?: HTMLInputElement } = $props();
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<header class="bar" style="--wails-draggable: drag" ondblclick={() => win.toggleMaximise()}>
  <div class="brand">
    <Logo size={24} />
    <span>Blister</span>
  </div>

  <label class="search" style="--wails-draggable: no-drag">
    <Icon name="search" size={16} />
    <input
      bind:this={search}
      bind:value={store.query}
      placeholder="Search downloads"
      spellcheck="false"
      onkeydown={(e) => e.key === "Escape" && ((store.query = ""), (e.currentTarget as HTMLInputElement).blur())}
    />
    {#if store.query}
      <button class="clear" onclick={() => (store.query = "")} aria-label="Clear search"><Icon name="x" size={14} /></button>
    {:else}
      <kbd>Ctrl F</kbd>
    {/if}
  </label>

  <div class="spacer"></div>

  {#if native}
    <div class="win" style="--wails-draggable: no-drag">
      <button onclick={() => win.minimise()} aria-label="Minimise"><Icon name="min" size={16} /></button>
      <button onclick={() => win.toggleMaximise()} aria-label="Maximise"><Icon name="max" size={14} /></button>
      <button class="close" onclick={() => win.quit()} aria-label="Close"><Icon name="x" size={16} /></button>
    </div>
  {/if}
</header>

<style>
  .bar {
    display: flex;
    align-items: center;
    gap: 14px;
    height: 52px;
    padding: 0 0 0 16px;
    border-bottom: 3px solid var(--ib-line);
    background: var(--ib-paper);
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
    width: calc(var(--side-w) - 30px);
    font-weight: 900;
    font-size: 19px;
    letter-spacing: -0.045em;
  }
  .search {
    display: flex;
    align-items: center;
    gap: 8px;
    width: min(440px, 40vw);
    height: 34px;
    padding: 0 10px;
    border: 2.5px solid var(--ib-line);
    border-radius: 10px;
    background: var(--ib-paper-2);
    color: var(--ib-faint);
    box-shadow: inset 2px 2px 0 rgba(0, 0, 0, 0.18);
  }
  .search:focus-within {
    color: var(--ib-text);
    background: var(--ib-card);
    box-shadow: var(--ib-ex2);
  }
  input {
    flex: 1;
    min-width: 0;
    border: 0;
    background: none;
    outline: none;
    color: var(--ib-text);
    font: inherit;
    font-weight: 650;
  }
  input::placeholder {
    color: var(--ib-faint);
  }
  kbd {
    font-family: var(--ib-mono);
    font-size: 10.5px;
    font-weight: 700;
    padding: 1px 6px;
    border: 1.5px solid var(--ib-faint);
    border-radius: 5px;
  }
  .clear {
    display: grid;
    place-items: center;
    border: 0;
    background: none;
    cursor: pointer;
    color: var(--ib-muted);
  }
  .spacer {
    flex: 1;
    align-self: stretch;
  }
  .win {
    display: flex;
    align-self: stretch;
  }
  .win button {
    width: 50px;
    display: grid;
    place-items: center;
    border: 0;
    border-left: 2px solid var(--ib-paper-2);
    background: none;
    cursor: pointer;
    color: var(--ib-muted);
  }
  .win button:hover {
    background: var(--ib-paper-2);
    color: var(--ib-text);
  }
  .win .close:hover {
    background: var(--bad);
    color: #1a0a0d;
  }
</style>
