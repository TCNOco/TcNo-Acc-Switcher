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

  if (acc.hasSteamGuard) {
    return { label: tr("Context_SteamGuard"), action: request("open") };
  }
  return {
    label: tr("Context_2Factor"),
    children: [
      { label: tr("Context_AddSteamGuard"), action: request("add") },
      { label: tr("Context_ImportMaFile"), action: request("import") },
    ],
  };
}
