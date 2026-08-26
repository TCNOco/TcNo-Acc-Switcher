import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { focusOnShow } from "./focusOnShow";

type Listener = () => void;

/**
 * Enough of a document and an element to drive the action's decisions - jsdom is
 * not configured for this project.
 */
function harness(options: { disabled?: boolean } = {}) {
  const listeners = new Map<string, Set<Listener>>();
  let active: unknown = null;
  let focusCalls = 0;

  const doc = {
    get activeElement() {
      return active;
    },
    addEventListener(type: string, handler: Listener) {
      if (!listeners.has(type)) listeners.set(type, new Set());
      listeners.get(type)!.add(handler);
    },
    removeEventListener(type: string, handler: Listener) {
      listeners.get(type)?.delete(handler);
    },
  };

  const node = {
    ownerDocument: doc,
    isConnected: true,
    disabled: options.disabled ?? false,
    focus() {
      focusCalls += 1;
      if (!node.disabled) active = node;
    },
  };

  return {
    node: node as unknown as HTMLElement,
    focusCalls: () => focusCalls,
    isFocused: () => active === node,
    setDisabled(value: boolean) {
      node.disabled = value;
    },
    stealFocus() {
      active = { other: true };
    },
    userDoes(type: "pointerdown" | "keydown") {
      for (const handler of listeners.get(type) ?? []) handler();
    },
    listenerCount() {
      let total = 0;
      for (const set of listeners.values()) total += set.size;
      return total;
    },
  };
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.stubGlobal("requestAnimationFrame", (callback: () => void) => setTimeout(callback, 0) as unknown as number);
  vi.stubGlobal("cancelAnimationFrame", (handle: number) => clearTimeout(handle));
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("focusOnShow", () => {
  it("takes the caret back when something else settles on top of it", () => {
    const dom = harness();
    focusOnShow(dom.node);
    vi.advanceTimersByTime(1);
    expect(dom.isFocused()).toBe(true);

    // Stands in for a modal focus trap or a context menu restoring focus, both of
    // which land a frame or two after the field appears.
    dom.stealFocus();
    vi.advanceTimersByTime(100);
    expect(dom.isFocused()).toBe(true);
  });

  it("leaves focus alone once the user has touched anything", () => {
    const dom = harness();
    focusOnShow(dom.node);
    vi.advanceTimersByTime(1);

    dom.userDoes("pointerdown");
    dom.stealFocus();
    vi.advanceTimersByTime(500);
    expect(dom.isFocused()).toBe(false);
  });

  it("stops claiming once the window has passed", () => {
    const dom = harness();
    focusOnShow(dom.node);
    vi.advanceTimersByTime(500);
    const before = dom.focusCalls();

    dom.stealFocus();
    vi.advanceTimersByTime(5_000);
    expect(dom.focusCalls()).toBe(before);
    expect(dom.listenerCount()).toBe(0);
  });

  // The unlock field is disabled while the vault is being opened, and a disabled
  // input cannot take focus - so the first try has to be allowed to fail.
  it("keeps trying while the field is still disabled", () => {
    const dom = harness({ disabled: true });
    focusOnShow(dom.node, true);
    vi.advanceTimersByTime(50);
    expect(dom.isFocused()).toBe(false);

    dom.setDisabled(false);
    vi.advanceTimersByTime(150);
    expect(dom.isFocused()).toBe(true);
  });

  it("waits for its parameter before claiming anything", () => {
    const dom = harness();
    const action = focusOnShow(dom.node, false);
    vi.advanceTimersByTime(500);
    expect(dom.focusCalls()).toBe(0);

    action.update?.(true);
    vi.advanceTimersByTime(1);
    expect(dom.isFocused()).toBe(true);
  });
});
