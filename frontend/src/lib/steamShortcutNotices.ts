import type { SteamShortcutState } from "../../bindings/TcNo-Acc-Switcher/internal/steam/models.js";

/**
 * The i18n keys to show under the "Add to Steam" toggle, in the order they are
 * worth reading.
 *
 * All of these are things the entry being written correctly does not fix on its
 * own, so none of them belong in a toast that scrolls away.
 */
export function addToSteamNotices(state: SteamShortcutState | null): string[] {
  if (!state || !state.enabled) return [];

  // Steam installed but never signed into: there is no userdata folder to put
  // the entry in, and nothing else below is worth saying yet.
  if (state.users === 0) return ["Settings_AddToSteam_NoUsers"];

  const notices: string[] = [];
  if (!state.present) notices.push("Settings_AddToSteam_Missing");
  // Steam reads the shortcut list once, when it starts.
  if (state.steamRunning) notices.push("Settings_AddToSteam_RestartSteam");
  if (state.flatpakSteam) notices.push("Settings_AddToSteam_Flatpak");
  return notices;
}
