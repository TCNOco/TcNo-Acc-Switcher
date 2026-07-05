import { describe, expect, it } from "vitest";
import { reorderShortcutByCommand } from "./dragReorderShortcuts";
import { reorderItemByCommand } from "./reorderList";

describe("reorderItemByCommand", () => {
  it("moves an item one slot left and right", () => {
    expect(reorderItemByCommand(["a", "b", "c"], "b", "left")).toMatchObject({
      items: ["b", "a", "c"],
      moved: true,
      position: 1,
      total: 3,
    });
    expect(reorderItemByCommand(["a", "b", "c"], "b", "right")).toMatchObject({
      items: ["a", "c", "b"],
      moved: true,
      position: 3,
      total: 3,
    });
  });

  it("moves an item to the start or end", () => {
    expect(reorderItemByCommand(["a", "b", "c", "d"], "c", "start")).toMatchObject({
      items: ["c", "a", "b", "d"],
      moved: true,
      position: 1,
    });
    expect(reorderItemByCommand(["a", "b", "c", "d"], "b", "end")).toMatchObject({
      items: ["a", "c", "d", "b"],
      moved: true,
      position: 4,
    });
  });

  it("reports boundary and missing-item no-ops", () => {
    expect(reorderItemByCommand(["a", "b"], "a", "left")).toMatchObject({
      items: ["a", "b"],
      moved: false,
      position: 1,
    });
    expect(reorderItemByCommand(["a", "b"], "x", "right")).toMatchObject({
      items: ["a", "b"],
      moved: false,
      position: 0,
      total: 2,
    });
  });
});

describe("reorderShortcutByCommand", () => {
  it("reorders within the current shortcut zone", () => {
    expect(reorderShortcutByCommand(["p1", "p2", "p3"], ["d1"], "pinned", "p2", "left")).toMatchObject({
      pins: ["p2", "p1", "p3"],
      drops: ["d1"],
      moved: true,
      zone: "pinned",
      position: 1,
      total: 3,
    });
  });

  it("moves shortcuts between pinned and dropdown zones", () => {
    expect(reorderShortcutByCommand(["p1"], ["d1", "d2"], "dropdown", "d1", "pin")).toMatchObject({
      pins: ["p1", "d1"],
      drops: ["d2"],
      moved: true,
      zone: "pinned",
      position: 2,
      total: 2,
    });
    expect(reorderShortcutByCommand(["p1", "p2"], ["d1"], "pinned", "p1", "move-to-dropdown")).toMatchObject({
      pins: ["p2"],
      drops: ["d1", "p1"],
      moved: true,
      zone: "dropdown",
      position: 2,
      total: 2,
    });
  });
});
