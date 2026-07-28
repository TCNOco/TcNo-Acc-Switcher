import type { AccountDTO, AccountPatch } from "../../../bindings/TcNo-Acc-Switcher/internal/steam/models.js";
import type { TagDefRow } from "../accountTagsContext";

export type SteamAccountRow = InstanceType<typeof AccountDTO> & {
  tags?: TagDefRow[];
  syncError?: string;
  currentSession: boolean;
  showShortNotes: boolean;
  note: string;
  staticImageUrl?: string;
  avatarFrameUrl?: string;
  miniProfileHtml?: string;
  showMiniProfile?: boolean;
  showAvatarFrame?: boolean;
  hasSteamGuard: boolean;
  steamGuardPending: boolean;
};

export type SteamGuardMenuAction = "open" | "all" | "add" | "import";

export type SteamGuardMenuRequest = {
  action: SteamGuardMenuAction;
  steamId64: string;
  accountName: string;
  displayName: string;
  pending: boolean;
  /**
   * Switcher avatar carried from the already-loaded account row. The Steam Guard
   * unlock screen runs while the vault is locked, so it cannot re-derive this from
   * the vault, and a fresh enrichment fetch there can fail while the page still
   * shows a cached avatar.
   */
  imageUrl?: string;
  staticImageUrl?: string;
};

export type SteamAccountPatch = AccountPatch & {
  avatarFrameUrl?: string;
  miniProfileHtml?: string;
  showMiniProfile?: boolean;
  showAvatarFrame?: boolean;
};
