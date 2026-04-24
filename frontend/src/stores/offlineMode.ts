import { writable } from "svelte/store";
import * as PlatformService from "../../bindings/TcNo-Acc-Switcher/internal/platform/platformservice.js";

const STORAGE_KEY = "tcno:offlineMode";

export const offlineMode = writable(false);

/** If offline and url is http(s), use fallback; otherwise return url or fallback when empty. */
export function offlineSafeImageSrc(
  offline: boolean,
  url: string | null | undefined,
  fallback: string,
): string {
  const u = (url ?? "").trim();
  if (!u) {
    return fallback;
  }
  if (offline && /^https?:\/\//i.test(u)) {
    return fallback;
  }
  return u;
}

export async function initOfflineMode(): Promise<void> {
  let on = false;
  try {
    on = await PlatformService.GetOfflineMode();
  } catch {
    const ls = localStorage.getItem(STORAGE_KEY);
    on = ls === "1" || ls === "true";
  }
  offlineMode.set(on);
}

export async function setUserOfflineMode(enabled: boolean): Promise<void> {
  await PlatformService.SetOfflineMode(enabled);
  localStorage.setItem(STORAGE_KEY, enabled ? "1" : "0");
  offlineMode.set(enabled);
}
