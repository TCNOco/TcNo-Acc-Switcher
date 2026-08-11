import { SteamGuardCapabilityError, SteamGuardContentProtectionLease } from "../steamGuardModal";
import type { SteamGuardModalController } from "../steamGuardModal";
import * as SteamBrowser from "../../../bindings/TcNo-Acc-Switcher/internal/steambrowser/service.js";
import type { SteamBrowserSite } from "./steamBrowserSites";

export type { SteamBrowserSite };

let controller: SteamGuardModalController | null = null;
let supported: boolean | null = null;

export function configureSteamBrowser(next: SteamGuardModalController | null): void {
  controller = next;
}

/**
 * Whether this build can open session windows at all. Cached after the first
 * answer, which cannot change while the app is running.
 */
export async function steamBrowserAvailable(): Promise<boolean> {
  if (supported === null) {
    try {
      supported = await SteamBrowser.Available();
    } catch {
      supported = false;
    }
  }
  return supported;
}

/**
 * Opens a browser window signed in as an account, straight from the context
 * menu without going through the Steam Guard window.
 *
 * The capability lease is the same one the quick code copy uses: it proves the
 * user has the vault open for this account, which is what the backend checks
 * before handing out a session. It is held for the single open call, because
 * the window's session is minted once and then outlives the vault entirely.
 *
 * Returns whether the account has to sign in again. That is an ordinary
 * outcome, not a failure: the caller's answer is the sign-in screen.
 */
export async function openSteamBrowserNow(
  steamId64: string,
  site: SteamBrowserSite,
): Promise<{ needsLogin: boolean }> {
  const current = controller;
  if (!current) throw new Error("Steam Guard is unavailable");

  const lease = new SteamGuardContentProtectionLease(current);
  try {
    await lease.acquire(steamId64);
    const capability = lease.capabilityFor(steamId64);
    if (!capability) throw new SteamGuardCapabilityError();
    const result = await SteamBrowser.OpenBrowser(steamId64, site, capability);
    return { needsLogin: result.needsLogin === true };
  } finally {
    await lease.close();
  }
}
