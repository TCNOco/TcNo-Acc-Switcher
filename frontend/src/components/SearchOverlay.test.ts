import { describe, expect, it, vi } from "vitest";
import {
  resolveSearchOverlayActiveDescendant,
  resolveSearchOverlayKey,
  searchOverlayOptionId,
} from "./SearchOverlay.svelte";

vi.mock("../lib/platformIcon", () => ({
  platformIconFgHref: (name: string) => `#${name}`,
}));

vi.mock("../stores/i18n", () => ({
  t: {
    subscribe(run: (translator: (key: string) => string) => void) {
      run((key: string) => key);
      return () => {};
    },
  },
}));

vi.mock("../stores/searchOverlay", () => ({
  searchOverlayCtrl: {
    subscribe(run: (state: { open: boolean }) => void) {
      run({ open: false });
      return () => {};
    },
  },
  searchOverlayPendingAppend: {
    subscribe() {
      return () => {};
    },
    set: vi.fn(),
  },
}));

describe("SearchOverlay keyboard resolution", () => {
  it("moves selection within available results", () => {
    expect(resolveSearchOverlayKey("ArrowDown", 3, 0, 3)).toEqual({ type: "move", selectedIndex: 1 });
    expect(resolveSearchOverlayKey("ArrowDown", 3, 2, 3)).toEqual({ type: "move", selectedIndex: 2 });
    expect(resolveSearchOverlayKey("ArrowUp", 3, 2, 3)).toEqual({ type: "move", selectedIndex: 1 });
    expect(resolveSearchOverlayKey("ArrowUp", 3, 0, 3)).toEqual({ type: "move", selectedIndex: 0 });
  });

  it("picks the selected result only when results exist", () => {
    expect(resolveSearchOverlayKey("Enter", 3, 1, 3)).toEqual({ type: "pick", selectedIndex: 1 });
    expect(resolveSearchOverlayKey("Enter", 3, 0, 0)).toEqual({ type: "noop" });
  });

  it("closes on Escape and on Backspace from an empty query", () => {
    expect(resolveSearchOverlayKey("Escape", 3, 1, 3)).toEqual({ type: "close" });
    expect(resolveSearchOverlayKey("Backspace", 0, 1, 3)).toEqual({ type: "close" });
    expect(resolveSearchOverlayKey("Backspace", 1, 1, 3)).toEqual({ type: "noop" });
  });

  it("derives stable active-descendant ids only for valid selections", () => {
    expect(searchOverlayOptionId(0)).toBe("searchOverlay_option_0");
    expect(resolveSearchOverlayActiveDescendant(1, 3)).toBe("searchOverlay_option_1");
    expect(resolveSearchOverlayActiveDescendant(-1, 3)).toBeUndefined();
    expect(resolveSearchOverlayActiveDescendant(3, 3)).toBeUndefined();
    expect(resolveSearchOverlayActiveDescendant(0, 0)).toBeUndefined();
  });
});
