import type { Action } from "svelte/action";

export type MiniProfileHoverParams =
  | undefined
  | {
      html: string;
      boundary: HTMLElement | null;
      offline: boolean;
      enabled: boolean;
    };

const SHOW_DELAY_MS = 100;
const Z_INDEX = 900;
const VIEWPORT_PAD = 8;

function clamp(n: number, lo: number, hi: number): number {
  return Math.min(Math.max(n, lo), hi);
}

function emPx(anchor: HTMLElement): number {
  const fs = parseFloat(getComputedStyle(anchor).fontSize);
  return Number.isFinite(fs) && fs > 0 ? fs : 16;
}

function clampToBounds(
  left: number,
  top: number,
  w: number,
  h: number,
  boundary: HTMLElement | null,
): { left: number; top: number } {
  let minL = VIEWPORT_PAD;
  let minT = VIEWPORT_PAD;
  let maxL = window.innerWidth - w - VIEWPORT_PAD;
  let maxT = window.innerHeight - h - VIEWPORT_PAD;
  if (boundary) {
    const br = boundary.getBoundingClientRect();
    const pad = 4;
    minL = Math.max(minL, br.left + pad);
    minT = Math.max(minT, br.top + pad);
    maxL = Math.min(maxL, br.right - w - pad);
    maxT = Math.min(maxT, br.bottom - h - pad);
  }
  if (maxL < minL) {
    minL = VIEWPORT_PAD;
    maxL = window.innerWidth - w - VIEWPORT_PAD;
  }
  if (maxT < minT) {
    minT = VIEWPORT_PAD;
    maxT = window.innerHeight - h - VIEWPORT_PAD;
  }
  return { left: clamp(left, minL, maxL), top: clamp(top, minT, maxT) };
}

function placePopover(pop: HTMLElement, anchor: HTMLElement, boundary: HTMLElement | null): void {
  const ar = anchor.getBoundingClientRect();
  const gap = emPx(anchor);
  void pop.offsetWidth;
  const w = pop.offsetWidth;
  const h = pop.offsetHeight;

  const vw = window.innerWidth;
  const vh = window.innerHeight;
  const br = boundary?.getBoundingClientRect() ?? new DOMRect(0, 0, vw, vh);
  const pad = 4;
  const maxR = Math.min(vw - VIEWPORT_PAD, br.right - pad);
  const maxB = Math.min(vh - VIEWPORT_PAD, br.bottom - pad);
  const minT = Math.max(VIEWPORT_PAD, br.top + pad);

  let left = ar.right + gap;
  let top = ar.top;

  if (left + w > maxR) {
    left = ar.left - gap - w;
  }

  if (top + h > maxB) {
    top = ar.top - gap - h;
  }

  if (top < minT) {
    top = ar.bottom + gap;
  }

  const c = clampToBounds(left, top, w, h, boundary);
  pop.style.left = `${Math.round(c.left)}px`;
  pop.style.top = `${Math.round(c.top)}px`;
}

export const miniProfileHover: Action<HTMLElement, MiniProfileHoverParams> = (node, initial) => {
  let slot: MiniProfileHoverParams = initial;
  let parsed: Exclude<MiniProfileHoverParams, undefined> | null = null;
  let pop: HTMLDivElement | null = null;
  let showTimer: number | null = null;

  const parse = (p: MiniProfileHoverParams): typeof parsed => {
    if (!p || !p.enabled || p.offline) return null;
    const html = (p.html ?? "").trim();
    if (!html) return null;
    return { html, boundary: p.boundary ?? null, offline: p.offline, enabled: p.enabled };
  };

  parsed = parse(slot);

  const clearShow = () => {
    if (showTimer != null) {
      clearTimeout(showTimer);
      showTimer = null;
    }
  };

  const hide = () => {
    clearShow();
    if (!pop) return;
    pop.remove();
    pop = null;
  };

  const show = () => {
    parsed = parse(slot);
    if (!parsed) return;
    if (pop) return;
    if (isDragging()) return;
    pop = document.createElement("div");
    pop.className = "steam-miniprofile-popover";
    pop.style.position = "fixed";
    pop.style.zIndex = String(Z_INDEX);
    pop.style.pointerEvents = "none";
    pop.setAttribute("role", "dialog");
    pop.setAttribute("aria-hidden", "false");

    const inner = document.createElement("div");
    inner.className = "steam-miniprofile-popover__inner";
    inner.innerHTML = parsed.html;
    pop.appendChild(inner);
    document.body.appendChild(pop);

    requestAnimationFrame(() => {
      if (!pop || !parsed) return;
      placePopover(pop, node, parsed.boundary);
    });
  };

  const isDragging = () =>
    !!document.querySelector(".reorder-pointer-grid__ghost") ||
    !!document.querySelector(".shortcutDndGhost") ||
    document.body.dataset.dragging === "true";

  const scheduleShow = () => {
    clearShow();
    if (!parsed) return;
    if (isDragging()) return;
    showTimer = window.setTimeout(show, SHOW_DELAY_MS);
  };

  const onEnter = () => {
    parsed = parse(slot);
    scheduleShow();
  };
  const onLeave = () => {
    clearShow();
    hide();
  };

  const onScrollResize = () => {
    hide();
  };
  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Escape") hide();
  };

  node.addEventListener("mouseenter", onEnter);
  node.addEventListener("mouseleave", onLeave);
  window.addEventListener("scroll", onScrollResize, true);
  window.addEventListener("resize", onScrollResize);
  document.addEventListener("keydown", onKey);

  return {
    update(newParam: MiniProfileHoverParams) {
      slot = newParam;
      parsed = parse(slot);
      if (!parsed) hide();
    },
    destroy() {
      clearShow();
      hide();
      node.removeEventListener("mouseenter", onEnter);
      node.removeEventListener("mouseleave", onLeave);
      window.removeEventListener("scroll", onScrollResize, true);
      window.removeEventListener("resize", onScrollResize);
      document.removeEventListener("keydown", onKey);
    },
  };
};
