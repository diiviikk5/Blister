import assert from "node:assert/strict";
import test from "node:test";
import { coverageCells } from "../src/lib/coverage.ts";

test("coverage preserves sparse received pieces instead of implying sequential progress", () => {
  assert.deepEqual(coverageCells("9090", 0.5, false, 4), [9, 0, 9, 0]);
  assert.deepEqual(coverageCells("9090", 0.5, false, 2), [5, 5]);
  assert.deepEqual(coverageCells("09", 0.5, false, 4), [0, 0, 9, 9]);
});

test("missing or invalid coverage safely falls back to bounded progress", () => {
  assert.deepEqual(coverageCells("invalid", 0.375, false, 4), [9, 5, 0, 0]);
  assert.deepEqual(coverageCells("", NaN, false, 4), [0, 0, 0, 0]);
  assert.deepEqual(coverageCells("", 2, false, 4), [9, 9, 9, 9]);
  assert.deepEqual(coverageCells("0000", 0, true, 4), [9, 9, 9, 9]);
  assert.equal(coverageCells("", 0, false, 999).length, 320);
});
