import type { SteamAccountRow } from "./types";

export function steamAccountVisualKey(account: SteamAccountRow): string {
  return JSON.stringify([
    account.steamId64,
    account.personaName ?? "",
    account.displayName ?? "",
    account.accountName ?? "",
    account.imageUrl ?? "",
    account.staticImageUrl ?? "",
    account.avatarFrameUrl ?? "",
    account.avatarPending ?? false,
    account.metaPending ?? false,
    account.manualProfileImage ?? false,
    account.currentSession ?? false,
    account.offline ?? false,
    account.vac ?? false,
    account.ltd ?? false,
    account.lastLogin ?? "",
    account.showSteamId ?? false,
    account.showVac ?? false,
    account.showLimited ?? false,
    account.showLastLogin ?? false,
    account.showAccUsername ?? false,
    account.collectInfo ?? false,
    account.showShortNotes ?? false,
    account.note ?? "",
    account.miniProfileHtml ?? "",
    account.showMiniProfile ?? false,
    account.showAvatarFrame ?? false,
    account.syncError ?? "",
    account.tags ?? [],
  ]);
}
