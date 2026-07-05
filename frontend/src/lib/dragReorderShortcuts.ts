import { writable } from "svelte/store";
import {
  insertionIndexExternalDrag,
  insertionIndexFromTileHover,
  moveItem,
  previewInsertGap,
  previewSlots,
  reorderItemByCommand,
  type ReorderCommand,
} from "./reorderList";

export type Zone = "pinned" | "dropdown";

export interface PendingDrag {
  zone: Zone;
  fromIndex: number;
  id: string;
  startX: number;
  startY: number;
  grabOffsetX: number;
  grabOffsetY: number;
  width: number;
  height: number;
  cellEl: HTMLElement;
}

export interface DragState {
  dragSourceZone: Zone | null;
  dragFromIndex: number | null;
  dragOverZone: Zone | null;
  dragOverIndex: number | null;
  draggingId: string | null;
  pendingDrag: PendingDrag | null;
  dragVisualClone: HTMLElement | null;
  ghostX: number;
  ghostY: number;
  ghostW: number;
  ghostH: number;
}

export interface DragReorderCallbacks {
  getPins(): string[];
  getDrops(): string[];
  setPins(p: string[]): void;
  setDrops(d: string[]): void;
  isDropdownOpen(): boolean;
  onPersist(): Promise<void>;
  afterDrag(): void;
}

const dragThresholdPx = 8;

export type ShortcutReorderCommand = ReorderCommand | "pin" | "unpin" | "move-to-pinned" | "move-to-dropdown";

export type ShortcutReorderCommandResult = {
  pins: string[];
  drops: string[];
  moved: boolean;
  zone: Zone;
  position: number;
  total: number;
};

function moveShortcutAcrossZones(
  pins: readonly string[],
  drops: readonly string[],
  id: string,
  toZone: Zone,
): ShortcutReorderCommandResult {
  const source = pins.includes(id) ? "pinned" : drops.includes(id) ? "dropdown" : null;
  const nextPins = pins.filter((entry) => entry !== id);
  const nextDrops = drops.filter((entry) => entry !== id);
  if (!source) {
    return {
      pins: [...pins],
      drops: [...drops],
      moved: false,
      zone: toZone,
      position: 0,
      total: toZone === "pinned" ? pins.length : drops.length,
    };
  }
  if (toZone === "pinned") {
    nextPins.push(id);
  } else {
    nextDrops.push(id);
  }
  const target = toZone === "pinned" ? nextPins : nextDrops;
  return {
    pins: nextPins,
    drops: nextDrops,
    moved: source !== toZone,
    zone: toZone,
    position: target.indexOf(id) + 1,
    total: target.length,
  };
}

export function reorderShortcutByCommand(
  pins: readonly string[],
  drops: readonly string[],
  zone: Zone,
  id: string,
  command: ShortcutReorderCommand,
): ShortcutReorderCommandResult {
  if (command === "pin" || command === "move-to-pinned") {
    return moveShortcutAcrossZones(pins, drops, id, "pinned");
  }
  if (command === "unpin" || command === "move-to-dropdown") {
    return moveShortcutAcrossZones(pins, drops, id, "dropdown");
  }

  const source = zone === "pinned" ? pins : drops;
  const reordered = reorderItemByCommand(source, id, command);
  return {
    pins: zone === "pinned" ? reordered.items : [...pins],
    drops: zone === "dropdown" ? reordered.items : [...drops],
    moved: reordered.moved,
    zone,
    position: reordered.position,
    total: reordered.total,
  };
}

function pointInRect(
  x: number,
  y: number,
  r: DOMRect,
  inset = 0,
): boolean {
  return (
    x >= r.left - inset &&
    x <= r.right + inset &&
    y >= r.top - inset &&
    y <= r.bottom + inset
  );
}

/** Remove data attributes and style the cloned node for ghost display. */
export function stripCellForGhostClone(el: HTMLElement): void {
  el.removeAttribute("data-dnd-cell");
  el.removeAttribute("data-dnd-visual");
  el.removeAttribute("data-dnd-name");
  el.removeAttribute("data-dnd-gap");
  el.removeAttribute("tabindex");
  el.removeAttribute("role");
  el.classList.remove("shortcutDndCell");
  el.style.cursor = "grabbing";
  el.style.width = "100%";
  el.style.height = "100%";
  el.style.boxSizing = "border-box";
  el.style.margin = "0";
  el.style.maxWidth = "none";
  el.style.maxHeight = "none";
  el.querySelectorAll("input").forEach((n) => n.remove());
  el.querySelectorAll("[id]").forEach((n) => n.removeAttribute("id"));
  el.querySelectorAll("label[for]").forEach((n) =>
    n.removeAttribute("for"),
  );
}

/** Pure computation of the pinned display slots during drag. */
export function computeDisplayPinned(
  pins: string[],
  draggingId: string | null,
  dragSourceZone: Zone | null,
  dragFromIndex: number | null,
  dragOverZone: Zone | null,
  dragOverIndex: number | null,
): (string | null)[] {
  if (
    draggingId == null ||
    dragSourceZone == null ||
    dragFromIndex == null
  ) {
    return [...pins];
  }
  const overIdx = dragOverIndex ?? dragFromIndex;
  const overZ = dragOverZone ?? dragSourceZone;
  if (dragSourceZone === "pinned") {
    if (overZ === "pinned") {
      return previewSlots(pins, dragFromIndex, overIdx);
    }
    return pins.filter((id) => id !== draggingId);
  }
  if (overZ === "pinned") {
    return previewInsertGap(pins, overIdx);
  }
  return [...pins];
}

/** Pure computation of the dropdown display slots during drag. */
export function computeDisplayDrop(
  drops: string[],
  draggingId: string | null,
  dragSourceZone: Zone | null,
  dragFromIndex: number | null,
  dragOverZone: Zone | null,
  dragOverIndex: number | null,
): (string | null)[] {
  if (
    draggingId == null ||
    dragSourceZone == null ||
    dragFromIndex == null
  ) {
    return [...drops];
  }
  const overIdx = dragOverIndex ?? dragFromIndex;
  const overZ = dragOverZone ?? dragSourceZone;
  if (dragSourceZone === "dropdown") {
    if (overZ === "dropdown") {
      return previewSlots(drops, dragFromIndex, overIdx);
    }
    return drops.filter((id) => id !== draggingId);
  }
  if (overZ === "dropdown") {
    return previewInsertGap(drops, overIdx);
  }
  return [...drops];
}

export function createDragReorderController(cbs: DragReorderCallbacks) {
  const initialState: DragState = {
    dragSourceZone: null,
    dragFromIndex: null,
    dragOverZone: null,
    dragOverIndex: null,
    draggingId: null,
    pendingDrag: null,
    dragVisualClone: null,
    ghostX: 0,
    ghostY: 0,
    ghostW: 0,
    ghostH: 0,
  };

  let state: DragState = { ...initialState };
  const store = writable<DragState>(state);

  function syncState(): void {
    store.set({ ...state });
  }

  let pinListEl: HTMLDivElement | null = null;
  let dropListEl: HTMLDivElement | null = null;

  function beginPointerDrag(
    e: PointerEvent,
    pd: NonNullable<DragState["pendingDrag"]>,
  ): void {
    const visual = pd.cellEl.cloneNode(true) as HTMLElement;
    stripCellForGhostClone(visual);
    state.dragVisualClone = visual;

    state.dragSourceZone = pd.zone;
    state.dragFromIndex = pd.fromIndex;
    state.draggingId = pd.id;
    state.dragOverZone = pd.zone;
    state.dragOverIndex = pd.fromIndex;
    state.ghostW = pd.width;
    state.ghostH = pd.height;
    state.ghostX = e.clientX - pd.grabOffsetX;
    state.ghostY = e.clientY - pd.grabOffsetY;
    document.body.style.userSelect = "none";
    document.body.dataset.dragging = "true";
    syncState();
    hitTestDragOver(e.clientX, e.clientY);
  }

  function moveWithinZone(
    zone: string,
    from: number,
    to: number,
  ): void {
    if (zone === "pinned") cbs.setPins(moveItem(cbs.getPins(), from, to));
    else cbs.setDrops(moveItem(cbs.getDrops(), from, to));
  }

  function moveAcrossZones(
    fromZ: string,
    toZ: string,
    from: number,
    to: number,
    id: string,
  ): void {
    let p = [...cbs.getPins()];
    let d = [...cbs.getDrops()];
    if (fromZ === "pinned") {
      p.splice(from, 1);
      d.splice(to, 0, id);
    } else {
      d.splice(from, 1);
      p.splice(to, 0, id);
    }
    cbs.setPins(p);
    cbs.setDrops(d);
  }

  function commitDrag(): void {
    if (
      state.dragSourceZone == null ||
      state.dragFromIndex == null ||
      state.dragOverZone == null ||
      state.dragOverIndex == null ||
      state.draggingId == null
    )
      return;

    if (state.dragSourceZone === state.dragOverZone) {
      if (state.dragFromIndex === state.dragOverIndex) return;
      moveWithinZone(
        state.dragSourceZone,
        state.dragFromIndex,
        state.dragOverIndex,
      );
    } else {
      moveAcrossZones(
        state.dragSourceZone,
        state.dragOverZone,
        state.dragFromIndex,
        state.dragOverIndex,
        state.draggingId,
      );
    }
    void cbs.onPersist();
  }

  function endPointerDrag(commit: boolean): void {
    if (state.pendingDrag && state.dragSourceZone === null) {
      state.pendingDrag = null;
      syncState();
      return;
    }
    if (state.dragSourceZone !== null) {
      const shouldCommit =
        commit &&
        state.dragOverZone != null &&
        state.dragOverIndex != null &&
        (state.dragSourceZone !== state.dragOverZone ||
          state.dragFromIndex !== state.dragOverIndex);
      cbs.afterDrag();
      if (shouldCommit) commitDrag();
      state.dragSourceZone = null;
      state.dragFromIndex = null;
      state.dragOverZone = null;
      state.dragOverIndex = null;
      state.draggingId = null;
      state.dragVisualClone = null;
      state.pendingDrag = null;
      document.body.style.userSelect = "";
      delete document.body.dataset.dragging;
      syncState();
    } else {
      state.pendingDrag = null;
      syncState();
    }
  }

  function hitTestDragOver(clientX: number, clientY: number): void {
    if (
      state.dragSourceZone === null ||
      state.dragFromIndex === null ||
      !state.draggingId
    )
      return;
    const fromIdx = state.dragFromIndex;
    const srcZone = state.dragSourceZone;

    function computeDropIndex(
      zone: Zone,
      srcZone: Zone,
      names: string[],
      fromIdx: number,
      id: string,
      cell: HTMLElement,
    ): number {
      const sameSource =
        (zone === "pinned" && srcZone === "pinned") ||
        (zone === "dropdown" && srcZone === "dropdown");
      if (sameSource)
        return insertionIndexFromTileHover(
          names,
          fromIdx,
          id,
          clientX,
          cell,
        );
      return insertionIndexExternalDrag(names, id, clientX, cell);
    }

    function tryBoundary(
      zone: Zone,
      srcZone: Zone,
      fromIdx: number,
      namesLen: number,
      root: HTMLDivElement,
    ): boolean {
      const br = root.getBoundingClientRect();
      if (!pointInRect(clientX, clientY, br, 4)) return false;

      if (
        zone === "pinned" &&
        cbs.isDropdownOpen() &&
        dropListEl &&
        pointInRect(clientX, clientY, dropListEl.getBoundingClientRect())
      )
        return false;

      state.dragOverZone = zone;
      if (srcZone === zone) state.dragOverIndex = fromIdx;
      else if (namesLen === 0) state.dragOverIndex = 0;
      else state.dragOverIndex = namesLen;
      return true;
    }

    const tryZone = (
      zone: Zone,
      root: HTMLDivElement | null,
      names: string[],
    ): boolean => {
      if (!root) return false;

      const cells = root.querySelectorAll("[data-dnd-cell]");
      for (const cell of [...cells]) {
        const r = cell.getBoundingClientRect();
        if (!pointInRect(clientX, clientY, r)) continue;

        const h = cell as HTMLElement;
        if (h.dataset.dndGap === "true") {
          const visual = Number(h.dataset.dndVisual);
          if (!Number.isNaN(visual)) {
            state.dragOverZone = zone;
            state.dragOverIndex = visual;
            return true;
          }
          continue;
        }

        const id = h.dataset.dndName ?? "";
        if (!id) continue;

        state.dragOverZone = zone;
        state.dragOverIndex = computeDropIndex(
          zone,
          srcZone,
          names,
          fromIdx,
          id,
          h,
        );
        return true;
      }

      return tryBoundary(zone, srcZone, fromIdx, names.length, root);
    };

    if (tryZone("pinned", pinListEl, cbs.getPins())) { syncState(); return; }
    if (cbs.isDropdownOpen() && tryZone("dropdown", dropListEl, cbs.getDrops())) { syncState(); return; }
  }

  function onPointerMove(e: PointerEvent): void {
    if (!state.pendingDrag && state.dragSourceZone === null) return;

    if (state.pendingDrag && state.dragSourceZone === null) {
      const dx = e.clientX - state.pendingDrag.startX;
      const dy = e.clientY - state.pendingDrag.startY;
      if (dx * dx + dy * dy < dragThresholdPx * dragThresholdPx) return;
      beginPointerDrag(e, state.pendingDrag);
    }

    if (state.dragSourceZone !== null && state.pendingDrag) {
      state.ghostX = e.clientX - state.pendingDrag.grabOffsetX;
      state.ghostY = e.clientY - state.pendingDrag.grabOffsetY;
      syncState();
      hitTestDragOver(e.clientX, e.clientY);
    }
  }

  function onPointerUp(_e: PointerEvent): void {
    if (state.pendingDrag && state.dragSourceZone === null) {
      state.pendingDrag = null;
      syncState();
      return;
    }
    endPointerDrag(true);
  }

  function onPointerCancel(_e: PointerEvent): void {
    endPointerDrag(false);
  }

  function onKeyDown(e: KeyboardEvent): void {
    if (e.key !== "Escape") return;
    if (state.dragSourceZone === null && !state.pendingDrag) return;
    e.preventDefault();
    endPointerDrag(false);
  }

  function onCellPointerDown(
    e: PointerEvent,
    z: string,
    id: string,
  ): void {
    if (e.button !== 0) return;
    const zone = z as Zone;
    const arr = zone === "pinned" ? cbs.getPins() : cbs.getDrops();
    const from = arr.indexOf(id);
    if (from < 0) return;
    const el = e.currentTarget as HTMLElement;
    const r = el.getBoundingClientRect();
    state.pendingDrag = {
      zone,
      fromIndex: from,
      id,
      startX: e.clientX,
      startY: e.clientY,
      grabOffsetX: e.clientX - r.left,
      grabOffsetY: e.clientY - r.top,
      width: r.width,
      height: r.height,
      cellEl: el,
    };
    syncState();
  }

  function resetState(): void {
    state = { ...initialState };
    document.body.style.userSelect = "";
    delete document.body.dataset.dragging;
    syncState();
  }

  return {
    subscribe: store.subscribe,
    setPinListEl(el: HTMLDivElement | null) { pinListEl = el; },
    setDropListEl(el: HTMLDivElement | null) { dropListEl = el; },
    onPointerMove,
    onPointerUp,
    onPointerCancel,
    onKeyDown,
    onCellPointerDown,
    resetState,
  };
}
