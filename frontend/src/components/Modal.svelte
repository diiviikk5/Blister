<script lang="ts">
  // Small confirm/prompt dialog used for remove and rename.
  import type { Snippet } from "svelte";

  let {
    title,
    onclose,
    children,
    actions,
  }: { title: string; onclose: () => void; children: Snippet; actions: Snippet } = $props();
</script>

<svelte:window onkeydown={(e) => e.key === "Escape" && onclose()} />

<!-- svelte-ignore a11y_click_events_have_key_events -->
<div class="scrim" role="presentation" onclick={(e) => e.target === e.currentTarget && onclose()}>
  <div class="box" role="dialog" aria-modal="true" aria-label={title}>
    <h3>{title}</h3>
    <div class="content">{@render children()}</div>
    <div class="actions">{@render actions()}</div>
  </div>
</div>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    z-index: 85;
    display: grid;
    place-items: center;
    background: color-mix(in srgb, var(--ib-night) 55%, transparent);
  }
  .box {
    width: min(460px, calc(100vw - 40px));
    padding: 20px 22px;
    border: 3px solid var(--ib-line);
    border-radius: 16px;
    background: var(--ib-paper);
    box-shadow: var(--ib-ex10);
    animation: drop 0.18s cubic-bezier(0.3, 1.4, 0.5, 1);
  }
  @keyframes drop {
    from {
      transform: translate(-6px, -10px);
      opacity: 0;
    }
  }
  h3 {
    margin: 0 0 10px;
    font-size: 22px;
    font-weight: 900;
    letter-spacing: -0.04em;
  }
  .content {
    font-size: 14px;
    line-height: 1.5;
    color: var(--ib-muted);
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
    margin-top: 20px;
  }
</style>
