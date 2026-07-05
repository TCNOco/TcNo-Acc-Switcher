export type SpatialDirection = "up" | "down" | "left" | "right";

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

function isDisabledControl(target: HTMLElement): boolean {
  if ("disabled" in target && typeof (target as { disabled?: unknown }).disabled === "boolean") {
    return Boolean((target as { disabled?: boolean }).disabled);
  }
  return target.getAttribute("aria-disabled") === "true";
}

function isVisibleFocusable(target: HTMLElement): boolean {
  if (isDisabledControl(target)) {
    return false;
  }
  if (target.getAttribute("tabindex") === "-1") {
    return false;
  }
  if (target.matches("[hidden], [aria-hidden='true']")) {
    return false;
  }
  return target.getClientRects().length > 0;
}

function focusableCandidates(): HTMLElement[] {
  return preferInnerControls(Array.from(document.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR))).filter(isVisibleFocusable);
}

function preferInnerControls(candidates: HTMLElement[]): HTMLElement[] {
  const candidateSet = new Set(candidates);
  return candidates.filter((candidate) => {
    if (!candidate.matches("[data-dnd-cell]")) {
      return true;
    }
    return !Array.from(candidate.querySelectorAll<HTMLElement>("button:not(:disabled), a[href], [role='button']:not([aria-disabled='true'])"))
      .some((child) => candidateSet.has(child));
  });
}

function rectCenterX(rect: DOMRect): number {
  return rect.left + rect.width / 2;
}

function rectCenterY(rect: DOMRect): number {
  return rect.top + rect.height / 2;
}

function rectsOverlapVertically(a: DOMRect, b: DOMRect): boolean {
  return a.top < b.bottom && b.top < a.bottom;
}

function rectsOverlapHorizontally(a: DOMRect, b: DOMRect): boolean {
  return a.left < b.right && b.left < a.right;
}

function focusElement(target: HTMLElement): void {
  target.focus({ preventScroll: true });
  target.scrollIntoView?.({ block: "nearest", inline: "nearest" });
}

function actionbarCandidates(active?: HTMLElement): HTMLElement[] {
  const previewRoot = active?.closest(".preview-css-page");
  const actionbar = previewRoot?.querySelector<HTMLElement>(".preview_accounts_actionbar")
    ?? document.querySelector<HTMLElement>(".actionbar__actions");
  if (!actionbar) return [];
  return preferInnerControls(Array.from(actionbar.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR))).filter(isVisibleFocusable);
}

function focusFirstActionbarControl(active: HTMLElement): boolean {
  const target = actionbarCandidates(active)[0];
  if (!target) return false;
  focusElement(target);
  return true;
}

function focusGridFromActionbar(active: HTMLElement): boolean {
  const cells = Array.from(document.querySelectorAll<HTMLElement>(".acc_list [data-dnd-cell][data-dnd-name], .platform_list [data-dnd-cell][data-dnd-name]"))
    .filter(isVisibleFocusable);
  if (!cells.length) return false;

  const activeRect = active.getBoundingClientRect();
  const lowestTop = Math.max(...cells.map((cell) => cell.getBoundingClientRect().top));
  const bottomRow = cells.filter((cell) => Math.abs(cell.getBoundingClientRect().top - lowestTop) <= 4);
  const candidates = bottomRow.length ? bottomRow : cells;
  candidates.sort((a, b) =>
    Math.abs(rectCenterX(activeRect) - rectCenterX(a.getBoundingClientRect()))
    - Math.abs(rectCenterX(activeRect) - rectCenterX(b.getBoundingClientRect())),
  );
  focusElement(candidates[0]);
  return true;
}

export function moveFocusSpatially(direction: SpatialDirection): boolean {
  if (typeof document === "undefined") {
    return false;
  }

  const candidates = focusableCandidates();
  if (!candidates.length) {
    return false;
  }

  const active = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  if (!active || !isVisibleFocusable(active)) {
    const fallback = direction === "up" || direction === "left" ? candidates.at(-1) : candidates[0];
    if (!fallback) return false;
    focusElement(fallback);
    return true;
  }

  if (direction === "down" && active.closest(".acc_list_item, .platform_list_item")) {
    if (focusFirstActionbarControl(active)) return true;
  }

  if (direction === "up" && active.closest(".actionbar__actions, .preview_accounts_actionbar")) {
    if (focusGridFromActionbar(active)) return true;
  }

  const activeRect = active.getBoundingClientRect();
  const activeCenterX = rectCenterX(activeRect);
  const activeCenterY = rectCenterY(activeRect);

  const directional = candidates.filter((candidate) => {
    if (candidate === active) {
      return false;
    }
    const rect = candidate.getBoundingClientRect();
    const centerX = rectCenterX(rect);
    const centerY = rectCenterY(rect);
    const dx = centerX - activeCenterX;
    const dy = centerY - activeCenterY;

    if (direction === "up" && dy >= -1) return false;
    if (direction === "down" && dy <= 1) return false;
    if (direction === "left" && dx >= -1) return false;
    if (direction === "right" && dx <= 1) return false;
    return true;
  });

  const aligned = directional.filter((candidate) => {
    const rect = candidate.getBoundingClientRect();
    return direction === "left" || direction === "right"
      ? rectsOverlapVertically(activeRect, rect)
      : rectsOverlapHorizontally(activeRect, rect);
  });

  let best: { node: HTMLElement; score: number } | null = null;

  for (const candidate of aligned.length ? aligned : directional) {
    const rect = candidate.getBoundingClientRect();
    const centerX = rectCenterX(rect);
    const centerY = rectCenterY(rect);
    const dx = centerX - activeCenterX;
    const dy = centerY - activeCenterY;
    const primary = direction === "up" || direction === "down" ? Math.abs(dy) : Math.abs(dx);
    const secondary = direction === "up" || direction === "down" ? Math.abs(dx) : Math.abs(dy);
    const score = primary * 1000 + secondary;

    if (!best || score < best.score) {
      best = { node: candidate, score };
    }
  }

  if (!best) return false;
  focusElement(best.node);
  return true;
}
