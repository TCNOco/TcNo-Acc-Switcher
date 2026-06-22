import type { AccountRowProjection } from "../../components/PlatformAccountAdapter";
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
  rows: Pick<AccountRowProjection<T>, "id">,
  existing: T[],
  list: T[],
): T[] {
  const byId = new Map(existing.map((a) => [rows.id(a), a] as const));
  return list.map((row) => {
    const prev = byId.get(rows.id(row));
    return prev ? ({ ...prev, ...row } as T) : row;
  });
}

export function mergeEnrichmentIntoExisting<T>(
  rows: Pick<AccountRowProjection<T>, "id">,
  existing: T[],
  enrich: T[],
): T[] {
  const byId = new Map(enrich.map((a) => [rows.id(a), a] as const));
  return existing.map((row) => {
    const patch = byId.get(rows.id(row));
    return patch ? ({ ...row, ...patch } as T) : row;
  });
}

function accountRowEqual<T>(rows: Pick<AccountRowProjection<T>, "visualKey">, a: T, b: T): boolean {
  return rows.visualKey(a) === rows.visualKey(b);
}

export function rowsVisuallyChanged<T>(
  projection: Pick<AccountRowProjection<T>, "id" | "visualKey">,
  rows: T[],
  prevById: Map<string, T>,
): boolean {
  for (const row of rows) {
    const prev = prevById.get(projection.id(row));
    if (!prev || !accountRowEqual(projection, prev, row)) return true;
  }
  return false;
}

export function applyLoadedAccounts<T>(
  projection: Pick<AccountRowProjection<T>, "id" | "visualKey" | "currentSession">,
  name: string,
  rows: T[],
  prevById: Map<string, T>,
  state: ApplyLoadedAccountsState<T>,
  touchStatus: () => void,
): boolean {
  let { avatarEpoch, accounts, accountIds, selectedId } = state;
  const newIds = rows.map((r) => projection.id(r));
  const idsChanged =
    newIds.length !== accountIds.length || newIds.some((id, i) => id !== accountIds[i]);
  const dataChanged = rowsVisuallyChanged(projection, rows, prevById);

  if (!idsChanged && !dataChanged) return false;

  if (dataChanged) {
    avatarEpoch = buildEpochMap(
      rows as unknown as Record<string, unknown>[],
      prevById as unknown as Map<string, Record<string, unknown>>,
      (r: unknown) => projection.id(r as T),
      avatarEpoch,
    );
    accounts = rows;
  } else if (idsChanged) {
    const byId = new Map(accounts.map((a) => [projection.id(a), a] as const));
    accounts = newIds.map((id) => byId.get(id)).filter((a): a is T => !!a);
  }

  accountIds = newIds;
  const liveRow = accounts.find((r) => projection.currentSession(r));
  platformLiveSessionId.set({ platformKey: name, uniqueId: liveRow ? projection.id(liveRow) : "" });
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
