import { get } from "svelte/store";
import { t } from "../../stores/i18n";
import type { MenuItemDef } from "../../stores/contextMenu";
import type { SharedMenuItems } from "../../components/PlatformAccountAdapter";
import type { SteamAccountRow } from "./types";
import { createSteamMenuCommands, type SteamMenuDeps } from "./menuCommands";

export function buildSteamExtraMenu(
  acc: SteamAccountRow,
  shared: SharedMenuItems,
  deps: SteamMenuDeps,
): MenuItemDef[] {
  const tr = get(t);
  const rid = acc.steamId64;
  const { installedGames, gameDataBySteamId } = deps;
  const commands = createSteamMenuCommands(acc, deps, tr);

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
    { label: tr("Context_CommunityUrl"), action: () => commands.copyCommunityUrl() },
    { label: tr("Context_CommunityUsername"), action: () => commands.copyCommunityUsername() },
    { label: tr("Context_LoginUsername"), action: () => commands.copyLoginUsername() },
    {
      label: tr("Context_CopySteamIdSubmenu"),
      children: [
        { label: tr("Context_Steam_Id64"), action: () => void commands.copySteamId("ID64") },
        { label: tr("Context_Steam_Id3"), action: () => void commands.copySteamId("ID3") },
        { label: tr("Context_Steam_Id32"), action: () => void commands.copySteamId("ID32") },
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

  const gsets = gameDataBySteamId[rid];
  const gameDataItems: MenuItemDef[] = [];
  for (const g of installedGames) {
    const aid = String(g.appId).trim();
    const hasUser = gsets?.userdata.has(aid) ?? false;
    const hasBackup = gsets?.backup.has(aid) ?? false;
    if (!hasUser && !hasBackup) continue;
    const children: MenuItemDef[] = [
      {
        label: "Open folder",
        action: () => void commands.openGameDataFolder(g.appId),
      },
    ];
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
    { label: tr("Context_Game_LoginAndLaunch"), children: launchChildren },
    { label: tr("Context_LoginAsSubmenu"), children: loginAsChildren },
    { label: tr("Context_CopySubmenu"), children: copyChildren },
    { ...shared.createShortcut, children: shortcutChildren },
    shared.forget,
    shared.notes,
    shared.tags,
    {
      label: tr("Context_ManageSubmenu"),
      children: ([
        { label: tr("Context_GameDataSubmenu"), children: gameChildren },
        shared.gameStats,
        {
          label: tr("Context_Steam_OpenUserdata"),
          action: () => void commands.openUserdataFolder(),
        },
        shared.changeImage,
      ] as (MenuItemDef | null)[]).filter((x): x is MenuItemDef => x != null),
    },
  ];
}
