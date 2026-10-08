// The rice engine. A Rice is Blister's entire look as data: colours, shape,
// type, layout, progress bars, effects and raw CSS. It applies live, saves to
// settings, and travels as a single "blister-rice:" string.

export type ShadowStyle = "extrude" | "hard" | "soft" | "glow" | "none";
export type BarStyle = "lanes" | "solid" | "blocks" | "ascii" | "braille" | "line" | "dots";
export type View = "list" | "table" | "grid";

export interface Colors {
  paper: string;
  paper2: string;
  card: string;
  text: string;
  muted: string;
  faint: string;
  line: string;
  ex: string;
  accent: string;
  accentInk: string;
  hot: string;
  hotInk: string;
  alt: string;
  ok: string;
  bad: string;
}

export interface Rice {
  v: 1;
  name: string;
  author?: string;
  colors: Colors;
  shape: { border: number; radius: number; depth: number; shadow: ShadowStyle };
  type: { ui: string; mono: string; scale: number; weight: number; tracking: number; monoUI: boolean };
  layout: {
    view: View;
    density: "compact" | "comfortable" | "cozy";
    sidebar: "left" | "right" | "hidden";
    detail: "right" | "bottom" | "off";
    rows: "lines" | "cards";
  };
  bar: { style: BarStyle; height: number; heads: boolean; stripes: boolean };
  fx: {
    motion: number; // 0 off, 1 normal, 2 springy
    grain: number; // 0..1
    scanlines: number; // 0..1
    vignette: number; // 0..1
    wallpaper: string; // data: or https: URL
    dim: number; // 0..1 darkening over the wallpaper
    blur: number; // px
    opacity: number; // 0..1 surface opacity (for wallpapers and window backdrops)
  };
  css: string;
}

const base: Omit<Rice, "name" | "colors"> = {
  v: 1,
  shape: { border: 2.5, radius: 11, depth: 6, shadow: "extrude" },
  type: { ui: "Geist Variable", mono: "Geist Mono Variable", scale: 1, weight: 900, tracking: -0.05, monoUI: false },
  layout: { view: "list", density: "comfortable", sidebar: "left", detail: "right", rows: "lines" },
  bar: { style: "lanes", height: 14, heads: true, stripes: true },
  fx: { motion: 1, grain: 0, scanlines: 0, vignette: 0, wallpaper: "", dim: 0.55, blur: 0, opacity: 1 },
  css: "",
};

type Patch = { name: string; colors: Colors } & {
  shape?: Partial<Rice["shape"]>;
  type?: Partial<Rice["type"]>;
  layout?: Partial<Rice["layout"]>;
  bar?: Partial<Rice["bar"]>;
  fx?: Partial<Rice["fx"]>;
  css?: string;
};

function make(p: Patch): Rice {
  return {
    ...structuredClone(base),
    name: p.name,
    colors: p.colors,
    shape: { ...base.shape, ...p.shape },
    type: { ...base.type, ...p.type },
    layout: { ...base.layout, ...p.layout },
    bar: { ...base.bar, ...p.bar },
    fx: { ...base.fx, ...p.fx },
    css: p.css ?? "",
  };
}

export const presets: Rice[] = [
  make({
    name: "Thermal",
    colors: {
      paper: "#0d0913", paper2: "#150f1e", card: "#1c1527", text: "#f3eefb", muted: "#b9afc8", faint: "#8b819b",
      line: "#f3eefb", ex: "#f3eefb", accent: "#ffd60a", accentInk: "#140d1f", hot: "#ff2e88", hotInk: "#ffffff",
      alt: "#6c4dff", ok: "#2fd27a", bad: "#ff4d6d",
    },
  }),
  make({
    name: "Lilac Paper",
    colors: {
      paper: "#f0edf6", paper2: "#e4dfee", card: "#fbfaff", text: "#140d1f", muted: "#4a4158", faint: "#6b6279",
      line: "#140d1f", ex: "#140d1f", accent: "#ffd60a", accentInk: "#140d1f", hot: "#ff2e88", hotInk: "#ffffff",
      alt: "#6c4dff", ok: "#14a85a", bad: "#e0234a",
    },
  }),
  make({
    name: "Phosphor",
    colors: {
      paper: "#040904", paper2: "#081208", card: "#0b170b", text: "#8dffa6", muted: "#5fd07a", faint: "#358a4b",
      line: "#5fd07a", ex: "#1f5a2c", accent: "#8dffa6", accentInk: "#031a08", hot: "#ffcc33", hotInk: "#1a1200",
      alt: "#33ccff", ok: "#8dffa6", bad: "#ff5f56",
    },
    shape: { border: 1.5, radius: 0, depth: 0, shadow: "none" },
    type: { ui: "Geist Mono Variable", monoUI: true, weight: 700, tracking: 0 },
    layout: { view: "table", density: "compact" },
    bar: { style: "ascii", heads: false, stripes: false },
    fx: { scanlines: 0.5, grain: 0.18, vignette: 0.5 },
  }),
  make({
    name: "Catppuccin Mocha",
    colors: {
      paper: "#1e1e2e", paper2: "#181825", card: "#313244", text: "#cdd6f4", muted: "#bac2de", faint: "#7f849c",
      line: "#585b70", ex: "#11111b", accent: "#cba6f7", accentInk: "#1e1e2e", hot: "#f5c2e7", hotInk: "#1e1e2e",
      alt: "#89b4fa", ok: "#a6e3a1", bad: "#f38ba8",
    },
    shape: { border: 1.5, radius: 14, depth: 6, shadow: "soft" },
    type: { weight: 800, tracking: -0.03 },
    layout: { rows: "cards" },
    bar: { style: "solid", height: 8, stripes: false },
  }),
  make({
    name: "Gruvbox",
    colors: {
      paper: "#282828", paper2: "#1d2021", card: "#32302f", text: "#ebdbb2", muted: "#d5c4a1", faint: "#928374",
      line: "#ebdbb2", ex: "#1d2021", accent: "#fabd2f", accentInk: "#282828", hot: "#fb4934", hotInk: "#282828",
      alt: "#83a598", ok: "#b8bb26", bad: "#fb4934",
    },
    shape: { border: 2, radius: 3, depth: 4, shadow: "hard" },
    bar: { style: "blocks" },
  }),
  make({
    name: "Nord",
    colors: {
      paper: "#2e3440", paper2: "#3b4252", card: "#3b4252", text: "#eceff4", muted: "#d8dee9", faint: "#7b88a1",
      line: "#4c566a", ex: "#242933", accent: "#88c0d0", accentInk: "#2e3440", hot: "#b48ead", hotInk: "#2e3440",
      alt: "#81a1c1", ok: "#a3be8c", bad: "#bf616a",
    },
    shape: { border: 1.5, radius: 10, depth: 0, shadow: "none" },
    type: { weight: 700, tracking: -0.02 },
    bar: { style: "line", height: 6, heads: false, stripes: false },
  }),
  make({
    name: "Tokyo Night",
    colors: {
      paper: "#1a1b26", paper2: "#16161e", card: "#24283b", text: "#c0caf5", muted: "#a9b1d6", faint: "#565f89",
      line: "#414868", ex: "#0f0f14", accent: "#7aa2f7", accentInk: "#1a1b26", hot: "#ff9e64", hotInk: "#1a1b26",
      alt: "#bb9af7", ok: "#9ece6a", bad: "#f7768e",
    },
    shape: { border: 1.5, radius: 12, depth: 8, shadow: "glow" },
    layout: { rows: "cards" },
    bar: { style: "lanes", height: 10 },
  }),
  make({
    name: "Rosé Pine",
    colors: {
      paper: "#191724", paper2: "#1f1d2e", card: "#26233a", text: "#e0def4", muted: "#908caa", faint: "#6e6a86",
      line: "#403d52", ex: "#12101b", accent: "#ebbcba", accentInk: "#191724", hot: "#eb6f92", hotInk: "#191724",
      alt: "#c4a7e7", ok: "#9ccfd8", bad: "#eb6f92",
    },
    shape: { border: 1.5, radius: 16, depth: 5, shadow: "soft" },
    type: { weight: 700, tracking: -0.025 },
    layout: { view: "grid", rows: "cards" },
    bar: { style: "dots", stripes: false },
  }),
  make({
    name: "Dracula",
    colors: {
      paper: "#282a36", paper2: "#21222c", card: "#343746", text: "#f8f8f2", muted: "#d6d6e0", faint: "#6272a4",
      line: "#f8f8f2", ex: "#191a21", accent: "#bd93f9", accentInk: "#282a36", hot: "#ff79c6", hotInk: "#282a36",
      alt: "#8be9fd", ok: "#50fa7b", bad: "#ff5555",
    },
    shape: { border: 2.5, radius: 8, depth: 6, shadow: "extrude" },
    bar: { style: "braille" },
  }),
  make({
    name: "Everforest",
    colors: {
      paper: "#2d353b", paper2: "#232a2e", card: "#343f44", text: "#d3c6aa", muted: "#9da9a0", faint: "#7a8478",
      line: "#475258", ex: "#1e2326", accent: "#a7c080", accentInk: "#2d353b", hot: "#e69875", hotInk: "#2d353b",
      alt: "#7fbbb3", ok: "#a7c080", bad: "#e67e80",
    },
    shape: { border: 1.5, radius: 12, depth: 3, shadow: "soft" },
    type: { weight: 700, tracking: -0.02 },
    layout: { density: "cozy" },
    bar: { style: "solid", height: 10, stripes: false },
  }),
  make({
    name: "Neon '84",
    colors: {
      paper: "#1a1027", paper2: "#140b20", card: "#241634", text: "#fdf1ff", muted: "#d4b8e8", faint: "#8a6aa8",
      line: "#ff7edb", ex: "#36f9f6", accent: "#ff7edb", accentInk: "#1a1027", hot: "#fede5d", hotInk: "#1a1027",
      alt: "#36f9f6", ok: "#72f1b8", bad: "#fe4450",
    },
    shape: { border: 2, radius: 6, depth: 10, shadow: "glow" },
    type: { weight: 900, tracking: -0.04 },
    bar: { style: "lanes", height: 12 },
    fx: { scanlines: 0.25, vignette: 0.6 },
  }),
  make({
    name: "Mono",
    colors: {
      paper: "#000000", paper2: "#0b0b0b", card: "#111111", text: "#ffffff", muted: "#bdbdbd", faint: "#7a7a7a",
      line: "#ffffff", ex: "#ffffff", accent: "#ffffff", accentInk: "#000000", hot: "#ff0040", hotInk: "#ffffff",
      alt: "#9a9a9a", ok: "#ffffff", bad: "#ff0040",
    },
    shape: { border: 2, radius: 0, depth: 8, shadow: "extrude" },
    type: { weight: 900, tracking: -0.06 },
    bar: { style: "blocks", heads: false, stripes: false },
  }),
  make({
    name: "Solar Paper",
    colors: {
      paper: "#fdf6e3", paper2: "#eee8d5", card: "#fffbf0", text: "#073642", muted: "#586e75", faint: "#93a1a1",
      line: "#073642", ex: "#073642", accent: "#b58900", accentInk: "#fdf6e3", hot: "#d33682", hotInk: "#fdf6e3",
      alt: "#268bd2", ok: "#859900", bad: "#dc322f",
    },
    shape: { border: 2, radius: 6, depth: 4, shadow: "hard" },
    bar: { style: "solid", stripes: false },
  }),
];

export const defaultRice = presets[0];

// --------------------------------------------------------------- apply

const exSteps = [2, 3, 4, 6, 8, 10, 14];

function stack(n: number, style: ShadowStyle): string {
  if (n <= 0 || style === "none") return "none";
  switch (style) {
    case "hard":
      return `${n}px ${n}px 0 var(--ib-ex)`;
    case "soft":
      return `0 ${Math.round(n / 2)}px ${n * 2.5}px color-mix(in srgb, var(--ib-ex) 32%, transparent)`;
    case "glow":
      return `0 0 ${Math.round(n * 1.6)}px color-mix(in srgb, var(--ib-accent) 50%, transparent)`;
  }
  const parts: string[] = [];
  for (let i = 1; i <= n; i++) parts.push(`${i}px ${i}px 0 var(--ib-ex)`);
  return parts.join(", ");
}

const fontStack = (f: string, fallback: string) => (f ? `"${f.replace(/"/g, "")}", ${fallback}` : fallback);

let styleEl: HTMLStyleElement | null = null;

/** Paints a rice onto the document. Cheap enough to call on every slider tick. */
export function applyRice(r: Rice, el: HTMLElement = document.body) {
  const c = r.colors;
  const set = (k: string, v: string) => el.style.setProperty(k, v);
  // Surfaces thin out over a wallpaper or a Mica/Acrylic window.
  const op = r.fx.opacity;
  const see = (col: string, o: number) => (o >= 1 ? col : `color-mix(in srgb, ${col} ${Math.round(Math.min(1, o) * 100)}%, transparent)`);
  set("--ib-paper", see(c.paper, op));
  set("--ib-paper-2", see(c.paper2, op + 0.1));
  set("--ib-card", see(c.card, op + 0.2));
  set("--paper-solid", c.paper);
  set("--ib-night", c.paper2);
  set("--ib-text", c.text);
  set("--ib-muted", c.muted);
  set("--ib-faint", c.faint);
  set("--ib-line", c.line);
  set("--ib-ex", c.ex);
  set("--ib-accent", c.accent);
  set("--ib-accent-ink", c.accentInk);
  set("--ib-hot", c.hot);
  set("--ib-hot-ink", c.hotInk);
  set("--ib-alt", c.alt);
  set("--ok", c.ok);
  set("--bad", c.bad);

  const s = r.shape;
  set("--bw", `${s.border}px`);
  set("--bw-lg", `${s.border === 0 ? 0 : s.border + 0.5}px`);
  set("--r", `${s.radius}px`);
  set("--r-sm", `${Math.round(s.radius * 0.72)}px`);
  set("--r-xs", `${Math.round(s.radius * 0.5)}px`);
  set("--r-lg", `${Math.round(s.radius * 1.45)}px`);
  for (const n of exSteps) set(`--ib-ex${n}`, stack(Math.round((n * s.depth) / 6), s.shadow));
  set("--press", s.depth === 0 || s.shadow === "none" || s.shadow === "glow" ? "0px" : `${Math.max(1, Math.round(s.depth / 2))}px`);

  const t = r.type;
  set("--ib-font", fontStack(t.monoUI ? t.mono : t.ui, "system-ui, sans-serif"));
  set("--ib-mono", fontStack(t.mono, "ui-monospace, monospace"));
  set("--zoom", String(t.scale));
  set("--h-weight", String(t.weight));
  set("--h-track", `${t.tracking}em`);

  const L = r.layout;
  el.dataset.view = L.view;
  el.dataset.density = L.density;
  el.dataset.sidebar = L.sidebar;
  el.dataset.detail = L.detail;
  el.dataset.rows = L.rows;

  el.dataset.bar = r.bar.style;
  set("--bar-h", `${r.bar.height}px`);
  el.dataset.heads = String(r.bar.heads);
  el.dataset.stripes = String(r.bar.stripes);

  const f = r.fx;
  el.dataset.motion = String(f.motion);
  set("--motion", f.motion === 0 ? "0" : f.motion === 2 ? "1.35" : "1");
  set("--grain", String(f.grain));
  set("--scan", String(f.scanlines));
  set("--vignette", String(f.vignette));
  set("--wall", f.wallpaper ? `url("${f.wallpaper}")` : "none");
  set("--wall-dim", String(f.dim));
  set("--wall-blur", `${f.blur}px`);
  set("--surface", `${Math.round(f.opacity * 100)}%`);
  el.dataset.wall = f.wallpaper ? "on" : "off";

  styleEl ??= Object.assign(document.createElement("style"), { id: "rice-css" });
  if (!styleEl.isConnected) document.head.appendChild(styleEl);
  styleEl.textContent = r.css;

  // Light or dark, for native widgets (scrollbars, pickers).
  document.documentElement.style.colorScheme = luminance(c.paper) > 0.5 ? "light" : "dark";
}

// --------------------------------------------------------------- share codes

const PREFIX = "blister-rice:";

export function encodeRice(r: Rice, withWallpaper = false): string {
  const copy: Rice = structuredClone(r);
  if (!withWallpaper && copy.fx.wallpaper.startsWith("data:")) copy.fx.wallpaper = "";
  const json = JSON.stringify(copy);
  return PREFIX + btoa(unescape(encodeURIComponent(json)));
}

/** Accepts a share code or raw JSON; fills anything missing from the defaults. */
export function decodeRice(text: string): Rice {
  let raw = text.trim();
  if (raw.startsWith(PREFIX)) raw = decodeURIComponent(escape(atob(raw.slice(PREFIX.length))));
  return normalize(JSON.parse(raw));
}

export function normalize(x: Partial<Rice> | null | undefined): Rice {
  const d = structuredClone(defaultRice);
  if (!x || typeof x !== "object") return d;
  return {
    v: 1,
    name: String(x.name ?? "Untitled"),
    author: x.author,
    colors: { ...d.colors, ...x.colors },
    shape: { ...d.shape, ...x.shape },
    type: { ...d.type, ...x.type },
    layout: { ...d.layout, ...x.layout },
    bar: { ...d.bar, ...x.bar },
    fx: { ...d.fx, ...x.fx },
    css: typeof x.css === "string" ? x.css : "",
  };
}

// --------------------------------------------------------------- colour helpers

export function hexToRgb(h: string): [number, number, number] {
  const m = /^#?([\da-f]{2})([\da-f]{2})([\da-f]{2})/i.exec(h);
  return m ? [parseInt(m[1], 16), parseInt(m[2], 16), parseInt(m[3], 16)] : [0, 0, 0];
}

export function luminance(h: string): number {
  const [r, g, b] = hexToRgb(h).map((v) => {
    const s = v / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

export function contrast(a: string, b: string): number {
  const [x, y] = [luminance(a), luminance(b)].sort((p, q) => q - p);
  return (x + 0.05) / (y + 0.05);
}

function hsl(h: number, s: number, l: number): string {
  h = ((h % 360) + 360) % 360;
  s /= 100;
  l /= 100;
  const k = (n: number) => (n + h / 30) % 12;
  const a = s * Math.min(l, 1 - l);
  const f = (n: number) => l - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)));
  return "#" + [f(0), f(8), f(4)].map((v) => Math.round(v * 255).toString(16).padStart(2, "0")).join("");
}

/** Ink that reads on a fill: whichever of the two candidates contrasts more. */
export function inkFor(fill: string, dark: string, light: string): string {
  return contrast(fill, dark) >= contrast(fill, light) ? dark : light;
}

/** A fresh, readable palette from a random hue, plus a random shape. */
export function shuffle(current: Rice): Rice {
  const h = Math.floor(Math.random() * 360);
  const light = Math.random() < 0.25;
  const harmony = [150, 180, 210, 120, 60][Math.floor(Math.random() * 5)];
  const sat = 18 + Math.random() * 22;
  const paper = light ? hsl(h, sat, 95) : hsl(h, sat, 7);
  const paper2 = light ? hsl(h, sat, 89) : hsl(h, sat, 10);
  const card = light ? hsl(h, sat, 98) : hsl(h, sat, 13);
  const text = light ? hsl(h, 40, 10) : hsl(h, 30, 94);
  const accent = hsl(h + harmony, 90, light ? 52 : 62);
  const hot = hsl(h + harmony + 120, 92, 60);
  const shadows: ShadowStyle[] = ["extrude", "hard", "soft", "glow", "none"];
  const bars: BarStyle[] = ["lanes", "solid", "blocks", "ascii", "braille", "line", "dots"];
  const pick = <T,>(a: T[]) => a[Math.floor(Math.random() * a.length)];
  return {
    ...structuredClone(current),
    name: "Shuffled",
    colors: {
      paper, paper2, card, text,
      muted: light ? hsl(h, 18, 32) : hsl(h, 18, 74),
      faint: light ? hsl(h, 12, 48) : hsl(h, 12, 54),
      line: text,
      ex: text,
      accent,
      accentInk: inkFor(accent, paper, text),
      hot,
      hotInk: inkFor(hot, "#000000", "#ffffff"),
      alt: hsl(h + harmony - 90, 70, 64),
      ok: "#2fd27a",
      bad: "#ff4d6d",
    },
    shape: {
      border: pick([1.5, 2, 2.5, 3]),
      radius: pick([0, 4, 8, 12, 16, 20]),
      depth: pick([0, 3, 6, 8, 10]),
      shadow: pick(shadows),
    },
    bar: { ...current.bar, style: pick(bars) },
  };
}
