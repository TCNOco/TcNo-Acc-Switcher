import { writable } from "svelte/store";

export type PlatformAccountsCacheEntry = {
  accounts: unknown[];
  accountIds: string[];
};

const cache = new Map<string, PlatformAccountsCacheEntry>();

/** Last-known account totals per platform (from Statistics.json via GetStartup). */
export const platformAccountCounts = writable<Record<string, number>>({});

export function getPlatformAccountsCache(platformKey: string): PlatformAccountsCacheEntry | undefined {
  const key = platformKey.trim();
  if (!key) return undefined;
  return cache.get(key);
}

export function setPlatformAccountsCache(platformKey: string, entry: PlatformAccountsCacheEntry): void {
  const key = platformKey.trim();
  if (!key) return;
  cache.set(key, entry);
  if (entry.accountIds.length > 0) {
    platformAccountCounts.update((counts) => ({ ...counts, [key]: entry.accountIds.length }));
  }
}

export function setPlatformAccountCounts(counts: Record<string, number> | null | undefined): void {
  platformAccountCounts.set(counts ?? {});
}
