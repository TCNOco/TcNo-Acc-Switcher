export type ModalFrameRect = {
  left: number;
  top: number;
  width: number;
  height: number;
};

export type ModalBounds = {
  width: number;
  height: number;
  pad: number;
};

export type ModalMinSize = {
  minW: number;
  minH: number;
};

export type ResizeEdge = "n" | "s" | "e" | "w" | "ne" | "nw" | "se" | "sw";

export const MODAL_FRAME_PAD = 16;
export const MODAL_FRAME_MIN_W = 320;
export const MODAL_FRAME_MIN_W_FOLDER = 720;
export const MODAL_FRAME_MIN_H = 160;
/** Hit zone thickness; half extends outside modalFG border for easier grabbing. */
export const MODAL_RESIZE_HANDLE_PX = 16;
export const MODAL_RESIZE_HANDLE_OUTSET_PX = MODAL_RESIZE_HANDLE_PX / 2;

export const MODAL_RESIZE_EDGES: ResizeEdge[] = [
  "n",
  "s",
  "e",
  "w",
  "ne",
  "nw",
  "se",
  "sw",
];

export const MODAL_RESIZE_CURSOR: Record<ResizeEdge, string> = {
  n: "ns-resize",
  s: "ns-resize",
  e: "ew-resize",
  w: "ew-resize",
  ne: "nesw-resize",
  nw: "nwse-resize",
  se: "nwse-resize",
  sw: "nesw-resize",
};

function clamp(n: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(hi, n));
}

export function getModalBounds(backdrop: HTMLElement): ModalBounds {
  const style = getComputedStyle(backdrop);
  const pad =
    parseFloat(style.paddingLeft) ||
    parseFloat(style.paddingTop) ||
    MODAL_FRAME_PAD;
  return {
    width: backdrop.clientWidth,
    height: backdrop.clientHeight,
    pad,
  };
}

function effectiveMinSize(bounds: ModalBounds, minSize: ModalMinSize): ModalMinSize {
  const maxW = bounds.width - bounds.pad * 2;
  const maxH = bounds.height - bounds.pad * 2;
  return {
    minW: Math.min(minSize.minW, maxW),
    minH: Math.min(minSize.minH, maxH),
  };
}

export function clampRect(
  rect: ModalFrameRect,
  bounds: ModalBounds,
  minSize: ModalMinSize,
): ModalFrameRect {
  const maxW = bounds.width - bounds.pad * 2;
  const maxH = bounds.height - bounds.pad * 2;
  const mins = effectiveMinSize(bounds, minSize);
  const width = clamp(rect.width, mins.minW, maxW);
  const height = clamp(rect.height, mins.minH, maxH);
  const left = clamp(rect.left, bounds.pad, bounds.pad + maxW - width);
  const top = clamp(rect.top, bounds.pad, bounds.pad + maxH - height);
  return { left, top, width, height };
}

export function centerFrame(
  naturalW: number,
  naturalH: number,
  bounds: ModalBounds,
  minSize: ModalMinSize,
): ModalFrameRect {
  const maxW = bounds.width - bounds.pad * 2;
  const maxH = bounds.height - bounds.pad * 2;
  const mins = effectiveMinSize(bounds, minSize);
  const width = clamp(naturalW, mins.minW, maxW);
  const height = clamp(naturalH, mins.minH, maxH);
  const left = bounds.pad + (maxW - width) / 2;
  const top = bounds.pad + (maxH - height) / 2;
  return clampRect({ left, top, width, height }, bounds, minSize);
}

function applyResize(
  edge: ResizeEdge,
  rect: ModalFrameRect,
  dx: number,
  dy: number,
): ModalFrameRect {
  let { left, top, width, height } = rect;

  if (edge.includes("e")) {
    width += dx;
  }
  if (edge.includes("w")) {
    left += dx;
    width -= dx;
  }
  if (edge.includes("s")) {
    height += dy;
  }
  if (edge.includes("n")) {
    top += dy;
    height -= dy;
  }

  return { left, top, width, height };
}

type PointerSessionOpts = {
  bounds: ModalBounds;
  minSize: ModalMinSize;
  onUpdate: (rect: ModalFrameRect) => void;
  cursor?: string;
};

function beginPointerSession(
  e: PointerEvent,
  target: HTMLElement,
  cursor: string,
): { startX: number; startY: number } {
  e.preventDefault();
  target.setPointerCapture(e.pointerId);
  document.body.style.userSelect = "none";
  document.body.style.cursor = cursor;
  return { startX: e.clientX, startY: e.clientY };
}

function endPointerSession(target: HTMLElement, pointerId: number): void {
  try {
    target.releasePointerCapture(pointerId);
  } catch {
    /* already released */
  }
  document.body.style.userSelect = "";
  document.body.style.cursor = "";
}

export function startModalDrag(
  e: PointerEvent,
  header: HTMLElement,
  rect: ModalFrameRect,
  opts: PointerSessionOpts,
): void {
  if (e.button !== 0) return;
  const t = e.target;
  if (!(t instanceof Element)) return;
  if (t.closest(".modal-window-controls")) return;

  const startRect = { ...rect };
  const { startX, startY } = beginPointerSession(e, header, "grabbing");

  const onMove = (ev: PointerEvent) => {
    if (ev.pointerId !== e.pointerId) return;
    const dx = ev.clientX - startX;
    const dy = ev.clientY - startY;
    opts.onUpdate(
      clampRect(
        {
          ...startRect,
          left: startRect.left + dx,
          top: startRect.top + dy,
        },
        opts.bounds,
        opts.minSize,
      ),
    );
  };

  const onEnd = (ev: PointerEvent) => {
    if (ev.pointerId !== e.pointerId) return;
    endPointerSession(header, e.pointerId);
    header.removeEventListener("pointermove", onMove);
    header.removeEventListener("pointerup", onEnd);
    header.removeEventListener("pointercancel", onEnd);
  };

  header.addEventListener("pointermove", onMove);
  header.addEventListener("pointerup", onEnd);
  header.addEventListener("pointercancel", onEnd);
}

export function startModalResize(
  e: PointerEvent,
  handle: HTMLElement,
  edge: ResizeEdge,
  rect: ModalFrameRect,
  opts: PointerSessionOpts,
): void {
  if (e.button !== 0) return;
  e.stopPropagation();

  const startRect = { ...rect };
  const { startX, startY } = beginPointerSession(
    e,
    handle,
    MODAL_RESIZE_CURSOR[edge],
  );

  const onMove = (ev: PointerEvent) => {
    if (ev.pointerId !== e.pointerId) return;
    const dx = ev.clientX - startX;
    const dy = ev.clientY - startY;
    opts.onUpdate(
      clampRect(applyResize(edge, startRect, dx, dy), opts.bounds, opts.minSize),
    );
  };

  const onEnd = (ev: PointerEvent) => {
    if (ev.pointerId !== e.pointerId) return;
    endPointerSession(handle, e.pointerId);
    handle.removeEventListener("pointermove", onMove);
    handle.removeEventListener("pointerup", onEnd);
    handle.removeEventListener("pointercancel", onEnd);
  };

  handle.addEventListener("pointermove", onMove);
  handle.addEventListener("pointerup", onEnd);
  handle.addEventListener("pointercancel", onEnd);
}
