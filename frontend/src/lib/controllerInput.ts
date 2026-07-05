import { Events } from "@wailsio/runtime";
import { get } from "svelte/store";
import { activeModal, cancelActiveModal } from "../stores/modal";
import { contextMenu, closeContextMenu } from "../stores/contextMenu";
import { navigateBackLikeButton, navigateForward } from "../stores/nav";
import { searchOverlayCtrl, openSearchOverlay, closeSearchOverlay } from "../stores/searchOverlay";
import { securityStatus } from "../stores/security";
import {
  inferControllerGlyphScheme,
  getInputModality,
  markControllerInput,
  markSyntheticControllerKeyEvent,
  type ControllerGlyphScheme,
} from "./inputModality";
import { moveFocusSpatially } from "./spatialFocus";
import { focusControllerSpatialNavigationTarget } from "./actions/controllerSpatialNavigation";

const BUTTON_A = 0;
const BUTTON_B = 1;
const BUTTON_Y = 3;
const BUTTON_LB = 4;
const BUTTON_RB = 5;
const BUTTON_VIEW = 8;
const BUTTON_MENU = 9;
const BUTTON_DPAD_UP = 12;
const BUTTON_DPAD_DOWN = 13;
const BUTTON_DPAD_LEFT = 14;
const BUTTON_DPAD_RIGHT = 15;

const LEFT_STICK_X = 0;
const LEFT_STICK_Y = 1;

const STICK_DEADZONE = 0.45;
const INITIAL_REPEAT_MS = 280;
const REPEAT_MS = 120;
const CROSS_SOURCE_DUPLICATE_ACTION_SUPPRESS_MS = 180;
const DISCOVERY_INTERVAL_MS = 1000;
const CONTROLLER_SPATIAL_NAV_SELECTOR = "[data-controller-spatial-nav]";
const PRIMARY_GRID_SELECTOR = ".platform_list [data-dnd-cell][data-dnd-name], .acc_list [data-dnd-cell][data-dnd-name]";
export type ControllerAction =
  | "up"
  | "down"
  | "left"
  | "right"
  | "activate"
  | "back"
  | "palette"
  | "context"
  | "historyBack"
  | "historyForward";

export const CONTROLLER_ACTION_EVENT = "controller:action";

type RepeatableAction = Extract<ControllerAction, "up" | "down" | "left" | "right">;

type GamepadLike = Pick<Gamepad, "connected" | "axes"> & {
  id?: string;
  buttons: readonly Pick<GamepadButton, "pressed" | "value">[];
};

type ParsedControllerAction = {
  action: ControllerAction;
  scheme?: ControllerGlyphScheme;
};

type ControllerActionSource = "native" | "poll";

export type ControllerPollState = {
  held: Record<ControllerAction, boolean>;
  nextRepeatAt: Record<RepeatableAction, number>;
};

const ACTIONS: ControllerAction[] = [
  "up",
  "down",
  "left",
  "right",
  "activate",
  "back",
  "palette",
  "context",
  "historyBack",
  "historyForward",
];

const REPEATABLE_ACTIONS: RepeatableAction[] = ["up", "down", "left", "right"];

function isControllerAction(value: unknown): value is ControllerAction {
  return typeof value === "string" && (ACTIONS as string[]).includes(value);
}

function isControllerGlyphScheme(value: unknown): value is ControllerGlyphScheme {
  return value === "xbox" || value === "playstation" || value === "nintendo" || value === "generic";
}

function readControllerSchemePayload(payload: Record<string, unknown>): ControllerGlyphScheme | undefined {
  const explicit = payload.scheme ?? payload.controllerScheme ?? payload.controllerType;
  if (isControllerGlyphScheme(explicit)) {
    return explicit;
  }
  const id = payload.id ?? payload.controllerId ?? payload.gamepadId;
  return typeof id === "string" ? inferControllerGlyphScheme(id) : undefined;
}

function parseNativeControllerAction(payload: unknown): ParsedControllerAction | null {
  if (isControllerAction(payload)) {
    return { action: payload };
  }
  if (typeof payload === "object" && payload && "action" in payload) {
    const record = payload as Record<string, unknown>;
    const action = record.action;
    return isControllerAction(action)
      ? { action, scheme: readControllerSchemePayload(record) }
      : null;
  }
  return null;
}

export function createInitialControllerPollState(): ControllerPollState {
  return {
    held: {
      up: false,
      down: false,
      left: false,
      right: false,
      activate: false,
      back: false,
      palette: false,
      context: false,
      historyBack: false,
      historyForward: false,
    },
    nextRepeatAt: {
      up: 0,
      down: 0,
      left: 0,
      right: 0,
    },
  };
}

function buttonPressed(gamepad: GamepadLike, index: number): boolean {
  const button = gamepad.buttons[index];
  return Boolean(button?.pressed || (button?.value ?? 0) > 0.5);
}

function readSignals(gamepads: readonly (GamepadLike | null | undefined)[]): Record<ControllerAction, boolean> {
  const signals = Object.fromEntries(ACTIONS.map((action) => [action, false])) as Record<ControllerAction, boolean>;

  for (const gamepad of gamepads) {
    if (!gamepad?.connected) {
      continue;
    }

    const axisX = gamepad.axes[LEFT_STICK_X] ?? 0;
    const axisY = gamepad.axes[LEFT_STICK_Y] ?? 0;

    signals.activate ||= buttonPressed(gamepad, BUTTON_A);
    signals.back ||= buttonPressed(gamepad, BUTTON_B);
    signals.context ||= buttonPressed(gamepad, BUTTON_Y) || buttonPressed(gamepad, BUTTON_VIEW);
    signals.palette ||= buttonPressed(gamepad, BUTTON_MENU);
    signals.historyBack ||= buttonPressed(gamepad, BUTTON_LB);
    signals.historyForward ||= buttonPressed(gamepad, BUTTON_RB);

    signals.up ||= buttonPressed(gamepad, BUTTON_DPAD_UP) || axisY <= -STICK_DEADZONE;
    signals.down ||= buttonPressed(gamepad, BUTTON_DPAD_DOWN) || axisY >= STICK_DEADZONE;
    signals.left ||= buttonPressed(gamepad, BUTTON_DPAD_LEFT) || axisX <= -STICK_DEADZONE;
    signals.right ||= buttonPressed(gamepad, BUTTON_DPAD_RIGHT) || axisX >= STICK_DEADZONE;
  }

  return signals;
}

export function advanceControllerPollState(
  state: ControllerPollState,
  gamepads: readonly (GamepadLike | null | undefined)[],
  now: number,
): { state: ControllerPollState; actions: ControllerAction[] } {
  const signals = readSignals(gamepads);
  const nextState: ControllerPollState = {
    held: { ...state.held },
    nextRepeatAt: { ...state.nextRepeatAt },
  };
  const actions: ControllerAction[] = [];

  for (const action of ACTIONS) {
    const active = signals[action];
    const wasHeld = state.held[action];
    nextState.held[action] = active;

    if (!active) {
      if ((REPEATABLE_ACTIONS as ControllerAction[]).includes(action)) {
        nextState.nextRepeatAt[action as RepeatableAction] = 0;
      }
      continue;
    }

    if (!wasHeld) {
      actions.push(action);
      if ((REPEATABLE_ACTIONS as ControllerAction[]).includes(action)) {
        nextState.nextRepeatAt[action as RepeatableAction] = now + INITIAL_REPEAT_MS;
      }
      continue;
    }

    if ((REPEATABLE_ACTIONS as ControllerAction[]).includes(action) && now >= state.nextRepeatAt[action as RepeatableAction]) {
      actions.push(action);
      nextState.nextRepeatAt[action as RepeatableAction] = now + REPEAT_MS;
    }
  }

  return { state: nextState, actions };
}

function controllerSchemeForGamepads(gamepads: readonly (GamepadLike | null | undefined)[]): ControllerGlyphScheme | undefined {
  const connected = gamepads.find((gamepad) => gamepad?.connected && gamepad.id);
  return connected?.id ? inferControllerGlyphScheme(connected.id) : undefined;
}

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  if (target.isContentEditable) {
    return true;
  }
  const tag = target.tagName;
  if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") {
    return true;
  }
  return target.closest("input, textarea, select, [contenteditable]") !== null;
}

function shouldPauseForTyping(): boolean {
  return isEditableTarget(document.activeElement);
}

function isInsideControllerSpatialNavigation(target: EventTarget | null): boolean {
  return target instanceof HTMLElement && target.closest(CONTROLLER_SPATIAL_NAV_SELECTOR) instanceof HTMLElement;
}

function isVisibleControllerTarget(target: HTMLElement): boolean {
  if (isDisabledControl(target)) return false;
  if (target.getAttribute("tabindex") === "-1") return false;
  if (target.matches("[hidden], [aria-hidden='true']")) return false;
  return target.getClientRects().length > 0;
}

function focusControllerTarget(target: HTMLElement): void {
  try {
    target.focus({ preventScroll: true });
  } catch {
    target.focus();
  }
  target.scrollIntoView?.({ block: "nearest", inline: "nearest" });
}

function canPrimePrimaryGridFocus(action: ControllerAction): boolean {
  return action === "up" || action === "down" || action === "left" || action === "right" || action === "activate" || action === "context";
}

function focusInitialPrimaryGridTarget(action: ControllerAction): boolean {
  if (!canPrimePrimaryGridFocus(action)) return false;
  if (get(activeModal) || get(contextMenu) || get(searchOverlayCtrl).open) return false;

  const active = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  if (active?.closest(PRIMARY_GRID_SELECTOR)) return false;
  if (active && active !== document.body && active !== document.documentElement && isVisibleControllerTarget(active)) {
    return false;
  }

  const target = Array.from(document.querySelectorAll<HTMLElement>(PRIMARY_GRID_SELECTOR))
    .find(isVisibleControllerTarget);
  if (!target) return false;
  focusControllerTarget(target);
  return true;
}

function focusInitialControllerSpatialNavigationTarget(action: ControllerAction): boolean {
  if (action !== "up" && action !== "down" && action !== "left" && action !== "right") return false;
  if (get(activeModal) || get(contextMenu) || get(searchOverlayCtrl).open) return false;

  const active = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  if (isInsideControllerSpatialNavigation(active)) return false;

  const root = document.querySelector<HTMLElement>(CONTROLLER_SPATIAL_NAV_SELECTOR);
  if (!root) return false;
  const edge = action === "up" || action === "left" ? "last" : "first";
  return focusControllerSpatialNavigationTarget(root, edge);
}

function isDisabledControl(target: HTMLElement): boolean {
  if ("disabled" in target && typeof (target as { disabled?: unknown }).disabled === "boolean") {
    return Boolean((target as { disabled?: boolean }).disabled);
  }
  return target.getAttribute("aria-disabled") === "true";
}

function dispatchKeyboardActivation(target: HTMLElement, key: "Enter" | " "): boolean {
  const keydown = new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true });
  const keyup = new KeyboardEvent("keyup", { key, bubbles: true, cancelable: true });
  markSyntheticControllerKeyEvent(keydown);
  markSyntheticControllerKeyEvent(keyup);
  const allowed = target.dispatchEvent(keydown);
  target.dispatchEvent(keyup);
  return !allowed;
}

function activateFocusedControl(): void {
  const target = document.activeElement;
  if (!(target instanceof HTMLElement) || isDisabledControl(target)) {
    return;
  }

  if (
    target instanceof HTMLButtonElement
    || target instanceof HTMLAnchorElement
    || target.tagName === "SUMMARY"
  ) {
    target.click();
    return;
  }

  if (target instanceof HTMLInputElement) {
    const activatableTypes = new Set([
      "button",
      "submit",
      "reset",
      "checkbox",
      "radio",
      "file",
      "color",
      "range",
      "image",
    ]);
    if (activatableTypes.has(target.type)) {
      target.click();
      return;
    }
  }

  if (target.getAttribute("role") === "button" || target.getAttribute("role") === "menuitem") {
    if (!dispatchKeyboardActivation(target, "Enter")) {
      target.click();
    }
  }
}

function dispatchDirectionalKey(key: "ArrowUp" | "ArrowDown" | "ArrowLeft" | "ArrowRight"): boolean {
  const target = document.activeElement instanceof HTMLElement ? document.activeElement : document.body;
  const before = document.activeElement;
  const keydown = new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true });
  markSyntheticControllerKeyEvent(keydown);
  const allowed = target.dispatchEvent(keydown);
  const keyup = new KeyboardEvent("keyup", { key, bubbles: true, cancelable: true });
  markSyntheticControllerKeyEvent(keyup);
  target.dispatchEvent(keyup);
  return !allowed || document.activeElement !== before;
}

function focusByDirection(direction: RepeatableAction): void {
  moveFocusSpatially(direction);
}

function openFocusedContextMenu(): void {
  if (get(contextMenu)) {
    return;
  }
  const target = document.activeElement instanceof HTMLElement ? document.activeElement : document.body;
  if (!target) {
    return;
  }
  const rect = target.getBoundingClientRect?.();
  const clientX = rect ? rect.left + rect.width / 2 : 0;
  const clientY = rect ? rect.top + rect.height / 2 : 0;
  const event = new MouseEvent("contextmenu", {
    bubbles: true,
    cancelable: true,
    clientX,
    clientY,
  });
  target.dispatchEvent(event);
}

function handleBackAction(): void {
  if (get(activeModal)) {
    cancelActiveModal();
    return;
  }
  if (get(contextMenu)) {
    closeContextMenu();
    return;
  }
  if (get(searchOverlayCtrl).open) {
    closeSearchOverlay();
    return;
  }
  if (dispatchEscapeKey()) {
    return;
  }
  navigateBackLikeButton();
}

function dispatchEscapeKey(): boolean {
  const target = document.activeElement instanceof HTMLElement ? document.activeElement : document.body;
  const keydown = new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true });
  markSyntheticControllerKeyEvent(keydown);
  const allowed = target.dispatchEvent(keydown);
  const keyup = new KeyboardEvent("keyup", { key: "Escape", bubbles: true, cancelable: true });
  markSyntheticControllerKeyEvent(keyup);
  target.dispatchEvent(keyup);
  return !allowed;
}

function canUseGlobalHistoryActions(): boolean {
  if (shouldPauseForTyping()) {
    return false;
  }
  if (get(activeModal) || get(contextMenu)) {
    return false;
  }
  return !get(searchOverlayCtrl).open;
}

function handleAction(action: ControllerAction, scheme?: ControllerGlyphScheme): void {
  const wasControllerInput = getInputModality() === "controller";
  markControllerInput(scheme);

  if (!wasControllerInput && focusInitialPrimaryGridTarget(action)) {
    return;
  }
  if (focusInitialControllerSpatialNavigationTarget(action)) {
    return;
  }

  if (get(securityStatus).appLocked) {
    switch (action) {
      case "up":
      case "down":
      case "left":
      case "right":
        focusByDirection(action);
        break;
      case "activate":
        activateFocusedControl();
        break;
    }
    return;
  }

  switch (action) {
    case "up":
      if (shouldPauseForTyping() && !get(contextMenu) && !isInsideControllerSpatialNavigation(document.activeElement)) return;
      if (!dispatchDirectionalKey("ArrowUp")) {
        focusByDirection("up");
      }
      break;
    case "down":
      if (shouldPauseForTyping() && !get(contextMenu) && !isInsideControllerSpatialNavigation(document.activeElement)) return;
      if (!dispatchDirectionalKey("ArrowDown")) {
        focusByDirection("down");
      }
      break;
    case "left":
      if (shouldPauseForTyping() && !get(contextMenu) && !isInsideControllerSpatialNavigation(document.activeElement)) return;
      if (!dispatchDirectionalKey("ArrowLeft")) {
        focusByDirection("left");
      }
      break;
    case "right":
      if (shouldPauseForTyping() && !get(contextMenu) && !isInsideControllerSpatialNavigation(document.activeElement)) return;
      if (!dispatchDirectionalKey("ArrowRight")) {
        focusByDirection("right");
      }
      break;
    case "activate":
      if (shouldPauseForTyping() && !isInsideControllerSpatialNavigation(document.activeElement)) return;
      activateFocusedControl();
      break;
    case "back":
      handleBackAction();
      break;
    case "palette":
      if (shouldPauseForTyping()) return;
      if (!get(activeModal) && !get(contextMenu)) {
        openSearchOverlay(">");
      }
      break;
    case "context":
      if (shouldPauseForTyping()) return;
      if (!get(activeModal)) {
        openFocusedContextMenu();
      }
      break;
    case "historyBack":
      if (canUseGlobalHistoryActions()) {
        navigateBackLikeButton();
      }
      break;
    case "historyForward":
      if (canUseGlobalHistoryActions()) {
        navigateForward();
      }
      break;
  }
}

export function installControllerInput(): () => void {
  if (typeof window === "undefined" || typeof document === "undefined") {
    return () => {};
  }

  let state = createInitialControllerPollState();
  let connected = false;
  let frameId = 0;
  let discoveryTimerId = 0;
  let disposed = false;
  const lastHandledByAction = Object.fromEntries(
    ACTIONS.map((action) => [action, { at: Number.NEGATIVE_INFINITY, source: null as ControllerActionSource | null }]),
  ) as Record<ControllerAction, { at: number; source: ControllerActionSource | null }>;

  const handleDedupedAction = (
    action: ControllerAction,
    scheme: ControllerGlyphScheme | undefined,
    source: ControllerActionSource,
    now = performance.now(),
  ): void => {
    const last = lastHandledByAction[action];
    if (last.source !== null && last.source !== source && now - last.at < CROSS_SOURCE_DUPLICATE_ACTION_SUPPRESS_MS) {
      return;
    }
    lastHandledByAction[action] = { at: now, source };
    handleAction(action, scheme);
  };

  const getPads = (): readonly (GamepadLike | null | undefined)[] => {
    if (typeof navigator === "undefined" || typeof navigator.getGamepads !== "function") {
      return [];
    }
    return navigator.getGamepads();
  };

  const refreshConnectionState = (): void => {
    connected = getPads().some((gamepad) => Boolean(gamepad?.connected));
  };

  const stopTicking = (): void => {
    if (!frameId) {
      return;
    }
    window.cancelAnimationFrame(frameId);
    frameId = 0;
  };

  const stopDiscovery = (): void => {
    if (!discoveryTimerId) {
      return;
    }
    window.clearTimeout(discoveryTimerId);
    discoveryTimerId = 0;
  };

  const scheduleDiscovery = (): void => {
    if (disposed || connected || discoveryTimerId) {
      return;
    }
    discoveryTimerId = window.setTimeout(() => {
      discoveryTimerId = 0;
      onGamepadStateChange();
      scheduleDiscovery();
    }, DISCOVERY_INTERVAL_MS);
  };

  const tick = (): void => {
    if (!connected) {
      frameId = 0;
      scheduleDiscovery();
      return;
    }
    const now = performance.now();
    const pads = getPads();
    const scheme = controllerSchemeForGamepads(pads);
    const next = advanceControllerPollState(state, pads, now);
    state = next.state;
    for (const action of next.actions) {
      handleDedupedAction(action, scheme, "poll", now);
    }
    frameId = window.requestAnimationFrame(tick);
  };

  const ensureTicking = (): void => {
    if (!connected || frameId) {
      return;
    }
    frameId = window.requestAnimationFrame(tick);
  };

  const onGamepadStateChange = (): void => {
    refreshConnectionState();
    if (!connected) {
      state = createInitialControllerPollState();
      stopTicking();
      scheduleDiscovery();
      return;
    }
    stopDiscovery();
    ensureTicking();
  };

  refreshConnectionState();
  onGamepadStateChange();

  // This installer is only active while App.svelte keeps controller support enabled.
  const offNativeControllerAction = Events.On(CONTROLLER_ACTION_EVENT, (event) => {
    const parsed = parseNativeControllerAction(event.data);
    if (!parsed) {
      return;
    }
    handleDedupedAction(parsed.action, parsed.scheme, "native");
  });

  window.addEventListener("gamepadconnected", onGamepadStateChange);
  window.addEventListener("gamepaddisconnected", onGamepadStateChange);
  window.addEventListener("focus", onGamepadStateChange);
  document.addEventListener("visibilitychange", onGamepadStateChange);

  return () => {
    disposed = true;
    offNativeControllerAction();
    stopDiscovery();
    stopTicking();
    window.removeEventListener("gamepadconnected", onGamepadStateChange);
    window.removeEventListener("gamepaddisconnected", onGamepadStateChange);
    window.removeEventListener("focus", onGamepadStateChange);
    document.removeEventListener("visibilitychange", onGamepadStateChange);
  };
}

export type ControllerInputInstaller = () => () => void;

export function createControllerInputController(install: ControllerInputInstaller = installControllerInput): {
  setEnabled: (enabled: boolean) => void;
  destroy: () => void;
} {
  let uninstall: (() => void) | null = null;

  const stop = (): void => {
    if (!uninstall) {
      return;
    }
    uninstall();
    uninstall = null;
  };

  return {
    setEnabled: (enabled: boolean): void => {
      if (enabled) {
        if (!uninstall) {
          uninstall = install();
        }
        return;
      }
      stop();
    },
    destroy: stop,
  };
}
