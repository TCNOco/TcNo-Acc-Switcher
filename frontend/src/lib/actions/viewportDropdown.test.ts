import { describe, expect, it } from "vitest";
import { applyViewportDropdownLayout, computeViewportDropdownLayout } from "./viewportDropdown";

describe("viewport dropdown layout", () => {
  it("opens above a trigger near the bottom of the viewport", () => {
    expect(
      computeViewportDropdownLayout(
        { top: 540, bottom: 578 },
        { viewportHeight: 600, menuHeight: 360 },
      ),
    ).toEqual({ placement: "above", maxHeight: 360 });
  });

  it("uses the larger side and caps a tall menu to the viewport", () => {
    expect(
      computeViewportDropdownLayout(
        { top: 280, bottom: 318 },
        { viewportHeight: 500, menuHeight: 400 },
      ),
    ).toEqual({ placement: "above", maxHeight: 272 });
  });

  it("keeps a menu below when it fits there", () => {
    expect(
      computeViewportDropdownLayout(
        { top: 80, bottom: 118 },
        { viewportHeight: 600, menuHeight: 180 },
      ),
    ).toEqual({ placement: "below", maxHeight: 360 });
  });

  it("keeps runtime placement authoritative over theme styles", () => {
    const declarations: Array<[string, string, string]> = [];
    const style = {
      setProperty(name: string, value: string, priority = "") {
        declarations.push([name, value, priority]);
      },
    };

    applyViewportDropdownLayout(style, { placement: "above", maxHeight: 272 });

    expect(declarations).toEqual([
      ["top", "auto", "important"],
      ["bottom", "100%", "important"],
      ["max-height", "272px", ""],
    ]);

    declarations.length = 0;
    applyViewportDropdownLayout(style, { placement: "below", maxHeight: 360 });

    expect(declarations).toEqual([
      ["top", "100%", "important"],
      ["bottom", "auto", "important"],
      ["max-height", "360px", ""],
    ]);
  });
});
