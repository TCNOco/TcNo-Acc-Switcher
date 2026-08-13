import { get, writable, type Readable } from "svelte/store";
import {
  SteamGuardCapabilityError,
  SteamGuardContentProtectionLease,
  type SteamGuardModalController,
  type SteamTradeLink,
} from "../steamGuardModal";

export type SteamGuardQuickCopyDeps = {
  controller: SteamGuardModalController;
  /** Whether the vault is open right now, which decides if the row is offered. */
  vaultUnlocked: () => Promise<boolean>;
};

let deps: SteamGuardQuickCopyDeps | null = null;
const unlocked = writable(false);

export const steamGuardVaultUnlocked: Readable<boolean> = { subscribe: unlocked.subscribe };

export function configureSteamGuardQuickCopy(next: SteamGuardQuickCopyDeps | null): void {
  deps = next;
  unlocked.set(false);
}

/**
 * The context menu is built synchronously, so it reads the last known answer.
 * Refreshing whenever a menu is built keeps that answer one right-click fresh.
 */
export async function refreshSteamGuardVaultUnlocked(): Promise<void> {
  if (!deps) {
    unlocked.set(false);
    return;
  }
  try {
    unlocked.set(await deps.vaultUnlocked());
  } catch {
    unlocked.set(false);
  }
}

export function steamGuardVaultUnlockedNow(): boolean {
  return get(unlocked);
}

/**
 * Copies one code straight to the secure clipboard, without opening the Steam
 * Guard window. The sensitive-view lease is held for the single call only, so
 * window content protection lapses again immediately.
 */
export async function copySteamGuardCodeNow(steamId64: string): Promise<void> {
  const current = deps;
  const copyCode = current?.controller.copyCode;
  if (!current || !copyCode) throw new Error("Secure clipboard unavailable");
  const lease = new SteamGuardContentProtectionLease(current.controller);
  try {
    await lease.acquire(steamId64);
    const capability = lease.capabilityFor(steamId64);
    if (!capability) throw new SteamGuardCapabilityError();
    await copyCode(steamId64, capability);
  } finally {
    await lease.close();
  }
}

/**
 * Reads one account's current trade URL, without opening the Steam Guard window.
 *
 * Same single-call lease as the code copy, but this one spans a request to Steam
 * rather than a local read, so the main window is content-protected for as long
 * as that takes. Releasing it before the caller toasts keeps that window as short
 * as the work actually is.
 */
export async function fetchTradeLinkNow(steamId64: string): Promise<SteamTradeLink> {
  const current = deps;
  const getTradeLink = current?.controller.getTradeLink;
  if (!current || !getTradeLink) throw new Error("Trade link unavailable");
  const lease = new SteamGuardContentProtectionLease(current.controller);
  try {
    await lease.acquire(steamId64);
    const capability = lease.capabilityFor(steamId64);
    if (!capability) throw new SteamGuardCapabilityError();
    return await getTradeLink(steamId64, capability);
  } finally {
    await lease.close();
  }
}
