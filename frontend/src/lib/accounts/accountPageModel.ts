import type { SearchResultRow } from "../../components/SearchOverlay.svelte";
import type { AccountRowProjection, AccountSearchProjection } from "../../components/PlatformAccountAdapter";
import type { TagFilterMode } from "../accountTagsContext";
import type { PlatformSortKind } from "../../stores/platformListSort";
import { offlineSafeImageSrc, withAssetCacheBust } from "../../stores/offlineMode";
import { shouldBumpEpoch } from "./epochManager";
import type { EpochCheckRow } from "./types";

export type AccountImagePickState = {
  open: boolean;
  accountId: string;
  displayName: string;
  manual: boolean;
};

export type SearchHayCache = Map<string, { v: number; text: string }>;

export function buildAccountMap<T>(
  accounts: T[],
  rows: AccountRowProjection<T>,
): Map<string, T> {
  return new Map(accounts.map((account) => [rows.id(account), account] as const));
}

export function displayIdsForTagFilter<T>(
  accountIds: string[],
  accountMap: Map<string, T>,
  rows: AccountRowProjection<T>,
  tagFilterMode: TagFilterMode,
): string[] {
  if (tagFilterMode.kind === "all") return accountIds;
  if (tagFilterMode.kind === "untagged") {
    return accountIds.filter((id) => {
      const account = accountMap.get(id);
      return account ? (rows.tags(account) ?? []).length === 0 : true;
    });
  }

  return accountIds.filter((id) => {
    const account = accountMap.get(id);
    return account ? (rows.tags(account) ?? []).some((tag) => tag.id === tagFilterMode.id) : false;
  });
}

export function createImagePickState<T>(
  rowId: string,
  account: T | undefined,
  rows: AccountRowProjection<T>,
): AccountImagePickState {
  return {
    open: true,
    accountId: rowId,
    displayName: (account ? rows.name(account) : rowId).trim(),
    manual: account ? rows.manualProfileImage(account) : false,
  };
}

export function sortAccountIds<T>(
  accountIds: string[],
  accountById: (id: string) => T | undefined,
  rows: AccountRowProjection<T>,
  kind: PlatformSortKind,
): string[] | null {
  const ids = [...accountIds];
  const displayKey = (uid: string) => {
    const account = accountById(uid);
    return (account ? rows.name(account) : uid).trim().toLowerCase();
  };
  const lastUsedMs = (uid: string) => {
    const account = accountById(uid);
    if (!account) return 0;
    const value = Date.parse((rows.lastUsed(account) ?? "").trim());
    return Number.isNaN(value) ? 0 : value;
  };
  const accountNameKey = (uid: string) => {
    const account = accountById(uid);
    return account ? rows.accountLogin(account).trim().toLowerCase() : uid.toLowerCase();
  };
  const cmpAlpha = (x: string, y: string) => displayKey(x).localeCompare(displayKey(y));
  const cmpUser = (x: string, y: string) => accountNameKey(x).localeCompare(accountNameKey(y));

  switch (kind) {
    case "alpha_asc": ids.sort(cmpAlpha); break;
    case "alpha_desc": ids.sort((x, y) => -cmpAlpha(x, y)); break;
    case "steam_user_asc": ids.sort(cmpUser); break;
    case "steam_user_desc": ids.sort((x, y) => -cmpUser(x, y)); break;
    case "lastused_new_old":
    case "date_new_old":
      ids.sort((x, y) => lastUsedMs(y) - lastUsedMs(x) || cmpAlpha(x, y));
      break;
    case "lastused_old_new":
    case "date_old_new":
      ids.sort((x, y) => lastUsedMs(x) - lastUsedMs(y) || cmpAlpha(x, y));
      break;
    default:
      return null;
  }

  return ids;
}

export function applyAccountPatch<T>(
  accounts: T[],
  rowVersions: Record<string, number>,
  avatarEpoch: Record<string, number>,
  rows: AccountRowProjection<T>,
  targetId: string,
  nextAccount: T,
): {
  accounts: T[];
  rowVersions: Record<string, number>;
  avatarEpoch: Record<string, number>;
  changed: boolean;
} {
  const idx = accounts.findIndex((account) => rows.id(account) === targetId);
  if (idx < 0) return { accounts, rowVersions, avatarEpoch, changed: false };

  const prev = accounts[idx];
  if (rows.visualKey(prev) === rows.visualKey(nextAccount)) {
    return { accounts, rowVersions, avatarEpoch, changed: false };
  }

  const nextAccounts = [...accounts];
  nextAccounts[idx] = nextAccount;
  const nextVersions = { ...rowVersions, [targetId]: (rowVersions[targetId] ?? 0) + 1 };
  let nextEpoch = avatarEpoch;
  if (shouldBumpEpoch(prev as unknown as EpochCheckRow, nextAccount as unknown as EpochCheckRow)) {
    nextEpoch = { ...avatarEpoch, [targetId]: (avatarEpoch[targetId] ?? 0) + 1 };
  }

  return {
    accounts: nextAccounts,
    rowVersions: nextVersions,
    avatarEpoch: nextEpoch,
    changed: true,
  };
}

export function buildAccountSearchRows<T>(
  params: {
    accounts: T[];
    rows: AccountRowProjection<T> & AccountSearchProjection<T>;
    query: string;
    max: number;
    rowVersions: Record<string, number>;
    avatarEpoch: Record<string, number>;
    searchHayCache: SearchHayCache;
    offlineMode: boolean;
    profileFallback: string;
    accountBadge: string;
  },
): SearchResultRow[] {
  const trimmed = params.query.trim();
  const queryWords = trimmed ? trimmed.toLowerCase().split(/\s+/).filter(Boolean) : [];
  const getHay = (account: T): string => {
    const id = params.rows.id(account);
    const version = params.rowVersions[id] ?? 0;
    const hit = params.searchHayCache.get(id);
    if (hit && hit.v === version) return hit.text;
    const text = params.rows.searchHay(account, trimmed).toLowerCase();
    params.searchHayCache.set(id, { v: version, text });
    return text;
  };
  const matches = (text: string) => queryWords.every((word) => text.includes(word));
  const accounts = trimmed
    ? params.accounts.filter((account) => matches(getHay(account))).slice(0, params.max)
    : params.accounts.slice(0, params.max);

  return accounts.map((account) => {
    const id = params.rows.id(account);
    return {
      key: `a:${id}`,
      title: params.rows.name(account) || id,
      badge: params.accountBadge,
      accountIconUrl: offlineSafeImageSrc(
        params.offlineMode,
        withAssetCacheBust(
          params.rows.imageUrl(account) && !params.rows.imagePending(account)
            ? params.rows.imageUrl(account)
            : undefined,
          params.avatarEpoch[id] ?? 0,
        ),
        params.profileFallback,
      ),
    };
  });
}
