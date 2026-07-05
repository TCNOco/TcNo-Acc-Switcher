import type { Action } from "svelte/action";
import { isSyntheticControllerKeyEvent } from "../inputModality";

type ArrowKey = "ArrowLeft" | "ArrowRight" | "ArrowUp" | "ArrowDown";
type SpatialRect = Pick<DOMRect, "left" | "right" | "top" | "bottom" | "width" | "height">;
type NavigationTarget = {
  element: HTMLElement;
  rect: SpatialRect;
  index: number;
  centerX: number;
  centerY: number;
};
type NavigationRow = {
  targets: NavigationTarget[];
  centerY: number;
};

const ROW_Y_TOLERANCE = 24;

const FOCUSABLE_SELECTOR = [
  "button:not(:disabled)",
  "a[href]",
  "input:not([type='hidden']):not(:disabled)",
  "select:not(:disabled)",
  "textarea:not(:disabled)",
  "summary",
  "[role='button']:not([aria-disabled='true'])",
  "[role='checkbox']:not([aria-disabled='true'])",
  "[role='switch']:not([aria-disabled='true'])",
  "[tabindex]:not([tabindex='-1'])",
].join(", ");

function isArrowKey(key: string): key is ArrowKey {
  return key === "ArrowLeft" || key === "ArrowRight" || key === "ArrowUp" || key === "ArrowDown";
}

function activeElementFor(root: HTMLElement): Element | null {
  return root.ownerDocument?.activeElement ?? (typeof document === "undefined" ? null : document.activeElement);
}

function isDisabled(el: HTMLElement): boolean {
  if ("disabled" in el && typeof (el as { disabled?: unknown }).disabled === "boolean") {
    return Boolean((el as { disabled?: boolean }).disabled);
  }
  return el.getAttribute("aria-disabled") === "true";
}

function inputType(el: HTMLElement): string {
  return (((el as HTMLInputElement).type || el.getAttribute("type") || "") as string).toLowerCase();
}

function isCheckboxLikeInput(el: HTMLElement): boolean {
  return el.tagName === "INPUT" && (inputType(el) === "checkbox" || inputType(el) === "radio");
}

function usableRect(el: HTMLElement): SpatialRect | null {
  const rects = typeof el.getClientRects === "function" ? el.getClientRects() : null;
  if (rects && rects.length === 0) return null;
  const rect = typeof el.getBoundingClientRect === "function" ? el.getBoundingClientRect() : null;
  if (!rect || (rect.width <= 0 && rect.height <= 0)) return null;
  return rect;
}

function visibleCheckboxProxy(el: HTMLElement): HTMLElement | null {
  if (!isCheckboxLikeInput(el)) return null;

  const next = (el as HTMLElement & { nextElementSibling?: Element | null }).nextElementSibling;
  if (next instanceof HTMLElement && next.tagName === "LABEL" && usableRect(next)) {
    return next;
  }

  const labels = (el as HTMLInputElement).labels;
  if (labels) {
    const label = Array.from(labels).find((item): item is HTMLLabelElement => item instanceof HTMLElement && Boolean(usableRect(item)));
    if (label) return label;
  }

  const row = el.closest?.(".form-check, .rowSetting, .rowDropdown, .multilineSetting");
  return row instanceof HTMLElement && usableRect(row) ? row : null;
}

function visualElementFor(el: HTMLElement): HTMLElement {
  return visibleCheckboxProxy(el) ?? el;
}

function rectForTarget(el: HTMLElement): SpatialRect | null {
  const visual = visualElementFor(el);
  return usableRect(visual) ?? (visual === el ? null : usableRect(el));
}

function isVisible(el: HTMLElement): boolean {
  if (el.getAttribute("hidden") !== null || el.getAttribute("aria-hidden") === "true") return false;
  return rectForTarget(el) !== null;
}

function focusableTargets(root: HTMLElement): HTMLElement[] {
  const seen = new Set<HTMLElement>();
  return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter((el) => {
    if (seen.has(el)) return false;
    seen.add(el);
    return !isDisabled(el) && isVisible(el);
  });
}

function navigationTargets(root: HTMLElement): NavigationTarget[] {
  return focusableTargets(root)
    .map((element, index) => {
      const rect = rectForTarget(element);
      if (!rect) return null;
      return {
        element,
        rect,
        index,
        centerX: rect.left + rect.width / 2,
        centerY: rect.top + rect.height / 2,
      };
    })
    .filter((target): target is NavigationTarget => target !== null);
}

function focusTarget(el: HTMLElement): void {
  try {
    el.focus({ preventScroll: true });
  } catch {
    el.focus();
  }
  visualElementFor(el).scrollIntoView?.({ block: "nearest", inline: "nearest" });
}

export function focusControllerSpatialNavigationTarget(
  root: HTMLElement | null,
  edge: "first" | "last" = "first",
): boolean {
  if (!root) return false;
  const targets = focusableTargets(root);
  const target = edge === "first" ? targets[0] : targets[targets.length - 1];
  if (!target) return false;
  focusTarget(target);
  return true;
}

function currentTargetFrom(active: Element, targets: HTMLElement[]): HTMLElement | null {
  if (targets.includes(active as HTMLElement)) {
    return active as HTMLElement;
  }
  const activeEl = active as HTMLElement;
  const childTarget = targets.find((target) => activeEl.contains?.(target));
  if (childTarget) return childTarget;
  return targets.find((target) => target.contains?.(active)) ?? null;
}

function currentNavigationTargetFrom(active: Element, targets: NavigationTarget[]): NavigationTarget | null {
  const current = currentTargetFrom(
    active,
    targets.map((target) => target.element),
  );
  return current ? targets.find((target) => target.element === current) ?? null : null;
}

function recalculateRowCenter(row: NavigationRow): void {
  row.centerY = row.targets.reduce((sum, target) => sum + target.centerY, 0) / row.targets.length;
}

function navigationRows(targets: NavigationTarget[]): NavigationRow[] {
  const rows: NavigationRow[] = [];
  const byPosition = [...targets].sort(
    (a, b) => a.centerY - b.centerY || a.rect.top - b.rect.top || a.centerX - b.centerX || a.index - b.index,
  );

  for (const target of byPosition) {
    const row = rows.find((candidate) => Math.abs(candidate.centerY - target.centerY) <= ROW_Y_TOLERANCE);
    if (row) {
      row.targets.push(target);
      recalculateRowCenter(row);
    } else {
      rows.push({ targets: [target], centerY: target.centerY });
    }
  }

  rows.sort((a, b) => a.centerY - b.centerY);
  for (const row of rows) {
    row.targets.sort((a, b) => a.centerX - b.centerX || a.index - b.index);
  }
  return rows;
}

function nearestTargetInRow(row: NavigationRow, current: NavigationTarget): NavigationTarget {
  return [...row.targets].sort((a, b) => {
    const byX = Math.abs(a.centerX - current.centerX) - Math.abs(b.centerX - current.centerX);
    return byX !== 0 ? byX : a.index - b.index;
  })[0];
}

function destinationForKey(current: NavigationTarget, rows: NavigationRow[], key: ArrowKey): NavigationTarget | null {
  const rowIndex = rows.findIndex((row) => row.targets.includes(current));
  if (rowIndex < 0) return null;
  const row = rows[rowIndex];
  const targetIndex = row.targets.indexOf(current);

  if (key === "ArrowLeft") {
    if (targetIndex > 0) return row.targets[targetIndex - 1];
    const previousRowTargets = rows[rowIndex - 1]?.targets;
    return previousRowTargets ? previousRowTargets[previousRowTargets.length - 1] : null;
  }

  if (key === "ArrowRight") {
    if (targetIndex >= 0 && targetIndex < row.targets.length - 1) return row.targets[targetIndex + 1];
    return rows[rowIndex + 1]?.targets[0] ?? null;
  }

  const nextRow = key === "ArrowUp" ? rows[rowIndex - 1] : rows[rowIndex + 1];
  return nextRow ? nearestTargetInRow(nextRow, current) : null;
}

function consumeControllerArrow(ev: KeyboardEvent): void {
  ev.preventDefault();
  ev.stopPropagation();
}

export function controllerSpatialNavigationMove(ev: KeyboardEvent, root: HTMLElement): boolean {
  if (!isArrowKey(ev.key)) return false;
  if (ev.altKey || ev.ctrlKey || ev.metaKey || ev.shiftKey) return false;
  if (!isSyntheticControllerKeyEvent(ev)) return false;
  const key = ev.key;

  const active = activeElementFor(root);
  if (!active || !root.contains(active)) return false;

  const targets = navigationTargets(root);
  if (targets.length === 0) return false;

  const currentTarget = currentNavigationTargetFrom(active, targets) ?? targets[0];
  const destination = destinationForKey(currentTarget, navigationRows(targets), key);

  consumeControllerArrow(ev);
  if (destination) {
    focusTarget(destination.element);
  }
  return true;
}

export const controllerSpatialNavigation: Action<HTMLElement> = (node) => {
  const hadAttribute = node.hasAttribute("data-controller-spatial-nav");
  node.setAttribute("data-controller-spatial-nav", "");

  const onKeydown = (ev: KeyboardEvent): void => {
    controllerSpatialNavigationMove(ev, node);
  };

  node.addEventListener("keydown", onKeydown);

  return {
    destroy() {
      node.removeEventListener("keydown", onKeydown);
      if (!hadAttribute) {
        node.removeAttribute("data-controller-spatial-nav");
      }
    },
  };
};
