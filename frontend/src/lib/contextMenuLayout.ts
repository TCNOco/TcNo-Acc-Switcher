/**
 * Rects carry every ancestor transform; CSS lengths do not. The menu scales in (`in:scale`), so a
 * rect read while that animation is live is 3% short of the box the menu settles into — divide the
 * scale back out before writing the value to a style, or the flyout is sized for a height it never
 * has and its last rows paint outside it. Only *differences* may be normalized this way: the
 * transform-origin term cancels in a subtraction, never in an absolute viewport coordinate.
 */
export function scaleFromComputedTransform(transform: string): number {
  const matrix = /^matrix(3d)?\(([^)]*)\)$/.exec(transform ?? "");
  if (!matrix) {
    return 1;
  }
  const is3d = matrix[1] === "3d";
  const parts = matrix[2]!.split(",").map((value) => Number.parseFloat(value));
  if (parts.length !== (is3d ? 16 : 6) || parts.some((value) => !Number.isFinite(value))) {
    return 1;
  }
  // Rotated or skewed: no single factor undoes it, and a wrong divisor is worse than none.
  if (parts[1]! !== 0 || (is3d ? parts[4]! : parts[2]!) !== 0) {
    return 1;
  }
  const scaleY = is3d ? parts[5]! : parts[3]!;
  return scaleY > 0 ? scaleY : 1;
}

/** Converts a rect-derived length back into the CSS pixels a style property is written in. */
export function toLayoutPx(renderedValue: number, scale: number): number {
  if (!Number.isFinite(scale) || scale <= 0 || scale === 1) {
    return renderedValue;
  }
  return renderedValue / scale;
}

export type SubmenuVerticalLayout = {
  naturalTop: number;
  naturalBottom: number;
  rootTop: number;
  rootBottom: number;
};

export function submenuTopOffset(layout: SubmenuVerticalLayout): number {
  const overflow = Math.max(0, layout.naturalBottom - layout.rootBottom);
  if (overflow === 0) {
    return 0;
  }

  const availableShift = Math.max(0, layout.naturalTop - layout.rootTop);
  if (availableShift === 0) {
    return 0;
  }
  return -Math.ceil(Math.min(overflow, availableShift));
}

export type SubmenuPageRange = {
  start: number;
  end: number;
};

function normalizedRowHeights(rowHeights: number[]): number[] {
  return rowHeights.map((height) =>
    Number.isFinite(height) && height > 0 ? height : 1,
  );
}

function greedyPageRanges(rowHeights: number[], capacity: number): SubmenuPageRange[] {
  const ranges: SubmenuPageRange[] = [];
  let start = 0;
  let height = 0;

  for (let index = 0; index < rowHeights.length; index++) {
    const nextHeight = rowHeights[index]!;
    if (index > start && height + nextHeight > capacity) {
      ranges.push({ start, end: index });
      start = index;
      height = 0;
    }
    height += nextHeight;
  }
  ranges.push({ start, end: rowHeights.length });
  return ranges;
}

/**
 * Splits measured rows into the fewest possible contiguous pages, then balances spare height
 * across those pages. Page boundaries stay fixed while navigating, so wrapped labels cannot
 * resize one page and reshuffle the rest of the submenu.
 */
export function balancedSubmenuPageRanges(
  rawRowHeights: number[],
  availableHeightWithoutPagination: number,
  availableHeightWithPagination: number,
): SubmenuPageRange[] {
  const rowHeights = normalizedRowHeights(rawRowHeights);
  const itemCount = rowHeights.length;
  if (itemCount === 0) {
    return [];
  }

  const totalHeight = rowHeights.reduce((total, height) => total + height, 0);
  if (totalHeight <= Math.max(0, availableHeightWithoutPagination)) {
    return [{ start: 0, end: itemCount }];
  }

  const capacity = Math.max(0, availableHeightWithPagination);
  const greedy = greedyPageRanges(rowHeights, capacity);
  const pageCount = greedy.length;
  if (pageCount <= 1) {
    return greedy;
  }

  const prefix = [0];
  for (const height of rowHeights) {
    prefix.push(prefix[prefix.length - 1]! + height);
  }

  const costs = Array.from({ length: pageCount + 1 }, () =>
    Array<number>(itemCount + 1).fill(Number.POSITIVE_INFINITY),
  );
  const previous = Array.from({ length: pageCount + 1 }, () =>
    Array<number>(itemCount + 1).fill(-1),
  );
  costs[0]![0] = 0;

  for (let pages = 1; pages <= pageCount; pages++) {
    for (let end = pages; end <= itemCount; end++) {
      // Prefer fuller earlier pages when two partitions have the same height variance.
      for (let start = end - 1; start >= pages - 1; start--) {
        const segmentHeight = prefix[end]! - prefix[start]!;
        const isSingleOversizedRow = end === start + 1;
        if (segmentHeight > capacity && !isSingleOversizedRow) {
          continue;
        }
        const priorCost = costs[pages - 1]![start]!;
        if (!Number.isFinite(priorCost)) {
          continue;
        }
        const itemBalance = (end - start) * pageCount - itemCount;
        const normalizedSpareHeight = capacity > 0
          ? Math.max(0, capacity - segmentHeight) / capacity
          : 0;
        // Item-count balance dominates height balance, preventing avoidable singleton pages.
        const candidateCost = priorCost +
          itemBalance * itemBalance * (pageCount + 1) +
          normalizedSpareHeight * normalizedSpareHeight;
        if (candidateCost < costs[pages]![end]!) {
          costs[pages]![end] = candidateCost;
          previous[pages]![end] = start;
        }
      }
    }
  }

  if (!Number.isFinite(costs[pageCount]![itemCount]!)) {
    return greedy;
  }

  const ranges: SubmenuPageRange[] = [];
  let end = itemCount;
  for (let pages = pageCount; pages > 0; pages--) {
    const start = previous[pages]![end]!;
    if (start < 0) {
      return greedy;
    }
    ranges.push({ start, end });
    end = start;
  }
  return ranges.reverse();
}

export type SubmenuRootHeightLayout = {
  naturalTop: number;
  topOffset: number;
  naturalHeight: number;
  rootTop: number;
  rootHeight: number;
  rowHeight: number;
};

export function submenuShouldFillRootHeight(layout: SubmenuRootHeightLayout): boolean {
  if (layout.rowHeight <= 0 || layout.naturalHeight > layout.rootHeight) {
    return false;
  }
  const topGap = Math.max(0, layout.naturalTop + layout.topOffset - layout.rootTop);
  return topGap <= layout.rowHeight * 1.5;
}
