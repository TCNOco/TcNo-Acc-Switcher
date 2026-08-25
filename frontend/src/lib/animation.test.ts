import { describe, expect, it } from "vitest";

import { staggerDelay } from "./animation";

describe("staggerDelay", () => {
  it("does not delay the first item", () => {
    expect(staggerDelay(0)).toBe(0);
  });

  it("treats a negative index as the first item", () => {
    expect(staggerDelay(-3)).toBe(0);
  });

  it("opens with roughly one base step between the first items", () => {
    const base = 40;
    const first = staggerDelay(1, base, 400);
    expect(first).toBeGreaterThan(base * 0.85);
    expect(first).toBeLessThanOrEqual(base);
  });

  it("never delays anything past the cap", () => {
    for (const index of [20, 100, 5000]) {
      expect(staggerDelay(index, 38, 420)).toBeLessThanOrEqual(420);
    }
  });

  it("never brings a later item forward", () => {
    let previous = -1;
    for (let index = 0; index < 200; index += 1) {
      const delay = staggerDelay(index, 38, 420);
      expect(delay).toBeGreaterThanOrEqual(previous);
      previous = delay;
    }
  });

  it("separates the tiles a person can actually watch arrive", () => {
    // The point of the curve over a linear delay with a hard cap: the old one
    // put everything past item 8 on the same frame. A screenful still has to
    // read as a cascade - only the far tail is allowed to bunch up.
    for (let index = 1; index < 24; index += 1) {
      expect(staggerDelay(index, 38, 420)).toBeGreaterThan(staggerDelay(index - 1, 38, 420));
    }
  });
});
