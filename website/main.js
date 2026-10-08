const root = document.documentElement;
const $ = (s, el = document) => el.querySelector(s);
const $$ = (s, el = document) => [...el.querySelectorAll(s)];
const reduced = matchMedia("(prefers-reduced-motion: reduce)").matches;
const store = (k, v) => { try { localStorage.setItem(k, v); } catch {} };

function syncThemeColor() {
  const meta = $('meta[name="theme-color"]');
  if (meta) meta.content = getComputedStyle(document.body).getPropertyValue("--ib-paper").trim();
}

// ---------------------------------------------------------------- light / dark
const modeBtn = $("#mode");
function syncMode() {
  const dark = root.dataset.theme === "dark";
  modeBtn.setAttribute("aria-pressed", String(dark));
  modeBtn.setAttribute("aria-label", dark ? "Switch to light mode" : "Switch to dark mode");
  syncThemeColor();
}
modeBtn.addEventListener("click", () => {
  const dark = root.dataset.theme !== "dark";
  if (dark) root.dataset.theme = "dark";
  else delete root.dataset.theme;
  store("blister-mode", dark ? "dark" : "light");
  syncMode();
});
syncMode();

// ---------------------------------------------------------------- seeded randomness
// Every simulation on this page is seeded, so it plays out the same way on every visit.
function rng(seed) {
  return () => {
    seed = (seed + 0x6d2b79f5) | 0;
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
const pad2 = (n) => String(n).padStart(2, "0");

// ---------------------------------------------------------------- the segmented bar
// This is how Blister's HTTP engine works: the file is cut into one range per
// connection, every connection pulls its own range, and when one finishes early
// it takes over the back half of whichever range has the most left.
class SegBar {
  constructor(el, { lanes = 16, duration = 8, seed = 7, minSplit = 0.006, onSplit, onDone } = {}) {
    Object.assign(this, { el, lanes, duration, minSplit, onSplit, onDone });
    this.reset(seed);
  }
  reset(seed) {
    this.r = rng(seed);
    this.t = 0;
    this.done = false;
    this.segs = [];
    this.el.textContent = "";
    // base so that `lanes` connections at average speed finish in `duration` seconds
    this.base = 1 / (this.lanes * this.duration);
    for (let i = 0; i < this.lanes; i++) this.add(i / this.lanes, (i + 1) / this.lanes, i + 1);
    this.render();
  }
  add(start, end, id, isNew = false) {
    const lane = document.createElement("span");
    lane.className = "seg__lane" + (isNew ? " is-new" : "");
    const fill = document.createElement("span");
    fill.className = "seg__fill";
    lane.append(fill);
    this.el.append(lane);
    const seg = { start, end, pos: start, id, lane, fill, speed: 0.3 + this.r() * 1.7, phase: this.r() * 6.28, v: 0 };
    this.place(seg);
    this.segs.push(seg);
    if (isNew) setTimeout(() => lane.classList.remove("is-new"), 900);
    return seg;
  }
  place(seg) {
    seg.lane.style.left = seg.start * 100 + "%";
    seg.lane.style.width = (seg.end - seg.start) * 100 + "%";
  }
  get active() { return this.segs.filter((s) => s.pos < s.end); }
  get progress() { return this.segs.reduce((a, s) => a + (s.pos - s.start), 0); }
  get rate() { return this.active.reduce((a, s) => a + s.v, 0); }
  step(dt) {
    if (this.done) return;
    this.t += dt;
    for (const s of this.active) {
      s.v = this.base * s.speed * (1 + 0.3 * Math.sin(this.t * 1.4 + s.phase));
      s.pos = Math.min(s.end, s.pos + s.v * dt);
      if (s.pos >= s.end) this.finish(s);
    }
    if (this.progress >= 0.99999) {
      this.done = true;
      this.segs.forEach((s) => (s.v = 0));
      this.onDone?.();
    }
  }
  finish(s) {
    s.lane.classList.add("is-done");
    s.v = 0;
    // steal half of the biggest remaining range
    let big = null;
    for (const o of this.active) if (!big || o.end - o.pos > big.end - big.pos) big = o;
    if (!big || big.end - big.pos < this.minSplit) return;
    const mid = big.pos + (big.end - big.pos) / 2;
    const oldEnd = big.end;
    big.end = mid;
    this.place(big);
    const n = this.add(mid, oldEnd, s.id, true);
    n.speed = s.speed;
    this.onSplit?.(s.id, big.id);
  }
  render() {
    for (const s of this.segs) {
      s.fill.style.width = ((s.pos - s.start) / (s.end - s.start || 1)) * 100 + "%";
    }
    this.el.setAttribute("aria-valuenow", String(Math.round(this.progress * 100)));
  }
}

// ---------------------------------------------------------------- hero: the HTTP download
const httpFiles = [
  { name: "blender-4.2-windows-x64.zip", mb: 342, folder: "Archives/", label: "ZIP" },
  { name: "texture-library-v3.7z", mb: 418, folder: "Archives/", label: "7Z" },
  { name: "lecture-pack-week-09.mp4", mb: 386, folder: "Videos/", label: "MP4" },
];
let fileIdx = 0;
const segEl = $("#seg");
const segNote = $("#seg-note");
const fmtMB = (mb) => (mb >= 1000 ? (mb / 1000).toFixed(2) + " GB" : Math.round(mb) + " MB");
const fmtEta = (s) => (s < 60 ? Math.ceil(s) + "s" : Math.floor(s / 60) + "m " + pad2(Math.ceil(s % 60)) + "s");
let holdUntil = 0;

const http = new SegBar(segEl, {
  lanes: 16,
  duration: httpFiles[0].mb / 46,
  seed: 11,
  onSplit(free, from) {
    segNote.innerHTML = `Connection ${pad2(free)} finished early and took half of <b>connection ${pad2(from)}</b>'s range.`;
  },
  onDone() {
    const f = httpFiles[fileIdx];
    $("#http-state").textContent = "sha256 verified";
    $("#http-eta").textContent = "done";
    segNote.innerHTML = `Stitched, checked and moved to <b>${f.folder}</b>`;
    holdUntil = clock + 2.6;
  },
});
function loadHttp(i) {
  fileIdx = i % httpFiles.length;
  const f = httpFiles[fileIdx];
  http.duration = f.mb / 46;
  http.reset(11 + fileIdx * 97);
  $("#http-name").textContent = f.name;
  $("#http-size").textContent = fmtMB(f.mb);
  $("#http-state").textContent = "downloading";
  $(".dl--http .dl__icon").dataset.label = f.label;
  segNote.textContent = "Each stripe is one connection, filling its own part of the file.";
}
function paintHttp() {
  http.render();
  const f = httpFiles[fileIdx];
  const mbps = http.rate * f.mb;
  $("#http-pct").textContent = Math.floor(http.progress * 100) + "%";
  $("#http-conns").textContent = (http.done ? 16 : http.active.length) + " connections";
  if (!http.done) $("#http-eta").textContent = mbps > 0 ? fmtEta(((1 - http.progress) * f.mb) / mbps) + " left" : "--";
  return mbps;
}

// ---------------------------------------------------------------- hero: the torrent piece map
const piecesEl = $("#pieces");
const PIECES = 200;
const bt = { r: null, state: [], flight: new Map(), have: 0, ratio: 0, seedFor: 0, peers: 38 };
const cells = [];
for (let i = 0; i < PIECES; i++) cells.push(piecesEl.appendChild(document.createElement("i")));
function resetBt(seed) {
  bt.r = rng(seed);
  bt.state = new Array(PIECES).fill(0);
  bt.flight.clear();
  bt.have = 0;
  bt.ratio = 0;
  bt.seedFor = 0;
  cells.forEach((c) => (c.className = ""));
  $("#bt-state").textContent = "downloading";
}
let btAcc = 0;
function stepBt(dt) {
  btAcc += dt;
  while (btAcc >= 0.07) {
    btAcc -= 0.07;
    if (bt.have < PIECES) {
      // pieces arrive out of order, a handful in flight at once
      while (bt.flight.size < 12 && bt.flight.size + bt.have < PIECES) {
        let i = Math.floor(bt.r() * PIECES);
        while (bt.state[i] !== 0) i = (i + 1) % PIECES;
        bt.state[i] = 1;
        bt.flight.set(i, 3 + Math.floor(bt.r() * 9));
        cells[i].className = "f";
      }
      for (const [i, left] of bt.flight) {
        if (left > 1) { bt.flight.set(i, left - 1); continue; }
        bt.flight.delete(i);
        bt.state[i] = 2;
        bt.have++;
        cells[i].className = "h";
      }
      if (bt.have === PIECES) $("#bt-state").textContent = "seeding to 2.0";
    } else if (bt.ratio < 2) {
      bt.ratio = Math.min(2, bt.ratio + 0.03);
    } else if ((bt.seedFor += 0.07) > 2.5) {
      resetBt(Math.floor(bt.r() * 1e6));
    }
    if (bt.r() < 0.08) bt.peers = Math.max(24, Math.min(52, bt.peers + (bt.r() < 0.5 ? -1 : 1)));
  }
}
function paintBt() {
  $("#bt-pct").textContent = Math.floor((bt.have / PIECES) * 100) + "%";
  $("#bt-ratio").textContent = "ratio " + bt.ratio.toFixed(2);
  $("#bt-peers").textContent = bt.peers + " peers";
  if (bt.ratio >= 2) $("#bt-state").textContent = "ratio reached, stopped";
}
resetBt(23);

// ---------------------------------------------------------------- hero: one clock, pausable
const playBtn = $("#playpause");
let paused = reduced;
let onScreen = true;
let clock = 0;
let last = 0;
let upWobble = 0;
function frame(now) {
  const dt = Math.min(0.1, (now - (last || now)) / 1000);
  last = now;
  if (!paused && onScreen && !document.hidden) {
    clock += dt;
    if (http.done && clock > holdUntil) loadHttp(fileIdx + 1);
    http.step(dt);
    stepBt(dt);
    upWobble += dt;
    paintAll();
  }
  requestAnimationFrame(frame);
}
function paintAll() {
  const down = paintHttp() + (bt.have < PIECES ? 6 + 2 * Math.sin(clock * 0.9) : 0);
  paintBt();
  $("#rate-down").textContent = down.toFixed(1) + " MB/s";
  $("#rate-up").textContent = (1.4 + 0.5 * Math.sin(upWobble * 0.7)).toFixed(1) + " MB/s";
}
function setPaused(p) {
  paused = p;
  playBtn.setAttribute("aria-pressed", String(p));
  segEl.classList.toggle("is-paused", p);
}
playBtn.addEventListener("click", () => setPaused(!paused));
new IntersectionObserver((e) => (onScreen = e[0].isIntersecting)).observe($(".app"));

loadHttp(0);
if (reduced) {
  // a still frame: run the simulation forward to an interesting moment, then hold
  for (let i = 0; i < 200 && http.progress < 0.62; i++) http.step(0.05);
  for (let i = 0; i < 120; i++) stepBt(0.05);
  paintAll();
}
setPaused(paused);
requestAnimationFrame(frame);

// ---------------------------------------------------------------- Ctrl + C: a link gets caught
const caughtSamples = [
  { name: "podcast-episode-112.mp3", meta: "Caught from clipboard · queued · Music/", label: "MP3" },
  { name: "design-systems-handbook.pdf", meta: "Caught from clipboard · queued · Documents/", label: "PDF" },
  { name: "youtube.com/watch?v=… (1080p)", meta: "Video page · yt-dlp picks the best format · Videos/", label: "MP4" },
  { name: "magnet:?xt=urn:btih:… (3 files)", meta: "Magnet link · choose files before it starts", label: "BT" },
];
let caughtIdx = 0;
const keys = $("#keys");
const caught = $("#caught");
$("#catch").addEventListener("click", () => {
  keys.classList.add("is-down");
  setTimeout(() => keys.classList.remove("is-down"), 160);
  const s = caughtSamples[caughtIdx++ % caughtSamples.length];
  $("#caught-name").textContent = s.name;
  $("#caught-meta").textContent = s.meta;
  $(".dl__icon", caught).dataset.label = s.label;
  caught.hidden = false;
  caught.style.animation = "none";
  void caught.offsetWidth;
  caught.style.animation = "";
});
