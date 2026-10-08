<script lang="ts">
  import Icon from "./Icon.svelte";
  import TaskRow from "./TaskRow.svelte";
  import SegBar from "./SegBar.svelte";
  import { api, copyText, readClipboard, errText, native } from "../lib/api";
  import { store } from "../lib/store.svelte";
  import {
    presets,
    shuffle,
    encodeRice,
    decodeRice,
    contrast,
    defaultRice,
    type Rice,
    type Colors,
    type ShadowStyle,
    type BarStyle,
  } from "../lib/rice";
  import type { Task } from "../lib/types";

  type Tab = "themes" | "colors" | "shape" | "type" | "layout" | "bars" | "fx" | "css";
  const tabs: [Tab, string][] = [
    ["themes", "Themes"],
    ["colors", "Colours"],
    ["shape", "Shape"],
    ["type", "Type"],
    ["layout", "Layout"],
    ["bars", "Bars"],
    ["fx", "Effects"],
    ["css", "CSS"],
  ];
  let tab = $state<Tab>("themes");

  const r = $derived(store.rice);

  /** Copy the live rice, change it, apply it. */
  function edit(fn: (x: Rice) => void) {
    const x = $state.snapshot(store.rice) as Rice;
    fn(x);
    store.setRice(x);
  }
  const num = (e: Event) => Number((e.currentTarget as HTMLInputElement).value);
  const str = (e: Event) => (e.currentTarget as HTMLInputElement).value;

  // ------------------------------------------------------------ preview rows
  const fake = (p: Partial<Task>): Task => ({
    id: "fake-" + p.name, kind: "http", url: "https://cdn.example.com/x", name: "x", dir: "", category: "other",
    status: "downloading", size: 1, done: 0, speed: 0, upSpeed: 0, uploaded: 0, eta: -1, conns: 0, seeds: 0,
    resumable: true, connections: 16, speedLimit: 0, priority: 0, request: { url: "" }, createdAt: new Date().toISOString(),
    ...p,
  } as Task);
  const sample = $derived.by(() => {
    const real = store.visible.slice(0, 3);
    if (real.length >= 3) return real;
    return [
      fake({ name: "arch-2026.10.01-x86_64.iso", category: "archive", size: 1.2e9, done: 7.4e8, speed: 4.1e7, eta: 11, conns: 16 }),
      fake({ name: "Lo-fi beats to rice to.mp4", kind: "media", category: "video", size: 3.1e8, done: 9e7, speed: 6e6, eta: 36, conns: 1 }),
      fake({ name: "dotfiles-final-final.tar.gz", category: "archive", status: "completed", size: 2.2e7, done: 2.2e7, completedAt: new Date().toISOString() }),
    ];
  });

  // ------------------------------------------------------------ colours
  const colorKeys: [keyof Colors, string][] = [
    ["paper", "Background"],
    ["paper2", "Sunken"],
    ["card", "Raised"],
    ["text", "Text"],
    ["muted", "Muted text"],
    ["faint", "Faint text"],
    ["line", "Outlines"],
    ["ex", "Extrusion"],
    ["accent", "Accent"],
    ["accentInk", "On accent"],
    ["hot", "Signal"],
    ["hotInk", "On signal"],
    ["alt", "Third"],
    ["ok", "Success"],
    ["bad", "Danger"],
  ];
  const checks = $derived([
    { label: "Text on background", v: contrast(r.colors.text, r.colors.paper) },
    { label: "Muted on background", v: contrast(r.colors.muted, r.colors.paper) },
    { label: "Ink on accent", v: contrast(r.colors.accentInk, r.colors.accent) },
    { label: "Ink on signal", v: contrast(r.colors.hotInk, r.colors.hot) },
  ]);

  // ------------------------------------------------------------ fonts
  const uiFonts = ["Geist Variable", "Segoe UI Variable", "Inter", "Bahnschrift", "Trebuchet MS", "Georgia", "system-ui"];
  const monoFonts = ["Geist Mono Variable", "Cascadia Code", "Consolas", "JetBrainsMono Nerd Font", "FiraCode Nerd Font", "Iosevka", "monospace"];

  // ------------------------------------------------------------ share + library
  let shareMsg = $state("");
  async function copyCode(withWall: boolean) {
    await copyText(encodeRice($state.snapshot(store.rice) as Rice, withWall));
    store.toast("Theme code copied. Paste it anywhere to share.", "ok");
  }
  async function pasteCode() {
    const text = (await readClipboard()).trim();
    try {
      const x = decodeRice(text);
      store.setRice(x);
      store.toast(`Loaded “${x.name}”`, "ok");
    } catch {
      store.toast("The clipboard doesn't hold a Blister theme code", "error");
    }
  }
  function saveToLibrary() {
    const x = $state.snapshot(store.rice) as Rice;
    if (!x.name || x.name === "Shuffled") x.name = `My rice ${store.library.length + 1}`;
    store.library = [...store.library.filter((l) => l.name !== x.name), x];
    store.setRice(x);
    store.toast(`Saved “${x.name}” to your library`, "ok");
  }
  function dropFromLibrary(name: string) {
    store.library = store.library.filter((l) => l.name !== name);
    store.persistRice();
  }

  // ------------------------------------------------------------ wallpaper + window
  async function pickWall() {
    try {
      const url = await api.pickImage();
      if (url) edit((x) => ((x.fx.wallpaper = url), (x.fx.opacity = Math.min(x.fx.opacity, 0.82))));
    } catch (e) {
      store.toast(errText(e), "error");
    }
  }
  let effect = $state(store.settings?.windowEffect ?? "none");
  const effectChanged = $derived(store.settings && effect !== store.settings.windowEffect);
  async function applyEffect() {
    if (!store.settings) return;
    await store.saveSettings({ ...store.settings, windowEffect: effect as "none" });
    if (effect !== "none") edit((x) => (x.fx.opacity = Math.min(x.fx.opacity, 0.72)));
    store.persistRice();
    setTimeout(() => store.run(api.relaunch()), 500);
  }

  // ------------------------------------------------------------ css snippets
  const snippets: [string, string][] = [
    ["Uppercase titles", ".row .name, h1 { text-transform: uppercase; letter-spacing: .04em; }"],
    ["Rainbow bars", ".seg .map, .seg .fill { filter: hue-rotate(calc(var(--i, 0) * 40deg)) saturate(1.4); }\n.row:nth-child(3n+1) .seg { --i: 1 } .row:nth-child(3n+2) .seg { --i: 3 } .row:nth-child(3n) .seg { --i: 5 }"],
    ["Hide speed graph", ".graph { display: none; }"],
    ["Italic serif names", ".row .name { font-family: Georgia, serif; font-style: italic; font-weight: 600; }"],
    ["Tilted active filter", ".side .link.on { transform: rotate(-2deg); }"],
    ["Big numbers", ".row .big { font-size: 22px; }"],
  ];
  function addSnippet(css: string) {
    edit((x) => (x.css = (x.css ? x.css.trimEnd() + "\n\n" : "") + css));
  }

  const shadowStyles: [ShadowStyle, string][] = [["extrude", "Extrude"], ["hard", "Hard"], ["soft", "Soft"], ["glow", "Glow"], ["none", "Flat"]];
  const barStyles: [BarStyle, string][] = [["lanes", "Lanes"], ["solid", "Solid"], ["line", "Line"], ["blocks", "Blocks"], ["ascii", "ASCII"], ["braille", "Braille"], ["dots", "Dots"]];
  const barDemo = { map: "999999999999877766655544443333222111000000000999999000111222333444555666777888999999999999999999999999998888777766665555444", heads: [0.31, 0.52, 0.66, 0.9] };
</script>

<div class="studio">
  <div class="top">
    <div class="title">
      <h1>Rice Studio</h1>
      <input class="name" value={r.name} onchange={(e) => edit((x) => (x.name = str(e)))} aria-label="Theme name" spellcheck="false" />
    </div>
    <div class="acts">
      <button class="btn" onclick={() => store.setRice(shuffle($state.snapshot(store.rice) as Rice))} title="Random palette and shape"><Icon name="bolt" size={15} /> Shuffle</button>
      <button class="btn" onclick={saveToLibrary}><Icon name="heart" size={15} /> Save</button>
      <button class="btn" onclick={() => copyCode(false)}><Icon name="copy" size={15} /> Share code</button>
      <button class="btn" onclick={pasteCode}><Icon name="link" size={15} /> Paste code</button>
      <button class="btn btn--accent" onclick={() => (store.view = "list")}><Icon name="check" size={15} stroke={3} /> Done</button>
    </div>
  </div>

  <div class="ib-tabs tabs" role="tablist">
    {#each tabs as [id, label] (id)}
      <button class="ib-tab" role="tab" aria-selected={tab === id} onclick={() => (tab = id)}>{label}</button>
    {/each}
  </div>

  <div class="split">
    <div class="controls">
      {#if tab === "themes"}
        <div class="gallery">
          {#each [...presets, ...store.library] as p, i (p.name + i)}
            {@const c = p.colors}
            {@const mine = i >= presets.length}
            <div class="card-wrap">
              <button
                class="theme"
                class:on={p.name === r.name}
                style:--tp={c.paper}
                style:--tc={c.card}
                style:--tt={c.text}
                style:--tl={c.line}
                style:--ta={c.accent}
                style:--th={c.hot}
                style:--tx={c.alt}
                style:--tr="{p.shape.radius}px"
                style:--tb="{Math.max(1, p.shape.border)}px"
                onclick={() => store.setRice(structuredClone(p))}
              >
                <span class="mini">
                  <span class="mini-side"><i></i><i></i><i></i></span>
                  <span class="mini-main">
                    <b>{p.name}</b>
                    <span class="mini-bar"><i style:width="72%"></i></span>
                    <span class="mini-bar"><i style:width="38%"></i></span>
                    <span class="mini-bar"><i style:width="91%"></i></span>
                  </span>
                </span>
                <span class="dots"><i style:background={c.accent}></i><i style:background={c.hot}></i><i style:background={c.alt}></i><i style:background={c.ok}></i></span>
              </button>
              {#if mine}
                <button class="del" onclick={() => dropFromLibrary(p.name)} aria-label="Delete {p.name}"><Icon name="x" size={12} stroke={3} /></button>
              {/if}
            </div>
          {/each}
        </div>
        <p class="note">Click a theme to wear it, then tweak anything in the other tabs. <b>Save</b> keeps your version; <b>Share code</b> copies it as text anyone can paste back.</p>
      {:else if tab === "colors"}
        <div class="colors">
          {#each colorKeys as [k, label] (k)}
            <label class="color">
              <input type="color" value={r.colors[k]} oninput={(e) => edit((x) => (x.colors[k] = str(e)))} />
              <span class="cl">{label}</span>
              <input class="hex mono" value={r.colors[k]} onchange={(e) => /^#[0-9a-f]{6}$/i.test(str(e)) && edit((x) => (x.colors[k] = str(e)))} spellcheck="false" />
            </label>
          {/each}
        </div>
        <div class="checks">
          {#each checks as c (c.label)}
            <span class="check" class:bad={c.v < 4.5}>
              <b class="num">{c.v.toFixed(1)}</b>
              {c.label}
              <i>{c.v >= 7 ? "AAA" : c.v >= 4.5 ? "AA" : "low"}</i>
            </span>
          {/each}
        </div>
      {:else if tab === "shape"}
        <div class="rows">
          <div class="ctl">
            <span>Outline width</span>
            <input type="range" min="0" max="4" step="0.5" value={r.shape.border} oninput={(e) => edit((x) => (x.shape.border = num(e)))} />
            <b class="num">{r.shape.border}px</b>
          </div>
          <div class="ctl">
            <span>Corner radius</span>
            <input type="range" min="0" max="24" value={r.shape.radius} oninput={(e) => edit((x) => (x.shape.radius = num(e)))} />
            <b class="num">{r.shape.radius}px</b>
          </div>
          <div class="ctl">
            <span>Depth</span>
            <input type="range" min="0" max="14" value={r.shape.depth} oninput={(e) => edit((x) => (x.shape.depth = num(e)))} />
            <b class="num">{r.shape.depth}</b>
          </div>
          <div class="ctl">
            <span>Shadow</span>
            <div class="seg-pick">
              {#each shadowStyles as [v, l] (v)}
                <button class:on={r.shape.shadow === v} onclick={() => edit((x) => (x.shape.shadow = v))}>{l}</button>
              {/each}
            </div>
          </div>
        </div>
        <div class="shape-demo">
          <button class="btn btn--accent btn--lg">Primary</button>
          <button class="btn">Button</button>
          <span class="pill pill--hot">Signal</span>
          <input class="field" placeholder="Sunken field" />
        </div>
      {:else if tab === "type"}
        <div class="rows">
          <div class="ctl">
            <span>Interface font</span>
            <input class="field" list="ui-fonts" value={r.type.ui} onchange={(e) => edit((x) => (x.type.ui = str(e)))} />
            <datalist id="ui-fonts">{#each uiFonts as f (f)}<option value={f}></option>{/each}</datalist>
          </div>
          <div class="ctl">
            <span>Mono font</span>
            <input class="field" list="mono-fonts" value={r.type.mono} onchange={(e) => edit((x) => (x.type.mono = str(e)))} />
            <datalist id="mono-fonts">{#each monoFonts as f (f)}<option value={f}></option>{/each}</datalist>
          </div>
          <div class="ctl">
            <span>Monospace everything</span>
            <input type="checkbox" class="ib-switch" checked={r.type.monoUI} onchange={(e) => edit((x) => (x.type.monoUI = (e.currentTarget as HTMLInputElement).checked))} />
          </div>
          <div class="ctl">
            <span>Scale</span>
            <input type="range" min="0.8" max="1.3" step="0.05" value={r.type.scale} oninput={(e) => edit((x) => (x.type.scale = num(e)))} />
            <b class="num">{Math.round(r.type.scale * 100)}%</b>
          </div>
          <div class="ctl">
            <span>Heading weight</span>
            <input type="range" min="400" max="900" step="100" value={r.type.weight} oninput={(e) => edit((x) => (x.type.weight = num(e)))} />
            <b class="num">{r.type.weight}</b>
          </div>
          <div class="ctl">
            <span>Heading tracking</span>
            <input type="range" min="-0.08" max="0.06" step="0.01" value={r.type.tracking} oninput={(e) => edit((x) => (x.type.tracking = num(e)))} />
            <b class="num">{r.type.tracking.toFixed(2)}</b>
          </div>
        </div>
        <p class="note">Any font installed on your system works, Nerd Fonts included. Type its exact name.</p>
        <div class="type-demo">
          <h2>Every file, sixteen ways.</h2>
          <p class="mono">blister://queue 16 lanes 112.4 MB/s</p>
        </div>
      {:else if tab === "layout"}
        <div class="rows">
          <div class="ctl">
            <span>View</span>
            <div class="seg-pick">
              {#each [["list", "List"], ["table", "Table"], ["grid", "Grid"]] as [v, l] (v)}
                <button class:on={r.layout.view === v} onclick={() => edit((x) => (x.layout.view = v as Rice["layout"]["view"]))}>{l}</button>
              {/each}
            </div>
          </div>
          <div class="ctl">
            <span>Rows</span>
            <div class="seg-pick">
              {#each [["lines", "Lines"], ["cards", "Cards"]] as [v, l] (v)}
                <button class:on={r.layout.rows === v} onclick={() => edit((x) => (x.layout.rows = v as Rice["layout"]["rows"]))}>{l}</button>
              {/each}
            </div>
          </div>
          <div class="ctl">
            <span>Density</span>
            <div class="seg-pick">
              {#each [["compact", "Compact"], ["comfortable", "Comfy"], ["cozy", "Cozy"]] as [v, l] (v)}
                <button class:on={r.layout.density === v} onclick={() => edit((x) => (x.layout.density = v as Rice["layout"]["density"]))}>{l}</button>
              {/each}
            </div>
          </div>
          <div class="ctl">
            <span>Sidebar</span>
            <div class="seg-pick">
              {#each [["left", "Left"], ["right", "Right"], ["hidden", "Hidden"]] as [v, l] (v)}
                <button class:on={r.layout.sidebar === v} onclick={() => edit((x) => (x.layout.sidebar = v as Rice["layout"]["sidebar"]))}>{l}</button>
              {/each}
            </div>
          </div>
          <div class="ctl">
            <span>Details panel</span>
            <div class="seg-pick">
              {#each [["right", "Right"], ["bottom", "Bottom"], ["off", "Off"]] as [v, l] (v)}
                <button class:on={r.layout.detail === v} onclick={() => edit((x) => (x.layout.detail = v as Rice["layout"]["detail"]))}>{l}</button>
              {/each}
            </div>
          </div>
        </div>
        <p class="note">With the sidebar hidden, <kbd>Ctrl K</kbd> still reaches every filter and action.</p>
      {:else if tab === "bars"}
        <div class="bars">
          {#each barStyles as [v, l] (v)}
            <button class="bar-pick" class:on={r.bar.style === v} onclick={() => edit((x) => (x.bar.style = v))}>
              <span>{l}</span>
              <span class="bar-demo" data-force={v}>
                <SegBar progress={0.63} map={barDemo.map} heads={barDemo.heads} status="downloading" force={v} />
              </span>
            </button>
          {/each}
        </div>
        <div class="rows">
          <div class="ctl">
            <span>Bar height</span>
            <input type="range" min="4" max="28" value={r.bar.height} oninput={(e) => edit((x) => (x.bar.height = num(e)))} />
            <b class="num">{r.bar.height}px</b>
          </div>
          <div class="ctl">
            <span>Connection heads</span>
            <input type="checkbox" class="ib-switch" checked={r.bar.heads} onchange={(e) => edit((x) => (x.bar.heads = (e.currentTarget as HTMLInputElement).checked))} />
          </div>
          <div class="ctl">
            <span>Heat stripes while live</span>
            <input type="checkbox" class="ib-switch" checked={r.bar.stripes} onchange={(e) => edit((x) => (x.bar.stripes = (e.currentTarget as HTMLInputElement).checked))} />
          </div>
        </div>
      {:else if tab === "fx"}
        <div class="rows">
          <div class="ctl">
            <span>Motion</span>
            <div class="seg-pick">
              {#each [[0, "Off"], [1, "Normal"], [2, "Springy"]] as [v, l] (v)}
                <button class:on={r.fx.motion === v} onclick={() => edit((x) => (x.fx.motion = v as number))}>{l}</button>
              {/each}
            </div>
          </div>
          <div class="ctl">
            <span>Film grain</span>
            <input type="range" min="0" max="0.6" step="0.02" value={r.fx.grain} oninput={(e) => edit((x) => (x.fx.grain = num(e)))} />
            <b class="num">{Math.round(r.fx.grain * 100)}</b>
          </div>
          <div class="ctl">
            <span>CRT scanlines</span>
            <input type="range" min="0" max="1" step="0.05" value={r.fx.scanlines} oninput={(e) => edit((x) => (x.fx.scanlines = num(e)))} />
            <b class="num">{Math.round(r.fx.scanlines * 100)}</b>
          </div>
          <div class="ctl">
            <span>Vignette</span>
            <input type="range" min="0" max="1" step="0.05" value={r.fx.vignette} oninput={(e) => edit((x) => (x.fx.vignette = num(e)))} />
            <b class="num">{Math.round(r.fx.vignette * 100)}</b>
          </div>
          <div class="ctl">
            <span>Wallpaper</span>
            <div class="wallrow">
              <button class="btn btn--sm" onclick={pickWall}><Icon name="image" size={13} /> Choose image</button>
              <input class="field" placeholder="…or an https:// image URL" value={r.fx.wallpaper.startsWith("data:") ? "" : r.fx.wallpaper} onchange={(e) => edit((x) => ((x.fx.wallpaper = str(e).trim()), str(e).trim() && (x.fx.opacity = Math.min(x.fx.opacity, 0.82))))} />
              {#if r.fx.wallpaper}<button class="btn btn--sm btn--icon" onclick={() => edit((x) => ((x.fx.wallpaper = ""), (x.fx.opacity = 1)))} aria-label="Remove wallpaper"><Icon name="x" size={13} /></button>{/if}
            </div>
          </div>
          <div class="ctl">
            <span>Wallpaper dim</span>
            <input type="range" min="0" max="0.95" step="0.05" value={r.fx.dim} oninput={(e) => edit((x) => (x.fx.dim = num(e)))} />
            <b class="num">{Math.round(r.fx.dim * 100)}</b>
          </div>
          <div class="ctl">
            <span>Wallpaper blur</span>
            <input type="range" min="0" max="30" value={r.fx.blur} oninput={(e) => edit((x) => (x.fx.blur = num(e)))} />
            <b class="num">{r.fx.blur}px</b>
          </div>
          <div class="ctl">
            <span>Surface opacity</span>
            <input type="range" min="0.3" max="1" step="0.02" value={r.fx.opacity} oninput={(e) => edit((x) => (x.fx.opacity = num(e)))} />
            <b class="num">{Math.round(r.fx.opacity * 100)}%</b>
          </div>
          {#if native}
            <div class="ctl">
              <span>Window backdrop</span>
              <div class="wallrow">
                <div class="seg-pick">
                  {#each [["none", "Solid"], ["mica", "Mica"], ["acrylic", "Acrylic"], ["tabbed", "Tabbed"]] as [v, l] (v)}
                    <button class:on={effect === v} onclick={() => (effect = v as "none")}>{l}</button>
                  {/each}
                </div>
                {#if effectChanged}<button class="btn btn--sm btn--accent" onclick={applyEffect}>Relaunch to apply</button>{/if}
              </div>
            </div>
          {/if}
        </div>
        <p class="note">Mica and Acrylic let your desktop show through the window on Windows 11. Lower the surface opacity to taste.</p>
      {:else}
        <div class="css">
          <textarea class="field mono" rows="14" spellcheck="false" value={r.css} oninput={(e) => edit((x) => (x.css = (e.currentTarget as HTMLTextAreaElement).value))} placeholder={"/* Your CSS, applied live. Tokens: --ib-accent, --ib-paper, --bw, --r, --bar-h… */\n.row .name { letter-spacing: .02em; }"}></textarea>
          <div class="snips">
            {#each snippets as [l, css] (l)}
              <button class="btn btn--sm" onclick={() => addSnippet(css)}><Icon name="plus" size={12} /> {l}</button>
            {/each}
          </div>
          <p class="note">Everything in the app can be restyled. Right-click → Inspect isn't available in release builds, so class names to start from: <code>.row .name .seg .side .link .tools .detail .status .btn .pill</code>.</p>
        </div>
      {/if}
    </div>

    <aside class="preview" aria-label="Live preview">
      <div class="label">Live preview</div>
      <div class="pv list" class:grid-pv={r.layout.view === "grid"}>
        {#each sample as t (t.id)}
          <TaskRow task={t} oncontext={() => {}} />
        {/each}
      </div>
      <div class="pv-actions">
        <button class="btn btn--accent">Add download</button>
        <button class="btn">Pause</button>
        <span class="pill">Torrent</span>
        <span class="pill pill--hot">New</span>
      </div>
      <button class="reset" onclick={() => store.setRice(structuredClone(defaultRice))}>Reset to Thermal</button>
    </aside>
  </div>
</div>

<style>
  .studio {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    min-width: 0;
  }
  .top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 16px 18px 12px;
    flex-wrap: wrap;
  }
  .title {
    display: flex;
    align-items: baseline;
    gap: 14px;
    min-width: 0;
  }
  h1 {
    margin: 0;
    font-size: 26px;
    font-weight: var(--h-weight);
    letter-spacing: var(--h-track);
    white-space: nowrap;
  }
  .name {
    min-width: 0;
    width: 200px;
    border: 0;
    border-bottom: calc(var(--bw) * 0.8) dashed var(--ib-faint);
    background: none;
    color: var(--ib-muted);
    font: inherit;
    font-weight: 800;
    font-size: 15px;
    outline: none;
  }
  .name:focus {
    border-bottom-color: var(--ib-accent);
    color: var(--ib-text);
  }
  .acts {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }
  .tabs {
    margin: 0 18px;
    align-self: flex-start;
    flex-wrap: wrap;
  }
  .split {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: minmax(0, 1fr) 360px;
    gap: 0;
    margin-top: 14px;
    border-top: var(--bw-lg) solid var(--ib-line);
  }
  .controls {
    overflow-y: auto;
    padding: 18px;
  }
  .preview {
    border-left: var(--bw-lg) solid var(--ib-line);
    padding: 16px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 12px;
    background: var(--ib-paper-2);
  }
  .pv {
    border: var(--bw) solid var(--ib-line);
    border-radius: var(--r);
    background: var(--ib-paper);
    overflow: hidden;
  }
  .pv :global(.row) {
    grid-template-columns: 40px minmax(0, 1fr) 78px;
    gap: 10px;
    height: auto;
    min-height: var(--row-h);
    padding: 12px;
  }
  .pv :global(.row .nums .small:last-child) {
    display: none;
  }
  .pv :global(.row .acts) {
    display: none;
  }
  .pv-actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 10px;
  }
  .reset {
    align-self: flex-start;
    margin-top: auto;
    border: 0;
    background: none;
    color: var(--ib-faint);
    font-weight: 750;
    font-size: 12px;
    text-decoration: underline;
    cursor: pointer;
  }
  .note {
    margin: 16px 2px 0;
    font-size: 12.5px;
    color: var(--ib-muted);
    line-height: 1.55;
  }
  .note code,
  kbd {
    font-family: var(--ib-mono);
    font-size: 11.5px;
  }

  /* themes */
  .gallery {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 14px;
  }
  .card-wrap {
    position: relative;
  }
  .theme {
    width: 100%;
    padding: 0;
    border: var(--tb) solid var(--tl);
    border-radius: var(--tr);
    background: var(--tp);
    color: var(--tt);
    cursor: pointer;
    overflow: hidden;
    text-align: left;
    box-shadow: var(--ib-ex3);
    transition: transform calc(0.12s * var(--motion, 1)) var(--ib-press);
  }
  .theme:hover {
    transform: translate(-2px, -2px);
  }
  .theme.on {
    outline: var(--bw-lg) solid var(--ib-accent);
    outline-offset: 3px;
  }
  .mini {
    display: flex;
    height: 96px;
  }
  .mini-side {
    width: 34px;
    display: flex;
    flex-direction: column;
    gap: 5px;
    padding: 9px 7px;
    border-right: 1px solid color-mix(in srgb, var(--tl) 40%, transparent);
  }
  .mini-side i {
    height: 6px;
    border-radius: 2px;
    background: color-mix(in srgb, var(--tt) 30%, transparent);
  }
  .mini-side i:first-child {
    background: var(--ta);
  }
  .mini-main {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 7px;
    padding: 9px 10px;
    min-width: 0;
  }
  .mini-main b {
    font-size: 13px;
    font-weight: 900;
    letter-spacing: -0.02em;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .mini-bar {
    height: 8px;
    border: 1.5px solid var(--tl);
    border-radius: calc(var(--tr) / 3);
    background: var(--tc);
    overflow: hidden;
  }
  .mini-bar i {
    display: block;
    height: 100%;
    background: var(--ta);
  }
  .dots {
    display: flex;
    gap: 5px;
    padding: 7px 10px;
    border-top: 1px solid color-mix(in srgb, var(--tl) 40%, transparent);
    background: var(--tc);
  }
  .dots i {
    width: 12px;
    height: 12px;
    border-radius: 50%;
    border: 1.5px solid var(--tl);
  }
  .del {
    position: absolute;
    top: -8px;
    right: -8px;
    width: 24px;
    height: 24px;
    display: grid;
    place-items: center;
    border: var(--bw) solid var(--ib-line);
    border-radius: 50%;
    background: var(--bad);
    color: #1a0a0d;
    cursor: pointer;
  }

  /* colours */
  .colors {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(210px, 1fr));
    gap: 10px;
  }
  .color {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px;
    border: var(--bw) solid var(--ib-line);
    border-radius: var(--r);
    background: var(--ib-card);
  }
  .color input[type="color"] {
    width: 38px;
    height: 38px;
    padding: 0;
    border: var(--bw) solid var(--ib-line);
    border-radius: var(--r-sm);
    background: none;
    cursor: pointer;
    flex: none;
  }
  .color input[type="color"]::-webkit-color-swatch-wrapper {
    padding: 0;
  }
  .color input[type="color"]::-webkit-color-swatch {
    border: 0;
    border-radius: calc(var(--r-sm) - 2px);
  }
  .cl {
    flex: 1;
    font-weight: 750;
    font-size: 13px;
  }
  .hex {
    width: 76px;
    border: 0;
    background: none;
    color: var(--ib-muted);
    font-size: 12px;
    outline: none;
  }
  .checks {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 16px;
  }
  .check {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 6px 10px;
    border: var(--bw) solid var(--ib-line);
    border-radius: var(--r-sm);
    font-size: 12.5px;
    font-weight: 700;
  }
  .check i {
    font-style: normal;
    font-size: 10.5px;
    font-weight: 900;
    padding: 1px 6px;
    border-radius: 4px;
    background: var(--ok);
    color: #06210f;
  }
  .check.bad i {
    background: var(--bad);
    color: #1a0a0d;
  }

  /* generic control rows */
  .rows {
    display: flex;
    flex-direction: column;
    border: var(--bw) solid var(--ib-line);
    border-radius: var(--r-lg);
    background: var(--ib-card);
  }
  .ctl {
    display: grid;
    grid-template-columns: 180px minmax(0, 1fr) auto;
    align-items: center;
    gap: 14px;
    padding: 12px 16px;
    border-bottom: calc(var(--bw) * 0.8) solid var(--ib-paper-2);
    min-height: 56px;
  }
  .ctl:last-child {
    border-bottom: 0;
  }
  .ctl > span {
    font-weight: 800;
    font-size: 13.5px;
  }
  .ctl > b {
    min-width: 44px;
    text-align: right;
    font-weight: 900;
    font-size: 13px;
  }
  .ctl input[type="range"] {
    width: 100%;
    accent-color: var(--ib-accent);
  }
  .ctl :global(.ib-switch) {
    justify-self: start;
  }
  .seg-pick {
    display: inline-flex;
    flex-wrap: wrap;
    gap: 4px;
    padding: 3px;
    border: var(--bw) solid var(--ib-line);
    border-radius: var(--r);
    background: var(--ib-paper-2);
    justify-self: start;
  }
  .seg-pick button {
    padding: 6px 12px;
    border: 0;
    border-radius: var(--r-sm);
    background: none;
    font-weight: 800;
    font-size: 12.5px;
    color: var(--ib-muted);
    cursor: pointer;
  }
  .seg-pick button.on {
    background: var(--ib-accent);
    color: var(--ib-accent-ink);
  }
  .wallrow {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }
  .wallrow .field {
    flex: 1;
    min-width: 160px;
    height: 32px;
  }
  .shape-demo,
  .type-demo {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 14px;
    margin-top: 18px;
    padding: 20px;
    border: var(--bw) dashed var(--ib-faint);
    border-radius: var(--r-lg);
  }
  .shape-demo .field {
    width: 180px;
  }
  .type-demo {
    display: block;
  }
  .type-demo h2 {
    margin: 0;
    font-size: 34px;
    font-weight: var(--h-weight);
    letter-spacing: var(--h-track);
  }
  .type-demo p {
    margin: 8px 0 0;
    color: var(--ib-muted);
  }

  /* bars */
  .bars {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(230px, 1fr));
    gap: 10px;
    margin-bottom: 16px;
  }
  .bar-pick {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 12px 14px;
    border: var(--bw) solid var(--ib-line);
    border-radius: var(--r);
    background: var(--ib-card);
    font-weight: 800;
    font-size: 13px;
    text-align: left;
    cursor: pointer;
  }
  .bar-pick.on {
    outline: var(--bw-lg) solid var(--ib-accent);
    outline-offset: 2px;
  }

  /* css */
  .css textarea {
    font-size: 12.5px;
    min-height: 260px;
  }
  .snips {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 12px;
  }

  @media (max-width: 1100px) {
    .split {
      grid-template-columns: minmax(0, 1fr);
    }
    .preview {
      display: none;
    }
  }
</style>
