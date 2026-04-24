import { writable } from "svelte/store";

export const platformExeIconUrl = writable<string>("");

export type PlatformActionKind = "login" | "addNew" | "launch" | "saveCurrent";

export const platformAction = writable<{ id: number; kind: PlatformActionKind } | null>(null);

export const selectedAccount = writable<{
  platformKey: string;
  uniqueId: string;
  displayName: string;
  accountLogin: string;
}>({
  platformKey: "",
  uniqueId: "",
  displayName: "",
  accountLogin: "",
});

/** Live session id for the visible platform (footer shortcuts compare against this). */
export const platformLiveSessionId = writable<{ platformKey: string; uniqueId: string }>({
  platformKey: "",
  uniqueId: "",
});

export type PlatformAccountsRefreshSignal = { seq: number; platformKey: string };

export const platformAccountsRefresh = writable<PlatformAccountsRefreshSignal>({
  seq: 0,
  platformKey: "",
});

let actionSeq = 0;
let refreshSeq = 0;

export function triggerPlatformAction(kind: PlatformActionKind): void {
  actionSeq += 1;
  platformAction.set({ id: actionSeq, kind });
}

export function requestPlatformAccountsRefresh(platformKey: string): void {
  const k = platformKey.trim();
  if (!k) {
    return;
  }
  refreshSeq += 1;
  platformAccountsRefresh.set({ seq: refreshSeq, platformKey: k });
}

/** Unique id for RunShortcut when "always swap" is on; "" if already on that account (launch only). */
export function swapUidForGameShortcut(
  platformName: string,
  selected: { platformKey: string; uniqueId: string },
  live: { platformKey: string; uniqueId: string },
): string {
  const pn = platformName.trim();
  if (!pn) {
    return "";
  }
  const uid =
    selected.platformKey === pn ? String(selected.uniqueId ?? "").trim() : "";
  if (!uid) {
    return "";
  }
  const lu = String(live.uniqueId ?? "").trim();
  if (live.platformKey === pn && lu !== "" && uid.toLowerCase() === lu.toLowerCase()) {
    return "";
  }
  return uid;
}
