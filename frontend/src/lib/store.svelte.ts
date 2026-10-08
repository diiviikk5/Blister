// App state as Svelte 5 runes. Tasks live in a map for O(1) tick updates;
// the list view derives filtered/sorted arrays from it.
import { api, on, errText } from "./api";
import type { External, Settings, Task, Tick, TickItem, ToolStatus } from "./types";

export type Filter =
  | "all"
  | "active"
  | "queued"
  | "paused"
  | "completed"
  | "failed"
  | `cat:${string}`;

export type SortKey = "smart" | "added" | "name" | "size" | "progress" | "speed";

export interface Toast {
  id: number;
  text: string;
  tone: "info" | "ok" | "error";
  action?: { label: string; run: () => void };
}

export interface Live {
  map?: string;
  heads?: number[];
}

class Store {
  tasks = $state<Record<string, Task>>({});
  order = $state<string[]>([]);
  live = $state<Record<string, Live>>({});
  settings = $state<Settings | null>(null);
  tools = $state<ToolStatus>({ ytDlp: "", ffmpeg: "" });
  version = $state("");

  speed = $state(0);
  upSpeed = $state(0);
  /** Last 120 samples of total download speed for the graph (2/s). */
  history = $state<number[]>(new Array(120).fill(0));

  filter = $state<Filter>("all");
  query = $state("");
  sort = $state<SortKey>("smart");
  selected = $state<string[]>([]);
  focus = $state<string | null>(null);

  view = $state<"list" | "settings">("list");
  addOpen = $state(false);
  addSeed = $state<External | null>(null);
  toasts = $state<Toast[]>([]);
  clipboard = $state<string[] | null>(null);
  /** Download whose link is being replaced (expired/broken links). */
  relink = $state<Task | null>(null);

  #toastId = 0;

  all = $derived(this.order.map((id) => this.tasks[id]).filter(Boolean));

  counts = $derived.by(() => {
    const c: Record<string, number> = { all: 0, active: 0, queued: 0, paused: 0, completed: 0, failed: 0 };
    for (const t of this.all) {
      c.all++;
      c[bucket(t)]++;
      const k = `cat:${catOf(t)}`;
      c[k] = (c[k] ?? 0) + 1;
    }
    return c;
  });

  visible = $derived.by(() => {
    const q = this.query.trim().toLowerCase();
    let list = this.all.filter((t) => matches(t, this.filter));
    if (q) list = list.filter((t) => t.name.toLowerCase().includes(q) || t.url.toLowerCase().includes(q));
    const by = this.sort;
    if (by === "smart") {
      // Working downloads first, then waiting, stopped, failed, finished;
      // newest first inside each group.
      const rank = { active: 0, queued: 1, paused: 2, failed: 3, completed: 4 };
      const idx = new Map(this.order.map((id, i) => [id, i]));
      list = [...list].sort((a, b) => rank[bucket(a)] - rank[bucket(b)] || idx.get(b.id)! - idx.get(a.id)!);
    } else if (by !== "added") {
      list = [...list].sort((a, b) => {
        switch (by) {
          case "name":
            return a.name.localeCompare(b.name);
          case "size":
            return b.size - a.size;
          case "progress":
            return prog(b) - prog(a);
          case "speed":
            return b.speed - a.speed;
        }
        return 0;
      });
    } else {
      // Newest first reads best; the queue order still drives scheduling.
      list = [...list].reverse();
    }
    return list;
  });

  focused = $derived(this.focus ? this.tasks[this.focus] ?? null : null);

  async init() {
    const b = await api.boot();
    this.version = b.version;
    this.settings = b.settings;
    this.tools = b.tools;
    const tasks: Record<string, Task> = {};
    for (const t of b.tasks ?? []) tasks[t.id] = t;
    this.tasks = tasks;
    this.order = (b.tasks ?? []).map((t) => t.id);

    on("task:added", (t: Task) => {
      this.tasks[t.id] = t;
      if (!this.order.includes(t.id)) this.order.push(t.id);
    });
    on("task:updated", (t: Task) => {
      if (!this.tasks[t.id]) this.order.push(t.id);
      this.tasks[t.id] = t;
      if (t.status !== "downloading" && t.status !== "starting") delete this.live[t.id];
    });
    on("task:removed", (id: string) => {
      delete this.tasks[id];
      delete this.live[id];
      this.order = this.order.filter((x) => x !== id);
      this.selected = this.selected.filter((x) => x !== id);
      if (this.focus === id) this.focus = null;
    });
    on("task:completed", (t: Task) => {
      if (this.settings?.sounds) chime();
      this.toast(`Finished: ${t.name}`, "ok", { label: "Open", run: () => api.open(t.id).catch((e) => this.toast(errText(e), "error")) });
    });
    on("task:failed", (t: Task) => this.toast(`${t.name}: ${t.error ?? "failed"}`, "error"));
    on("tasks:tick", (tick: Tick) => this.onTick(tick));
    on("settings:changed", (s: Settings) => (this.settings = s));
    on("external:links", (e: External) => this.openAdd(e));
    on("clipboard:links", (links: string[]) => {
      if (this.addOpen) return;
      const fresh = links.filter((l) => !this.all.some((t) => t.url === l));
      if (fresh.length) this.clipboard = fresh;
    });
  }

  onTick(tick: Tick) {
    const items: TickItem[] = tick.items ?? [];
    for (const it of items) {
      const t = this.tasks[it.id];
      if (!t) continue;
      t.status = it.status;
      t.done = it.done;
      if (it.size !== 0) t.size = it.size;
      t.speed = it.speed;
      t.upSpeed = it.upSpeed;
      t.eta = it.eta;
      t.conns = it.conns;
      t.seeds = it.seeds;
      if (it.map || it.heads) this.live[it.id] = { map: it.map, heads: it.heads };
    }
    this.speed = tick.speed;
    this.upSpeed = tick.upSpeed;
    this.history = [...this.history.slice(1), tick.speed];
  }

  toast(text: string, tone: Toast["tone"] = "info", action?: Toast["action"]) {
    const id = ++this.#toastId;
    this.toasts = [...this.toasts.slice(-3), { id, text, tone, action }];
    setTimeout(() => this.dismiss(id), tone === "error" ? 8000 : 5000);
  }

  dismiss(id: number) {
    this.toasts = this.toasts.filter((t) => t.id !== id);
  }

  openAdd(seed: External | null = null) {
    this.addSeed = seed;
    this.addOpen = true;
    this.clipboard = null;
  }

  select(id: string, mode: "one" | "toggle" | "range" = "one") {
    if (mode === "toggle") {
      this.selected = this.selected.includes(id) ? this.selected.filter((x) => x !== id) : [...this.selected, id];
    } else if (mode === "range" && this.selected.length) {
      const ids = this.visible.map((t) => t.id);
      const a = ids.indexOf(this.selected[this.selected.length - 1]);
      const b = ids.indexOf(id);
      const [lo, hi] = a < b ? [a, b] : [b, a];
      this.selected = Array.from(new Set([...this.selected, ...ids.slice(lo, hi + 1)]));
    } else {
      this.selected = [id];
    }
    this.focus = id;
  }

  /** Selection, or the focused task when nothing is selected. */
  targets(): string[] {
    if (this.selected.length) return this.selected;
    return this.focus ? [this.focus] : [];
  }

  async run(p: Promise<unknown>) {
    try {
      await p;
    } catch (e) {
      this.toast(errText(e), "error");
    }
  }

  async saveSettings(s: Settings) {
    try {
      this.settings = await api.saveSettings(s);
    } catch (e) {
      this.toast(errText(e), "error");
    }
  }
}

export function catOf(t: Task): string {
  return t.kind === "torrent" ? "torrent" : t.category || "other";
}

export function bucket(t: Task): "active" | "queued" | "paused" | "completed" | "failed" {
  switch (t.status) {
    case "downloading":
    case "starting":
    case "seeding":
      return "active";
    case "queued":
      return "queued";
    case "paused":
      return "paused";
    case "completed":
      return "completed";
    default:
      return "failed";
  }
}

function matches(t: Task, f: Filter): boolean {
  if (f === "all") return true;
  if (f.startsWith("cat:")) return catOf(t) === f.slice(4);
  return bucket(t) === f;
}

export function prog(t: Task): number {
  if (t.status === "completed") return 1;
  return t.size > 0 ? t.done / t.size : 0;
}

let audio: AudioContext | null = null;
/** Two quick rising notes; synthesised so there's no asset to ship. */
function chime() {
  try {
    audio ??= new AudioContext();
    const now = audio.currentTime;
    [660, 990].forEach((f, i) => {
      const o = audio!.createOscillator();
      const g = audio!.createGain();
      o.type = "triangle";
      o.frequency.value = f;
      g.gain.setValueAtTime(0.0001, now + i * 0.09);
      g.gain.exponentialRampToValueAtTime(0.12, now + i * 0.09 + 0.02);
      g.gain.exponentialRampToValueAtTime(0.0001, now + i * 0.09 + 0.28);
      o.connect(g).connect(audio!.destination);
      o.start(now + i * 0.09);
      o.stop(now + i * 0.09 + 0.3);
    });
  } catch {
    /* audio is a nicety */
  }
}

export const store = new Store();
