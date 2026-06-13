import { writable } from "svelte/store";

export type PlatformAccountsCacheEntry = {
  accounts: unknown[];
  accountIds: string[];
};

const cache = new Map<string, PlatformAccountsCacheEntry>();

/** Last-known account totals per platform (from GetStartup). */
export const platformAccountCounts = writable<Record<string, number>>({});

export type PlatformTagCountEntry = {
  tagCount: number;
  taggedAccountCount: number;
};

/** Last-known tag & tagged-account totals per platform (from GetStartup). */
export const platformTagCounts = writable<Record<string, PlatformTagCountEntry>>({});

export function setPlatformTagCounts(counts: Record<string, { tagCount?: number; taggedAccountCount?: number } | undefined> | null | undefined): void {
  const next: Record<string, PlatformTagCountEntry> = {};
  for (const [key, value] of Object.entries(counts ?? {})) {
    if (value && typeof value.tagCount === "number" && typeof value.taggedAccountCount === "number") {
      next[key] = { tagCount: value.tagCount, taggedAccountCount: value.taggedAccountCount };
    }
  }
  platformTagCounts.set(next);
}

export function getPlatformAccountsCache(platformKey: string): PlatformAccountsCacheEntry | undefined {
  const key = platformKey.trim();
  if (!key) return undefined;
  return cache.get(key);
}

export function setPlatformAccountsCache(platformKey: string, entry: PlatformAccountsCacheEntry): void {
  const key = platformKey.trim();
  if (!key) return;
  cache.set(key, entry);
  platformAccountCounts.update((counts) => ({ ...counts, [key]: entry.accountIds.length }));
}

export function setPlatformAccountCounts(counts: Record<string, number | undefined> | null | undefined): void {
  const next: Record<string, number> = {};
  for (const [key, value] of Object.entries(counts ?? {})) {
    if (typeof value === "number" && value >= 0) next[key] = value;
  }
  platformAccountCounts.set(next);
}
