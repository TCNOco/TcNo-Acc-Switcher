import { afterEach, describe, expect, it, vi } from "vitest";
import {
  focusShortcutArrowNavigationTarget,
  moveShortcutArrowFocus,
} from "./shortcutArrowNavigation";

class TestClassList {
  private readonly values = new Set<string>();

  constructor(values: string[] = []) {
    values.forEach((value) => this.values.add(value));
  }

  contains(value: string): boolean {
    return this.values.has(value);
  }
}

class TestElement {
  readonly classList: TestClassList;
  readonly children: TestElement[] = [];
  readonly attrs = new Map<string, string>();
  parentElement: TestElement | null = null;
  ownerDocument: { activeElement: TestElement | null };
  disabled = false;
  scrollIntoView = vi.fn();

  constructor(
    readonly tagName: string,
    readonly id = "",
    classes: string[] = [],
    ownerDocument: { activeElement: TestElement | null } = { activeElement: null },
  ) {
    this.classList = new TestClassList(classes);
    this.ownerDocument = ownerDocument;
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

  getAttributeNames(): string[] {
    return Array.from(this.attrs.keys());
  }

  setAttribute(name: string, value: string): void {
    this.attrs.set(name, value);
  }

  getBoundingClientRect(): Pick<DOMRect, "width" | "height"> {
    return { width: 32, height: 32 };
  }

  closest(selector: string): TestElement | null {
    let current: TestElement | null = this;
    while (current) {
      if (matchesSelector(current, selector)) return current;
      current = current.parentElement;
    }
    return null;
  }

  querySelector<T extends TestElement = TestElement>(selector: string): T | null {
    return (this.querySelectorAll<T>(selector)[0] ?? null) as T | null;
  }

  querySelectorAll<T extends TestElement = TestElement>(selector: string): T[] {
    const all = descendants(this);
    if (selector === ".shortcutDropdown.open") {
      return all.filter((el) => matchesSelector(el, selector)) as T[];
    }
    return all.filter((el) => isFocusable(el)) as T[];
  }
}

function descendants(root: TestElement): TestElement[] {
  return root.children.flatMap((child) => [child, ...descendants(child)]);
}

function matchesSelector(el: TestElement, selector: string): boolean {
  if (selector === ".shortcutDropdown.open") {
    return el.classList.contains("shortcutDropdown") && el.classList.contains("open");
  }
  return false;
}

function isFocusable(el: TestElement): boolean {
  return el.tagName === "BUTTON" && !el.disabled;
}

function button(id = ""): TestElement {
  return new TestElement("BUTTON", id);
}

function keyEvent(key: string): KeyboardEvent {
  return {
    key,
    altKey: false,
    ctrlKey: false,
    metaKey: false,
    shiftKey: false,
    preventDefault: vi.fn(),
    stopPropagation: vi.fn(),
  } as unknown as KeyboardEvent;
}

function buildShortcutBar() {
  const doc = { activeElement: null as TestElement | null };
  const root = new TestElement("DIV", "", ["gameShortcutBar"], doc);
  const pinnedA = button("pinned-a");
  const pinnedB = button("pinned-b");
  const dropdownButton = button("shortcutDropdownBtn");
  const dropdown = new TestElement("DIV", "shortcutDropdown", ["shortcutDropdown", "open"], doc);
  const dropdownA = button("dropdown-a");
  const dropdownB = button("dropdown-b");
  const folder = button("btnOpenShortcutFolder");
  const launch = button("btnStartPlat");

  dropdown.append(dropdownA, dropdownB, folder);
  root.append(pinnedA, pinnedB, dropdownButton, dropdown, launch);

  return { root, pinnedA, pinnedB, dropdownButton, dropdownA, dropdownB, folder, launch };
}

describe("shortcut arrow navigation", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("moves left and right through shortcut bar controls", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const bar = buildShortcutBar();
    bar.pinnedA.focus();

    expect(moveShortcutArrowFocus(keyEvent("ArrowRight"), bar.root as unknown as HTMLElement)).toBe(true);
    expect(bar.root.ownerDocument.activeElement).toBe(bar.pinnedB);

    expect(moveShortcutArrowFocus(keyEvent("ArrowLeft"), bar.root as unknown as HTMLElement)).toBe(true);
    expect(bar.root.ownerDocument.activeElement).toBe(bar.pinnedA);
  });

  it("lets the dropdown button handle ArrowDown when the dropdown is closed", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const bar = buildShortcutBar();
    bar.dropdownButton.focus();
    bar.root.children.splice(bar.root.children.indexOf(bar.dropdownButton) + 1, 1);

    const event = keyEvent("ArrowDown");
    expect(moveShortcutArrowFocus(event, bar.root as unknown as HTMLElement)).toBe(false);
    expect(event.preventDefault).not.toHaveBeenCalled();
  });

  it("moves from an expanded dropdown button into the dropdown with ArrowDown", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const bar = buildShortcutBar();
    bar.dropdownButton.focus();

    expect(moveShortcutArrowFocus(keyEvent("ArrowDown"), bar.root as unknown as HTMLElement)).toBe(true);
    expect(bar.root.ownerDocument.activeElement).toBe(bar.dropdownA);
  });

  it("keeps arrow navigation scoped inside the open dropdown", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const bar = buildShortcutBar();
    bar.dropdownA.focus();

    expect(moveShortcutArrowFocus(keyEvent("ArrowDown"), bar.root as unknown as HTMLElement)).toBe(true);
    expect(bar.root.ownerDocument.activeElement).toBe(bar.dropdownB);

    expect(moveShortcutArrowFocus(keyEvent("ArrowDown"), bar.root as unknown as HTMLElement)).toBe(true);
    expect(bar.root.ownerDocument.activeElement).toBe(bar.folder);
  });

  it("focuses the first or last dropdown target directly", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const bar = buildShortcutBar();

    expect(focusShortcutArrowNavigationTarget(bar.root as unknown as HTMLElement, "first")).toBe(true);
    expect(bar.root.ownerDocument.activeElement).toBe(bar.pinnedA);

    expect(focusShortcutArrowNavigationTarget(bar.root as unknown as HTMLElement, "last")).toBe(true);
    expect(bar.root.ownerDocument.activeElement).toBe(bar.launch);
  });
});
