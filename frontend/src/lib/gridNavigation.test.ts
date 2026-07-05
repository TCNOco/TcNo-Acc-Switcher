import { describe, expect, it } from "vitest";
import { nextGridNavigationId, type GridNavigationCell } from "./gridNavigation";

function cell(id: string, index: number, x: number, y: number): GridNavigationCell {
  return {
    id,
    index,
    left: x,
    right: x + 100,
    top: y,
    bottom: y + 100,
    width: 100,
    height: 100,
  };
}

describe("nextGridNavigationId", () => {
  const cells = [
    cell("a", 0, 0, 0),
    cell("b", 1, 120, 0),
    cell("c", 2, 240, 0),
    cell("d", 3, 0, 120),
    cell("e", 4, 120, 120),
  ];

  it("moves left and right by visual order", () => {
    expect(nextGridNavigationId(cells, "b", "ArrowLeft")).toBe("a");
    expect(nextGridNavigationId(cells, "b", "ArrowRight")).toBe("c");
  });

  it("moves up and down to the nearest cell in the target row", () => {
    expect(nextGridNavigationId(cells, "b", "ArrowDown")).toBe("e");
    expect(nextGridNavigationId(cells, "e", "ArrowUp")).toBe("b");
  });

  it("stays put when there is no cell in that direction", () => {
    expect(nextGridNavigationId(cells, "a", "ArrowLeft")).toBeNull();
    expect(nextGridNavigationId(cells, "a", "ArrowUp")).toBeNull();
  });
});
