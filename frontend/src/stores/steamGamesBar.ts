import { writable } from "svelte/store";

export type SteamGamesBarAccount = {
  steamId64: string;
  displayName: string;
  avatarUrl: string;
};

/** Checked: picking an account also starts the selected game. */
export const steamGamesLaunchOnSwitch = writable(true);

/** Accounts the games view publishes for the action bar to draw as avatar tiles. */
export const steamGamesBarAccounts = writable<SteamGamesBarAccount[]>([]);

type AccountPickHandler = (steamId64: string) => void;

let pickHandler: AccountPickHandler | null = null;

export function setSteamGamesAccountPickHandler(handler: AccountPickHandler | null): void {
  pickHandler = handler;
}

export function pickSteamGamesAccount(steamId64: string): void {
  pickHandler?.(steamId64);
}

export function clearSteamGamesBar(): void {
  steamGamesBarAccounts.set([]);
  pickHandler = null;
}
