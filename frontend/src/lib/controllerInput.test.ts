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

const inputModalityMock = vi.hoisted(() => {
  const mock = {
    currentModality: "pointer" as "keyboard" | "pointer" | "controller",
    getInputModality: vi.fn(() => mock.currentModality),
    markControllerInput: vi.fn(() => {
      mock.currentModality = "controller";
    }),
    markSyntheticControllerKeyEvent: vi.fn((event: KeyboardEvent) => {
      Object.defineProperty(event, "__tcnoControllerKeyEvent", { value: true, configurable: true });
    }),
    inferControllerGlyphScheme: vi.fn((id: string) => {
      const normalized = id.toLowerCase();
      if (normalized.includes("sony") || normalized.includes("dualsense")) return "playstation";
      if (normalized.includes("nintendo") || normalized.includes("switch")) return "nintendo";
      if (normalized.includes("xbox")) return "xbox";
      return "generic";
    }),
  };
  return mock;
});

const contextMenuMock = vi.hoisted(() => ({
  value: null as unknown,
  closeContextMenu: vi.fn(),
}));

vi.mock("@wailsio/runtime", () => runtimeMock);
vi.mock("./inputModality", () => inputModalityMock);
vi.mock("../stores/modal", () => ({
  activeModal: { subscribe: () => () => {} },
  cancelActiveModal: vi.fn(),
}));

vi.mock("../stores/contextMenu", () => ({
  contextMenu: {
    subscribe: (run: (value: unknown) => void) => {
      run(contextMenuMock.value);
      return () => {};
    },
  },
  closeContextMenu: contextMenuMock.closeContextMenu,
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
import { closeContextMenu } from "../stores/contextMenu";

type MockGamepadButton = { pressed: boolean; value: number };
type MockGamepad = ReturnType<typeof snapshot>;

function button(pressed = false, value = pressed ? 1 : 0): MockGamepadButton {
  return { pressed, value };
}

function snapshot(opts: {
  buttons?: Record<number, MockGamepadButton>;
  axes?: number[];
  connected?: boolean;
  id?: string;
} = {}) {
  const buttons = Array.from({ length: 16 }, (_, index) => opts.buttons?.[index] ?? button(false));
  return {
    id: opts.id ?? "Xbox Wireless Controller",
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
  vi.stubGlobal("KeyboardEvent", class FakeKeyboardEvent extends Event {
    readonly key: string;
    readonly altKey: boolean;
    readonly ctrlKey: boolean;
    readonly metaKey: boolean;
    readonly shiftKey: boolean;

    constructor(type: string, init: KeyboardEventInit = {}) {
      super(type, init);
      this.key = init.key ?? "";
      this.altKey = init.altKey ?? false;
      this.ctrlKey = init.ctrlKey ?? false;
      this.metaKey = init.metaKey ?? false;
      this.shiftKey = init.shiftKey ?? false;
    }
  });

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
  scrollIntoView = vi.fn();

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

  querySelectorAll(_selector: string): FakeHTMLElement[] {
    return [];
  }

  getAttribute(name: string): string | null {
    return this.attrs[name] ?? null;
  }

  focus(): void {
    Object.defineProperty(document, "activeElement", { value: this, configurable: true });
  }
}

class FakeHTMLInputElement extends FakeHTMLElement {
  type = "text";
  readonly click = vi.fn();

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

function installFocusableBrowserEnv(
  getGamepads: () => MockGamepad[],
  candidates: FakeHTMLElement[],
  querySelectorAllImpl?: (selector: string) => FakeHTMLElement[],
) {
  const env = installBrowserEnv(getGamepads);
  const body = new FakeHTMLElement("BODY", { left: 0, top: 0, width: 300, height: 300 });

  Object.assign(env.document, {
    body,
    querySelectorAll: (selector: string) => querySelectorAllImpl?.(selector) ?? candidates,
    querySelector: (selector: string) => querySelectorAllImpl?.(selector)[0] ?? null,
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
  contextMenuMock.value = null;
  contextMenuMock.closeContextMenu.mockClear();
  inputModalityMock.currentModality = "pointer";
  runtimeMock.handlers.clear();
  runtimeMock.Events.On.mockClear();
  inputModalityMock.markSyntheticControllerKeyEvent.mockClear();
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
    emitNativeControllerAction({ action: "palette", id: "Sony DualSense Wireless Controller" });
    emitNativeControllerAction("nope");
    emitNativeControllerAction({ action: "still-nope" });

    expect(navigateBackLikeButton).toHaveBeenCalledTimes(1);
    expect(navigateForward).toHaveBeenCalledTimes(1);
    expect(openSearchOverlay).toHaveBeenCalledWith(">");
    expect(inputModalityMock.markControllerInput).toHaveBeenCalledTimes(3);
    expect(inputModalityMock.markControllerInput).toHaveBeenLastCalledWith("playstation");

    uninstall();
    expect(runtimeMock.handlers.has(CONTROLLER_ACTION_EVENT)).toBe(false);
  });

  it("focuses the first platform or account cell on the first controller interaction", () => {
    const firstCell = new FakeHTMLElement("DIV", { left: 20, top: 20, width: 100, height: 80 });
    const secondCell = new FakeHTMLElement("DIV", { left: 140, top: 20, width: 100, height: 80 });
    installFocusableBrowserEnv(
      () => [],
      [firstCell, secondCell],
      (selector) => selector.includes("[data-dnd-cell]") ? [firstCell, secondCell] : [firstCell, secondCell],
    );

    const uninstall = installControllerInput();
    emitNativeControllerAction("right");

    expect(document.activeElement).toBe(firstCell);
    uninstall();
  });

  it("does not reprime the first grid cell after controller focus is established", () => {
    inputModalityMock.currentModality = "controller";
    const firstCell = new FakeHTMLElement("DIV", { left: 20, top: 20, width: 100, height: 80 });
    const secondCell = new FakeHTMLElement("DIV", { left: 140, top: 20, width: 100, height: 80 });
    installFocusableBrowserEnv(
      () => [],
      [firstCell, secondCell],
      (selector) => selector.includes("[data-dnd-cell]") ? [firstCell, secondCell] : [firstCell, secondCell],
    );
    firstCell.focus();

    const uninstall = installControllerInput();
    emitNativeControllerAction("right");

    expect(document.activeElement).toBe(secondCell);
    uninstall();
  });

  it("enters settings spatial navigation from outside focused chrome controls", () => {
    inputModalityMock.currentModality = "controller";
    const backButton = new FakeHTMLButtonElement({ left: 0, top: 0, width: 46, height: 32 });
    const settingsRoot = new FakeHTMLElement("DIV", { left: 0, top: 40, width: 360, height: 420 });
    const firstSetting = new FakeHTMLInputElement({ left: 20, top: 80, width: 32, height: 32 });
    firstSetting.type = "checkbox";
    (settingsRoot as unknown as { querySelectorAll: (selector: string) => FakeHTMLElement[] }).querySelectorAll = () => [firstSetting];
    installFocusableBrowserEnv(
      () => [],
      [backButton],
      (selector) => {
        if (selector === "[data-controller-spatial-nav]") return [settingsRoot];
        if (selector.includes("[data-dnd-cell]")) return [];
        return [backButton];
      },
    );
    backButton.focus();

    const uninstall = installControllerInput();
    emitNativeControllerAction("down");

    expect(document.activeElement).toBe(firstSetting);
    uninstall();
  });

  it("keeps horizontal fallback on the same visual row", () => {
    const active = new FakeHTMLButtonElement({ left: 200, top: 220, width: 40, height: 40 });
    const aboveRight = new FakeHTMLButtonElement({ left: 220, top: 80, width: 40, height: 40 });
    const sameRowRight = new FakeHTMLButtonElement({ left: 270, top: 220, width: 40, height: 40 });
    installFocusableBrowserEnv(() => [], [active, aboveRight, sameRowRight]);
    active.focus();

    const uninstall = installControllerInput();
    emitNativeControllerAction("right");

    expect(document.activeElement).toBe(sameRowRight);
    uninstall();
  });

  it("deduplicates native and browser-polled direction actions from the same controller press", () => {
    let pads: MockGamepad[] = [snapshot({ buttons: { 15: button(true) } })];
    let frame: FrameRequestCallback | null = null;
    const active = new FakeHTMLButtonElement({ left: 20, top: 20, width: 40, height: 40 });
    const rightOne = new FakeHTMLButtonElement({ left: 90, top: 20, width: 40, height: 40 });
    const rightTwo = new FakeHTMLButtonElement({ left: 160, top: 20, width: 40, height: 40 });
    const { window } = installFocusableBrowserEnv(() => pads, [active, rightOne, rightTwo]);
    vi.spyOn(performance, "now").mockReturnValue(1000);
    vi.spyOn(window, "requestAnimationFrame").mockImplementation((callback) => {
      frame = callback;
      return 1;
    });
    active.focus();

    const uninstall = installControllerInput();
    emitNativeControllerAction("right");
    expect(document.activeElement).toBe(rightOne);

    expect(frame).not.toBeNull();
    const runFrame = frame as unknown as FrameRequestCallback;
    runFrame(1000);
    expect(document.activeElement).toBe(rightOne);

    pads = [snapshot()];
    uninstall();
  });

  it("returns from bottom actionbar controls to the closest bottom-row account cell on controller up", () => {
    const actionbar = new FakeHTMLElement("DIV", { left: 0, top: 210, width: 340, height: 50 });
    const active = new FakeHTMLButtonElement({ left: 235, top: 220, width: 40, height: 40 });
    active.closest = ((selector: string) =>
      selector.includes(".actionbar__actions") ? actionbar : null) as typeof active.closest;

    const topLeft = new FakeHTMLElement("DIV", { left: 20, top: 20, width: 100, height: 80 });
    const bottomLeft = new FakeHTMLElement("DIV", { left: 20, top: 125, width: 100, height: 80 });
    const bottomRight = new FakeHTMLElement("DIV", { left: 220, top: 125, width: 100, height: 80 });
    installFocusableBrowserEnv(
      () => [],
      [topLeft, bottomLeft, bottomRight, active],
      (selector) => selector.includes("[data-dnd-cell]") ? [topLeft, bottomLeft, bottomRight] : [topLeft, bottomLeft, bottomRight, active],
    );
    active.focus();

    const uninstall = installControllerInput();
    emitNativeControllerAction("up");

    expect(document.activeElement).toBe(bottomRight);
    uninstall();
  });

  it("lets Back close focused popups that handle Escape before navigating back", () => {
    const focused = new FakeHTMLButtonElement({ left: 20, top: 20, width: 40, height: 40 });
    installFocusableBrowserEnv(() => [], [focused]);
    focused.addEventListener("keydown", (event) => {
      if ((event as KeyboardEvent).key === "Escape") {
        event.preventDefault();
      }
    });
    focused.focus();

    const uninstall = installControllerInput();
    emitNativeControllerAction("back");

    expect(navigateBackLikeButton).not.toHaveBeenCalled();
    uninstall();
  });

  it("lets Back close a context menu even when its search input has focus", () => {
    contextMenuMock.value = { x: 10, y: 10, items: [{ label: "Rename" }] };
    const search = new FakeHTMLInputElement({ left: 20, top: 20, width: 160, height: 32 });
    installFocusableBrowserEnv(() => [], [search]);
    search.focus();

    const uninstall = installControllerInput();
    emitNativeControllerAction("back");

    expect(closeContextMenu).toHaveBeenCalledTimes(1);
    expect(navigateBackLikeButton).not.toHaveBeenCalled();
    uninstall();
  });

  it("dispatches controller directions to focused settings inputs inside spatial navigation roots", () => {
    const root = new FakeHTMLElement("DIV", { left: 0, top: 0, width: 320, height: 240 });
    const input = new FakeHTMLInputElement({ left: 20, top: 20, width: 160, height: 32 });
    input.closest = ((selector: string) =>
      selector === "[data-controller-spatial-nav]" ? root : null) as typeof input.closest;
    const onKeydown = vi.fn((event: Event) => {
      if ((event as KeyboardEvent).key === "ArrowDown") {
        event.preventDefault();
      }
    });
    input.addEventListener("keydown", onKeydown);
    installFocusableBrowserEnv(() => [], [input]);
    input.focus();

    const uninstall = installControllerInput();
    emitNativeControllerAction("down");

    expect(onKeydown).toHaveBeenCalledTimes(1);
    expect((onKeydown.mock.calls[0][0] as KeyboardEvent).key).toBe("ArrowDown");
    uninstall();
  });

  it("activates focused settings checkboxes inside spatial navigation roots", () => {
    const root = new FakeHTMLElement("DIV", { left: 0, top: 0, width: 320, height: 240 });
    const checkbox = new FakeHTMLInputElement({ left: 20, top: 20, width: 32, height: 32 });
    checkbox.type = "checkbox";
    checkbox.closest = ((selector: string) =>
      selector === "[data-controller-spatial-nav]" ? root : null) as typeof checkbox.closest;
    installFocusableBrowserEnv(() => [], [checkbox]);
    checkbox.focus();

    const uninstall = installControllerInput();
    emitNativeControllerAction("activate");

    expect(checkbox.click).toHaveBeenCalledTimes(1);
    uninstall();
  });

  it("passes the connected gamepad glyph scheme when polling actions", () => {
    let pads: MockGamepad[] = [
      snapshot({
        id: "Nintendo Switch Pro Controller",
        buttons: { 0: button(true) },
      }),
    ];
    let frame: FrameRequestCallback | null = null;
    const rightButton = new FakeHTMLButtonElement({ left: 80, top: 20, width: 40, height: 40 });
    const { window } = installFocusableBrowserEnv(() => pads, [rightButton]);
    vi.spyOn(window, "requestAnimationFrame").mockImplementation((callback) => {
      frame = callback;
      return 1;
    });

    const uninstall = installControllerInput();
    expect(frame).not.toBeNull();
    const runFrame = frame as unknown as FrameRequestCallback;
    runFrame(0);

    expect(inputModalityMock.markControllerInput).toHaveBeenCalledWith("nintendo");

    uninstall();
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
