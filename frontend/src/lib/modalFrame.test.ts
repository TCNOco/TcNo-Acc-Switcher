import { afterEach, describe, expect, it, vi } from "vitest";
import { startModalDrag, startModalResize, type ModalFrameRect } from "./modalFrame";

function pointerEvent(
  type: string,
  props: Partial<PointerEvent> & { pointerId: number; clientX: number; clientY: number },
): PointerEvent {
  const event = new Event(type) as PointerEvent;
  for (const [key, value] of Object.entries(props)) {
    Object.defineProperty(event, key, { value, configurable: true });
  }
  Object.defineProperty(event, "preventDefault", { value: vi.fn(), configurable: true });
  Object.defineProperty(event, "stopPropagation", { value: vi.fn(), configurable: true });
  return event;
}

function setupPointerGlobals(): EventTarget {
  const windowTarget = new EventTarget();
  vi.stubGlobal("window", windowTarget);
  vi.stubGlobal("document", {
    body: {
      style: {
        userSelect: "",
        cursor: "",
      },
    },
  });
  return windowTarget;
}

function pointerTarget() {
  return {
    setPointerCapture: vi.fn(),
    releasePointerCapture: vi.fn(),
  };
}

describe("modalFrame pointer sessions", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("drags from window-level pointer movement", () => {
    const windowTarget = setupPointerGlobals();
    class TestElement {
      closest() {
        return null;
      }
    }
    vi.stubGlobal("Element", TestElement);

    const header = pointerTarget();
    const updates: ModalFrameRect[] = [];

    startModalDrag(
      pointerEvent("pointerdown", {
        button: 0,
        pointerId: 7,
        clientX: 100,
        clientY: 80,
        target: new TestElement() as unknown as Element,
      }),
      header as unknown as HTMLElement,
      { left: 100, top: 120, width: 400, height: 260 },
      {
        bounds: { width: 1000, height: 800, pad: 16 },
        minSize: { minW: 320, minH: 160 },
        onUpdate: (rect) => updates.push(rect),
      },
    );

    windowTarget.dispatchEvent(pointerEvent("pointermove", { pointerId: 7, clientX: 130, clientY: 130 }));

    expect(updates.at(-1)).toMatchObject({ left: 130, top: 170 });
  });

  it("resizes vertically from window-level pointer movement", () => {
    const windowTarget = setupPointerGlobals();
    const handle = pointerTarget();
    const updates: ModalFrameRect[] = [];

    startModalResize(
      pointerEvent("pointerdown", { button: 0, pointerId: 12, clientX: 400, clientY: 300 }),
      handle as unknown as HTMLElement,
      "s",
      { left: 100, top: 120, width: 400, height: 260 },
      {
        bounds: { width: 1000, height: 800, pad: 16 },
        minSize: { minW: 320, minH: 160 },
        onUpdate: (rect) => updates.push(rect),
      },
    );

    windowTarget.dispatchEvent(pointerEvent("pointermove", { pointerId: 12, clientX: 400, clientY: 390 }));
    windowTarget.dispatchEvent(pointerEvent("pointerup", { pointerId: 12, clientX: 400, clientY: 390 }));
    windowTarget.dispatchEvent(pointerEvent("pointermove", { pointerId: 12, clientX: 400, clientY: 420 }));

    expect(updates).toHaveLength(1);
    expect(updates[0]).toMatchObject({ left: 100, top: 120, width: 400, height: 350 });
    expect(handle.releasePointerCapture).toHaveBeenCalledWith(12);
  });
});
