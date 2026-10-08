<script lang="ts">
  import Icon from "./Icon.svelte";
  import { store } from "../lib/store.svelte";
  import { host } from "../lib/format";
</script>

<div class="stack" aria-live="polite">
  {#if store.clipboard}
    {@const links = store.clipboard}
    <div class="toast clip">
      <div class="ic"><Icon name="link" size={16} stroke={2.8} /></div>
      <div class="txt">
        <strong>Copied {links.length === 1 ? "a link" : `${links.length} links`}</strong>
        <span class="ell faint">{links.length === 1 ? host(links[0]) : `${host(links[0])} and more`}</span>
      </div>
      <button class="btn btn--sm btn--accent" onclick={() => store.openAdd({ urls: links, request: { url: "" }, source: "clipboard" })}>Download</button>
      <button class="btn btn--sm btn--icon btn--ghost" onclick={() => (store.clipboard = null)} aria-label="Dismiss"><Icon name="x" size={13} /></button>
    </div>
  {/if}
  {#each store.toasts as t (t.id)}
    <div class="toast" data-tone={t.tone}>
      <div class="ic"><Icon name={t.tone === "error" ? "alert" : t.tone === "ok" ? "check" : "dots"} size={15} stroke={2.8} /></div>
      <div class="txt"><span>{t.text}</span></div>
      {#if t.action}
        {@const a = t.action}
        <button class="btn btn--sm" onclick={() => (a.run(), store.dismiss(t.id))}>{a.label}</button>
      {/if}
      <button class="btn btn--sm btn--icon btn--ghost" onclick={() => store.dismiss(t.id)} aria-label="Dismiss"><Icon name="x" size={13} /></button>
    </div>
  {/each}
</div>

<style>
  .stack {
    position: fixed;
    right: 18px;
    bottom: 50px;
    z-index: 90;
    display: flex;
    flex-direction: column;
    gap: 10px;
    width: 380px;
    pointer-events: none;
  }
  .toast {
    pointer-events: auto;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 10px 10px 12px;
    border: 2.5px solid var(--ib-line);
    border-radius: 12px;
    background: var(--ib-card);
    box-shadow: var(--ib-ex6);
    font-size: 13px;
    font-weight: 700;
    animation: in 0.2s cubic-bezier(0.3, 1.4, 0.5, 1);
  }
  @keyframes in {
    from {
      transform: translateX(30px);
      opacity: 0;
    }
  }
  .ic {
    width: 28px;
    height: 28px;
    flex: none;
    display: grid;
    place-items: center;
    border: 2px solid var(--ib-line);
    border-radius: 8px;
    background: var(--ib-paper-2);
  }
  [data-tone="ok"] .ic {
    background: var(--ok);
    color: #06210f;
  }
  [data-tone="error"] .ic {
    background: var(--bad);
    color: #1a0a0d;
  }
  .clip .ic {
    background: var(--ib-hot);
    color: var(--ib-hot-ink);
  }
  .txt {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    line-height: 1.35;
  }
  .txt span:not(.ell) {
    overflow-wrap: anywhere;
  }
  .txt .ell {
    font-size: 12px;
  }
</style>
