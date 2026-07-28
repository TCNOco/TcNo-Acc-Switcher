import { afterEach, describe, expect, it, vi } from "vitest";
import {
  measureModalNaturalSize,
  startModalDrag,
  startModalResize,
  type ModalFrameRect,
} from "./modalFrame";

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

class FakeElement {
  offsetWidth = 0;
  offsetHeight = 0;
  scrollWidth = 0;
  scrollHeight = 0;
  children: FakeElement[] = [];
}

/** Frame chrome with one body child; `body` sizes describe what the browser would report. */
function fakeModal(body: Partial<FakeElement>): FakeElement {
  const child = Object.assign(new FakeElement(), body);
  const scroll = Object.assign(new FakeElement(), { children: [child] });
  const header = Object.assign(new FakeElement(), { offsetHeight: 32 });
  const modalFg = new FakeElement() as FakeElement & {
    querySelector: (selector: string) => FakeElement | null;
  };
  modalFg.querySelector = (selector: string) =>
    selector === ".modal-scroll" ? scroll : selector === ".modal-headerbar" ? header : null;
  return modalFg;
}

function stubMeasurementGlobals(): void {
  vi.stubGlobal("HTMLElement", FakeElement);
  vi.stubGlobal("getComputedStyle", () => ({
    paddingLeft: "24px",
    paddingRight: "24px",
    paddingTop: "20px",
    paddingBottom: "20px",
    borderLeftWidth: "1px",
    borderRightWidth: "1px",
    borderTopWidth: "1px",
    borderBottomWidth: "1px",
    rowGap: "normal",
    gap: "normal",
  }));
}

describe("measureModalNaturalSize", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("adds chrome to the body's own box", () => {
    stubMeasurementGlobals();

    expect(
      measureModalNaturalSize(
        fakeModal({ offsetWidth: 480, scrollWidth: 480, offsetHeight: 300, scrollHeight: 300 }) as unknown as HTMLElement,
      ),
    ).toEqual({ width: 530, height: 374 });
  });

  it("measures the overflowing content of a clamped body, not its clamped box", () => {
    stubMeasurementGlobals();

    // A body clamped by max-height (or holding an inner scroller) reports the clamped
    // offsetHeight; without scrollHeight the frame would freeze at its current height.
    expect(
      measureModalNaturalSize(
        fakeModal({ offsetWidth: 480, scrollWidth: 480, offsetHeight: 120, scrollHeight: 420 }) as unknown as HTMLElement,
      ).height,
    ).toBe(494);
  });
});

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
