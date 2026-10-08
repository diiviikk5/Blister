// A pretend engine for running the UI in a plain browser. It fakes segmented
// HTTP downloads, a torrent and a video so every state can be seen.
import type { AddRequest, Boot, Kind, Settings, Task, Tick, TickItem } from "./types";

type Handler = (...data: any[]) => void;

const defaults: Settings = {
  downloadDir: "C:\\Users\\you\\Downloads\\Blister",
  categorizeFolders: true,
  maxActive: 4,
  connections: 16,
  speedLimit: 0,
  uploadLimit: 0,
  maxRetries: 10,
  proxy: "",
  userAgent: "",
  insecureTls: false,
  torrentPort: 42069,
  seedAfter: true,
  seedRatio: 1,
  enableDht: true,
  enableUpnp: true,
  encryptPeers: true,
  ytDlpPath: "",
  ffmpegPath: "",
  clipboardWatch: true,
  browserPort: 7323,
  browserToken: "3f9c2a71b0d84e6a9c11f2e7d5a0b8c4",
  theme: "dark",
  accent: "#ffd60a",
  density: "comfortable",
  notifications: true,
  sounds: true,
  confirmDelete: true,
  startOnBoot: false,
  closeToTray: false,
  scheduleStart: "",
  scheduleEnd: "",
};

const cats: Record<string, string> = {
  mp4: "video", mkv: "video", mp3: "audio", flac: "audio", zip: "archive", "7z": "archive",
  iso: "archive", exe: "program", msi: "program", pdf: "document", png: "image",
};

interface Sim {
  conns: { start: number; end: number; pos: number; rate: number }[];
  fin: [number, number][]; // ranges finished by connections that moved on
  base: number;
}

let n = 0;
const id = () => `m${Date.now().toString(36)}${(n++).toString(36)}`;

function task(p: Partial<Task> & { name: string; kind?: Kind }): Task {
  const ext = p.name.split(".").pop()?.toLowerCase() ?? "";
  return {
    id: id(),
    kind: "http",
    url: `https://mirror.example.com/${encodeURIComponent(p.name)}`,
    dir: defaults.downloadDir,
    category: cats[ext] ?? "other",
    status: "queued",
    size: -1,
    done: 0,
    speed: 0,
    upSpeed: 0,
    uploaded: 0,
    eta: -1,
    conns: 0,
    seeds: 0,
    resumable: true,
    connections: 0,
    speedLimit: 0,
    priority: 0,
    request: { url: "" },
    createdAt: new Date().toISOString(),
    ...p,
  };
}

export function createMock() {
  const handlers = new Map<string, Set<Handler>>();
  const emit = (ev: string, ...data: any[]) => handlers.get(ev)?.forEach((h) => h(...data));
  let settings = { ...defaults };
  const sims = new Map<string, Sim>();

  const tasks: Task[] = [
    task({ name: "ubuntu-24.04.1-desktop-amd64.iso", size: 6_114_656_256, status: "downloading", connections: 16 }),
    task({ name: "Blender-4.3-windows-x64.msi", size: 382_730_240, status: "downloading", connections: 8 }),
    task({
      name: "Big Buck Bunny 4K",
      kind: "torrent",
      category: "video",
      size: 2_400_000_000,
      status: "downloading",
      url: "magnet:?xt=urn:btih:dd8255ecdc7ca55fb0bbf81323d87062db1f6d1c&dn=Big+Buck+Bunny",
      torrent: {
        infoHash: "dd8255ecdc7ca55fb0bbf81323d87062db1f6d1c",
        ratio: 0.12,
        pieces: 4578,
        pieceMap: "",
        files: [
          { path: "Big Buck Bunny/bbb_sunflower_2160p_60fps.mp4", size: 2_300_000_000, done: 0, selected: true },
          { path: "Big Buck Bunny/poster.jpg", size: 1_200_000, done: 0, selected: true },
          { path: "Big Buck Bunny/subtitles.srt", size: 40_000, done: 0, selected: false },
        ],
      },
    }),
    task({
      name: "Lo-fi beats to download to [jfKfPfyJRdk].mp4",
      kind: "media",
      category: "video",
      url: "https://www.youtube.com/watch?v=jfKfPfyJRdk",
      size: 512_000_000,
      status: "downloading",
      media: { format: "best", audioOnly: false, title: "Lo-fi beats to download to", stage: "video" },
    }),
    task({ name: "node-v22.11.0-x64.msi", size: 30_000_000, status: "queued" }),
    task({ name: "annual-report-2025.pdf", size: 8_400_000, status: "paused", done: 3_100_000 }),
    task({ name: "dataset-shard-07.7z", size: 1_900_000_000, status: "error", error: "The server refused the connection", done: 220_000_000 }),
    task({ name: "vacation-photos.zip", size: 744_000_000, status: "completed", done: 744_000_000, completedAt: new Date(Date.now() - 3600e3).toISOString() }),
    task({ name: "podcast-ep-142.mp3", size: 61_000_000, status: "completed", done: 61_000_000, completedAt: new Date(Date.now() - 86400e3).toISOString() }),
  ];
  tasks.forEach((t, i) => (t.done ||= i === 0 ? t.size * 0.41 : i === 1 ? t.size * 0.12 : i === 2 ? t.size * 0.3 : 0));

  function sim(t: Task): Sim {
    let s = sims.get(t.id);
    if (!s) {
      const k = t.kind === "http" ? t.connections || settings.connections : 1;
      const size = Math.max(t.size, 1);
      const per = size / k;
      s = {
        fin: [],
        base: t.kind === "torrent" ? 3.2e6 : t.kind === "media" ? 2.4e6 : 1.1e6,
        conns: Array.from({ length: k }, (_, i) => ({
          start: i * per,
          end: (i + 1) * per,
          pos: i * per + per * (t.done / size),
          rate: 0.6 + Math.random() * 0.8,
        })),
      };
      sims.set(t.id, s);
    }
    return s;
  }

  function step() {
    const active = tasks.filter((t) => t.status === "downloading" || t.status === "starting" || t.status === "seeding");
    const running = active.filter((t) => t.status !== "seeding").length;
    for (const t of tasks) {
      if (t.status === "queued" && running < settings.maxActive) {
        t.status = "downloading";
        if (t.size < 0) t.size = 50_000_000 + Math.random() * 900_000_000;
        emit("task:updated", { ...t });
        break;
      }
    }
    const tick: Tick = { items: [], speed: 0, upSpeed: 0, active: 0 };
    for (const t of tasks) {
      if (t.status !== "downloading") continue;
      const s = sim(t);
      for (const c of s.conns) {
        const wobble = 0.7 + Math.random() * 0.6;
        c.pos = Math.min(c.end, c.pos + s.base * c.rate * wobble * 0.5);
      }
      // Dynamic splitting: a finished connection steals half of the biggest gap.
      for (const c of s.conns) {
        if (c.pos < c.end) continue;
        const big = s.conns.reduce((a, b) => (b.end - b.pos > a.end - a.pos ? b : a));
        const left = big.end - big.pos;
        if (left > 4e6) {
          s.fin.push([c.start, c.end]);
          const mid = big.pos + left / 2;
          c.start = c.pos = mid;
          c.end = big.end;
          big.end = mid;
        }
      }
      let done = s.fin.reduce((a, [x, y]) => a + y - x, 0);
      const heads: number[] = [];
      for (const c of s.conns) {
        done += c.pos - c.start;
        if (c.pos < c.end) heads.push(c.pos / t.size);
      }
      const prev = t.done;
      t.done = Math.min(t.size, Math.max(prev, done));
      t.speed = Math.round((t.done - prev) * 2);
      t.conns = heads.length;
      t.eta = t.speed > 0 ? Math.round((t.size - t.done) / t.speed) : -1;
      const map = cover(t.size, [...s.fin.map(([a, b]) => ({ start: a, pos: b })), ...s.conns]);
      if (t.kind === "torrent" && t.torrent) {
        t.torrent.pieceMap = map;
        t.seeds = 12;
        t.conns = 37;
      }
      if (t.done >= t.size) {
        t.status = "completed";
        t.speed = 0;
        t.completedAt = new Date().toISOString();
        emit("task:completed", { ...t });
        emit("task:updated", { ...t });
        continue;
      }
      const item: TickItem = {
        id: t.id, status: t.status, done: t.done, size: t.size, speed: t.speed, upSpeed: 0,
        eta: t.eta, conns: t.conns, seeds: t.seeds, map, heads: t.kind === "http" ? heads : undefined,
      };
      tick.items!.push(item);
      tick.speed += t.speed;
      tick.active++;
    }
    emit("tasks:tick", tick);
  }

  function cover(size: number, ranges: { start: number; pos: number }[]): string {
    const cells = 120;
    const fill = new Array(cells).fill(0);
    const cell = size / cells;
    for (const r of ranges) {
      for (let a = r.start; a < r.pos; ) {
        const i = Math.floor(a / cell);
        if (i >= cells) break;
        const b = Math.min(r.pos, (i + 1) * cell);
        fill[i] += b - a;
        a = b <= a ? a + 1 : b;
      }
    }
    return fill.map((f) => String(Math.max(0, Math.min(9, Math.floor((f / cell) * 9))))).join("");
  }

  setInterval(step, 500);

  const find = (tid: string) => tasks.find((t) => t.id === tid);
  const upd = (t: Task) => emit("task:updated", { ...t });

  const impl: Record<string, (...a: any[]) => any> = {
    Boot: (): Boot => ({ version: "0.1.0-preview", tasks: tasks.map((t) => ({ ...t })), settings, tools: { ytDlp: "", ffmpeg: "" }, dataDir: "C:\\Users\\you\\AppData\\Roaming\\Blister" }),
    Tools: () => ({ ytDlp: "", ffmpeg: "" }),
    InstallYtDlp: async () => {
      await new Promise((r) => setTimeout(r, 1200));
      return { ytDlp: "C:\\Blister\\tools\\yt-dlp.exe", ffmpeg: "" };
    },
    Parse: (text: string) => {
      const out: any[] = [];
      for (const m of text.matchAll(/(magnet:\?[^\s"'<>]+|https?:\/\/[^\s"'<>]+)/gi)) {
        const url = m[0];
        const kind: Kind = url.startsWith("magnet:") || url.endsWith(".torrent") ? "torrent" : /youtu|vimeo|tiktok|instagram|x\.com|twitter/.test(url) ? "media" : url.includes(".m3u8") ? "hls" : "http";
        const name = kind === "torrent" ? decodeURIComponent(/dn=([^&]+)/.exec(url)?.[1]?.replace(/\+/g, " ") ?? "Torrent") : decodeURIComponent(url.split("?")[0].split("/").pop() || url);
        out.push({ url, kind, name });
      }
      return out;
    },
    Probe: async (url: string) => {
      await new Promise((r) => setTimeout(r, 300 + Math.random() * 600));
      const name = decodeURIComponent(url.split("?")[0].split("/").pop() || "download");
      const ext = name.split(".").pop() ?? "";
      return { name, size: Math.round(20e6 + Math.random() * 3e9), resumable: true, category: cats[ext] ?? "other", type: "application/octet-stream" };
    },
    Add: (reqs: AddRequest[]) =>
      reqs.map((r) => {
        const t = task({
          name: r.name || decodeURIComponent(r.url.split("?")[0].split("/").pop() || "download"),
          url: r.url,
          kind: r.kind ?? "http",
          size: r.size ?? -1,
          status: r.paused ? "paused" : "queued",
          connections: r.connections ?? 0,
        });
        tasks.push(t);
        emit("task:added", { ...t });
        return { ...t };
      }),
    Pause: (ids: string[]) => ids.forEach((i) => { const t = find(i); if (t && t.status !== "completed") { t.status = "paused"; t.speed = 0; upd(t); } }),
    Resume: (ids: string[]) => ids.forEach((i) => { const t = find(i); if (t && (t.status === "paused" || t.status === "error")) { t.status = "queued"; t.error = ""; upd(t); } }),
    Restart: (ids: string[]) => ids.forEach((i) => { const t = find(i); if (t) { t.done = 0; t.status = "queued"; sims.delete(t.id); upd(t); } }),
    Remove: (ids: string[]) => ids.forEach((i) => { const k = tasks.findIndex((t) => t.id === i); if (k >= 0) { tasks.splice(k, 1); emit("task:removed", i); } }),
    PauseAll: () => impl.Pause(tasks.map((t) => t.id)),
    ResumeAll: () => impl.Resume(tasks.map((t) => t.id)),
    ClearCompleted: () => impl.Remove(tasks.filter((t) => t.status === "completed").map((t) => t.id)),
    Move: () => {},
    Tune: (i: string, c: number, l: number) => { const t = find(i); if (t) { t.connections = c; t.speedLimit = l; sims.delete(t.id); upd(t); } },
    SelectFiles: (i: string, sel: boolean[]) => { const t = find(i); t?.torrent?.files?.forEach((f, k) => (f.selected = sel[k])); if (t) upd(t); },
    SetURL: (i: string, url: string) => { const t = find(i); if (t) { t.url = url; t.status = "queued"; t.error = ""; upd(t); } },
    Rename: (i: string, name: string) => { const t = find(i); if (t) { t.name = name; upd(t); } },
    Open: () => {},
    Reveal: () => {},
    OpenDownloads: () => {},
    OpenURL: (u: string) => window.open(u, "_blank"),
    Settings: () => settings,
    SaveSettings: (s: Settings) => { settings = { ...s }; emit("settings:changed", settings); return settings; },
    ResetToken: () => { settings = { ...settings, browserToken: Math.random().toString(16).slice(2).padEnd(32, "0") }; return settings; },
    SetSpeedLimit: (bps: number) => { settings = { ...settings, speedLimit: bps }; emit("settings:changed", settings); return settings; },
    PickFolder: (cur: string) => cur,
    PickTorrents: () => [],
  };

  return {
    async call(name: string, ...args: any[]) {
      const f = impl[name];
      if (!f) throw new Error(`mock: ${name} not implemented`);
      return structuredClone(await f(...args));
    },
    on(ev: string, h: Handler) {
      if (!handlers.has(ev)) handlers.set(ev, new Set());
      handlers.get(ev)!.add(h);
      return () => handlers.get(ev)!.delete(h);
    },
  };
}
