<script lang="ts">
  // Automation: rules that shape new downloads, and hooks that run when they finish.
  import Icon from "./Icon.svelte";
  import { api } from "../lib/api";
  import { store } from "../lib/store.svelte";
  import { bytes, parseRate } from "../lib/format";
  import type { Rule, Settings } from "../lib/types";

  let { s, save }: { s: Settings; save: (patch: Partial<Settings>) => void } = $props();

  const rules = $derived(s.rules ?? []);
  const kinds: [string, string][] = [["http", "Files"], ["torrent", "Torrents"], ["media", "Video sites"], ["hls", "Streams"]];

  function set(i: number, patch: Partial<Rule>) {
    const next = rules.map((r, k) => (k === i ? { ...r, ...patch } : r));
    save({ rules: next });
  }
  function add(preset?: Partial<Rule>) {
    const r: Rule = {
      id: Math.random().toString(36).slice(2, 10),
      name: "New rule",
      enabled: true,
      hosts: "",
      exts: "",
      kinds: [],
      minSize: 0,
      maxSize: 0,
      dir: "",
      connections: 0,
      speedLimit: 0,
      paused: false,
      tags: [],
      run: "",
      ...preset,
    };
    save({ rules: [...rules, r] });
  }
  function remove(i: number) {
    save({ rules: rules.filter((_, k) => k !== i) });
  }
  function move(i: number, d: number) {
    const j = i + d;
    if (j < 0 || j >= rules.length) return;
    const next = [...rules];
    [next[i], next[j]] = [next[j], next[i]];
    save({ rules: next });
  }
  async function pickDir(i: number) {
    const d = await api.pickFolder(rules[i].dir || s.downloadDir);
    if (d) set(i, { dir: d });
  }

  const examples: [string, Partial<Rule>][] = [
    ["Linux ISOs → own folder, 32 lanes", { name: "Linux ISOs", exts: "iso img", connections: 32, dir: "" }],
    ["Big files wait for the night", { name: "Big files overnight", minSize: 4 * 1024 ** 3, paused: true, tags: ["night"] }],
    ["Unzip archives when done", { name: "Unzip archives", exts: "zip", run: 'powershell -NoProfile -Command Expand-Archive -LiteralPath {path} -DestinationPath {dir} -Force' }],
    ["Throttle a slow host", { name: "Gentle on mirror", hosts: "example.org", speedLimit: 2 * 1024 ** 2 }],
  ];
</script>

<div class="auto">
  <div class="hook">
    <div class="txt">
      <b>After every download</b>
      <span>Runs a command when any download finishes. Use <code>{"{path}"}</code>, <code>{"{name}"}</code>, <code>{"{dir}"}</code>, <code>{"{url}"}</code>, <code>{"{size}"}</code>.</span>
    </div>
    <input class="field mono" placeholder={'e.g. notify-send "done" {name}  or  explorer {dir}'} value={s.onComplete} onchange={(e) => save({ onComplete: e.currentTarget.value })} />
  </div>

  <div class="head">
    <h2>Rules <span class="faint num">{rules.length}</span></h2>
    <button class="btn btn--accent" onclick={() => add()}><Icon name="plus" size={15} stroke={3} /> New rule</button>
  </div>

  {#if !rules.length}
    <div class="empty">
      <p>Rules shape downloads the moment they're added: where they go, how many connections they get, how fast they may run, whether they wait, and what runs when they finish. They apply to links from anywhere: the add box, the clipboard and the browser.</p>
      <div class="ex">
        {#each examples as [label, preset] (label)}
          <button class="btn btn--sm" onclick={() => add(preset)}><Icon name="plus" size={12} /> {label}</button>
        {/each}
      </div>
    </div>
  {/if}

  {#each rules as r, i (r.id)}
    <div class="rule" class:off={!r.enabled}>
      <div class="rhead">
        <input type="checkbox" class="ib-switch" checked={r.enabled} onchange={(e) => set(i, { enabled: e.currentTarget.checked })} aria-label="Enabled" />
        <input class="rname" value={r.name} onchange={(e) => set(i, { name: e.currentTarget.value })} aria-label="Rule name" />
        <span class="faint num">#{i + 1}</span>
        <button class="btn btn--sm btn--icon btn--ghost" onclick={() => move(i, -1)} disabled={i === 0} aria-label="Move up"><Icon name="up" size={13} /></button>
        <button class="btn btn--sm btn--icon btn--ghost" onclick={() => move(i, 1)} disabled={i === rules.length - 1} aria-label="Move down"><Icon name="down" size={13} /></button>
        <button class="btn btn--sm btn--icon btn--ghost" onclick={() => remove(i)} aria-label="Delete rule"><Icon name="trash" size={13} /></button>
      </div>
      <div class="cols">
        <div class="col">
          <div class="label">When</div>
          <label class="f"><span>Host is</span><input class="field" placeholder="any, or github.com *.cdn.net" value={r.hosts} onchange={(e) => set(i, { hosts: e.currentTarget.value })} /></label>
          <label class="f"><span>Extension</span><input class="field mono" placeholder="any, or zip rar 7z" value={r.exts} onchange={(e) => set(i, { exts: e.currentTarget.value })} /></label>
          <div class="f">
            <span>Kind</span>
            <div class="chips">
              {#each kinds as [k, l] (k)}
                <button class="chip" class:on={r.kinds?.includes(k)} onclick={() => set(i, { kinds: r.kinds?.includes(k) ? r.kinds.filter((x) => x !== k) : [...(r.kinds ?? []), k] })}>{l}</button>
              {/each}
            </div>
          </div>
          <label class="f"><span>Bigger than</span><input class="field" placeholder="e.g. 1 GB" value={r.minSize ? bytes(r.minSize, 0) : ""} onchange={(e) => set(i, { minSize: parseRate(e.currentTarget.value) })} /></label>
          <label class="f"><span>Smaller than</span><input class="field" placeholder="e.g. 50 MB" value={r.maxSize ? bytes(r.maxSize, 0) : ""} onchange={(e) => set(i, { maxSize: parseRate(e.currentTarget.value) })} /></label>
        </div>
        <div class="col">
          <div class="label">Then</div>
          <div class="f">
            <span>Save to</span>
            <div class="row2">
              <input class="field" placeholder="default folder" value={r.dir} onchange={(e) => set(i, { dir: e.currentTarget.value })} />
              <button class="btn btn--icon" onclick={() => pickDir(i)} aria-label="Choose folder"><Icon name="folder" size={15} /></button>
            </div>
          </div>
          <label class="f"><span>Connections</span><input class="field" type="number" min="0" max="64" placeholder="default" value={r.connections || ""} onchange={(e) => set(i, { connections: Number(e.currentTarget.value) || 0 })} /></label>
          <label class="f"><span>Speed cap</span><input class="field" placeholder="unlimited, or 2 MB" value={r.speedLimit ? bytes(r.speedLimit, 0) : ""} onchange={(e) => set(i, { speedLimit: parseRate(e.currentTarget.value) })} /></label>
          <label class="f"><span>Tags</span><input class="field" placeholder="work, linux" value={(r.tags ?? []).join(", ")} onchange={(e) => set(i, { tags: e.currentTarget.value.split(",").map((x) => x.trim()).filter(Boolean) })} /></label>
          <label class="f chk"><input type="checkbox" class="ib-switch" checked={r.paused} onchange={(e) => set(i, { paused: e.currentTarget.checked })} /><span>Add paused</span></label>
          <label class="f"><span>When done, run</span><input class="field mono" placeholder={"e.g. 7z x {path} -o{dir}"} value={r.run} onchange={(e) => set(i, { run: e.currentTarget.value })} /></label>
        </div>
      </div>
    </div>
  {/each}
  {#if rules.length}<p class="foot faint">Rules run top to bottom; later rules win when two set the same thing. Choices you make in the add dialog always beat rules.</p>{/if}
</div>

<style>
  .auto {
    max-width: 920px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .hook {
    display: grid;
    gap: 10px;
    padding: 16px 18px;
    border: var(--bw-lg) solid var(--ib-line);
    border-radius: var(--r-lg);
    background: var(--ib-card);
    box-shadow: var(--ib-ex4);
  }
  .txt {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .txt span {
    font-size: 12.5px;
    color: var(--ib-muted);
  }
  code {
    font-family: var(--ib-mono);
    font-size: 11.5px;
    padding: 0 4px;
    border-radius: 4px;
    background: var(--ib-paper-2);
  }
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: 6px;
  }
  h2 {
    margin: 0;
    font-size: 22px;
    font-weight: var(--h-weight);
    letter-spacing: var(--h-track);
  }
  h2 span {
    font-size: 14px;
  }
  .empty {
    padding: 20px;
    border: var(--bw) dashed var(--ib-faint);
    border-radius: var(--r-lg);
  }
  .empty p {
    margin: 0 0 14px;
    color: var(--ib-muted);
    line-height: 1.55;
    font-size: 13.5px;
  }
  .ex {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }
  .rule {
    border: var(--bw-lg) solid var(--ib-line);
    border-radius: var(--r-lg);
    background: var(--ib-card);
    box-shadow: var(--ib-ex4);
  }
  .rule.off {
    opacity: 0.6;
  }
  .rhead {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 12px 10px 14px;
    border-bottom: calc(var(--bw) * 0.8) solid var(--ib-paper-2);
  }
  .rname {
    flex: 1;
    min-width: 0;
    border: 0;
    background: none;
    color: var(--ib-text);
    font: inherit;
    font-size: 16px;
    font-weight: 850;
    outline: none;
  }
  .cols {
    display: grid;
    grid-template-columns: 1fr 1fr;
  }
  .col {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 14px 16px;
  }
  .col + .col {
    border-left: calc(var(--bw) * 0.8) solid var(--ib-paper-2);
  }
  .f {
    display: grid;
    grid-template-columns: 110px minmax(0, 1fr);
    align-items: center;
    gap: 10px;
    font-size: 12.5px;
    font-weight: 750;
    color: var(--ib-muted);
  }
  .f .field {
    height: 32px;
    font-size: 13px;
  }
  .chk {
    grid-template-columns: auto 1fr;
  }
  .row2 {
    display: flex;
    gap: 6px;
  }
  .row2 .btn {
    width: 32px;
    height: 32px;
    flex: none;
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 5px;
  }
  .chip {
    padding: 4px 9px;
    border: calc(var(--bw) * 0.8) solid var(--ib-line);
    border-radius: 999px;
    background: none;
    font-size: 11.5px;
    font-weight: 800;
    cursor: pointer;
    color: var(--ib-muted);
  }
  .chip.on {
    background: var(--ib-accent);
    color: var(--ib-accent-ink);
  }
  .foot {
    font-size: 12px;
    margin: 0;
  }
  @media (max-width: 1050px) {
    .cols {
      grid-template-columns: 1fr;
    }
    .col + .col {
      border-left: 0;
      border-top: calc(var(--bw) * 0.8) solid var(--ib-paper-2);
    }
  }
</style>
