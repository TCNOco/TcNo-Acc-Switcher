import { writable } from "svelte/store";

/** Public URL from GetPlatformExeIcon, or "" */
export const platformExeIconUrl = writable<string>("");

export type PlatformActionKind = "login" | "addNew" | "launch" | "saveCurrent";

/** Fired when the ActionBar requests an action on the platform page (increment id each time). */
export const platformAction = writable<{ id: number; kind: PlatformActionKind } | null>(null);

/** Currently selected account on the active platform page (for shortcut swap-before-launch). */
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

/**
 * Last known active session for the visible platform page (from GetSteamAccounts / GetAccounts).
 * Used so footer game shortcuts can skip swap-before-launch when already on that account.
 */
export const platformLiveSessionId = writable<{ platformKey: string; uniqueId: string }>({
  platformKey: "",
  uniqueId: "",
});

export type PlatformAccountsRefreshSignal = { seq: number; platformKey: string };

/** Bumped after external swaps (e.g. RunShortcut); platform pages subscribe and stagger-reload rows. */
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

/** Ask the platform account list (Steam or basic) to reload so current-session highlighting stays accurate. */
export function requestPlatformAccountsRefresh(platformKey: string): void {
  const k = platformKey.trim();
  if (!k) {
    return;
  }
  refreshSeq += 1;
  platformAccountsRefresh.set({ seq: refreshSeq, platformKey: k });
}

/**
 * Unique id to pass into RunShortcut when "always swap on shortcut" is on.
 * Returns "" when the selected row is already the live session so the shortcut only launches (no redundant swap/restart).
 */
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
