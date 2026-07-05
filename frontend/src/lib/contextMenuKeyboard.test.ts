import { afterEach, describe, expect, it, vi } from "vitest";
import {
  focusFirstNavigable,
  handleContextMenuKeydown,
  handleContextMenuQuickFilter,
  type KeyboardNavDeps,
} from "./contextMenuKeyboard";

class TestClassList {
  private readonly values = new Set<string>();

  constructor(values: string[] = []) {
    values.forEach((value) => this.values.add(value));
  }

  add(value: string): void {
    this.values.add(value);
  }

  contains(value: string): boolean {
    return this.values.has(value);
  }
}

class TestElement {
  readonly classList: TestClassList;
  readonly children: TestElement[] = [];
  parentElement: TestElement | null = null;

  constructor(
    readonly tagName: string,
    classes: string[] = [],
  ) {
    this.classList = new TestClassList(classes);
  }

  append(...children: TestElement[]): this {
    for (const child of children) {
      child.parentElement = this;
      this.children.push(child);
    }
    return this;
  }

  focus(): void {
    Object.defineProperty(document, "activeElement", { value: this, configurable: true });
  }

  contains(candidate: TestElement): boolean {
    return this === candidate || this.children.some((child) => child.contains(candidate));
  }

  matches(selector: string): boolean {
    if (selector === "ul.ctx-menu-root") return this.tagName === "UL" && this.classList.contains("ctx-menu-root");
    if (selector === ".ctx-menu-root") return this.classList.contains("ctx-menu-root");
    if (selector === "li.hasSubmenu") return this.tagName === "LI" && this.classList.contains("hasSubmenu");
    if (selector === "li") return this.tagName === "LI";
    return false;
  }

  getAttributeNames(): string[] {
    return [];
  }

  closest(selector: string): TestElement | null {
    let current: TestElement | null = this;
    while (current) {
      if (current.matches(selector)) {
        return current;
      }
      current = current.parentElement;
    }
    return null;
  }

  querySelector<T extends TestElement = TestElement>(selector: string): T | null {
    return (this.querySelectorAll<T>(selector)[0] ?? null) as T | null;
  }

  querySelectorAll<T extends TestElement = TestElement>(selector: string): T[] {
    if (selector === ":scope > li:not(.row-hidden):not(.ctx-sep):not(.ctx-pagination-li)") {
      return this.children.filter(
        (child) =>
          child.tagName === "LI" &&
          !child.classList.contains("row-hidden") &&
          !child.classList.contains("ctx-sep") &&
          !child.classList.contains("ctx-pagination-li"),
      ) as T[];
    }
    if (selector.startsWith(":scope > ")) {
      return this.children.filter((child) => matchesSimpleSelector(child, selector.slice(":scope > ".length))) as T[];
    }
    if (selector === "li.hasSubmenu.submenu-expanded > ul.submenu") {
      return descendants(this)
        .filter(
          (child) =>
            child.tagName === "LI" &&
            child.classList.contains("hasSubmenu") &&
            child.classList.contains("submenu-expanded"),
        )
        .flatMap((li) => li.children.filter((child) => child.tagName === "UL" && child.classList.contains("submenu"))) as T[];
    }
    if (selector === "li.contextSearch input.ctx-menu__search") {
      return descendants(this)
        .filter((child) => child.tagName === "LI" && child.classList.contains("contextSearch"))
        .flatMap((li) =>
          descendants(li).filter((child) => child.tagName === "INPUT" && child.classList.contains("ctx-menu__search")),
        ) as T[];
    }
    return descendants(this).filter((child) => matchesSimpleSelector(child, selector)) as T[];
  }

  dispatchEvent(_event: Event): boolean {
    return true;
  }
}

class TestUListElement extends TestElement {
  constructor(classes: string[] = []) {
    super("UL", classes);
  }
}

class TestLiElement extends TestElement {
  constructor(classes: string[] = []) {
    super("LI", classes);
  }
}

class TestButtonElement extends TestElement {
  disabled = false;
  readonly click = vi.fn();

  constructor(classes: string[] = []) {
    super("BUTTON", classes);
  }
}

class TestInputElement extends TestElement {
  value = "";
  selectionStart = 0;
  selectionEnd = 0;
  dispatchedInputCount = 0;

  constructor(classes: string[] = []) {
    super("INPUT", classes);
  }

  setSelectionRange(start: number, end: number): void {
    this.selectionStart = start;
    this.selectionEnd = end;
  }

  dispatchEvent(event: Event): boolean {
    if (event.type === "input") {
      this.dispatchedInputCount += 1;
    }
    return true;
  }
}

function descendants(root: TestElement): TestElement[] {
  return root.children.flatMap((child) => [child, ...descendants(child)]);
}

function matchesSimpleSelector(el: TestElement, selector: string): boolean {
  if (selector === ".ctx-menu__label") return el.classList.contains("ctx-menu__label");
  const [tag, ...classes] = selector.split(".");
  return el.tagName === tag.toUpperCase() && classes.every((className) => el.classList.contains(className));
}

function li(classes: string[], ...children: TestElement[]): TestLiElement {
  return new TestLiElement(classes).append(...children);
}

function button(classes: string[] = []): TestButtonElement {
  return new TestButtonElement(classes);
}

function input(value = ""): TestInputElement {
  const el = new TestInputElement(["ctx-menu__search"]);
  el.value = value;
  el.selectionStart = value.length;
  el.selectionEnd = value.length;
  return el;
}

function keyboardEvent(key: string, props: Partial<KeyboardEvent> = {}): KeyboardEvent {
  const event = new Event("keydown") as KeyboardEvent;
  Object.defineProperty(event, "key", { value: key, configurable: true });
  Object.defineProperty(event, "ctrlKey", { value: false, configurable: true });
  Object.defineProperty(event, "altKey", { value: false, configurable: true });
  Object.defineProperty(event, "metaKey", { value: false, configurable: true });
  Object.defineProperty(event, "isComposing", { value: false, configurable: true });
  for (const [name, value] of Object.entries(props)) {
    Object.defineProperty(event, name, { value, configurable: true });
  }
  Object.defineProperty(event, "preventDefault", { value: vi.fn(), configurable: true });
  Object.defineProperty(event, "stopPropagation", { value: vi.fn(), configurable: true });
  return event;
}

function setupGlobals(active: TestElement | null = null): void {
  const body = new TestElement("BODY");
  vi.stubGlobal("HTMLElement", TestElement);
  vi.stubGlobal("HTMLUListElement", TestUListElement);
  vi.stubGlobal("HTMLButtonElement", TestButtonElement);
  vi.stubGlobal("HTMLInputElement", TestInputElement);
  vi.stubGlobal("document", { activeElement: active, body });
  vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => {
    callback(0);
    return 1;
  });
}

function buildMenu() {
  const search = input();
  const alpha = button(["ctx-menu__btn"]);
  const parentAction = button(["ctx-menu__parent-action"]);
  const submenuSearch = input();
  const submenuLeaf = button(["ctx-menu__btn"]);
  const omega = button(["ctx-menu__btn"]);
  const disabled = button(["ctx-menu__btn"]);
  disabled.disabled = true;

  const submenu = new TestUListElement(["submenu"]).append(
    li(["contextSearch"], submenuSearch),
    li([], submenuLeaf),
  );
  const parentLi = li(["hasSubmenu"], parentAction, submenu);
  const root = new TestUListElement(["ctx-menu-root"]).append(
    li(["contextSearch"], search),
    li([], alpha),
    parentLi,
    li([], disabled),
    li([], omega),
  );

  return { root, search, alpha, parentAction, parentLi, submenuSearch, submenuLeaf, omega };
}

function navDeps(): KeyboardNavDeps & { expandSubmenuForLi: ReturnType<typeof vi.fn> } {
  return { expandSubmenuForLi: vi.fn() };
}

describe("context menu keyboard navigation", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("focuses the first navigable row and moves with ArrowDown and ArrowUp", () => {
    const menu = buildMenu();
    setupGlobals();

    focusFirstNavigable(menu.root as unknown as HTMLElement);
    expect(document.activeElement).toBe(menu.search);

    const down = keyboardEvent("ArrowDown");
    expect(handleContextMenuKeydown(down, menu.root as unknown as HTMLElement, navDeps())).toBe(true);
    expect(document.activeElement).toBe(menu.alpha);

    const up = keyboardEvent("ArrowUp");
    expect(handleContextMenuKeydown(up, menu.root as unknown as HTMLElement, navDeps())).toBe(true);
    expect(document.activeElement).toBe(menu.search);
  });

  it("wraps ArrowUp from search to the last row in the menu column", () => {
    const menu = buildMenu();
    setupGlobals(menu.search);
    menu.search.focus();

    expect(handleContextMenuKeydown(keyboardEvent("ArrowUp"), menu.root as unknown as HTMLElement, navDeps())).toBe(true);

    expect(document.activeElement).toBe(menu.omega);
  });

  it("jumps to the first and last rows with Home and End", () => {
    const menu = buildMenu();
    setupGlobals(menu.alpha);
    menu.alpha.focus();

    expect(handleContextMenuKeydown(keyboardEvent("End"), menu.root as unknown as HTMLElement, navDeps())).toBe(true);
    expect(document.activeElement).toBe(menu.omega);

    expect(handleContextMenuKeydown(keyboardEvent("Home"), menu.root as unknown as HTMLElement, navDeps())).toBe(true);
    expect(document.activeElement).toBe(menu.search);
  });

  it("expands a submenu with ArrowRight and focuses its first row", () => {
    const menu = buildMenu();
    const deps = navDeps();
    setupGlobals(menu.parentAction);
    menu.parentAction.focus();

    const event = keyboardEvent("ArrowRight");
    expect(handleContextMenuKeydown(event, menu.root as unknown as HTMLElement, deps)).toBe(true);

    expect(deps.expandSubmenuForLi).toHaveBeenCalledWith(menu.parentLi);
    expect(document.activeElement).toBe(menu.submenuSearch);
    expect(event.preventDefault).toHaveBeenCalled();
  });

  it("returns focus to the parent row with ArrowLeft from a submenu", () => {
    const menu = buildMenu();
    const deps = { ...navDeps(), collapseSubmenuForLi: vi.fn() };
    setupGlobals(menu.submenuLeaf);
    menu.submenuLeaf.focus();

    expect(handleContextMenuKeydown(keyboardEvent("ArrowLeft"), menu.root as unknown as HTMLElement, deps)).toBe(true);

    expect(deps.collapseSubmenuForLi).toHaveBeenCalledWith(menu.parentLi);
    expect(document.activeElement).toBe(menu.parentAction);
  });

  it("moves focus into an open menu when a navigation key starts outside it", () => {
    const menu = buildMenu();
    const outside = button();
    setupGlobals(outside);
    outside.focus();

    const event = keyboardEvent("ArrowDown");
    expect(handleContextMenuKeydown(event, menu.root as unknown as HTMLElement, navDeps())).toBe(true);

    expect(document.activeElement).toBe(menu.search);
    expect(event.preventDefault).toHaveBeenCalled();
    expect(event.stopPropagation).toHaveBeenCalled();
  });

  it("activates leaf and parent-action rows with Enter and Space", () => {
    const menu = buildMenu();
    const deps = navDeps();
    setupGlobals(menu.alpha);
    menu.alpha.focus();

    expect(handleContextMenuKeydown(keyboardEvent("Enter"), menu.root as unknown as HTMLElement, deps)).toBe(true);
    expect(menu.alpha.click).toHaveBeenCalledTimes(1);

    menu.parentAction.focus();
    expect(handleContextMenuKeydown(keyboardEvent(" "), menu.root as unknown as HTMLElement, deps)).toBe(true);
    expect(menu.parentAction.click).toHaveBeenCalledTimes(1);
  });

  it("leaves Escape for the caller to close the menu", () => {
    const menu = buildMenu();
    setupGlobals(menu.alpha);
    menu.alpha.focus();
    const event = keyboardEvent("Escape");

    expect(handleContextMenuKeydown(event, menu.root as unknown as HTMLElement, navDeps())).toBe(false);
    expect(event.preventDefault).not.toHaveBeenCalled();
  });

  it("quick-filters by moving outside typing into the latest search field", () => {
    const menu = buildMenu();
    menu.parentLi.classList.add("submenu-expanded");
    const outside = button();
    setupGlobals(outside);
    outside.focus();

    const event = keyboardEvent("z");
    expect(handleContextMenuQuickFilter(event, menu.root as unknown as HTMLElement)).toBe(true);

    expect(document.activeElement).toBe(menu.submenuSearch);
    expect(menu.submenuSearch.value).toBe("z");
    expect(menu.submenuSearch.selectionStart).toBe(1);
    expect(menu.submenuSearch.dispatchedInputCount).toBe(1);
    expect(event.preventDefault).toHaveBeenCalled();
  });

  it("does not quick-filter Space or modified shortcuts", () => {
    const menu = buildMenu();
    const outside = button();
    setupGlobals(outside);
    outside.focus();

    expect(handleContextMenuQuickFilter(keyboardEvent(" "), menu.root as unknown as HTMLElement)).toBe(false);
    expect(handleContextMenuQuickFilter(keyboardEvent("x", { ctrlKey: true }), menu.root as unknown as HTMLElement)).toBe(false);
  });
});
