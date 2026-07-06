import { describe, expect, it } from "vitest";
import { reorderShortcutByCommand, stripCellForGhostClone } from "./dragReorderShortcuts";
import { reorderItemByCommand } from "./reorderList";

class FakeElement {
  readonly children: FakeElement[] = [];
  parent: FakeElement | null = null;
  removed = false;
  style: Record<string, string> = {};
  private readonly attrs = new Map<string, string>();
  readonly classList = {
    remove: (name: string) => {
      const classes = new Set((this.attrs.get("class") ?? "").split(/\s+/).filter(Boolean));
      classes.delete(name);
      this.attrs.set("class", Array.from(classes).join(" "));
    },
  };

  constructor(readonly tagName: string, attrs: Record<string, string> = {}) {
    for (const [key, value] of Object.entries(attrs)) {
      this.attrs.set(key, value);
    }
  }

  append(...children: FakeElement[]): this {
    for (const child of children) {
      child.parent = this;
      this.children.push(child);
    }
    return this;
  }

  remove(): void {
    this.removed = true;
    if (this.parent) {
      const index = this.parent.children.indexOf(this);
      if (index >= 0) this.parent.children.splice(index, 1);
    }
  }

  removeAttribute(name: string): void {
    this.attrs.delete(name);
  }

  getAttribute(name: string): string | null {
    return this.attrs.get(name) ?? null;
  }

  querySelectorAll(selector: string): FakeElement[] {
    const selectors = selector.split(",").map((item) => item.trim());
    return descendants(this).filter((element) => selectors.some((item) => matchesSelector(element, item)));
  }
}

function descendants(root: FakeElement): FakeElement[] {
  return root.children.flatMap((child) => [child, ...descendants(child)]);
}

function matchesSelector(element: FakeElement, selector: string): boolean {
  if (selector === "input") return element.tagName === "INPUT";
  if (selector === "button") return element.tagName === "BUTTON";
  if (selector === "img") return element.tagName === "IMG";
  if (selector === "svg") return element.tagName === "SVG";
  if (selector === ".button") return (element.getAttribute("class") ?? "").split(/\s+/).includes("button");
  if (selector === "[role='button']") return element.getAttribute("role") === "button";
  if (selector === "[id]") return element.getAttribute("id") !== null;
  if (selector === "label[for]") return element.tagName === "LABEL" && element.getAttribute("for") !== null;
  return false;
}

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

describe("stripCellForGhostClone", () => {
  it("normalizes shortcut clone media into the ghost bounds", () => {
    const root = new FakeElement("DIV", {
      class: "shortcutDndCell",
      "data-dnd-cell": "",
      "data-dnd-name": "Windrose.url",
      role: "listitem",
      tabindex: "0",
    });
    const button = new FakeElement("BUTTON", { class: "HasContextMenu", id: "shortcut-button" });
    const image = new FakeElement("IMG", { id: "shortcut-image" });
    const input = new FakeElement("INPUT");
    const label = new FakeElement("LABEL", { for: "shortcut-button" });
    root.append(button.append(image), input, label);

    stripCellForGhostClone(root as unknown as HTMLElement);

    expect(root.getAttribute("data-dnd-cell")).toBeNull();
    expect(root.getAttribute("role")).toBeNull();
    expect(root.getAttribute("class")).not.toContain("shortcutDndCell");
    expect(input.removed).toBe(true);
    expect(button.getAttribute("id")).toBeNull();
    expect(image.getAttribute("id")).toBeNull();
    expect(label.getAttribute("for")).toBeNull();
    expect(button.style).toMatchObject({
      width: "100%",
      height: "100%",
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
      overflow: "hidden",
    });
    expect(image.style).toMatchObject({
      width: "100%",
      height: "100%",
      maxWidth: "none",
      maxHeight: "none",
      objectFit: "contain",
      objectPosition: "center",
    });
  });
});
