import { afterEach, describe, expect, it, vi } from "vitest";
import { get } from "svelte/store";
import { contextMenu as contextMenuAction } from "./contextMenu";
import {
  closeContextMenu,
  contextMenu,
  openContextMenuAtRect,
  type ContextMenuAnchorRect,
} from "../../stores/contextMenu";

class FakeElement {
  private listeners = new Map<string, EventListener[]>();
  public parent: FakeElement | null = null;
  public dndCell = false;

  constructor(private readonly rect: ContextMenuAnchorRect) {}

  addEventListener(type: string, listener: EventListener): void {
    const list = this.listeners.get(type) ?? [];
    list.push(listener);
    this.listeners.set(type, list);
  }

  removeEventListener(type: string, listener: EventListener): void {
    const list = this.listeners.get(type) ?? [];
    this.listeners.set(
      type,
      list.filter((entry) => entry !== listener),
    );
  }

  emit(type: string, event: Event): void {
    for (const listener of this.listeners.get(type) ?? []) {
      listener.call(this, event);
    }
  }

  getBoundingClientRect(): ContextMenuAnchorRect {
    return this.rect;
  }

  closest(selector: string): FakeElement | null {
    if (selector !== "[data-dnd-cell]") {
      return null;
    }
    let el: FakeElement | null = this;
    while (el) {
      if (el.dndCell) {
        return el;
      }
      el = el.parent;
    }
    return null;
  }
}

function mouseEvent(type: string, props: Partial<MouseEvent>): MouseEvent {
  const event = new Event(type) as MouseEvent;
  for (const [key, value] of Object.entries(props)) {
    Object.defineProperty(event, key, { value, configurable: true });
  }
  Object.defineProperty(event, "preventDefault", { value: vi.fn(), configurable: true });
  Object.defineProperty(event, "stopPropagation", { value: vi.fn(), configurable: true });
  return event;
}

function keyEvent(key: string, target: FakeElement, props: Partial<KeyboardEvent> = {}): KeyboardEvent {
  const event = new Event("keydown") as KeyboardEvent;
  Object.defineProperty(event, "key", { value: key, configurable: true });
  Object.defineProperty(event, "target", { value: target, configurable: true });
  Object.defineProperty(event, "shiftKey", { value: false, configurable: true });
  for (const [name, value] of Object.entries(props)) {
    Object.defineProperty(event, name, { value, configurable: true });
  }
  Object.defineProperty(event, "preventDefault", { value: vi.fn(), configurable: true });
  Object.defineProperty(event, "stopPropagation", { value: vi.fn(), configurable: true });
  return event;
}

describe("contextMenu action", () => {
  afterEach(() => {
    closeContextMenu();
    vi.unstubAllGlobals();
  });

  it("opens from pointer coordinates on contextmenu", () => {
    const node = new FakeElement({
      left: 10,
      right: 30,
      top: 20,
      bottom: 40,
      width: 20,
      height: 20,
    });
    vi.stubGlobal("HTMLElement", FakeElement);

    const action = contextMenuAction(node as unknown as HTMLElement, () => [{ label: "Open" }]);
    node.emit(
      "contextmenu",
      mouseEvent("contextmenu", {
        clientX: 120,
        clientY: 240,
        ctrlKey: false,
      }),
    );

    expect(get(contextMenu)).toMatchObject({
      x: 120,
      y: 240,
      items: [{ label: "Open" }],
    });

    action?.destroy?.();
  });

  it("opens from the focused element rectangle on Shift+F10 and ContextMenu", () => {
    const node = new FakeElement({
      left: 10,
      right: 50,
      top: 20,
      bottom: 60,
      width: 40,
      height: 40,
    });
    const child = new FakeElement({
      left: 100,
      right: 180,
      top: 200,
      bottom: 224,
      width: 80,
      height: 24,
    });
    vi.stubGlobal("HTMLElement", FakeElement);

    const action = contextMenuAction(node as unknown as HTMLElement, () => [{ label: "Inspect" }]);
    node.emit("keydown", keyEvent("F10", child, { shiftKey: true }));

    expect(get(contextMenu)).toMatchObject({
      x: 100,
      y: 224,
      items: [{ label: "Inspect" }],
      anchorRect: child.getBoundingClientRect(),
    });

    closeContextMenu();
    node.emit("keydown", keyEvent("ContextMenu", child));

    expect(get(contextMenu)?.anchorRect).toEqual(child.getBoundingClientRect());

    action?.destroy?.();
  });

  it("opens when the keyboard context-menu key lands on a focusable reorder cell ancestor", () => {
    const wrapper = new FakeElement({
      left: 30,
      right: 130,
      top: 40,
      bottom: 96,
      width: 100,
      height: 56,
    });
    const node = new FakeElement({
      left: 35,
      right: 125,
      top: 45,
      bottom: 91,
      width: 90,
      height: 46,
    });
    wrapper.dndCell = true;
    node.parent = wrapper;
    vi.stubGlobal("HTMLElement", FakeElement);

    const action = contextMenuAction(node as unknown as HTMLElement, () => [{ label: "Open app menu" }]);
    wrapper.emit("keydown", keyEvent("ContextMenu", wrapper));

    expect(get(contextMenu)).toMatchObject({
      x: 30,
      y: 96,
      items: [{ label: "Open app menu" }],
      anchorRect: wrapper.getBoundingClientRect(),
    });

    action?.destroy?.();
  });
});

describe("contextMenu store", () => {
  afterEach(() => {
    closeContextMenu();
  });

  it("stores anchor rectangles for keyboard-opened menus", () => {
    const rect: ContextMenuAnchorRect = {
      left: 40,
      right: 140,
      top: 75,
      bottom: 99,
      width: 100,
      height: 24,
    };

    openContextMenuAtRect(rect, [{ label: "Open" }]);

    expect(get(contextMenu)).toEqual({
      x: rect.left,
      y: rect.bottom,
      items: [{ label: "Open" }],
      anchorRect: rect,
    });
  });
});
