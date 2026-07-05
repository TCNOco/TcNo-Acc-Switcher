import { afterEach, describe, expect, it, vi } from "vitest";
import {
  focusShortcutArrowNavigationTarget,
  shortcutArrowNavigation,
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
  private readonly listeners = new Map<string, Set<(event: KeyboardEvent) => void>>();

  constructor(
    readonly tagName: string,
    readonly id = "",
    classes: string[] = [],
    ownerDocument: { activeElement: TestElement | null } = { activeElement: null },
    private readonly rect = { left: 0, top: 0, width: 32, height: 32 },
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

  addEventListener(type: string, listener: EventListenerOrEventListenerObject): void {
    const listeners = this.listeners.get(type) ?? new Set<(event: KeyboardEvent) => void>();
    listeners.add(typeof listener === "function" ? listener as (event: KeyboardEvent) => void : listener.handleEvent.bind(listener) as (event: KeyboardEvent) => void);
    this.listeners.set(type, listeners);
  }

  removeEventListener(type: string, listener: EventListenerOrEventListenerObject): void {
    const listeners = this.listeners.get(type);
    if (!listeners) return;
    const callback = typeof listener === "function" ? listener as (event: KeyboardEvent) => void : listener.handleEvent.bind(listener) as (event: KeyboardEvent) => void;
    listeners.delete(callback);
  }

  dispatchEvent(event: KeyboardEvent): boolean {
    this.listeners.get(event.type)?.forEach((listener) => listener(event));
    return !(event as KeyboardEvent & { defaultPrevented?: boolean }).defaultPrevented;
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

  getBoundingClientRect(): Pick<DOMRect, "left" | "right" | "top" | "bottom" | "width" | "height"> {
    return {
      ...this.rect,
      right: this.rect.left + this.rect.width,
      bottom: this.rect.top + this.rect.height,
    };
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
  if (selector === ".shortcutDropdownWrap") {
    return el.classList.contains("shortcutDropdownWrap");
  }
  return false;
}

function isFocusable(el: TestElement): boolean {
  return el.tagName === "BUTTON" && !el.disabled;
}

function button(id = "", left = 0, top = 0): TestElement {
  return new TestElement("BUTTON", id, [], { activeElement: null }, { left, top, width: 32, height: 32 });
}

function keyEvent(key: string): KeyboardEvent {
  const event: {
    type: string;
    key: string;
    altKey: boolean;
    ctrlKey: boolean;
    metaKey: boolean;
    shiftKey: boolean;
    defaultPrevented: boolean;
    preventDefault: ReturnType<typeof vi.fn>;
    stopPropagation: ReturnType<typeof vi.fn>;
  } = {
    type: "keydown",
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
  return event as unknown as KeyboardEvent;
}

function buildShortcutBar() {
  const doc = { activeElement: null as TestElement | null };
  const root = new TestElement("DIV", "", ["gameShortcutBar"], doc);
  const pinnedA = button("pinned-a", 0, 100);
  const pinnedB = button("pinned-b", 40, 100);
  const dropdownWrap = new TestElement("DIV", "", ["shortcutDropdownWrap"], doc, { left: 80, top: 100, width: 32, height: 32 });
  const dropdownButton = button("shortcutDropdownBtn", 80, 100);
  const dropdown = new TestElement("DIV", "shortcutDropdown", ["shortcutDropdown", "open"], doc);
  const dropdownA = button("dropdown-a", 80, 0);
  const dropdownB = button("dropdown-b", 80, 40);
  const folder = button("btnOpenShortcutFolder", 80, 80);
  const launch = button("btnStartPlat", 120, 100);

  dropdown.append(dropdownA, dropdownB, folder);
  dropdownWrap.append(dropdownButton, dropdown);
  root.append(pinnedA, pinnedB, dropdownWrap, launch);

  return { root, pinnedA, pinnedB, dropdownWrap, dropdownButton, dropdownA, dropdownB, folder, launch };
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

  it("does not trap focus at the shortcut bar edge", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const bar = buildShortcutBar();
    bar.launch.focus();

    const event = keyEvent("ArrowRight");
    expect(moveShortcutArrowFocus(event, bar.root as unknown as HTMLElement)).toBe(false);
    expect(event.preventDefault).not.toHaveBeenCalled();
    expect(bar.root.ownerDocument.activeElement).toBe(bar.launch);
  });

  it("lets the dropdown button handle ArrowDown when the dropdown is closed", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const bar = buildShortcutBar();
    bar.dropdownButton.focus();
    bar.dropdownWrap.children.splice(bar.dropdownWrap.children.indexOf(bar.dropdownButton) + 1, 1);

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

  it("uses vertical geometry inside the open dropdown", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const bar = buildShortcutBar();
    bar.folder.focus();

    expect(moveShortcutArrowFocus(keyEvent("ArrowUp"), bar.root as unknown as HTMLElement)).toBe(true);
    expect(bar.root.ownerDocument.activeElement).toBe(bar.dropdownB);
  });

  it("focuses the first or last dropdown target directly", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const bar = buildShortcutBar();

    expect(focusShortcutArrowNavigationTarget(bar.root as unknown as HTMLElement, "first")).toBe(true);
    expect(bar.root.ownerDocument.activeElement).toBe(bar.pinnedA);

    expect(focusShortcutArrowNavigationTarget(bar.root as unknown as HTMLElement, "last")).toBe(true);
    expect(bar.root.ownerDocument.activeElement).toBe(bar.launch);
  });

  it("lets the action handle Escape without making the group itself focusable", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const bar = buildShortcutBar();
    const onEscape = vi.fn();
    const action = shortcutArrowNavigation(bar.root as unknown as HTMLElement, { onEscape });
    const event = keyEvent("Escape");

    bar.pinnedA.focus();
    bar.root.dispatchEvent(event);

    expect(onEscape).toHaveBeenCalledTimes(1);
    expect(event.preventDefault).toHaveBeenCalled();
    expect(event.stopPropagation).toHaveBeenCalled();
    action?.destroy?.();
  });

  it("does not let pinned shortcuts own ArrowUp for opening the dropdown", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const bar = buildShortcutBar();
    const onHotbarUp = vi.fn();
    const event = keyEvent("ArrowUp");

    bar.pinnedA.focus();

    expect(moveShortcutArrowFocus(event, bar.root as unknown as HTMLElement, {
      canOpenDropdownFromUp: (active) => active.closest(".shortcutDropdownWrap") instanceof HTMLElement,
      onHotbarUp,
    })).toBe(false);
    expect(onHotbarUp).not.toHaveBeenCalled();
    expect(event.preventDefault).not.toHaveBeenCalled();
  });

  it("lets the dropdown control use ArrowUp to enter an open dropdown", () => {
    vi.stubGlobal("HTMLElement", TestElement);
    const bar = buildShortcutBar();

    bar.dropdownButton.focus();

    expect(moveShortcutArrowFocus(keyEvent("ArrowUp"), bar.root as unknown as HTMLElement, {
      canOpenDropdownFromUp: (active) => active.closest(".shortcutDropdownWrap") instanceof HTMLElement,
      onHotbarUp: vi.fn(),
    })).toBe(true);
    expect(bar.root.ownerDocument.activeElement).toBe(bar.folder);
  });
});
