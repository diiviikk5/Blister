<script lang="ts" module>
  export interface MenuItem {
    label?: string;
    icon?: string;
    hint?: string;
    danger?: boolean;
    disabled?: boolean;
    run?: () => void;
    sep?: boolean;
  }
</script>

<script lang="ts">
  import Icon from "./Icon.svelte";

  let { x, y, items, onclose }: { x: number; y: number; items: MenuItem[]; onclose: () => void } = $props();
  let el = $state<HTMLDivElement>();

  // Keep the menu on screen.
  const pos = $derived.by(() => {
    const w = 236;
    const h = items.length * 34 + 12;
    return {
      left: Math.min(x, window.innerWidth - w - 12),
      top: Math.min(y, window.innerHeight - h - 12),
    };
  });

  $effect(() => {
    el?.querySelector<HTMLButtonElement>("button:not(:disabled)")?.focus();
    const close = (e: Event) => {
      if (e instanceof KeyboardEvent && e.key !== "Escape") return;
      if (e instanceof MouseEvent && el?.contains(e.target as Node)) return;
      onclose();
    };
    const t = setTimeout(() => {
      window.addEventListener("mousedown", close);
      window.addEventListener("keydown", close);
      window.addEventListener("blur", close);
    });
    return () => {
      clearTimeout(t);
      window.removeEventListener("mousedown", close);
      window.removeEventListener("keydown", close);
      window.removeEventListener("blur", close);
    };
  });

  function nav(e: KeyboardEvent) {
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
    e.preventDefault();
    const btns = [...el!.querySelectorAll<HTMLButtonElement>("button:not(:disabled)")];
    const i = btns.indexOf(document.activeElement as HTMLButtonElement);
    btns[(i + (e.key === "ArrowDown" ? 1 : btns.length - 1)) % btns.length]?.focus();
  }
</script>

<div class="menu" bind:this={el} style:left="{pos.left}px" style:top="{pos.top}px" role="menu" tabindex="-1" onkeydown={nav}>
  {#each items as it, i (i)}
    {#if it.sep}
      <hr />
    {:else}
      <button
        role="menuitem"
        class:danger={it.danger}
        disabled={it.disabled}
        onclick={() => {
          onclose();
          it.run?.();
        }}
      >
        {#if it.icon}<Icon name={it.icon} size={15} />{:else}<span class="pad"></span>{/if}
        <span class="l">{it.label}</span>
        {#if it.hint}<span class="hint mono">{it.hint}</span>{/if}
      </button>
    {/if}
  {/each}
</div>

<style>
  .menu {
    position: fixed;
    z-index: 100;
    width: 236px;
    padding: 6px;
    border: var(--bw) solid var(--ib-line);
    border-radius: var(--r);
    background: var(--paper-solid, var(--ib-card));
    box-shadow: var(--ib-ex6);
    animation: pop 0.1s var(--ib-press);
  }
  @keyframes pop {
    from {
      transform: translate(-3px, -3px);
      opacity: 0;
    }
  }
  button {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    height: 32px;
    padding: 0 9px;
    border: 0;
    border-radius: var(--r-sm);
    background: none;
    font-weight: 700;
    font-size: 13.5px;
    text-align: left;
    cursor: pointer;
  }
  button:hover:not(:disabled),
  button:focus-visible {
    outline: none;
    background: var(--ib-accent);
    color: var(--ib-accent-ink);
  }
  button:disabled {
    opacity: 0.4;
    cursor: default;
  }
  .danger {
    color: var(--bad);
  }
  .danger:hover:not(:disabled) {
    background: var(--bad);
    color: #1a0a0d;
  }
  .l {
    flex: 1;
  }
  .pad {
    width: 15px;
  }
  .hint {
    font-size: 10.5px;
    opacity: 0.6;
  }
  hr {
    border: 0;
    border-top: 2px solid var(--ib-paper-2);
    margin: 5px 4px;
  }
</style>
