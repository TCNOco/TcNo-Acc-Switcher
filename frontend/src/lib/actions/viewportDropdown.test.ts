import { describe, expect, it } from "vitest";
import { applyViewportDropdownLayout, computeViewportDropdownLayout } from "./viewportDropdown";

const VIEWPORT = { viewportHeight: 600, viewportWidth: 1000 };

describe("viewport dropdown layout", () => {
  it("keeps a menu below when it fits there, and asks for no scroll", () => {
    expect(
      computeViewportDropdownLayout(
        { top: 80, bottom: 118, left: 20 },
        { ...VIEWPORT, menuHeight: 180, menuWidth: 200 },
      ),
    ).toEqual({ maxHeight: 180, shift: 0, scrollBy: 0 });
  });

  it("stays below a trigger near the bottom and asks to scroll it into room", () => {
    const layout = computeViewportDropdownLayout(
      { top: 540, bottom: 578, left: 20 },
      { ...VIEWPORT, menuHeight: 360, menuWidth: 200 },
    );

    expect(layout.scrollBy).toBe(346);
    // 14px of real space is unusable, so it takes the floor and the scroll fixes it.
    expect(layout.maxHeight).toBe(144);
  });

  it("caps a tall menu to a share of the viewport rather than the full list", () => {
    const layout = computeViewportDropdownLayout(
      { top: 80, bottom: 118, left: 20 },
      { ...VIEWPORT, menuHeight: 900, menuWidth: 200 },
    );

    expect(layout.maxHeight).toBe(360);
  });

  it("pulls a menu back from the right edge", () => {
    const layout = computeViewportDropdownLayout(
      { top: 80, bottom: 118, left: 900 },
      { ...VIEWPORT, menuHeight: 180, menuWidth: 220 },
    );

    expect(layout.shift).toBe(-128);
  });

  it("never pushes a menu off the left edge to save the right", () => {
    const layout = computeViewportDropdownLayout(
      { top: 80, bottom: 118, left: 12 },
      { ...VIEWPORT, menuHeight: 180, menuWidth: 1200 },
    );

    expect(layout.shift).toBe(-4);
  });

  it("keeps runtime placement authoritative over theme styles", () => {
    const declarations: Array<[string, string, string]> = [];
    const style = {
      setProperty(name: string, value: string, priority = "") {
        declarations.push([name, value, priority]);
      },
    };

    applyViewportDropdownLayout(style, { maxHeight: 272, shift: -40, scrollBy: 0 });

    expect(declarations).toEqual([
      ["top", "100%", "important"],
      ["bottom", "auto", "important"],
      ["left", "-40px", "important"],
      ["max-height", "272px", ""],
    ]);
  });
});
