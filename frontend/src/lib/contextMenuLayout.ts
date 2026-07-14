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
