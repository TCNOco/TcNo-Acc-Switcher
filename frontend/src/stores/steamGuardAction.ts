import { writable } from "svelte/store";

export type SteamGuardActionAccount = {
  steamId64: string;
  accountName: string;
  displayName: string;
  /** Switcher avatar, so the Steam Guard unlock screen needs no locked-vault refetch. */
  imageUrl?: string;
  staticImageUrl?: string;
};

type SteamGuardAccountSource = {
  steamId64?: string;
  accountName?: string;
  displayName?: string;
  personaName?: string;
  hasSteamGuard?: boolean;
  steamGuardLoginOnly?: boolean;
  imageUrl?: string;
  staticImageUrl?: string;
};

export function steamGuardActionAccountFrom(
  accounts: SteamGuardAccountSource[],
): SteamGuardActionAccount | null {
  // The toolbar entry opens the vault, and a login-only record is in the vault
  // just as much as an authenticator is. Authenticators still win the tie so the
  // default target is one that can actually produce a code.
  const account =
    accounts.find((candidate) => candidate.hasSteamGuard === true)
    ?? accounts.find((candidate) => candidate.steamGuardLoginOnly === true);
  const steamId64 = account?.steamId64?.trim() ?? "";
  if (!account || !steamId64) return null;

  const accountName = account.accountName?.trim() || steamId64;
  return {
    steamId64,
    accountName,
    displayName:
      account.displayName?.trim()
      || account.personaName?.trim()
      || accountName,
    imageUrl: account.imageUrl?.trim() || undefined,
    staticImageUrl: account.staticImageUrl?.trim() || undefined,
  };
}

export const steamGuardActionAccount = writable<SteamGuardActionAccount | null>(null);

export function publishSteamGuardActionAccounts(accounts: SteamGuardAccountSource[]): void {
  steamGuardActionAccount.set(steamGuardActionAccountFrom(accounts));
}

export function clearSteamGuardActionAccount(): void {
  steamGuardActionAccount.set(null);
}
