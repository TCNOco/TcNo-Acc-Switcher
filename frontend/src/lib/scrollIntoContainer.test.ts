import { describe, expect, it } from "vitest";
import { scrollDeltaIntoView } from "./scrollIntoContainer";

const view = { top: 100, bottom: 400 };

describe("scrollDeltaIntoView", () => {
  it("leaves a row that is already in sight alone", () => {
    expect(scrollDeltaIntoView({ top: 200, bottom: 220 }, view)).toBe(0);
  });

  it("treats a row inside the margin as out of sight", () => {
    expect(scrollDeltaIntoView({ top: 104, bottom: 124 }, view)).toBe(-4);
    expect(scrollDeltaIntoView({ top: 376, bottom: 396 }, view)).toBe(4);
  });

  it("scrolls up by the smallest amount that reveals a row above", () => {
    expect(scrollDeltaIntoView({ top: 40, bottom: 60 }, view)).toBe(-68);
  });

  it("scrolls down by the smallest amount that reveals a row below", () => {
    expect(scrollDeltaIntoView({ top: 500, bottom: 520 }, view)).toBe(128);
  });

  it("shows the top of a row taller than the container", () => {
    expect(scrollDeltaIntoView({ top: 50, bottom: 900 }, view)).toBe(-58);
  });

  it("honours a margin of zero", () => {
    expect(scrollDeltaIntoView({ top: 400, bottom: 420 }, view, 0)).toBe(20);
    expect(scrollDeltaIntoView({ top: 100, bottom: 120 }, view, 0)).toBe(0);
  });
});
