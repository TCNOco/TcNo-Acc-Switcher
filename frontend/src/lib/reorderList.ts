/** Build a preview row with `null` marking the drop gap (item at `from` removed). */
export function previewSlots<T>(
  order: T[],
  from: number,
  to: number,
): (T | null)[] {
  const n = order.length;
  const without = order.filter((_, i) => i !== from);
  const out: (T | null)[] = [];
  let wi = 0;
  for (let i = 0; i < n; i++) {
    if (i === to) {
      out.push(null);
    } else {
      out.push(without[wi++]!);
    }
  }
  return out;
}

/** Same as `splice(from,1)` then `splice(to,0,moved)` on a copy. */
export function moveItem<T>(order: readonly T[], from: number, to: number): T[] {
  if (from === to) return [...order];
  const next = [...order];
  const [moved] = next.splice(from, 1);
  next.splice(to, 0, moved);
  return next;
}

export type ReorderCommand = "left" | "right" | "start" | "end";

export type ReorderCommandResult<T> = {
  items: T[];
  moved: boolean;
  fromIndex: number;
  toIndex: number;
  position: number;
  total: number;
};

function indexForReorderCommand(length: number, from: number, command: ReorderCommand): number {
  switch (command) {
    case "left":
      return Math.max(0, from - 1);
    case "right":
      return Math.min(length - 1, from + 1);
    case "start":
      return 0;
    case "end":
      return Math.max(0, length - 1);
  }
}

export function reorderItemByCommand<T>(
  order: readonly T[],
  item: T,
  command: ReorderCommand,
): ReorderCommandResult<T> {
  const fromIndex = order.indexOf(item);
  if (fromIndex < 0) {
    return {
      items: [...order],
      moved: false,
      fromIndex: -1,
      toIndex: -1,
      position: 0,
      total: order.length,
    };
  }
  const toIndex = indexForReorderCommand(order.length, fromIndex, command);
  const items = moveItem(order, fromIndex, toIndex);
  return {
    items,
    moved: fromIndex !== toIndex,
    fromIndex,
    toIndex,
    position: toIndex + 1,
    total: order.length,
  };
}

/**
 * Insert index into the list with `from` removed (short array), for left/right half hit-testing.
 */
export function insertionIndexFromTileHover(
  order: readonly string[],
  dragIndex: number,
  slotId: string,
  clientX: number,
  cell: HTMLElement,
): number {
  const short = order.filter((_, i) => i !== dragIndex);
  const refInShort = short.indexOf(slotId);
  if (refInShort < 0) return 0;
  const rect = cell.getBoundingClientRect();
  const after = clientX >= rect.left + rect.width / 2;
  let pos = after ? refInShort + 1 : refInShort;
  return Math.max(0, Math.min(pos, short.length));
}

/**
 * Insert index when the dragged item is not in `targetList` (cross-list drop).
 * Uses the same left/right half rule as [insertionIndexFromTileHover].
 */
export function insertionIndexExternalDrag(
  targetList: readonly string[],
  slotId: string,
  clientX: number,
  cell: HTMLElement,
): number {
  const ref = targetList.indexOf(slotId);
  if (ref < 0) {
    return targetList.length;
  }
  const rect = cell.getBoundingClientRect();
  const after = clientX >= rect.left + rect.width / 2;
  const pos = after ? ref + 1 : ref;
  return Math.max(0, Math.min(pos, targetList.length));
}

/** Preview with a gap at `gapIndex` (0..list.length); item is not in `list` yet. */
export function previewInsertGap<T>(list: readonly T[], gapIndex: number): (T | null)[] {
  const g = Math.max(0, Math.min(gapIndex, list.length));
  const out: (T | null)[] = [];
  let li = 0;
  for (let i = 0; i < list.length + 1; i++) {
    if (i === g) {
      out.push(null);
    } else {
      out.push(list[li++]!);
    }
  }
  return out;
}
