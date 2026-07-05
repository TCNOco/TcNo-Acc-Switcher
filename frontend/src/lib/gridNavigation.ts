export type GridNavigationKey = "ArrowLeft" | "ArrowRight" | "ArrowUp" | "ArrowDown";

export type GridNavigationCell = {
  id: string;
  index: number;
  left: number;
  right: number;
  top: number;
  bottom: number;
  width: number;
  height: number;
};

function centerX(cell: GridNavigationCell): number {
  return cell.left + cell.width / 2;
}

function centerY(cell: GridNavigationCell): number {
  return cell.top + cell.height / 2;
}

function spatialDistanceScore(from: GridNavigationCell, to: GridNavigationCell, direction: "up" | "down"): number {
  const verticalDistance = direction === "up" ? from.top - to.bottom : to.top - from.bottom;
  const horizontalDistance = Math.abs(centerX(from) - centerX(to));
  return Math.max(0, verticalDistance) * 10000 + horizontalDistance;
}

function nearestVerticalCell(
  cells: readonly GridNavigationCell[],
  current: GridNavigationCell,
  direction: "up" | "down",
): GridNavigationCell | null {
  const fromCenterY = centerY(current);
  const candidates = cells.filter((cell) =>
    direction === "up" ? centerY(cell) < fromCenterY : centerY(cell) > fromCenterY,
  );

  candidates.sort((a, b) => {
    const byDistance = spatialDistanceScore(current, a, direction) - spatialDistanceScore(current, b, direction);
    if (byDistance !== 0) return byDistance;
    return a.index - b.index;
  });

  return candidates[0] ?? null;
}

export function nextGridNavigationId(
  cells: readonly GridNavigationCell[],
  currentId: string,
  key: GridNavigationKey,
): string | null {
  const currentIndex = cells.findIndex((cell) => cell.id === currentId);
  if (currentIndex < 0) return null;

  if (key === "ArrowLeft") return cells[currentIndex - 1]?.id ?? null;
  if (key === "ArrowRight") return cells[currentIndex + 1]?.id ?? null;

  const current = cells[currentIndex];
  const next = nearestVerticalCell(cells, current, key === "ArrowUp" ? "up" : "down");
  return next?.id ?? null;
}
