<script lang="ts">
  // Ctrl+K: every action, theme, view and download behind one fuzzy box.
  import Icon from "./Icon.svelte";
  import { api, readClipboard } from "../lib/api";
  import { store, type Filter } from "../lib/store.svelte";
  import { presets, shuffle, type BarStyle, type Rice } from "../lib/rice";
  import { iconFor } from "../lib/kinds";

  interface Cmd {
    id: string;
    label: string;
    hint?: string;
    group: string;
    icon: string;
    swatch?: string;
    run: () => void;
  }

  let q = $state("");
  let active = $state(0);
  let input = $state<HTMLInputElement>();
  let list = $state<HTMLDivElement>();

  const close = () => (store.paletteOpen = false);
  const patch = (fn: (r: Rice) => void) => {
    const r = $state.snapshot(store.rice) as Rice;
    fn(r);
    store.setRice(r);
  };

  const commands = $derived.by<Cmd[]>(() => {
    const t = store.targets();
    const c: Cmd[] = [
      { id: "add", label: "Add download", hint: "Ctrl N", group: "Actions", icon: "plus", run: () => store.openAdd() },
      { id: "paste", label: "Add from clipboard", hint: "Ctrl V", group: "Actions", icon: "link", run: async () => {
        const text = await readClipboard();
        store.openAdd(text ? { urls: [text], request: { url: "" }, source: "clipboard" } : null);
      } },
      { id: "pauseall", label: "Pause everything", group: "Actions", icon: "pause", run: () => store.run(api.pauseAll()) },
      { id: "resumeall", label: "Resume everything", group: "Actions", icon: "play", run: () => store.run(api.resumeAll()) },
      { id: "clear", label: "Clear finished", group: "Actions", icon: "check", run: () => store.run(api.clearCompleted()) },
      { id: "folder", label: "Open download folder", group: "Actions", icon: "folder", run: () => store.run(api.openDownloads()) },
      { id: "rice", label: "Open Rice Studio", hint: "Ctrl Shift R", group: "Look", icon: "sliders", run: () => (store.view = "rice") },
      { id: "settings", label: "Open settings", hint: "Ctrl ,", group: "Actions", icon: "gear", run: () => (store.view = "settings") },
      { id: "mini", label: "Mini bar mode", hint: "Ctrl M", group: "Look", icon: "min", run: () => store.setMini(true) },
      { id: "shuffle", label: "Shuffle a new look", group: "Look", icon: "bolt", run: () => store.setRice(shuffle($state.snapshot(store.rice) as Rice)) },
    ];
    if (t.length) {
      c.push(
        { id: "pausesel", label: `Pause selected (${t.length})`, group: "Selection", icon: "pause", run: () => store.run(api.pause(t)) },
        { id: "resumesel", label: `Resume selected (${t.length})`, group: "Selection", icon: "play", run: () => store.run(api.resume(t)) },
      );
    }
    for (const p of [...presets, ...store.library]) {
      c.push({ id: "theme-" + p.name, label: `Theme: ${p.name}`, group: "Themes", icon: "sun", swatch: p.colors.accent, run: () => store.setRice(structuredClone(p)) });
    }
    for (const [v, l] of [["list", "List"], ["table", "Table"], ["grid", "Grid"]] as const) {
      c.push({ id: "view-" + v, label: `View: ${l}`, group: "Layout", icon: "list", run: () => patch((r) => (r.layout.view = v)) });
    }
    for (const b of ["lanes", "solid", "line", "blocks", "ascii", "braille", "dots"] as BarStyle[]) {
      c.push({ id: "bar-" + b, label: `Progress bars: ${b}`, group: "Layout", icon: "sliders", run: () => patch((r) => (r.bar.style = b)) });
    }
    for (const [s, l] of [["left", "left"], ["right", "right"], ["hidden", "hidden"]] as const) {
      c.push({ id: "side-" + s, label: `Sidebar: ${l}`, group: "Layout", icon: "app", run: () => patch((r) => (r.layout.sidebar = s)) });
    }
    for (const [d, l] of [["right", "right"], ["bottom", "bottom"], ["off", "off"]] as const) {
      c.push({ id: "det-" + d, label: `Details panel: ${l}`, group: "Layout", icon: "doc", run: () => patch((r) => (r.layout.detail = d)) });
    }
    const filters: [Filter, string][] = [["all", "All"], ["active", "Active"], ["queued", "Queued"], ["paused", "Paused"], ["completed", "Finished"], ["failed", "Failed"]];
    for (const [f, l] of filters) {
      c.push({ id: "f-" + f, label: `Show: ${l}`, group: "Filters", icon: "search", run: () => ((store.filter = f), (store.view = "list")) });
    }
    for (const task of store.all.slice(-200)) {
      const look = iconFor(task);
      c.push({
        id: "t-" + task.id,
        label: task.name,
        hint: task.status,
        group: "Downloads",
        icon: look.icon,
        run: () => {
          store.view = "list";
          store.filter = "all";
          store.select(task.id);
        },
      });
    }
    return c;
  });

  // Subsequence fuzzy match; consecutive and word-start hits score higher.
  function score(text: string, query: string): number {
    if (!query) return 1;
    const t = text.toLowerCase();
    const qq = query.toLowerCase();
    let ti = 0, s = 0, run = 0;
    for (const ch of qq) {
      const at = t.indexOf(ch, ti);
      if (at < 0) return 0;
      run = at === ti ? run + 1 : 0;
      s += 1 + run * 2 + (at === 0 || " :-_./".includes(t[at - 1]) ? 3 : 0);
      ti = at + 1;
    }
    return s - t.length * 0.01;
  }

  const results = $derived(
    commands
      .map((c) => ({ c, s: score(`${c.label} ${c.group}`, q.trim()) }))
      .filter((x) => x.s > 0)
      .sort((a, b) => b.s - a.s)
      .slice(0, 60)
      .map((x) => x.c),
  );

  $effect(() => {
    q;
    active = 0;
  });
  $effect(() => {
    input?.focus();
  });
  $effect(() => {
    list?.querySelector(`[data-i="${active}"]`)?.scrollIntoView({ block: "nearest" });
  });

  function keys(e: KeyboardEvent) {
    if (e.key === "Escape") return close();
    if (e.key === "ArrowDown") {
      e.preventDefault();
      active = Math.min(results.length - 1, active + 1);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      active = Math.max(0, active - 1);
    } else if (e.key === "Enter") {
      e.preventDefault();
      run(results[active]);
    }
  }

  function run(c?: Cmd) {
    if (!c) return;
    close();
    c.run();
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<div class="scrim" role="presentation" onclick={(e) => e.target === e.currentTarget && close()}>
  <div class="pal" role="dialog" aria-modal="true" aria-label="Command palette">
    <label class="box">
      <Icon name="search" size={18} />
      <input bind:this={input} bind:value={q} onkeydown={keys} placeholder="Type a command, theme or download…" spellcheck="false" />
      <kbd>Esc</kbd>
    </label>
    <div class="list" bind:this={list} role="listbox">
      {#each results as c, i (c.id)}
        {@const prev = results[i - 1]}
        {#if !prev || prev.group !== c.group}<div class="grp">{c.group}</div>{/if}
        <button
          class="item"
          class:on={i === active}
          data-i={i}
          role="option"
          aria-selected={i === active}
          onmouseenter={() => (active = i)}
          onclick={() => run(c)}
        >
          {#if c.swatch}<i class="sw" style:background={c.swatch}></i>{:else}<Icon name={c.icon} size={15} />{/if}
          <span class="ell">{c.label}</span>
          {#if c.hint}<span class="hint mono">{c.hint}</span>{/if}
        </button>
      {:else}
        <p class="none faint">No matches.</p>
      {/each}
    </div>
  </div>
</div>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    z-index: 95;
    display: flex;
    justify-content: center;
    padding-top: 12vh;
    background: color-mix(in srgb, #000 45%, transparent);
  }
  .pal {
    width: min(640px, calc(100vw - 40px));
    max-height: 64vh;
    display: flex;
    flex-direction: column;
    border: var(--bw-lg) solid var(--ib-line);
    border-radius: var(--r-lg);
    background: var(--paper-solid, var(--ib-paper));
    box-shadow: var(--ib-ex10);
    overflow: hidden;
    animation: drop calc(0.16s * var(--motion, 1)) cubic-bezier(0.3, 1.4, 0.5, 1);
  }
  @keyframes drop {
    from {
      transform: translateY(-10px);
      opacity: 0;
    }
  }
  .box {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 0 16px;
    height: 58px;
    border-bottom: var(--bw) solid var(--ib-line);
    color: var(--ib-faint);
  }
  input {
    flex: 1;
    border: 0;
    background: none;
    outline: none;
    color: var(--ib-text);
    font: inherit;
    font-size: 17px;
    font-weight: 700;
  }
  input::placeholder {
    color: var(--ib-faint);
  }
  kbd {
    font-family: var(--ib-mono);
    font-size: 11px;
    padding: 2px 6px;
    border: 1.5px solid var(--ib-faint);
    border-radius: var(--r-xs);
  }
  .list {
    overflow-y: auto;
    padding: 6px;
  }
  .grp {
    padding: 10px 10px 4px;
    font-size: 10.5px;
    font-weight: 900;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--ib-faint);
  }
  .item {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 12px;
    height: 38px;
    padding: 0 10px;
    border: 0;
    border-radius: var(--r-sm);
    background: none;
    font-weight: 700;
    font-size: 14px;
    text-align: left;
    cursor: pointer;
  }
  .item.on {
    background: var(--ib-accent);
    color: var(--ib-accent-ink);
  }
  .item span:nth-of-type(1) {
    flex: 1;
  }
  .sw {
    width: 15px;
    height: 15px;
    border: 2px solid currentColor;
    border-radius: 4px;
    flex: none;
  }
  .hint {
    font-size: 11px;
    opacity: 0.6;
  }
  .none {
    padding: 24px;
    text-align: center;
  }
</style>
