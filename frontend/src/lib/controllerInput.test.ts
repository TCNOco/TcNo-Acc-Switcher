import { afterEach, describe, expect, it, vi } from "vitest";

const runtimeMock = vi.hoisted(() => {
  const handlers = new Map<string, (event: { data: unknown }) => void>();
  return {
    handlers,
    Events: {
      On: vi.fn((name: string, handler: (event: { data: unknown }) => void) => {
        handlers.set(name, handler);
        return () => handlers.delete(name);
      }),
    },
  };
});

const securityMock = vi.hoisted(() => ({
  status: { appLocked: false },
}));

const inputModalityMock = vi.hoisted(() => ({
  markControllerInput: vi.fn(),
}));

vi.mock("@wailsio/runtime", () => runtimeMock);
vi.mock("./inputModality", () => inputModalityMock);
vi.mock("../stores/modal", () => ({
  activeModal: { subscribe: () => () => {} },
  cancelActiveModal: vi.fn(),
}));

vi.mock("../stores/contextMenu", () => ({
  contextMenu: { subscribe: () => () => {} },
  closeContextMenu: vi.fn(),
}));

vi.mock("../stores/nav", () => ({
  navigateBackLikeButton: vi.fn(),
  navigateForward: vi.fn(),
}));

vi.mock("../stores/searchOverlay", () => ({
  searchOverlayCtrl: {
    subscribe: (run: (value: { open: boolean }) => void) => {
      run({ open: false });
      return () => {};
    },
  },
  openSearchOverlay: vi.fn(),
  closeSearchOverlay: vi.fn(),
}));

vi.mock("../stores/security", () => ({
  securityStatus: {
    subscribe: (run: (value: { appLocked: boolean }) => void) => {
      run(securityMock.status);
      return () => {};
    },
  },
}));
import {
  CONTROLLER_ACTION_EVENT,
  advanceControllerPollState,
  createControllerInputController,
  createInitialControllerPollState,
  installControllerInput,
  type ControllerPollState,
} from "./controllerInput";
import { navigateBackLikeButton, navigateForward } from "../stores/nav";
import { openSearchOverlay } from "../stores/searchOverlay";

type MockGamepadButton = { pressed: boolean; value: number };
type MockGamepad = ReturnType<typeof snapshot>;

function button(pressed = false, value = pressed ? 1 : 0): MockGamepadButton {
  return { pressed, value };
}

function snapshot(opts: {
  buttons?: Record<number, MockGamepadButton>;
  axes?: number[];
  connected?: boolean;
} = {}) {
  const buttons = Array.from({ length: 16 }, (_, index) => opts.buttons?.[index] ?? button(false));
  return {
    connected: opts.connected ?? true,
    buttons,
    axes: opts.axes ?? [0, 0, 0, 0],
  };
}

function advance(
  state: ControllerPollState,
  now: number,
  gamepads: ReturnType<typeof snapshot>[],
) {
  return advanceControllerPollState(state, gamepads, now);
}

function installBrowserEnv(getGamepads: () => MockGamepad[]) {
  const fakeWindow = new EventTarget() as EventTarget & typeof globalThis & {
    requestAnimationFrame: (callback: FrameRequestCallback) => number;
    cancelAnimationFrame: (handle: number) => void;
  };
  const fakeDocument = new EventTarget() as EventTarget & {
    activeElement: Element | null;
  };

  fakeWindow.setTimeout = globalThis.setTimeout.bind(globalThis);
  fakeWindow.clearTimeout = globalThis.clearTimeout.bind(globalThis);
  fakeWindow.requestAnimationFrame = (_callback) => 1;
  fakeWindow.cancelAnimationFrame = () => {};
  fakeDocument.activeElement = null;

  vi.stubGlobal("window", fakeWindow);
  vi.stubGlobal("document", fakeDocument);
  vi.stubGlobal("navigator", { getGamepads });

  return { window: fakeWindow, document: fakeDocument };
}

function emitNativeControllerAction(data: unknown): void {
  const handler = runtimeMock.handlers.get(CONTROLLER_ACTION_EVENT);
  if (!handler) {
    throw new Error("controller action handler not registered");
  }
  handler({ data });
}

class FakeHTMLElement extends EventTarget {
  readonly isContentEditable = false;
  disabled = false;

  constructor(
    readonly tagName: string,
    private readonly rect: { left: number; top: number; width: number; height: number },
    private readonly attrs: Record<string, string | null> = {},
  ) {
    super();
  }

  getClientRects(): unknown[] {
    return [this.rect];
  }

  getBoundingClientRect(): DOMRect {
    return {
      ...this.rect,
      x: this.rect.left,
      y: this.rect.top,
      right: this.rect.left + this.rect.width,
      bottom: this.rect.top + this.rect.height,
      toJSON: () => this.rect,
    } as DOMRect;
  }

  matches(selector: string): boolean {
    return selector === "[hidden], [aria-hidden='true']" ? false : selector.toUpperCase() === this.tagName;
  }

  closest(_selector: string): null {
    return null;
  }

  getAttribute(name: string): string | null {
    return this.attrs[name] ?? null;
  }

  focus(): void {
    Object.defineProperty(document, "activeElement", { value: this, configurable: true });
  }
}

class FakeHTMLInputElement extends FakeHTMLElement {
  constructor(rect: { left: number; top: number; width: number; height: number }) {
    super("INPUT", rect);
  }
}

class FakeHTMLButtonElement extends FakeHTMLElement {
  readonly click = vi.fn();

  constructor(rect: { left: number; top: number; width: number; height: number }) {
    super("BUTTON", rect);
  }
}

class FakeHTMLAnchorElement extends FakeHTMLElement {}

function installFocusableBrowserEnv(getGamepads: () => MockGamepad[], candidates: FakeHTMLElement[]) {
  const env = installBrowserEnv(getGamepads);
  const body = new FakeHTMLElement("BODY", { left: 0, top: 0, width: 300, height: 300 });

  Object.assign(env.document, {
    body,
    querySelectorAll: () => candidates,
  });
  Object.defineProperty(env.document, "activeElement", { value: null, configurable: true });

  vi.stubGlobal("HTMLElement", FakeHTMLElement);
  vi.stubGlobal("HTMLInputElement", FakeHTMLInputElement);
  vi.stubGlobal("HTMLButtonElement", FakeHTMLButtonElement);
  vi.stubGlobal("HTMLAnchorElement", FakeHTMLAnchorElement);

  return env;
}

afterEach(() => {
  securityMock.status = { appLocked: false };
  runtimeMock.handlers.clear();
  runtimeMock.Events.On.mockClear();
  vi.restoreAllMocks();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("controller input polling", () => {
  it("maps face and shoulder buttons to one-shot actions", () => {
    const initial = createInitialControllerPollState();
    const pressed = snapshot({
      buttons: {
        0: button(true),
        1: button(true),
        3: button(true),
        4: button(true),
        5: button(true),
        8: button(true),
        9: button(true),
      },
    });

    const first = advance(initial, 0, [pressed]);
    expect(first.actions).toEqual([
      "activate",
      "back",
      "palette",
      "context",
      "historyBack",
      "historyForward",
    ]);

    const held = advance(first.state, 50, [pressed]);
    expect(held.actions).toEqual([]);
  });

  it("repeats d-pad directions after the initial delay", () => {
    const initial = createInitialControllerPollState();
    const dpadRight = snapshot({ buttons: { 15: button(true) } });

    const first = advance(initial, 0, [dpadRight]);
    expect(first.actions).toEqual(["right"]);

    const beforeRepeat = advance(first.state, 200, [dpadRight]);
    expect(beforeRepeat.actions).toEqual([]);

    const repeated = advance(beforeRepeat.state, 300, [dpadRight]);
    expect(repeated.actions).toEqual(["right"]);
  });

  it("uses the left stick only after the deadzone", () => {
    const initial = createInitialControllerPollState();

    const belowDeadzone = advance(initial, 0, [snapshot({ axes: [0.3, -0.35] })]);
    expect(belowDeadzone.actions).toEqual([]);

    const aboveDeadzone = advance(initial, 0, [snapshot({ axes: [-0.6, 0.7] })]);
    expect(aboveDeadzone.actions).toEqual(["down", "left"]);
  });

  it("clears held repeat state when the direction is released", () => {
    const initial = createInitialControllerPollState();
    const heldLeft = snapshot({ buttons: { 14: button(true) } });

    const first = advance(initial, 0, [heldLeft]);
    const released = advance(first.state, 100, [snapshot()]);
    const pressedAgain = advance(released.state, 150, [heldLeft]);

    expect(released.actions).toEqual([]);
    expect(pressedAgain.actions).toEqual(["left"]);
  });

  it("installs only while controller support is enabled", () => {
    const uninstall = vi.fn();
    const install = vi.fn(() => uninstall);
    const controller = createControllerInputController(install);

    controller.setEnabled(false);
    expect(install).not.toHaveBeenCalled();

    controller.setEnabled(true);
    controller.setEnabled(true);
    expect(install).toHaveBeenCalledTimes(1);
    expect(uninstall).not.toHaveBeenCalled();

    controller.setEnabled(false);
    controller.setEnabled(false);
    expect(uninstall).toHaveBeenCalledTimes(1);
  });

  it("can restart controller input after being disabled", () => {
    const uninstallFirst = vi.fn();
    const uninstallSecond = vi.fn();
    const install = vi.fn()
      .mockReturnValueOnce(uninstallFirst)
      .mockReturnValueOnce(uninstallSecond);
    const controller = createControllerInputController(install);

    controller.setEnabled(true);
    controller.setEnabled(false);
    controller.setEnabled(true);
    controller.destroy();

    expect(install).toHaveBeenCalledTimes(2);
    expect(uninstallFirst).toHaveBeenCalledTimes(1);
    expect(uninstallSecond).toHaveBeenCalledTimes(1);
  });

  it("keeps discovery alive until a controller appears and rechecks on focus or visibility changes", () => {
    vi.useFakeTimers();

    const getGamepads = vi.fn<() => ReturnType<typeof snapshot>[]>(() => []);
    const { window, document } = installBrowserEnv(getGamepads);
    const requestAnimationFrame = vi.spyOn(window, "requestAnimationFrame").mockReturnValue(1);

    const uninstall = installControllerInput();

    expect(runtimeMock.Events.On).toHaveBeenCalledWith(CONTROLLER_ACTION_EVENT, expect.any(Function));
    expect(requestAnimationFrame).not.toHaveBeenCalled();
    expect(getGamepads).toHaveBeenCalledTimes(2);

    vi.advanceTimersByTime(1000);
    expect(getGamepads).toHaveBeenCalledTimes(3);
    expect(requestAnimationFrame).not.toHaveBeenCalled();

    getGamepads.mockReturnValue([snapshot()]);
    window.dispatchEvent(new Event("focus"));
    expect(getGamepads).toHaveBeenCalledTimes(4);
    expect(requestAnimationFrame).toHaveBeenCalledTimes(1);

    requestAnimationFrame.mockClear();
    getGamepads.mockReturnValue([]);
    document.dispatchEvent(new Event("visibilitychange"));
    expect(getGamepads).toHaveBeenCalledTimes(5);
    expect(requestAnimationFrame).not.toHaveBeenCalled();

    uninstall();
  });

  it("resets polling on disconnect and cleans up scheduled work on uninstall", () => {
    vi.useFakeTimers();

    const getGamepads = vi.fn<() => ReturnType<typeof snapshot>[]>(() => [snapshot()]);
    const { window } = installBrowserEnv(getGamepads);
    const requestAnimationFrame = vi.spyOn(window, "requestAnimationFrame").mockReturnValue(7);
    const cancelAnimationFrame = vi.spyOn(window, "cancelAnimationFrame").mockImplementation(() => {});
    const clearTimeout = vi.spyOn(window, "clearTimeout");

    const uninstall = installControllerInput();
    expect(requestAnimationFrame).toHaveBeenCalledTimes(1);

    getGamepads.mockReturnValue([]);
    window.dispatchEvent(new Event("gamepaddisconnected"));
    expect(cancelAnimationFrame).toHaveBeenCalledWith(7);

    uninstall();
    expect(clearTimeout).toHaveBeenCalled();

    requestAnimationFrame.mockClear();
    vi.advanceTimersByTime(1000);
    expect(requestAnimationFrame).not.toHaveBeenCalled();
  });

  it("handles native controller actions with string and object payloads", () => {
    installFocusableBrowserEnv(() => [], []);
    const uninstall = installControllerInput();

    emitNativeControllerAction("historyBack");
    emitNativeControllerAction({ action: "historyForward" });
    emitNativeControllerAction({ action: "palette" });
    emitNativeControllerAction("nope");
    emitNativeControllerAction({ action: "still-nope" });

    expect(navigateBackLikeButton).toHaveBeenCalledTimes(1);
    expect(navigateForward).toHaveBeenCalledTimes(1);
    expect(openSearchOverlay).toHaveBeenCalledWith(">");
    expect(inputModalityMock.markControllerInput).toHaveBeenCalledTimes(3);

    uninstall();
    expect(runtimeMock.handlers.has(CONTROLLER_ACTION_EVENT)).toBe(false);
  });

  it("allows controller navigation and activation while the app lock overlay is active", () => {
    securityMock.status = { appLocked: true };

    let pads: MockGamepad[] = [];
    let frame: FrameRequestCallback | null = null;
    const passwordInput = new FakeHTMLInputElement({ left: 20, top: 20, width: 180, height: 36 });
    const unlockButton = new FakeHTMLButtonElement({ left: 20, top: 80, width: 120, height: 36 });
    const { window } = installFocusableBrowserEnv(() => pads, [passwordInput, unlockButton]);
    vi.spyOn(window, "requestAnimationFrame").mockImplementation((callback) => {
      frame = callback;
      return 1;
    });
    const runFrame = (now: number): void => {
      expect(frame).not.toBeNull();
      (frame as FrameRequestCallback)(now);
    };

    pads = [snapshot({ buttons: { 13: button(true) } })];
    const uninstall = installControllerInput();
    runFrame(0);
    expect(document.activeElement).toBe(passwordInput);

    pads = [snapshot()];
    runFrame(16);
    pads = [snapshot({ buttons: { 13: button(true) } })];
    runFrame(32);
    expect(document.activeElement).toBe(unlockButton);

    pads = [snapshot()];
    runFrame(48);
    pads = [snapshot({ buttons: { 0: button(true) } })];
    runFrame(64);
    expect(unlockButton.click).toHaveBeenCalledTimes(1);

    uninstall();
  });

  it("allows native controller navigation and activation while the app lock overlay is active", () => {
    securityMock.status = { appLocked: true };

    const passwordInput = new FakeHTMLInputElement({ left: 20, top: 20, width: 180, height: 36 });
    const unlockButton = new FakeHTMLButtonElement({ left: 20, top: 80, width: 120, height: 36 });
    installFocusableBrowserEnv(() => [], [passwordInput, unlockButton]);

    const uninstall = installControllerInput();

    emitNativeControllerAction("down");
    expect(document.activeElement).toBe(passwordInput);

    emitNativeControllerAction({ action: "down" });
    expect(document.activeElement).toBe(unlockButton);

    emitNativeControllerAction("activate");
    expect(unlockButton.click).toHaveBeenCalledTimes(1);

    uninstall();
  });
});
