import { afterEach, describe, expect, it, vi } from "vitest";
import { controllerSpatialNavigationMove, focusControllerSpatialNavigationTarget } from "./controllerSpatialNavigation";
import { markSyntheticControllerKeyEvent } from "../inputModality";

class TestElement {
  readonly children: TestElement[] = [];
  parentElement: TestElement | null = null;
  ownerDocument: { activeElement: TestElement | null };
  disabled = false;
  labels: TestElement[] | null = null;
  scrollIntoView = vi.fn();
  private readonly attrs = new Map<string, string>();

  constructor(
    readonly tagName: string,
    ownerDocument: { activeElement: TestElement | null },
    private readonly rect: { left: number; top: number; width: number; height: number } | null = {
      left: 0,
      top: 0,
      width: 32,
      height: 32,
    },
  ) {
    this.ownerDocument = ownerDocument;
  }

  get nextElementSibling(): TestElement | null {
    if (!this.parentElement) return null;
    const index = this.parentElement.children.indexOf(this);
    return index >= 0 ? this.parentElement.children[index + 1] ?? null : null;
  }

  append(...children: TestElement[]): this {
    for (const child of children) {
      child.parentElement = this;
      child.ownerDocument = this.ownerDocument;
      this.children.push(child);
    }
    return this;
  }

  focus(): void {
    this.ownerDocument.activeElement = this;
  }

  contains(candidate: TestElement | Element): boolean {
    return this === candidate || this.children.some((child) => child.contains(candidate));
  }

  getAttribute(name: string): string | null {
    return this.attrs.get(name) ?? null;
  }

  setAttribute(name: string, value: string): void {
    this.attrs.set(name, value);
  }

  hasAttribute(name: string): boolean {
    return this.attrs.has(name);
  }

  removeAttribute(name: string): void {
    this.attrs.delete(name);
  }

  getClientRects(): unknown[] {
    return this.rect ? [this.rect] : [];
  }

  getBoundingClientRect(): Pick<DOMRect, "left" | "right" | "top" | "bottom" | "width" | "height"> {
    const rect = this.rect ?? { left: 0, top: 0, width: 0, height: 0 };
    return {
      ...rect,
      right: rect.left + rect.width,
      bottom: rect.top + rect.height,
    };
  }

  querySelectorAll<T extends TestElement = TestElement>(_selector: string): T[] {
    return descendants(this).filter((el) => isFocusable(el)) as T[];
  }
}

function descendants(root: TestElement): TestElement[] {
  return root.children.flatMap((child) => [child, ...descendants(child)]);
}

function isFocusable(el: TestElement): boolean {
  return (el.tagName === "INPUT" || el.tagName === "BUTTON" || el.tagName === "SELECT") && !el.disabled;
}

function keyEvent(key: string, synthetic = true): KeyboardEvent {
  const event: {
    key: string;
    altKey: boolean;
    ctrlKey: boolean;
    metaKey: boolean;
    shiftKey: boolean;
    defaultPrevented: boolean;
    preventDefault: ReturnType<typeof vi.fn>;
    stopPropagation: ReturnType<typeof vi.fn>;
  } = {
    key,
    altKey: false,
    ctrlKey: false,
    metaKey: false,
    shiftKey: false,
    defaultPrevented: false,
    preventDefault: vi.fn(() => {
      event.defaultPrevented = true;
    }),
    stopPropagation: vi.fn(),
  };
  if (synthetic) {
    markSyntheticControllerKeyEvent(event as unknown as KeyboardEvent);
  }
  return event as unknown as KeyboardEvent;
}

function buildSettingsGrid() {
  const doc = { activeElement: null as TestElement | null };
  const root = new TestElement("DIV", doc, { left: 0, top: 0, width: 400, height: 400 });
  const row1Left = new TestElement("INPUT", doc, { left: 0, top: 0, width: 32, height: 32 });
  const row1Right = new TestElement("BUTTON", doc, { left: 240, top: 0, width: 80, height: 32 });
  const row2Left = new TestElement("INPUT", doc, { left: 0, top: 56, width: 32, height: 32 });
  const row2Right = new TestElement("SELECT", doc, { left: 240, top: 56, width: 80, height: 32 });
  root.append(row1Left, row1Right, row2Left, row2Right);
  return { root, row1Left, row1Right, row2Left, row2Right };
}

describe("controller spatial navigation", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("moves side to side within a settings row", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const grid = buildSettingsGrid();
    grid.row1Left.focus();

    const event = keyEvent("ArrowRight");
    expect(controllerSpatialNavigationMove(event, grid.root as unknown as HTMLElement)).toBe(true);

    expect(grid.root.ownerDocument.activeElement).toBe(grid.row1Right);
    expect(event.preventDefault).toHaveBeenCalled();
  });

  it("moves up and down between settings rows by column", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const grid = buildSettingsGrid();
    grid.row1Right.focus();

    expect(controllerSpatialNavigationMove(keyEvent("ArrowDown"), grid.root as unknown as HTMLElement)).toBe(true);
    expect(grid.root.ownerDocument.activeElement).toBe(grid.row2Right);

    expect(controllerSpatialNavigationMove(keyEvent("ArrowUp"), grid.root as unknown as HTMLElement)).toBe(true);
    expect(grid.root.ownerDocument.activeElement).toBe(grid.row1Right);
  });

  it("uses visible checkbox labels as controller navigation targets", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const doc = { activeElement: null as TestElement | null };
    const root = new TestElement("DIV", doc, { left: 0, top: 0, width: 400, height: 400 });
    const textInput = new TestElement("INPUT", doc, { left: 0, top: 0, width: 120, height: 32 });
    const row = new TestElement("DIV", doc, { left: 0, top: 56, width: 320, height: 32 });
    const hiddenCheckbox = new TestElement("INPUT", doc, null);
    hiddenCheckbox.setAttribute("type", "checkbox");
    const visibleSwitch = new TestElement("LABEL", doc, { left: 0, top: 56, width: 20, height: 20 });
    const rightButton = new TestElement("BUTTON", doc, { left: 240, top: 56, width: 80, height: 32 });
    row.append(hiddenCheckbox, visibleSwitch, rightButton);
    root.append(textInput, row);
    textInput.focus();

    expect(controllerSpatialNavigationMove(keyEvent("ArrowDown"), root as unknown as HTMLElement)).toBe(true);

    expect(root.ownerDocument.activeElement).toBe(hiddenCheckbox);
    expect(visibleSwitch.scrollIntoView).toHaveBeenCalled();
  });

  it("moves down to the next row instead of skipping to a farther aligned control", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const doc = { activeElement: null as TestElement | null };
    const root = new TestElement("DIV", doc, { left: 0, top: 0, width: 400, height: 400 });
    const row1Input = new TestElement("INPUT", doc, { left: 0, top: 0, width: 120, height: 32 });
    const row2 = new TestElement("DIV", doc, { left: 0, top: 56, width: 320, height: 32 });
    const row2Checkbox = new TestElement("INPUT", doc, null);
    row2Checkbox.setAttribute("type", "checkbox");
    const row2Switch = new TestElement("LABEL", doc, { left: 80, top: 56, width: 20, height: 20 });
    const row3Button = new TestElement("BUTTON", doc, { left: 0, top: 112, width: 120, height: 32 });
    row2.append(row2Checkbox, row2Switch);
    root.append(row1Input, row2, row3Button);
    row1Input.focus();

    expect(controllerSpatialNavigationMove(keyEvent("ArrowDown"), root as unknown as HTMLElement)).toBe(true);

    expect(root.ownerDocument.activeElement).toBe(row2Checkbox);
  });

  it("wraps right and left between adjacent settings rows", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const grid = buildSettingsGrid();
    grid.row1Right.focus();

    expect(controllerSpatialNavigationMove(keyEvent("ArrowRight"), grid.root as unknown as HTMLElement)).toBe(true);
    expect(grid.root.ownerDocument.activeElement).toBe(grid.row2Left);

    expect(controllerSpatialNavigationMove(keyEvent("ArrowLeft"), grid.root as unknown as HTMLElement)).toBe(true);
    expect(grid.root.ownerDocument.activeElement).toBe(grid.row1Right);
  });

  it("keeps controller arrows inside the settings navigation root at the edges", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const grid = buildSettingsGrid();
    grid.row2Right.focus();

    const event = keyEvent("ArrowDown");
    expect(controllerSpatialNavigationMove(event, grid.root as unknown as HTMLElement)).toBe(true);

    expect(grid.root.ownerDocument.activeElement).toBe(grid.row2Right);
    expect(event.preventDefault).toHaveBeenCalled();
    expect(event.stopPropagation).toHaveBeenCalled();
  });

  it("can enter a settings page when the first control is a visible-label checkbox", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const doc = { activeElement: null as TestElement | null };
    const root = new TestElement("DIV", doc, { left: 0, top: 0, width: 400, height: 400 });
    const row = new TestElement("DIV", doc, { left: 0, top: 0, width: 320, height: 32 });
    const hiddenCheckbox = new TestElement("INPUT", doc, null);
    hiddenCheckbox.setAttribute("type", "checkbox");
    const visibleSwitch = new TestElement("LABEL", doc, { left: 0, top: 0, width: 20, height: 20 });
    row.append(hiddenCheckbox, visibleSwitch);
    root.append(row);

    expect(focusControllerSpatialNavigationTarget(root as unknown as HTMLElement)).toBe(true);

    expect(root.ownerDocument.activeElement).toBe(hiddenCheckbox);
    expect(visibleSwitch.scrollIntoView).toHaveBeenCalled();
  });

  it("ignores keyboard arrow events", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const grid = buildSettingsGrid();
    grid.row1Left.focus();

    const event = keyEvent("ArrowRight", false);
    expect(controllerSpatialNavigationMove(event, grid.root as unknown as HTMLElement)).toBe(false);

    expect(grid.root.ownerDocument.activeElement).toBe(grid.row1Left);
    expect(event.preventDefault).not.toHaveBeenCalled();
  });
});
