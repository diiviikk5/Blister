<script lang="ts">
  import Icon from "./Icon.svelte";
  import { api, copyText, errText } from "../lib/api";
  import { store } from "../lib/store.svelte";
  import { bytes, parseRate } from "../lib/format";
  import type { Settings } from "../lib/types";

  type Tab = "general" | "speed" | "torrents" | "video" | "browser" | "look";
  const tabs: { id: Tab; label: string }[] = [
    { id: "general", label: "General" },
    { id: "speed", label: "Speed" },
    { id: "torrents", label: "Torrents" },
    { id: "video", label: "Video sites" },
    { id: "browser", label: "Browser" },
    { id: "look", label: "Look" },
  ];
  let tab = $state<Tab>("general");

  // Edit a copy; save shortly after each change.
  let s = $state<Settings>({ ...store.settings! });
  let timer: ReturnType<typeof setTimeout>;
  function save() {
    clearTimeout(timer);
    timer = setTimeout(() => store.saveSettings($state.snapshot(s)), 350);
  }
  function set<K extends keyof Settings>(k: K, v: Settings[K]) {
    s[k] = v;
    save();
  }

  let down = $state(store.settings!.speedLimit ? bytes(store.settings!.speedLimit) : "");
  let up = $state(store.settings!.uploadLimit ? bytes(store.settings!.uploadLimit) : "");

  async function browse() {
    const d = await api.pickFolder(s.downloadDir);
    if (d) set("downloadDir", d);
  }

  let installing = $state(false);
  async function install() {
    installing = true;
    try {
      store.tools = await api.installYtDlp();
      store.toast("yt-dlp is ready", "ok");
    } catch (e) {
      store.toast(errText(e), "error");
    } finally {
      installing = false;
    }
  }

  async function newToken() {
    s = { ...(await api.resetToken()) };
    store.settings = s;
    store.toast("New pairing code. Paste it into the extension.", "info");
  }

  const accents = ["#ff5c39", "#ffd23f", "#d2ff2e", "#2fd27a", "#3dd6ff", "#3d7bff", "#b18cff", "#ff8fb1"];
</script>

<div class="settings">
  <div class="top">
    <h1>Settings</h1>
    <div class="ib-tabs" role="tablist">
      {#each tabs as t (t.id)}
        <button class="ib-tab" role="tab" aria-selected={tab === t.id} onclick={() => (tab = t.id)}>{t.label}</button>
      {/each}
    </div>
  </div>

  <div class="scroll">
    <div class="card">
      {#if tab === "general"}
        <div class="row">
          <div class="txt"><b>Download folder</b><span>Where finished files go.</span></div>
          <div class="ctl folder">
            <input class="field" value={s.downloadDir} onchange={(e) => set("downloadDir", e.currentTarget.value)} />
            <button class="btn btn--icon" onclick={browse} aria-label="Choose folder"><Icon name="folder" size={16} /></button>
          </div>
        </div>
        <div class="row">
          <div class="txt"><b>Sort into folders</b><span>Videos, Music, Archives, Programs and so on, inside the download folder.</span></div>
          <input type="checkbox" class="ib-switch" checked={s.categorizeFolders} onchange={(e) => set("categorizeFolders", e.currentTarget.checked)} />
        </div>
        <div class="row">
          <div class="txt"><b>Catch copied links</b><span>When you copy a link anywhere, offer to download it.</span></div>
          <input type="checkbox" class="ib-switch" checked={s.clipboardWatch} onchange={(e) => set("clipboardWatch", e.currentTarget.checked)} />
        </div>
        <div class="row">
          <div class="txt"><b>Notifications</b><span>A Windows notification when a download finishes or fails.</span></div>
          <input type="checkbox" class="ib-switch" checked={s.notifications} onchange={(e) => set("notifications", e.currentTarget.checked)} />
        </div>
        <div class="row">
          <div class="txt"><b>Sound on finish</b><span>A short two-note chime.</span></div>
          <input type="checkbox" class="ib-switch" checked={s.sounds} onchange={(e) => set("sounds", e.currentTarget.checked)} />
        </div>
        <div class="row">
          <div class="txt"><b>Ask before removing</b><span>Confirm before removing downloads from the list.</span></div>
          <input type="checkbox" class="ib-switch" checked={s.confirmDelete} onchange={(e) => set("confirmDelete", e.currentTarget.checked)} />
        </div>
        <div class="row">
          <div class="txt"><b>Start with Windows</b><span>Launch minimised when you sign in, so queued downloads carry on.</span></div>
          <input type="checkbox" class="ib-switch" checked={s.startOnBoot} onchange={(e) => set("startOnBoot", e.currentTarget.checked)} />
        </div>
        <div class="row">
          <div class="txt"><b>Download window</b><span>Only download between these times, e.g. overnight. Leave empty to always download.</span></div>
          <div class="ctl times">
            <input class="field" type="time" value={s.scheduleStart} onchange={(e) => set("scheduleStart", e.currentTarget.value)} />
            <span class="faint">to</span>
            <input class="field" type="time" value={s.scheduleEnd} onchange={(e) => set("scheduleEnd", e.currentTarget.value)} />
          </div>
        </div>
      {:else if tab === "speed"}
        <div class="row">
          <div class="txt"><b>Downloads at once</b><span>The rest wait in the queue.</span></div>
          <div class="ctl stepper">
            <button class="btn btn--sm btn--icon" onclick={() => set("maxActive", Math.max(1, s.maxActive - 1))} aria-label="Fewer">−</button>
            <span class="num big">{s.maxActive}</span>
            <button class="btn btn--sm btn--icon" onclick={() => set("maxActive", Math.min(32, s.maxActive + 1))} aria-label="More">+</button>
          </div>
        </div>
        <div class="row">
          <div class="txt"><b>Connections per download</b><span>More connections beat per-connection throttling. 16 suits most servers; some cap it lower.</span></div>
          <div class="ctl slider">
            <input type="range" min="1" max="64" value={s.connections} oninput={(e) => set("connections", +e.currentTarget.value)} />
            <span class="num big">{s.connections}</span>
          </div>
        </div>
        <div class="row">
          <div class="txt"><b>Download speed cap</b><span>For everything combined. Empty means unlimited. Try “5 MB”.</span></div>
          <input class="field ctl" placeholder="Unlimited" bind:value={down} onchange={() => set("speedLimit", parseRate(down))} />
        </div>
        <div class="row">
          <div class="txt"><b>Retries</b><span>How often a dropped connection is retried before the download fails.</span></div>
          <input class="field ctl short" type="number" min="1" max="100" value={s.maxRetries} onchange={(e) => set("maxRetries", +e.currentTarget.value)} />
        </div>
        <div class="row">
          <div class="txt"><b>Proxy</b><span>http://, https:// or socks5:// — empty uses the system proxy.</span></div>
          <input class="field ctl mono" placeholder="socks5://127.0.0.1:1080" value={s.proxy} onchange={(e) => set("proxy", e.currentTarget.value)} />
        </div>
        <div class="row">
          <div class="txt"><b>User agent</b><span>Leave empty to look like Chrome, which most servers prefer.</span></div>
          <input class="field ctl mono" placeholder="Browser default" value={s.userAgent} onchange={(e) => set("userAgent", e.currentTarget.value)} />
        </div>
        <div class="row">
          <div class="txt"><b>Allow untrusted certificates</b><span>Only for mirrors with self-signed HTTPS. Leave off otherwise.</span></div>
          <input type="checkbox" class="ib-switch" checked={s.insecureTls} onchange={(e) => set("insecureTls", e.currentTarget.checked)} />
        </div>
      {:else if tab === "torrents"}
        <div class="row">
          <div class="txt"><b>Seed after downloading</b><span>Give back to the swarm until the ratio below is reached.</span></div>
          <input type="checkbox" class="ib-switch" checked={s.seedAfter} onchange={(e) => set("seedAfter", e.currentTarget.checked)} />
        </div>
        <div class="row">
          <div class="txt"><b>Seed ratio</b><span>Upload this many times the download size, then stop.</span></div>
          <input class="field ctl short" type="number" step="0.1" min="0" value={s.seedRatio} onchange={(e) => set("seedRatio", +e.currentTarget.value)} />
        </div>
        <div class="row">
          <div class="txt"><b>Upload speed cap</b><span>Empty means unlimited.</span></div>
          <input class="field ctl" placeholder="Unlimited" bind:value={up} onchange={() => set("uploadLimit", parseRate(up))} />
        </div>
        <div class="row">
          <div class="txt"><b>Listening port</b><span>Takes effect after restarting Blister.</span></div>
          <input class="field ctl short" type="number" value={s.torrentPort} onchange={(e) => set("torrentPort", +e.currentTarget.value)} />
        </div>
        <div class="row">
          <div class="txt"><b>DHT</b><span>Find peers without trackers. Needed for most magnet links.</span></div>
          <input type="checkbox" class="ib-switch" checked={s.enableDht} onchange={(e) => set("enableDht", e.currentTarget.checked)} />
        </div>
        <div class="row">
          <div class="txt"><b>Open port on router (UPnP)</b><span>Lets more peers connect to you.</span></div>
          <input type="checkbox" class="ib-switch" checked={s.enableUpnp} onchange={(e) => set("enableUpnp", e.currentTarget.checked)} />
        </div>
        <div class="row">
          <div class="txt"><b>Prefer encrypted peers</b><span>Helps on networks that throttle BitTorrent.</span></div>
          <input type="checkbox" class="ib-switch" checked={s.encryptPeers} onchange={(e) => set("encryptPeers", e.currentTarget.checked)} />
        </div>
      {:else if tab === "video"}
        <div class="row">
          <div class="txt">
            <b>yt-dlp</b>
            <span>Downloads from YouTube, X, Instagram, TikTok, Reddit, Vimeo, Twitch and about 1,800 more. Blister installs and updates it for you.</span>
          </div>
          <div class="ctl tool">
            {#if store.tools.ytDlp}
              <span class="pill pill--ok"><Icon name="check" size={11} stroke={3} /> Ready</span>
            {:else}
              <button class="btn btn--accent" disabled={installing} onclick={install}>{installing ? "Installing…" : "Install now"}</button>
            {/if}
          </div>
        </div>
        <div class="row">
          <div class="txt">
            <b>ffmpeg</b>
            <span>Needed for 1080p and above (video and audio are separate streams) and for MP3. Without it Blister picks the best single-file format.</span>
          </div>
          <div class="ctl tool">
            {#if store.tools.ffmpeg}
              <span class="pill pill--ok"><Icon name="check" size={11} stroke={3} /> Found</span>
            {:else}
              <span class="pill pill--hot">Not found</span>
            {/if}
          </div>
        </div>
        <div class="row">
          <div class="txt"><b>Custom yt-dlp path</b><span>Use your own copy instead.</span></div>
          <input class="field ctl mono" placeholder="Automatic" value={s.ytDlpPath} onchange={(e) => set("ytDlpPath", e.currentTarget.value)} />
        </div>
        <div class="row">
          <div class="txt"><b>Custom ffmpeg path</b><span>Point at ffmpeg.exe if it isn't on PATH.</span></div>
          <input class="field ctl mono" placeholder="Automatic" value={s.ffmpegPath} onchange={(e) => set("ffmpegPath", e.currentTarget.value)} />
        </div>
      {:else if tab === "browser"}
        <div class="row">
          <div class="txt"><b>Pairing code</b><span>Paste this into the Blister browser extension so it can hand downloads over. Keep it private.</span></div>
          <div class="ctl token">
            <code class="mono">{s.browserToken}</code>
            <button class="btn btn--sm btn--icon" onclick={() => (copyText(s.browserToken), store.toast("Pairing code copied"))} aria-label="Copy"><Icon name="copy" size={13} /></button>
            <button class="btn btn--sm" onclick={newToken}>New code</button>
          </div>
        </div>
        <div class="row">
          <div class="txt"><b>Local port</b><span>The extension talks to Blister on 127.0.0.1 at this port. Nothing is reachable from other machines.</span></div>
          <input class="field ctl short" type="number" value={s.browserPort} onchange={(e) => set("browserPort", +e.currentTarget.value)} />
        </div>
      {:else}
        <div class="row">
          <div class="txt"><b>Theme</b><span>Blister runs hot either way.</span></div>
          <div class="ib-tabs ctl-tabs" role="tablist">
            {#each [["dark", "Dark"], ["light", "Light"], ["system", "System"]] as [v, l] (v)}
              <button class="ib-tab" role="tab" aria-selected={s.theme === v} onclick={() => set("theme", v as Settings["theme"])}>{l}</button>
            {/each}
          </div>
        </div>
        <div class="row">
          <div class="txt"><b>Accent</b><span>Buttons, bars and the active filter.</span></div>
          <div class="swatches">
            {#each accents as c (c)}
              <button class="sw" class:on={s.accent === c} style:background={c} onclick={() => set("accent", c)} aria-label="Accent {c}"></button>
            {/each}
          </div>
        </div>
        <div class="row">
          <div class="txt"><b>Density</b><span>Compact fits more downloads on screen.</span></div>
          <div class="ib-tabs ctl-tabs" role="tablist">
            {#each [["comfortable", "Comfortable"], ["compact", "Compact"]] as [v, l] (v)}
              <button class="ib-tab" role="tab" aria-selected={s.density === v} onclick={() => set("density", v as Settings["density"])}>{l}</button>
            {/each}
          </div>
        </div>
      {/if}
    </div>
    <p class="foot faint">Blister {store.version} · settings save as you go</p>
  </div>
</div>

<style>
  .settings {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }
  .top {
    display: flex;
    align-items: center;
    gap: 20px;
    padding: 16px 18px 14px;
    border-bottom: 3px solid var(--ib-line);
  }
  h1 {
    margin: 0;
    font-size: 26px;
    font-weight: 900;
    letter-spacing: -0.05em;
  }
  .scroll {
    flex: 1;
    overflow-y: auto;
    padding: 22px 24px 30px;
  }
  .card {
    max-width: 860px;
    border: 3px solid var(--ib-line);
    border-radius: 16px;
    background: var(--ib-card);
    box-shadow: var(--ib-ex6);
  }
  .row {
    display: flex;
    align-items: center;
    gap: 24px;
    padding: 16px 20px;
    border-bottom: 2px solid var(--ib-paper-2);
  }
  .row:last-child {
    border-bottom: 0;
  }
  .txt {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 3px;
    min-width: 0;
  }
  .txt b {
    font-size: 14.5px;
    font-weight: 850;
    letter-spacing: -0.01em;
  }
  .txt span {
    font-size: 12.5px;
    color: var(--ib-muted);
    line-height: 1.45;
    font-weight: 550;
  }
  .ctl {
    width: 300px;
    flex: none;
  }
  .ctl.short {
    width: 110px;
  }
  .folder,
  .times,
  .stepper,
  .slider,
  .token {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .folder .btn {
    flex: none;
    width: 38px;
    height: 38px;
  }
  .times .field {
    width: 128px;
  }
  .stepper {
    width: auto;
  }
  .big {
    min-width: 32px;
    text-align: center;
    font-weight: 900;
    font-size: 17px;
  }
  .slider input {
    flex: 1;
    accent-color: var(--ib-accent);
  }
  .tool {
    width: auto;
  }
  .token code {
    flex: 1;
    padding: 6px 10px;
    border: 2px solid var(--ib-line);
    border-radius: 8px;
    background: var(--ib-paper-2);
    font-size: 12px;
    overflow: hidden;
    text-overflow: ellipsis;
    filter: blur(3.5px);
    transition: filter 0.15s;
    user-select: all;
  }
  .token code:hover {
    filter: none;
  }
  .swatches {
    display: flex;
    gap: 8px;
  }
  .sw {
    width: 30px;
    height: 30px;
    border: 2.5px solid var(--ib-line);
    border-radius: 8px;
    cursor: pointer;
    box-shadow: 2px 2px 0 var(--ib-ex);
    transition: transform 0.1s var(--ib-press);
  }
  .sw:hover {
    transform: translate(-1px, -1px);
  }
  .sw.on {
    transform: translate(2px, 2px);
    box-shadow: none;
    outline: 3px solid var(--ib-line);
    outline-offset: 2px;
  }
  .foot {
    font-size: 12px;
    font-weight: 700;
    margin: 16px 4px 0;
  }
</style>
