import { describe, expect, it } from "vitest";
import { submenuTopOffset } from "./contextMenuLayout";

describe("context menu submenu layout", () => {
  it("keeps a short submenu beside its parent row", () => {
    expect(
      submenuTopOffset({
        naturalTop: 320,
        naturalBottom: 400,
        viewportHeight: 900,
        padding: 8,
      }),
    ).toBe(0);
  });

  it("moves a submenu up only by its viewport overflow", () => {
    expect(
      submenuTopOffset({
        naturalTop: 700,
        naturalBottom: 920,
        viewportHeight: 800,
        padding: 8,
      }),
    ).toBe(-128);
  });

  it("pins an over-tall submenu to the viewport padding", () => {
    expect(
      submenuTopOffset({
        naturalTop: 100,
        naturalBottom: 1100,
        viewportHeight: 800,
        padding: 8,
      }),
    ).toBe(-92);
  });
});
