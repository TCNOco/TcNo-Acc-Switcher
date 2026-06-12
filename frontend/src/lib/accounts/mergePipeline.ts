import type { PlatformAccountAdapter } from "../../components/PlatformAccountAdapter";
import { buildEpochMap } from "./epochManager";
import { platformLiveSessionId } from "../../stores/platformPage";
import { setPlatformAccountsCache } from "../../stores/platformAccountsCache";

export interface ApplyLoadedAccountsState<T> {
  avatarEpoch: Record<string, number>;
  accounts: T[];
  accountIds: string[];
  selectedId: string;
}

export function mergeListIntoExisting<T>(
  adapter: PlatformAccountAdapter<T>,
  existing: T[],
  list: T[],
): T[] {
  const byId = new Map(existing.map((a) => [adapter.id(a), a] as const));
  return list.map((row) => {
    const prev = byId.get(adapter.id(row));
    return prev ? ({ ...prev, ...row } as T) : row;
  });
}

export function mergeEnrichmentIntoExisting<T>(
  adapter: PlatformAccountAdapter<T>,
  existing: T[],
  enrich: T[],
): T[] {
  const byId = new Map(enrich.map((a) => [adapter.id(a), a] as const));
  return existing.map((row) => {
    const patch = byId.get(adapter.id(row));
    return patch ? ({ ...row, ...patch } as T) : row;
  });
}

function accountRowEqual<T>(a: T, b: T): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

export function rowsVisuallyChanged<T>(
  adapter: PlatformAccountAdapter<T>,
  rows: T[],
  prevById: Map<string, T>,
): boolean {
  for (const row of rows) {
    const prev = prevById.get(adapter.id(row));
    if (!prev || !accountRowEqual(prev, row)) return true;
  }
  return false;
}

export function applyLoadedAccounts<T>(
  adapter: PlatformAccountAdapter<T>,
  name: string,
  rows: T[],
  prevById: Map<string, T>,
  state: ApplyLoadedAccountsState<T>,
  touchStatus: () => void,
): boolean {
  let { avatarEpoch, accounts, accountIds, selectedId } = state;
  const newIds = rows.map((r) => adapter.id(r));
  const idsChanged =
    newIds.length !== accountIds.length || newIds.some((id, i) => id !== accountIds[i]);
  const dataChanged = rowsVisuallyChanged(adapter, rows, prevById);

  if (!idsChanged && !dataChanged) return false;

  if (dataChanged) {
    avatarEpoch = buildEpochMap(
      rows as unknown as Record<string, unknown>[],
      prevById as unknown as Map<string, Record<string, unknown>>,
      (r: unknown) => adapter.id(r as T),
      avatarEpoch,
    );
    accounts = rows;
  } else if (idsChanged) {
    const byId = new Map(accounts.map((a) => [adapter.id(a), a] as const));
    accounts = newIds.map((id) => byId.get(id)).filter((a): a is T => !!a);
  }

  accountIds = newIds;
  const liveRow = accounts.find((r) => adapter.currentSession(r));
  platformLiveSessionId.set({ platformKey: name, uniqueId: liveRow ? adapter.id(liveRow) : "" });
  const stillValid = selectedId && newIds.includes(selectedId);
  selectedId = stillValid ? selectedId : "";
  touchStatus();
  setPlatformAccountsCache(name, { accounts, accountIds });

  state.avatarEpoch = avatarEpoch;
  state.accounts = accounts;
  state.accountIds = accountIds;
  state.selectedId = selectedId;
  return true;
}
