type EpochCheckRow = {
  imageUrl?: string | null;
  manualProfileImage?: boolean | null;
  avatarPending?: boolean | null;
};

function shouldBumpEpoch(prev: EpochCheckRow | undefined, row: EpochCheckRow): boolean {
  if (!prev) return false;
  if ((prev.imageUrl ?? "").trim() !== (row.imageUrl ?? "").trim()) return true;
  if ((prev.manualProfileImage ?? false) !== (row.manualProfileImage ?? false)) return true;
  if ((prev.avatarPending ?? false) !== (row.avatarPending ?? false)) return true;
  return false;
}

export function buildEpochMap<T extends EpochCheckRow>(
  rows: T[],
  prevById: Map<string, T>,
  idKey: (r: T) => string,
  currentEpoch: Record<string, number>,
): Record<string, number> {
  const next: Record<string, number> = { ...currentEpoch };
  for (const r of rows) {
    const id = idKey(r);
    if (shouldBumpEpoch(prevById.get(id), r)) {
      next[id] = (next[id] ?? 0) + 1;
    }
  }
  return next;
}
