import assert from "node:assert/strict";
import test from "node:test";
import { normalize, decodeRice, encodeRice, presets, contrast, restoreRice, defaultRice } from "../src/lib/rice.ts";

test("imported themes keep numeric settings, fonts and colors usable", () => {
  const bad = normalize({ colors: { text: "broken" }, type: { scale: 0, ui: 42 }, layout: { view: "oops" }, fx: { opacity: -8, wallpaper: 42 }, shape: { depth: Infinity } });
  assert.ok(bad.type.scale >= 0.8);
  assert.equal(bad.type.ui, presets[0].type.ui);
  assert.equal(bad.colors.text, presets[0].colors.text);
  assert.equal(bad.layout.view, "list");
  assert.equal(bad.fx.wallpaper, "");
  assert.ok(bad.fx.opacity >= 0.3);
  assert.ok(Number.isFinite(bad.shape.depth));
});

test("every preset survives sharing without losing its settings", () => {
  for (const p of presets) assert.deepEqual(decodeRice(encodeRice(p, true)), normalize(p));
});

test("invalid or unsupported theme codes are rejected", () => {
  for (const raw of ["[]", "null", "{}", '{"v":2,"colors":{}}', "blister-rice:invalid"]) assert.throws(() => decodeRice(raw));
});

test("Readable uses AA text contrast on every surface and disables motion", () => {
  const p = presets.find((p) => p.name === "Readable");
  for (const bg of [p.colors.paper, p.colors.paper2, p.colors.card]) {
    for (const fg of [p.colors.text, p.colors.muted, p.colors.faint]) assert.ok(contrast(fg, bg) >= 4.5);
  }
  assert.ok(contrast(p.colors.accentInk, p.colors.accent) >= 4.5);
  assert.ok(contrast(p.colors.hotInk, p.colors.hot) >= 4.5);
  assert.equal(p.fx.motion, 0);
});

test("desktop upgrade replaces bundled legacy rice and preserves personal changes", () => {
  const legacy = structuredClone(presets.find((p) => p.name === "Solar Paper"));
  delete legacy.chrome;
  delete legacy.layout.gap;
  delete legacy.layout.sidebarWidth;
  delete legacy.layout.detailWidth;
  delete legacy.bar.animation;
  assert.deepEqual(restoreRice(JSON.stringify(legacy)), defaultRice);
  legacy.colors.accent = "#123456";
  assert.equal(restoreRice(JSON.stringify(legacy)).colors.accent, "#123456");
});

test("desktop layout imports respect useful panel sizes and animation choices", () => {
  const r = normalize({ layout: { gap: -20, sidebarWidth: 999, detailWidth: 2 }, bar: { animation: "invalid" } });
  assert.equal(r.layout.gap, 0);
  assert.equal(r.layout.sidebarWidth, 320);
  assert.equal(r.layout.detailWidth, 280);
  assert.equal(r.bar.animation, "flow");
});

test("new desktop presets provide AA body and secondary text contrast", () => {
  for (const p of presets.slice(0, 3)) {
    for (const bg of [p.colors.paper, p.colors.paper2, p.colors.card]) {
      for (const fg of [p.colors.text, p.colors.muted]) assert.ok(contrast(fg, bg) >= 4.5, `${p.name}: ${fg} on ${bg}`);
    }
  }
});
