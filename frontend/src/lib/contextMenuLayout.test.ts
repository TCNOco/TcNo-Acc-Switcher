import { describe, expect, it } from "vitest";
import {
  balancedSubmenuPageRanges,
  submenuShouldFillRootHeight,
  submenuTopOffset,
} from "./contextMenuLayout";

describe("context menu submenu layout", () => {
  it("keeps a short submenu beside its parent row", () => {
    expect(
      submenuTopOffset({
        naturalTop: 320,
        naturalBottom: 400,
        rootTop: 100,
        rootBottom: 500,
      }),
    ).toBe(0);
  });

  it("moves a submenu up only by its overflow past the root menu bottom", () => {
    expect(
      submenuTopOffset({
        naturalTop: 700,
        naturalBottom: 920,
        rootTop: 200,
        rootBottom: 792,
      }),
    ).toBe(-128);
  });

  it("stops at the root menu top when the submenu is taller than level zero", () => {
    expect(
      submenuTopOffset({
        naturalTop: 450,
        naturalBottom: 900,
        rootTop: 100,
        rootBottom: 500,
      }),
    ).toBe(-350);
  });

  it("continues growing downward once aligned with the root menu top", () => {
    expect(
      submenuTopOffset({
        naturalTop: 100,
        naturalBottom: 1100,
        rootTop: 100,
        rootBottom: 500,
      }),
    ).toBe(0);
  });
});

describe("context menu measured pagination", () => {
  it("shows all eight items when they fit without a pagination footer", () => {
    expect(balancedSubmenuPageRanges(Array(8).fill(40), 320, 280)).toEqual([
      { start: 0, end: 8 },
    ]);
  });

  it("balances the last page instead of leaving a one-item orphan", () => {
    const ranges = balancedSubmenuPageRanges(Array(11).fill(40), 200, 200);
    const pageSizes = ranges.map(({ start, end }) => end - start);

    expect(pageSizes).toEqual([4, 4, 3]);
    expect(ranges.at(-1)?.end).toBe(11);
  });

  it("uses actual wrapped row heights without overflowing a page", () => {
    const heights = [40, 40, 80, 40, 40, 80, 40];
    const ranges = balancedSubmenuPageRanges(heights, 200, 200);

    expect(ranges[0]?.start).toBe(0);
    expect(ranges.at(-1)?.end).toBe(heights.length);
    for (const { start, end } of ranges) {
      expect(heights.slice(start, end).reduce((sum, height) => sum + height, 0)).toBeLessThanOrEqual(200);
    }
  });

  it("avoids a singleton even when filling by height would favor one", () => {
    const ranges = balancedSubmenuPageRanges([160, 20, 20, 20, 20, 20], 200, 200);

    expect(ranges.map(({ start, end }) => end - start)).toEqual([3, 3]);
  });

  it("keeps an oversized row on a page by itself", () => {
    expect(balancedSubmenuPageRanges([40, 240, 40], 200, 200)).toEqual([
      { start: 0, end: 1 },
      { start: 1, end: 2 },
      { start: 2, end: 3 },
    ]);
  });
});

describe("context menu root-height snapping", () => {
  it("fills level zero when a paginated submenu is within one and a half rows of its top", () => {
    expect(
      submenuShouldFillRootHeight({
        naturalTop: 200,
        topOffset: -60,
        naturalHeight: 350,
        rootTop: 100,
        rootHeight: 400,
        rowHeight: 32,
      }),
    ).toBe(true);
  });

  it("keeps natural height outside the snap zone or when content is taller than level zero", () => {
    expect(
      submenuShouldFillRootHeight({
        naturalTop: 200,
        topOffset: -50,
        naturalHeight: 350,
        rootTop: 100,
        rootHeight: 400,
        rowHeight: 32,
      }),
    ).toBe(false);
    expect(
      submenuShouldFillRootHeight({
        naturalTop: 200,
        topOffset: -60,
        naturalHeight: 420,
        rootTop: 100,
        rootHeight: 400,
        rowHeight: 32,
      }),
    ).toBe(false);
  });
});
