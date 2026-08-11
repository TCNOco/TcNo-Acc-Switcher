import type { MenuItemDef } from "../../stores/contextMenu";
import type { SteamAccountRow, SteamGuardMenuRequest } from "./types";

type Translate = (key: string, vars?: Record<string, string | number>) => string;
type OpenSteamGuard = (request: SteamGuardMenuRequest) => void;
type OpenBrowser = (steamId64: string, site: "store" | "community") => void;

export type SteamGuardMenuDeps = {
  openSteamGuard: OpenSteamGuard;
  /** Absent on a build with no session-browser support, which hides the submenu. */
  openBrowser?: OpenBrowser;
  /** Only an unlocked vault can mint a session. */
  vaultUnlocked: boolean;
};

export function buildSteamGuardMenuItem(
  acc: SteamAccountRow,
  deps: SteamGuardMenuDeps,
  tr: Translate,
): MenuItemDef {
  const request = (action: SteamGuardMenuRequest["action"]) => () => deps.openSteamGuard({
    action,
    steamId64: acc.steamId64,
    accountName: (acc.accountName ?? "").trim(),
    displayName: acc.displayName?.trim() || acc.personaName?.trim() || acc.steamId64,
    pending: acc.steamGuardPending === true,
    imageUrl: acc.imageUrl?.trim() || undefined,
    staticImageUrl: acc.staticImageUrl?.trim() || undefined,
  });

  // One entry either way. An account the vault holds - in any shape, including a
  // half-finished enrollment or a session-only record - opens on itself; one it
  // does not hold opens the page that offers the ways to add it. The flows are
  // chosen there rather than in a submenu that named three of them up front.
  const inVault = acc.hasSteamGuard || acc.steamGuardPending === true || acc.steamGuardLoginOnly === true;
  const item: MenuItemDef = {
    label: tr("Context_SteamGuard"),
    action: request(inVault ? "open" : "setup"),
  };

  // The browsing entries hang off that same row rather than replacing its click,
  // which ContextMenuNest already renders as a clickable parent. They need a
  // session, so they appear only for a held account with the vault open - the
  // same condition the code-copy row uses.
  if (inVault && deps.vaultUnlocked && deps.openBrowser) {
    const open = deps.openBrowser;
    item.children = [
      { label: tr("Context_Steam_OpenStore"), action: () => open(acc.steamId64, "store") },
      { label: tr("Context_Steam_OpenCommunity"), action: () => open(acc.steamId64, "community") },
    ];
  }
  return item;
}
