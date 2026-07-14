import { describe, expect, it } from "vitest";
import {
  backgroundObjectPosition,
  normalizeBackgroundAlignment,
  normalizeBackgroundFit,
} from "./backgroundDisplay";

describe("background display settings", () => {
  it("defaults missing and invalid values to center cover", () => {
    expect(normalizeBackgroundAlignment(undefined)).toBe("center");
    expect(normalizeBackgroundAlignment("diagonal")).toBe("center");
    expect(normalizeBackgroundFit(undefined)).toBe("cover");
    expect(normalizeBackgroundFit("crop")).toBe("cover");
  });

  it("maps the five alignment options to object positions", () => {
    expect(backgroundObjectPosition("center")).toBe("center center");
    expect(backgroundObjectPosition("left")).toBe("left center");
    expect(backgroundObjectPosition("right")).toBe("right center");
    expect(backgroundObjectPosition("top")).toBe("center top");
    expect(backgroundObjectPosition("bottom")).toBe("center bottom");
  });

  it("preserves every supported image fit mode", () => {
    for (const fit of ["cover", "contain", "fill", "none", "scale-down"] as const) {
      expect(normalizeBackgroundFit(fit)).toBe(fit);
    }
  });
});
