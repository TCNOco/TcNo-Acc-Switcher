import type { SteamAccountRow } from "./types";

/**
 * Which lock the avatar draws. "stored" is a working authenticator; the other
 * two are vault records that cannot produce a code, so they must not wear the
 * colour that means "protected".
 */
export type SteamGuardBadge = {
  variant: "stored" | "pending" | "login-only";
  labelKey: string;
};

export function steamGuardBadge(account: SteamAccountRow): SteamGuardBadge | undefined {
  // Defaults to shown: the setting arrives with account enrichment, so it is
  // briefly undefined on first paint and the lock should not flicker off.
  if (account.showSteamGuardLock === false) return undefined;
  // Ordered by strength, not by flag: a record can carry more than one of these
  // and only the strongest state should reach the screen.
  if (account.hasSteamGuard) return { variant: "stored", labelKey: "SteamGuard_Badge_Stored" };
  if (account.steamGuardPending) return { variant: "pending", labelKey: "SteamGuard_Badge_Unfinished" };
  if (account.steamGuardLoginOnly) return { variant: "login-only", labelKey: "SteamGuard_Badge_LoginOnly" };
  return undefined;
}
