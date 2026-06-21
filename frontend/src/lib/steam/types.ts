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
};

export type SteamAccountPatch = AccountPatch & {
  avatarFrameUrl?: string;
  miniProfileHtml?: string;
  showMiniProfile?: boolean;
  showAvatarFrame?: boolean;
};
