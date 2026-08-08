import type { MenuItemDef } from "../../stores/contextMenu";
import type { SteamAccountRow, SteamGuardMenuRequest } from "./types";

type Translate = (key: string, vars?: Record<string, string | number>) => string;
type OpenSteamGuard = (request: SteamGuardMenuRequest) => void;

export function buildSteamGuardMenuItem(
  acc: SteamAccountRow,
  openSteamGuard: OpenSteamGuard,
  tr: Translate,
): MenuItemDef {
  const request = (action: SteamGuardMenuRequest["action"]) => () => openSteamGuard({
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
  return { label: tr("Context_SteamGuard"), action: request(inVault ? "open" : "setup") };
}
