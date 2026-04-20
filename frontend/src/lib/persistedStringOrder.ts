/**
 * Load a persisted string order and merge with `defaults` (new ids append in default order).
 * Swap the storage backend later when account/order APIs exist.
 */
export function loadStringOrder(storageKey: string, defaults: string[]): string[] {
  if (typeof localStorage === "undefined") return defaults;
  try {
    const raw = localStorage.getItem(storageKey);
    if (!raw) return defaults;
    const saved = JSON.parse(raw) as unknown;
    if (!Array.isArray(saved)) return defaults;
    const set = new Set(defaults);
    const ordered = saved.filter(
      (x): x is string => typeof x === "string" && set.has(x),
    );
    const rest = defaults.filter((x) => !ordered.includes(x));
    return [...ordered, ...rest];
  } catch {
    return defaults;
  }
}

export function saveStringOrder(storageKey: string, order: string[]): void {
  if (typeof localStorage === "undefined") return;
  try {
    localStorage.setItem(storageKey, JSON.stringify(order));
  } catch {
    /* quota / private mode */
  }
}
