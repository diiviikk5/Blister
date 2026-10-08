<script lang="ts">
  import Icon from "./Icon.svelte";
  import { api, readClipboard, errText } from "../lib/api";
  import { store } from "../lib/store.svelte";
  import { bytes } from "../lib/format";
  import { kindLabel } from "../lib/kinds";
  import type { AddRequest, Link, ProbeResult } from "../lib/types";

  const seed = store.addSeed;
  let text = $state(seed?.urls.join("\n") ?? "");
  let links = $state<Link[]>([]);
  let probes = $state<Record<string, ProbeResult | "loading">>({});
  let names = $state<Record<string, string>>({});
  let dropped = $state<Set<string>>(new Set());
  let parseErr = $state("");

  let dir = $state("");
  let connections = $state(store.settings?.connections ?? 16);
  let paused = $state(false);
  let quality = $state("best");
  let checksum = $state("");
  let more = $state(false);
  let busy = $state(false);
  let area = $state<HTMLTextAreaElement>();

  const chosen = $derived(links.filter((l) => !dropped.has(l.url)));
  const hasMedia = $derived(chosen.some((l) => l.kind === "media" || l.kind === "hls"));
  const single = $derived(chosen.length === 1 ? chosen[0] : null);
  const total = $derived(
    chosen.reduce((a, l) => {
      const p = probes[l.url];
      return a + (p && p !== "loading" && p.size > 0 ? p.size : 0);
    }, 0),
  );

  // Prefill from the clipboard when opened empty.
  $effect(() => {
    if (text) return;
    readClipboard().then((c) => {
      if (!text && /(https?:\/\/|magnet:\?)/i.test(c)) text = c.trim();
    });
    area?.focus();
  });

  // Parse as the user types; probe new HTTP links for name and size.
  let timer: ReturnType<typeof setTimeout>;
  $effect(() => {
    const t = text;
    clearTimeout(timer);
    timer = setTimeout(async () => {
      try {
        links = (await api.parse(t)) ?? [];
        // The extension marks video pages explicitly; trust it over URL rules.
        if (seed?.media) links = links.map((l) => (l.kind === "http" ? { ...l, kind: "media" } : l));
        parseErr = "";
      } catch (e) {
        parseErr = errText(e);
        links = [];
      }
      if (seed?.fileName && links.length === 1 && !names[links[0].url]) names[links[0].url] = seed.fileName;
      probeAll();
    }, 180);
  });

  let inflight = 0;
  function probeAll() {
    for (const l of links) {
      if (l.kind !== "http" || probes[l.url]) continue;
      if (inflight >= 12) {
        setTimeout(probeAll, 250);
        return;
      }
      probes[l.url] = "loading";
      inflight++;
      api
        .probe(l.url, seed?.request ?? {})
        .then((p) => (probes[l.url] = p))
        .catch((e) => (probes[l.url] = { name: "", size: -1, resumable: false, category: "other", type: "", error: errText(e) }))
        .finally(() => inflight--);
    }
  }

  function nameOf(l: Link): string {
    const p = probes[l.url];
    return names[l.url] ?? (p && p !== "loading" && p.name ? p.name : l.name);
  }

  async function browse() {
    const d = await api.pickFolder(dir || store.settings?.downloadDir || "");
    if (d) dir = d;
  }

  async function torrents() {
    const files = await api.pickTorrents();
    if (files?.length) text = [text.trim(), ...files].filter(Boolean).join("\n");
  }

  async function submit() {
    if (!chosen.length || busy) return;
    busy = true;
    const req = seed?.request;
    const reqs: AddRequest[] = chosen.map((l) => {
      const p = probes[l.url];
      const r: AddRequest = { url: l.url, kind: l.kind, paused };
      if (dir) r.dir = dir;
      if (names[l.url]) r.name = names[l.url];
      if (l.kind === "http") {
        r.connections = connections;
        if (p && p !== "loading" && p.size > 0) r.size = p.size;
      }
      if (l.kind === "media" || l.kind === "hls") {
        r.audioOnly = quality === "audio";
        r.format = quality === "audio" ? "" : quality;
      }
      if (req) r.request = { ...req, url: l.url };
      if (single && checksum.trim()) r.checksum = checksum.trim();
      return r;
    });
    try {
      const added = (await api.add(reqs)) ?? [];
      store.toast(added.length === 1 ? `Added ${added[0].name}` : `Added ${added.length} downloads`, "ok");
      store.filter = "all";
      store.view = "list";
      if (added.length) store.select(added[added.length - 1].id);
      close();
    } catch (e) {
      store.toast(errText(e), "error");
    } finally {
      busy = false;
    }
  }

  function close() {
    store.addOpen = false;
    store.addSeed = null;
  }

  function keys(e: KeyboardEvent) {
    if (e.key === "Escape") close();
    if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) submit();
  }
</script>

<svelte:window onkeydown={keys} />

<!-- svelte-ignore a11y_click_events_have_key_events -->
<div class="scrim" role="presentation" onclick={(e) => e.target === e.currentTarget && close()}>
  <div class="dialog" role="dialog" aria-modal="true" aria-labelledby="add-title">
    <header>
      <h2 id="add-title">Add <span class="ib-block">downloads</span></h2>
      {#if seed?.source === "browser"}<span class="pill pill--hot">From your browser</span>{/if}
      <button class="btn btn--ghost btn--icon x" onclick={close} aria-label="Close"><Icon name="x" size={18} /></button>
    </header>

    <div class="body">
      <textarea
        bind:this={area}
        class="field links mono"
        rows="4"
        bind:value={text}
        spellcheck="false"
        placeholder={"Paste links: one per line, or a whole page of text.\nhttps://example.com/file.zip\nmagnet:?xt=urn:btih:…\nhttps://youtu.be/…\nhttps://site.com/part[01-12].rar"}
      ></textarea>
      {#if parseErr}<div class="perr">{parseErr}</div>{/if}

      {#if links.length}
        <div class="found">
          <div class="label">
            {chosen.length} of {links.length} link{links.length === 1 ? "" : "s"}
            {#if total > 0}<span class="faint">· {bytes(total)}</span>{/if}
          </div>
          <div class="items">
            {#each links as l (l.url)}
              {@const p = probes[l.url]}
              {@const off = dropped.has(l.url)}
              <div class="item" class:off>
                <span class="pill" class:pill--alt={l.kind === "torrent"} class:pill--accent={l.kind === "media" || l.kind === "hls"}>{kindLabel[l.kind]}</span>
                {#if single && !off}
                  <input class="name" value={nameOf(l)} oninput={(e) => (names[l.url] = (e.currentTarget as HTMLInputElement).value)} aria-label="File name" />
                {:else}
                  <span class="name ell" title={l.url}>{nameOf(l)}</span>
                {/if}
                <span class="size num">
                  {#if p === "loading"}
                    <span class="spin"></span>
                  {:else if p?.error}
                    <span class="bad" title={p.error}>can't reach</span>
                  {:else if p && p.size > 0}
                    {bytes(p.size)}
                  {/if}
                </span>
                <button
                  class="btn btn--sm btn--icon btn--ghost"
                  onclick={() => {
                    const d = new Set(dropped);
                    if (off) d.delete(l.url);
                    else d.add(l.url);
                    dropped = d;
                  }}
                  aria-label={off ? "Include" : "Skip"}
                  title={off ? "Include" : "Skip"}
                >
                  <Icon name={off ? "plus" : "x"} size={14} />
                </button>
              </div>
            {/each}
          </div>
        </div>
      {/if}

      <div class="opts">
        <div class="opt grow">
          <span class="label">Save to</span>
          <div class="folder">
            <input class="field" bind:value={dir} placeholder="Automatic: Downloads\Blister, sorted by type" />
            <button class="btn btn--icon" onclick={browse} aria-label="Choose folder"><Icon name="folder" size={16} /></button>
          </div>
        </div>
        {#if hasMedia}
          <div class="opt">
            <span class="label">Video quality</span>
            <select class="field" bind:value={quality}>
              <option value="best">Best available</option>
              <option value="2160">4K</option>
              <option value="1440">1440p</option>
              <option value="1080">1080p</option>
              <option value="720">720p</option>
              <option value="480">480p</option>
              <option value="audio">Audio only (MP3)</option>
            </select>
          </div>
        {/if}
        <div class="opt">
          <span class="label">Connections</span>
          <select class="field" bind:value={connections}>
            {#each [1, 2, 4, 8, 16, 24, 32, 48, 64] as n (n)}<option value={n}>{n}</option>{/each}
          </select>
        </div>
      </div>

      {#if more}
        <div class="opts">
          <div class="opt grow">
            <span class="label">Checksum (optional)</span>
            <input class="field mono" bind:value={checksum} disabled={!single} placeholder={single ? "sha256:… or md5:… — verified when done" : "Only for a single file"} />
          </div>
        </div>
      {/if}
    </div>

    <footer>
      <label class="ib-check start"><input type="checkbox" bind:checked={paused} /> Add paused</label>
      <button class="btn btn--ghost" onclick={() => (more = !more)}>{more ? "Fewer options" : "More options"}</button>
      <button class="btn btn--ghost" onclick={torrents}><Icon name="magnet" size={15} /> Open .torrent</button>
      <div class="spacer"></div>
      <button class="btn" onclick={close}>Cancel</button>
      <button class="btn btn--accent btn--lg go" disabled={!chosen.length || busy} onclick={submit}>
        <Icon name="down" size={18} stroke={3} />
        {chosen.length > 1 ? `Download ${chosen.length}` : "Download"}
        <kbd>Ctrl ↵</kbd>
      </button>
    </footer>
  </div>
</div>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    z-index: 80;
    display: grid;
    place-items: center;
    padding: 24px;
    background: color-mix(in srgb, var(--ib-night) 62%, transparent);
    animation: fade 0.12s ease;
  }
  @keyframes fade {
    from {
      opacity: 0;
    }
  }
  .dialog {
    width: min(760px, 100%);
    max-height: calc(100vh - 48px);
    display: flex;
    flex-direction: column;
    border: 3px solid var(--ib-line);
    border-radius: 18px;
    background: var(--ib-paper);
    box-shadow: var(--ib-ex14);
    animation: drop 0.22s cubic-bezier(0.3, 1.4, 0.5, 1);
  }
  @keyframes drop {
    from {
      transform: translate(-8px, -14px);
      opacity: 0;
    }
  }
  header {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 20px 22px 6px;
  }
  h2 {
    margin: 0;
    font-size: 34px;
    font-weight: 900;
    letter-spacing: -0.06em;
    line-height: 1;
  }
  h2 :global(.ib-block) {
    box-shadow: var(--ib-ex6);
    font-size: 0.95em;
  }
  .x {
    margin-left: auto;
  }
  .body {
    padding: 16px 22px 8px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .links {
    font-size: 12.5px;
    min-height: 96px;
  }
  .perr,
  .bad {
    color: var(--bad);
    font-weight: 700;
    font-size: 12.5px;
  }
  .items {
    max-height: 230px;
    overflow-y: auto;
    border: 2.5px solid var(--ib-line);
    border-radius: 12px;
    background: var(--ib-card);
  }
  .item {
    display: grid;
    grid-template-columns: 70px minmax(0, 1fr) auto 28px;
    align-items: center;
    gap: 10px;
    padding: 7px 8px 7px 10px;
    border-bottom: 2px solid var(--ib-paper-2);
  }
  .item:last-child {
    border-bottom: 0;
  }
  .item .pill {
    justify-content: center;
  }
  .item.off {
    opacity: 0.45;
  }
  .item.off .name {
    text-decoration: line-through;
  }
  .name {
    font-weight: 750;
    font-size: 13.5px;
  }
  input.name {
    border: 0;
    border-bottom: 2px dashed var(--ib-faint);
    background: none;
    color: var(--ib-text);
    font: inherit;
    font-weight: 750;
    padding: 2px 0;
    outline: none;
    min-width: 0;
  }
  input.name:focus {
    border-bottom-color: var(--ib-accent);
  }
  .size {
    font-size: 12px;
    font-weight: 750;
    color: var(--ib-muted);
  }
  .spin {
    display: inline-block;
    width: 12px;
    height: 12px;
    border: 2.5px solid var(--ib-faint);
    border-top-color: var(--ib-accent);
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .opts {
    display: flex;
    gap: 14px;
  }
  .opt {
    display: flex;
    flex-direction: column;
    min-width: 150px;
  }
  .grow {
    flex: 1;
  }
  .folder {
    display: flex;
    gap: 8px;
  }
  .folder .btn {
    height: 38px;
    width: 38px;
    flex: none;
  }
  footer {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 16px 22px 20px;
    border-top: 3px solid var(--ib-line);
    margin-top: 10px;
  }
  .start {
    font-size: 13px;
    margin-right: 6px;
  }
  .start input {
    width: 20px;
    height: 20px;
    border-width: 2.5px;
    border-radius: 6px;
  }
  .spacer {
    flex: 1;
  }
  .go kbd {
    font-family: var(--ib-mono);
    font-size: 10.5px;
    font-weight: 700;
    opacity: 0.65;
    margin-left: 2px;
  }
</style>
