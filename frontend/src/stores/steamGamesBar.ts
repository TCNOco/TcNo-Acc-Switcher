import { writable } from "svelte/store";

export type SteamGamesBarAccount = {
  steamId64: string;
  displayName: string;
  avatarUrl: string;
};

/** Why the strip is showing no tiles. Empty while it has some. */
export type SteamGamesBarEmptyReason = "" | "no-game" | "owners-unknown";

export type SteamGamesBarState = {
  accounts: SteamGamesBarAccount[];
  reason: SteamGamesBarEmptyReason;
};

/** Checked: picking an account also starts the selected game. */
export const steamGamesLaunchOnSwitch = writable(true);

/**
 * Tiles the action bar draws, published by the games view. Only the selected game's
 * owners belong here — clicking a tile switches to that account, so an account that
 * cannot play the selected game has no business being offered.
 */
export const steamGamesBar = writable<SteamGamesBarState>({ accounts: [], reason: "no-game" });

type AccountPickHandler = (steamId64: string) => void;

let pickHandler: AccountPickHandler | null = null;

export function setSteamGamesAccountPickHandler(handler: AccountPickHandler | null): void {
  pickHandler = handler;
}

export function pickSteamGamesAccount(steamId64: string): void {
  pickHandler?.(steamId64);
}

export function clearSteamGamesBar(): void {
  steamGamesBar.set({ accounts: [], reason: "no-game" });
  pickHandler = null;
}
