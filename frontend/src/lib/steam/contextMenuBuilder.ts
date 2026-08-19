import { get } from "svelte/store";
import { t } from "../../stores/i18n";
import type { MenuItemDef } from "../../stores/contextMenu";
import type { SharedMenuItems } from "../../components/PlatformAccountAdapter";
import type { SteamAccountRow } from "./types";
import { createSteamMenuCommands, type SteamMenuDeps } from "./menuCommands";
import { buildSteamGuardMenuItem, canForgetSteamAccount, heldInSteamGuardVault } from "./steamGuardMenu";
import { gameDataSite } from "./steamBrowserSites";
import { steamGuardCodeRemaining } from "./steamGuardCodePeriod";

export function buildSteamExtraMenu(
  acc: SteamAccountRow,
  shared: SharedMenuItems,
  deps: SteamMenuDeps,
): MenuItemDef[] {
  const tr = get(t);
  const rid = acc.steamId64;
  const { installedGames, gameDataBySteamId } = deps;
  const commands = createSteamMenuCommands(acc, deps, tr);
  const steamGuard = buildSteamGuardMenuItem(acc, {
    openSteamGuard: deps.openSteamGuard,
    openBrowser: deps.openSteamBrowser,
    vaultUnlocked: deps.steamGuardVaultUnlocked,
  }, tr);

  const loginStates = [
    { st: 7, lab: tr("Invisible") }, { st: 0, lab: tr("Offline") }, { st: 1, lab: tr("Online") },
    { st: 2, lab: tr("Busy") }, { st: 3, lab: tr("Away") }, { st: 4, lab: tr("Snooze") },
    { st: 5, lab: tr("LookingToTrade") }, { st: 6, lab: tr("LookingToPlay") },
  ];

  const loginAsChildren: MenuItemDef[] = [
    { type: "search", label: tr("Context_Search") },
    ...loginStates.map((x) => ({
      label: x.lab,
      action: () => void commands.loginAs(x.st),
    })),
  ];

  const copyChildren: MenuItemDef[] = [
    // Only an open vault can produce a code, and only an authenticator has one.
    // The underline counts the code down so the click lands on a live code.
    ...(deps.steamGuardVaultUnlocked && acc.hasSteamGuard
      ? [{
          label: tr("Context_CopySteamGuardCode"),
          action: () => void commands.copySteamGuardCode(),
          progress: steamGuardCodeRemaining,
        }]
      : []),
    { label: tr("Context_CommunityUrl"), action: () => commands.copyCommunityUrl() },
    { label: tr("Context_CommunityUsername"), action: () => commands.copyCommunityUsername() },
    { label: tr("Context_LoginUsername"), action: () => commands.copyLoginUsername() },
    // Steam only shows an account its own trade URL, so this needs the session
    // the vault holds. A half-finished enrollment is excluded deliberately: it is
    // in the vault, but whether it holds a usable session is not guaranteed.
    ...(deps.steamGuardVaultUnlocked && (acc.hasSteamGuard || acc.steamGuardLoginOnly === true)
      ? [{
          label: tr("Context_Steam_TradeLink"),
          action: () => void commands.copyTradeLink(),
        }]
      : []),
    {
      label: tr("Context_CopySteamIdSubmenu"),
      children: [
        { label: tr("Context_Steam_Id64"), action: () => void commands.copySteamId("ID64") },
        { label: tr("Context_Steam_Id3"), action: () => void commands.copySteamId("ID3") },
        { label: tr("Context_Steam_Id32"), action: () => void commands.copySteamId("ID32") },
        // Same number as SteamID32, under the name Steam itself uses for it.
        { label: tr("Context_Steam_FriendCode"), action: () => void commands.copySteamId("FriendCode") },
        { label: tr("Context_Steam_Cs2FriendCode"), action: () => void commands.copySteamId("CS2FriendCode") },
      ],
    },
  ];

  const shortcutChildren: MenuItemDef[] = [
    { type: "search", label: tr("Context_Search") },
    ...loginStates.map((x) => ({
      label: x.lab,
      action: () => void commands.createShortcut(x.st, x.lab),
    })),
  ];

  // Steam's own Personal Game Data pages, which need a session window to open as
  // the account. Same condition as the browsing entries on the Steam Guard row:
  // an account the vault holds, with the vault open, on a build that can browse.
  const browse = deps.steamGuardVaultUnlocked && heldInSteamGuardVault(acc)
    ? deps.openSteamBrowser
    : undefined;

  const gsets = gameDataBySteamId[rid];
  const gameDataItems: MenuItemDef[] = [];
  for (const g of installedGames) {
    const aid = String(g.appId).trim();
    const hasUser = gsets?.userdata.has(aid) ?? false;
    const hasBackup = gsets?.backup.has(aid) ?? false;
    const gcpd = browse ? gameDataSite(aid) : undefined;
    // A game with a page but no local files still earns an entry; that page is
    // held by Steam, not in userdata, so having neither folder says nothing
    // about it.
    if (!hasUser && !hasBackup && !gcpd) continue;
    const children: MenuItemDef[] = [];
    if (hasUser || hasBackup) {
      children.push({
        label: "Open folder",
        action: () => void commands.openGameDataFolder(g.appId),
      });
    }
    if (hasUser) {
      children.push({
        label: tr("Context_Game_CopySettingsFrom"),
        action: () => void commands.copyGameSettingsFrom(g.appId),
      });
    }
    if (hasBackup) {
      children.push({
        label: tr("Context_Game_RestoreSettingsTo"),
        action: () => void commands.restoreGameSettingsTo(g.appId),
      });
    }
    if (hasUser) {
      children.push({
        label: tr("Context_Game_BackupData"),
        action: () => void commands.backupGameData(g.appId),
      });
    }
    if (browse && gcpd) {
      children.push({
        label: tr("Context_Steam_PersonalGameData"),
        action: () => browse(acc, gcpd),
      });
    }
    gameDataItems.push({ label: g.name, children });
  }

  const gameChildren: MenuItemDef[] =
    gameDataItems.length === 0
      ? [{ type: "item", label: tr("Context_GameData_NoFolders") }]
      : [{ type: "search", label: tr("Context_Search") }, ...gameDataItems];

  const launchChildren: MenuItemDef[] = [
    { type: "search", label: tr("Context_Search") },
    ...installedGames.map((g) => ({
      label: g.name,
      action: () => void commands.loginAndLaunchGame(g.appId, g.name),
    })),
  ];

  return [
    shared.swapTo,
    steamGuard,
    { label: tr("Context_Game_LoginAndLaunch"), children: launchChildren },
    { label: tr("Context_LoginAsSubmenu"), children: loginAsChildren },
    { label: tr("Context_CopySubmenu"), children: copyChildren },
    { ...shared.createShortcut, children: shortcutChildren },
    shared.tags,
    {
      label: tr("Context_ManageSubmenu"),
      children: ([
        { label: tr("Context_GameDataSubmenu"), children: gameChildren },
        shared.gameStats,
        // Only for an account whose ban the settings would actually paint.
        // Nothing to hide otherwise, and nothing to restore.
        acc.hasVisibleBan
          ? {
              label: acc.banStatusHidden
                ? tr("Context_Steam_ShowBanStatus")
                : tr("Context_Steam_HideBanStatus"),
              action: () => void commands.setBanStatusHidden(!acc.banStatusHidden),
            }
          : null,
        {
          label: tr("Context_Steam_OpenUserdata"),
          action: () => void commands.openUserdataFolder(),
        },
        shared.changeImage,
        shared.notes,
        // No Forget for an account the vault holds an authenticator for: see
        // canForgetSteamAccount. A session-only record still forgets, and the
        // record goes with it.
        canForgetSteamAccount(acc) ? shared.forget : null,
      ] as (MenuItemDef | null)[]).filter((x): x is MenuItemDef => x != null),
    },
  ];
}
