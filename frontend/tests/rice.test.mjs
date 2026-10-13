import assert from "node:assert/strict";
import test from "node:test";
import { normalize, decodeRice, encodeRice, presets, contrast } from "../src/lib/rice.ts";

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
