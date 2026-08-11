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
  /**
   * Vault record holding a Steam session but no authenticator. Deliberately not
   * hasSteamGuard: there is no code to show, so the lock icon would be a lie.
   */
  steamGuardLoginOnly?: boolean;
  /** Display setting, not a property of the account. */
  showSteamGuardLock?: boolean;
  /** RFC3339 UTC instant, empty when there is no CS2 cooldown. */
  cs2CooldownExpiresAt?: string;
  cs2CooldownPermanent?: boolean;
  showCs2Cooldown?: boolean;
};

/**
 * "open" is an account the vault already holds; "setup" is one it does not, and
 * lands on the page offering the ways to add it. The context menu picks between
 * them so there is a single Steam Guard entry rather than a submenu of flows.
 * "add" belongs to no account at all - it is how an account that is in neither
 * the vault nor the switcher gets in - so its request carries empty ids.
 */
export type SteamGuardMenuAction = "open" | "all" | "setup" | "add" | "login-again";

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
