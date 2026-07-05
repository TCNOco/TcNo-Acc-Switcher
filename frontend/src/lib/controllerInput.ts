import { Events } from "@wailsio/runtime";
import { get } from "svelte/store";
import { activeModal, cancelActiveModal } from "../stores/modal";
import { contextMenu, closeContextMenu } from "../stores/contextMenu";
import { navigateBackLikeButton, navigateForward } from "../stores/nav";
import { searchOverlayCtrl, openSearchOverlay, closeSearchOverlay } from "../stores/searchOverlay";
import { securityStatus } from "../stores/security";
import { markControllerInput } from "./inputModality";

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
const DISCOVERY_INTERVAL_MS = 1000;
const FOCUSABLE_SELECTOR = [
  "button",
  "a[href]",
  "input:not([type='hidden'])",
  "select",
  "textarea",
  "summary",
  "[tabindex]:not([tabindex='-1'])",
  "[role='button']",
  "[role='menuitem']",
  "[role='option']",
  "[role='tab']",
].join(",");

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
  buttons: readonly Pick<GamepadButton, "pressed" | "value">[];
};

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

function parseNativeControllerAction(payload: unknown): ControllerAction | null {
  if (isControllerAction(payload)) {
    return payload;
  }
  if (typeof payload === "object" && payload && "action" in payload) {
    const action = (payload as { action?: unknown }).action;
    return isControllerAction(action) ? action : null;
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

function isDisabledControl(target: HTMLElement): boolean {
  if ("disabled" in target && typeof (target as { disabled?: unknown }).disabled === "boolean") {
    return Boolean((target as { disabled?: boolean }).disabled);
  }
  return target.getAttribute("aria-disabled") === "true";
}

function dispatchKeyboardActivation(target: HTMLElement, key: "Enter" | " "): boolean {
  const keydown = new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true });
  const keyup = new KeyboardEvent("keyup", { key, bubbles: true, cancelable: true });
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
  const allowed = target.dispatchEvent(keydown);
  const keyup = new KeyboardEvent("keyup", { key, bubbles: true, cancelable: true });
  target.dispatchEvent(keyup);
  return !allowed || document.activeElement !== before;
}

function isVisibleFocusable(target: HTMLElement): boolean {
  if (isDisabledControl(target)) {
    return false;
  }
  if (target.matches("[hidden], [aria-hidden='true']")) {
    return false;
  }
  return target.getClientRects().length > 0;
}

function focusableCandidates(): HTMLElement[] {
  return Array.from(document.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(isVisibleFocusable);
}

function focusByDirection(direction: RepeatableAction): void {
  const candidates = focusableCandidates();
  if (!candidates.length) {
    return;
  }

  const active = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  if (!active || !isVisibleFocusable(active)) {
    const fallback = direction === "up" || direction === "left" ? candidates.at(-1) : candidates[0];
    fallback?.focus({ preventScroll: true });
    return;
  }

  const activeRect = active.getBoundingClientRect();
  const activeCenterX = activeRect.left + activeRect.width / 2;
  const activeCenterY = activeRect.top + activeRect.height / 2;

  let best: { node: HTMLElement; score: number } | null = null;

  for (const candidate of candidates) {
    if (candidate === active) {
      continue;
    }
    const rect = candidate.getBoundingClientRect();
    const centerX = rect.left + rect.width / 2;
    const centerY = rect.top + rect.height / 2;
    const dx = centerX - activeCenterX;
    const dy = centerY - activeCenterY;

    if (direction === "up" && dy >= -1) continue;
    if (direction === "down" && dy <= 1) continue;
    if (direction === "left" && dx >= -1) continue;
    if (direction === "right" && dx <= 1) continue;

    const primary = direction === "up" || direction === "down" ? Math.abs(dy) : Math.abs(dx);
    const secondary = direction === "up" || direction === "down" ? Math.abs(dx) : Math.abs(dy);
    const score = primary * 1000 + secondary;

    if (!best || score < best.score) {
      best = { node: candidate, score };
    }
  }

  best?.node.focus({ preventScroll: true });
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
  navigateBackLikeButton();
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

function handleAction(action: ControllerAction): void {
  markControllerInput();

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

  if (shouldPauseForTyping()) {
    return;
  }

  switch (action) {
    case "up":
      if (!dispatchDirectionalKey("ArrowUp")) {
        focusByDirection("up");
      }
      break;
    case "down":
      if (!dispatchDirectionalKey("ArrowDown")) {
        focusByDirection("down");
      }
      break;
    case "left":
      if (!dispatchDirectionalKey("ArrowLeft")) {
        focusByDirection("left");
      }
      break;
    case "right":
      if (!dispatchDirectionalKey("ArrowRight")) {
        focusByDirection("right");
      }
      break;
    case "activate":
      activateFocusedControl();
      break;
    case "back":
      handleBackAction();
      break;
    case "palette":
      if (!get(activeModal) && !get(contextMenu)) {
        openSearchOverlay(">");
      }
      break;
    case "context":
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
    const next = advanceControllerPollState(state, getPads(), performance.now());
    state = next.state;
    for (const action of next.actions) {
      handleAction(action);
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
    const action = parseNativeControllerAction(event.data);
    if (!action) {
      return;
    }
    handleAction(action);
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
