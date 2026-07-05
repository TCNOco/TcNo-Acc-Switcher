import type { Action } from "svelte/action";
import { moveFocusSpatially, type SpatialDirection } from "../spatialFocus";

type ArrowKey = "ArrowLeft" | "ArrowRight" | "ArrowUp" | "ArrowDown";

export type ShortcutArrowNavigationOptions = {
  selector?: string;
  wrap?: boolean;
  capture?: boolean;
  onEscape?: () => void;
  canOpenDropdownFromUp?: (active: HTMLElement) => boolean;
  onHotbarUp?: () => void;
};

const DEFAULT_SELECTOR = [
  "button:not(:disabled)",
  "a[href]",
  "[role='button']:not([aria-disabled='true'])",
].join(", ");

function isArrowKey(key: string): key is ArrowKey {
  return key === "ArrowLeft" || key === "ArrowRight" || key === "ArrowUp" || key === "ArrowDown";
}

function arrowDirection(key: ArrowKey): SpatialDirection {
  if (key === "ArrowUp") return "up";
  if (key === "ArrowDown") return "down";
  if (key === "ArrowLeft") return "left";
  return "right";
}

function activeElementFor(root: HTMLElement): Element | null {
  return root.ownerDocument?.activeElement ?? (typeof document === "undefined" ? null : document.activeElement);
}

function isVisible(el: HTMLElement): boolean {
  const rect = typeof el.getBoundingClientRect === "function" ? el.getBoundingClientRect() : null;
  return !rect || rect.width > 0 || rect.height > 0;
}

function isInsideOpenDropdown(el: HTMLElement): boolean {
  return el.closest?.(".shortcutDropdown.open") instanceof HTMLElement;
}

function focusableShortcutTargets(root: HTMLElement, selector = DEFAULT_SELECTOR): HTMLElement[] {
  const seen = new Set<HTMLElement>();
  return Array.from(root.querySelectorAll<HTMLElement>(selector)).filter((el) => {
    if (seen.has(el)) return false;
    seen.add(el);
    if (el.getAttribute("aria-hidden") === "true") return false;
    if (el.getAttribute("aria-disabled") === "true") return false;
    return isVisible(el);
  });
}

function hotbarShortcutTargets(root: HTMLElement, selector = DEFAULT_SELECTOR): HTMLElement[] {
  return focusableShortcutTargets(root, selector).filter((el) => !isInsideOpenDropdown(el));
}

function focusTarget(el: HTMLElement): void {
  try {
    el.focus({ preventScroll: true });
  } catch {
    el.focus();
  }
  el.scrollIntoView?.({ block: "nearest", inline: "nearest" });
}

function currentTargetFrom(active: Element, targets: HTMLElement[]): HTMLElement | null {
  if (targets.includes(active as HTMLElement)) {
    return active as HTMLElement;
  }
  const activeEl = active as HTMLElement;
  const childTarget = targets.find((target) => activeEl.contains?.(target));
  if (childTarget) {
    return childTarget;
  }
  return targets.find((target) => target.contains?.(active)) ?? null;
}

function dropdownRootFor(root: HTMLElement, active: Element, key: ArrowKey): HTMLElement | null {
  const activeEl = active as HTMLElement;
  const ownDropdown = activeEl.closest?.(".shortcutDropdown.open");
  if (ownDropdown instanceof HTMLElement && root.contains(ownDropdown)) {
    return ownDropdown;
  }
  if (
    (key === "ArrowDown" || key === "ArrowUp") &&
    activeEl.id === "shortcutDropdownBtn"
  ) {
    const openDropdown = root.querySelector<HTMLElement>(".shortcutDropdown.open");
    return openDropdown ?? null;
  }
  return null;
}

function nextIndex(current: number, total: number, key: ArrowKey, wrap: boolean): number {
  const delta = key === "ArrowRight" || key === "ArrowDown" ? 1 : -1;
  const next = current + delta;
  if (next >= 0 && next < total) return next;
  if (!wrap) return current;
  return next < 0 ? total - 1 : 0;
}

function centerX(el: HTMLElement): number {
  const rect = el.getBoundingClientRect();
  return rect.left + rect.width / 2;
}

function centerY(el: HTMLElement): number {
  const rect = el.getBoundingClientRect();
  return rect.top + rect.height / 2;
}

function verticalTarget(current: HTMLElement, targets: HTMLElement[], key: "ArrowUp" | "ArrowDown"): HTMLElement | null {
  const currentCenterY = centerY(current);
  const candidates = targets.filter((target) =>
    key === "ArrowUp" ? centerY(target) < currentCenterY : centerY(target) > currentCenterY,
  );

  candidates.sort((a, b) => {
    const aRect = a.getBoundingClientRect();
    const bRect = b.getBoundingClientRect();
    const fromRect = current.getBoundingClientRect();
    const aPrimary = key === "ArrowUp" ? fromRect.top - aRect.bottom : aRect.top - fromRect.bottom;
    const bPrimary = key === "ArrowUp" ? fromRect.top - bRect.bottom : bRect.top - fromRect.bottom;
    const byPrimary = Math.max(0, aPrimary) - Math.max(0, bPrimary);
    if (byPrimary !== 0) return byPrimary;
    return Math.abs(centerX(current) - centerX(a)) - Math.abs(centerX(current) - centerX(b));
  });

  return candidates[0] ?? null;
}

export function focusShortcutArrowNavigationTarget(
  root: HTMLElement | null,
  edge: "first" | "last" = "first",
  options: ShortcutArrowNavigationOptions = {},
): boolean {
  if (!root) return false;
  const targets = focusableShortcutTargets(root, options.selector);
  const target = edge === "first" ? targets[0] : targets[targets.length - 1];
  if (!target) return false;
  focusTarget(target);
  return true;
}

export function moveShortcutArrowFocus(
  ev: KeyboardEvent,
  root: HTMLElement,
  options: ShortcutArrowNavigationOptions = {},
): boolean {
  if (!isArrowKey(ev.key)) return false;
  if (ev.altKey || ev.ctrlKey || ev.metaKey || ev.shiftKey) return false;

  const active = activeElementFor(root);
  if (!active || !root.contains(active)) return false;

  const dropdownRoot = dropdownRootFor(root, active, ev.key);
  if ((ev.key === "ArrowDown" || ev.key === "ArrowUp") && (active as HTMLElement).id === "shortcutDropdownBtn" && !dropdownRoot) {
    return false;
  }
  const navRoot = dropdownRoot ?? root;
  const targets = dropdownRoot
    ? focusableShortcutTargets(navRoot, options.selector)
    : hotbarShortcutTargets(navRoot, options.selector);
  if (targets.length === 0) return false;

  if ((active as HTMLElement).id === "shortcutDropdownBtn" && dropdownRoot) {
    ev.preventDefault();
    ev.stopPropagation();
    focusTarget(ev.key === "ArrowUp" ? targets[targets.length - 1] : targets[0]);
    return true;
  }

  const currentTarget = currentTargetFrom(active, targets) ?? targets[0];
  const current = targets.indexOf(currentTarget);
  if (current < 0) return false;

  if (ev.key === "ArrowUp" && !dropdownRoot && currentOptionsCanOpenDropdownFromUp(options, active as HTMLElement)) {
    ev.preventDefault();
    ev.stopPropagation();
    options.onHotbarUp?.();
    return true;
  }

  if (ev.key === "ArrowUp" || ev.key === "ArrowDown") {
    const nextTarget = verticalTarget(currentTarget, targets, ev.key);
    if (!nextTarget) {
      if (dropdownRoot || !moveFocusSpatially(arrowDirection(ev.key))) return false;
      ev.preventDefault();
      ev.stopPropagation();
      return true;
    }
    ev.preventDefault();
    ev.stopPropagation();
    focusTarget(nextTarget);
    return true;
  }

  const next = nextIndex(current, targets.length, ev.key, options.wrap ?? false);
  if (next === current) {
    if (dropdownRoot || !moveFocusSpatially(arrowDirection(ev.key))) return false;
    ev.preventDefault();
    ev.stopPropagation();
    return true;
  }

  ev.preventDefault();
  ev.stopPropagation();
  focusTarget(targets[next]);
  return true;
}

function currentOptionsHasHotbarUp(options: ShortcutArrowNavigationOptions): boolean {
  return typeof options.onHotbarUp === "function";
}

function currentOptionsCanOpenDropdownFromUp(options: ShortcutArrowNavigationOptions, active: HTMLElement): boolean {
  if (!currentOptionsHasHotbarUp(options)) return false;
  return options.canOpenDropdownFromUp?.(active) ?? true;
}

export const shortcutArrowNavigation: Action<HTMLElement, ShortcutArrowNavigationOptions | undefined> = (node, options = {}) => {
  let currentOptions = options;
  let listenerCapture = currentOptions.capture ?? false;
  const onKeydown = (ev: KeyboardEvent) => {
    if (ev.key === "Escape" && currentOptions.onEscape) {
      ev.preventDefault();
      ev.stopPropagation();
      currentOptions.onEscape();
      return;
    }
    moveShortcutArrowFocus(ev, node, currentOptions);
  };

  node.addEventListener("keydown", onKeydown, listenerCapture);

  return {
    update(next = {}) {
      const nextCapture = next.capture ?? false;
      if (nextCapture !== listenerCapture) {
        node.removeEventListener("keydown", onKeydown, listenerCapture);
        listenerCapture = nextCapture;
        node.addEventListener("keydown", onKeydown, listenerCapture);
      }
      currentOptions = next;
    },
    destroy() {
      node.removeEventListener("keydown", onKeydown, listenerCapture);
    },
  };
};
