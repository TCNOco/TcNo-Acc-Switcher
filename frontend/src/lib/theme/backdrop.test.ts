import { describe, it, expect } from "vitest";
import { compositeLuma, resolveBackdropInk, LIGHT_INK, DARK_INK, BUSY_SPREAD } from "./backdrop";

const uniform = (mean: number) => ({ measured: true, mean, low: mean, high: mean });

describe("compositeLuma", () => {
  it("blends the image with the surface behind it", () => {
    expect(compositeLuma(1, 0.5, 0)).toBeCloseTo(0.5, 5);
    expect(compositeLuma(0, 0.25, 0.8)).toBeCloseTo(0.6, 5);
  });

  it("treats a fully opaque image as the only thing that matters", () => {
    expect(compositeLuma(0.2, 1, 0.9)).toBeCloseTo(0.2, 5);
  });

  it("clamps opacity rather than extrapolating past the endpoints", () => {
    expect(compositeLuma(1, 5, 0)).toBeCloseTo(1, 5);
    expect(compositeLuma(1, -3, 0.4)).toBeCloseTo(0.4, 5);
  });
});

describe("resolveBackdropInk", () => {
  it("declines to guess when nobody has measured the background", () => {
    expect(resolveBackdropInk(null, 1, 0.1)).toBeNull();
    expect(resolveBackdropInk(undefined, 1, 0.1)).toBeNull();
    expect(resolveBackdropInk({ measured: false }, 1, 0.1)).toBeNull();
    // Measured-but-empty is still not something to act on.
    expect(resolveBackdropInk({ measured: true }, 1, 0.1)).toBeNull();
  });

  it("puts light text on a dark backdrop and dark text on a light one", () => {
    const dark = resolveBackdropInk(uniform(0.02), 1, 0.02);
    expect(dark?.ink).toBe(LIGHT_INK);
    expect(dark?.polarity).toBe("light");
    expect(dark?.busy).toBe(false);

    const light = resolveBackdropInk(uniform(0.95), 1, 0.95);
    expect(light?.ink).toBe(DARK_INK);
    expect(light?.polarity).toBe("dark");
    expect(light?.busy).toBe(false);
  });

  it("distinguishes a pure black background from an unmeasured one", () => {
    const black = resolveBackdropInk({ measured: true, mean: 0, low: 0, high: 0 }, 1, 0);
    expect(black).not.toBeNull();
    expect(black?.ink).toBe(LIGHT_INK);
  });

  // The case a naive average gets wrong: mean says mid-grey, but half the picture
  // is black and half is white, so neither ink is readable across it.
  it("flags a high-contrast image as busy instead of trusting its average", () => {
    const split = resolveBackdropInk({ measured: true, mean: 0.5, low: 0, high: 1 }, 1, 0.5);
    expect(split?.busy).toBe(true);
  });

  // Around luminance 0.18 both inks land near 4:1 — the backdrop is perfectly
  // flat, so nothing is "busy" about it, yet neither colour is actually legible.
  // Spread alone would miss this, which is why contrast is checked too.
  it("flags a backdrop as busy when neither ink clears the contrast bar", () => {
    const deadZone = resolveBackdropInk(uniform(0.18), 1, 0.18);
    expect(deadZone?.spread).toBeLessThan(BUSY_SPREAD);
    expect(deadZone?.busy).toBe(true);
  });

  it("is not busy once the backdrop is clearly light or clearly dark", () => {
    expect(resolveBackdropInk(uniform(0.35), 1, 0.35)?.busy).toBe(false);
    expect(resolveBackdropInk(uniform(0.05), 1, 0.05)?.busy).toBe(false);
  });

  it("dilutes the image's variation by the same opacity that dilutes its brightness", () => {
    const wide = { measured: true, mean: 0.5, low: 0, high: 1 };
    expect(resolveBackdropInk(wide, 1, 0)?.spread).toBeCloseTo(1, 5);
    expect(resolveBackdropInk(wide, 0.25, 0)?.spread).toBeCloseTo(0.25, 5);
    // Faded far enough over a dark theme, the picture stops driving the decision.
    expect(resolveBackdropInk(wide, 0.2, 0)?.busy).toBe(false);
  });

  it("lets the theme colour behind a faint image decide the polarity", () => {
    // A bright image at low opacity over a dark theme still reads as dark.
    const faint = resolveBackdropInk(uniform(0.9), 0.15, 0.03);
    expect(faint?.ink).toBe(LIGHT_INK);
  });
});
