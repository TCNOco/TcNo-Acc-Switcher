import { get } from "svelte/store";
import { t } from "../../stores/i18n";
import { pushToast } from "../../stores/toast";
import type { MenuItemDef } from "../../stores/contextMenu";
import type { SharedMenuItems } from "../../components/PlatformAccountAdapter";
import type { SteamAccountRow } from "./types";
import { formatWailsError, formatToastWithError } from "../formatWailsError";
import { reportLaunchFailure } from "../adminFlow";
import * as SteamService from "../../../bindings/TcNo-Acc-Switcher/internal/steam/steamservice.js";
import * as Shortcuts from "wails-shortcuts-service";

const STEAM_USERDATA_ERR_KEYS = new Set([
  "Toast_NoValidSteamId",
  "Toast_SameAccount",
  "Toast_NoFindSteamUserdata",
  "Toast_NoFindGameBackup",
]);

function mapSteamUserdataI18nError(err: unknown, tr: (k: string, v?: Record<string, string | number>) => string): string {
  return formatWailsError(err, { translateMessage: (k) => tr(k), i18nFirstLineKeys: STEAM_USERDATA_ERR_KEYS });
}

async function clipboardWrite(text: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(text);
    pushToast({ type: "success", message: get(t)("Toast_Copied"), duration: 2500 });
  } catch {
    pushToast({ type: "error", message: get(t)("Toast_CopyFailed"), duration: 4000 });
  }
}

export interface SteamMenuDeps {
  name: string;
  installedGames: { appId: string; name: string }[];
  gameDataBySteamId: Record<string, { userdata: Set<string>; backup: Set<string> }>;
  steamIds: string[];
  refreshGameDataAppSets: (ids: string[]) => Promise<void>;
}

export function buildSteamExtraMenu(
  acc: SteamAccountRow,
  shared: SharedMenuItems,
  deps: SteamMenuDeps,
): MenuItemDef[] {
  const tr = get(t);
  const rid = acc.steamId64;
  const { name, installedGames, gameDataBySteamId, steamIds, refreshGameDataAppSets } = deps;

  const loginStates = [
    { st: 7, lab: tr("Invisible") }, { st: 0, lab: tr("Offline") }, { st: 1, lab: tr("Online") },
    { st: 2, lab: tr("Busy") }, { st: 3, lab: tr("Away") }, { st: 4, lab: tr("Snooze") },
    { st: 5, lab: tr("LookingToTrade") }, { st: 6, lab: tr("LookingToPlay") },
  ];

  const loginAsChildren: MenuItemDef[] = [
    { type: "search", label: tr("Context_Search") },
    ...loginStates.map((x) => ({
      label: x.lab,
      action: async () => {
        try {
          await SteamService.SwapToSteamAccount(rid, x.st, []);
          pushToast({ type: "success", message: tr("Toast_AccountSwitched"), duration: 4000 });
        } catch (e) {
          pushToast({ type: "error", message: formatToastWithError(tr("Toast_SwitchFailed"), e), duration: 8000 });
        }
      },
    })),
  ];

  const copyChildren: MenuItemDef[] = [
    { label: tr("Context_CommunityUrl"), action: () => void clipboardWrite(`https://steamcommunity.com/profiles/${rid}`) },
    { label: tr("Context_CommunityUsername"), action: () => void clipboardWrite((acc.displayName ?? "").trim() || (acc.personaName ?? "").trim() || rid) },
    { label: tr("Context_LoginUsername"), action: () => void clipboardWrite((acc.accountName ?? "").trim() || rid) },
    {
      label: tr("Context_CopySteamIdSubmenu"),
      children: [
        {
          label: tr("Context_Steam_Id64"),
          action: async () => {
            try {
              const f = await SteamService.GetSteamIDFormats(rid);
              void clipboardWrite(f["ID64"] ?? rid);
            } catch {
              void clipboardWrite(rid);
            }
          },
        },
        {
          label: tr("Context_Steam_Id3"),
          action: async () => {
            try {
              const f = await SteamService.GetSteamIDFormats(rid);
              void clipboardWrite(f["ID3"] ?? "");
            } catch {}
          },
        },
        {
          label: tr("Context_Steam_Id32"),
          action: async () => {
            try {
              const f = await SteamService.GetSteamIDFormats(rid);
              void clipboardWrite(f["ID32"] ?? "");
            } catch {}
          },
        },
      ],
    },
  ];

  const shortcutChildren: MenuItemDef[] = [
    { type: "search", label: tr("Context_Search") },
    ...loginStates.map((x) => ({
      label: x.lab,
      action: async () => {
        try {
          const p = await Shortcuts.CreateAccountShortcut(
            "Steam",
            rid,
            acc.displayName?.trim() || acc.personaName?.trim() || rid,
            String(x.st),
            x.lab,
            (acc.accountName ?? "").trim(),
          );
          pushToast({ type: "success", message: `${tr("Toast_ShortcutCreated")}\n${p}`, duration: 6000 });
        } catch (e) {
          pushToast({ type: "error", message: formatToastWithError(tr("Toast_SwitchFailed"), e), duration: 8000 });
        }
      },
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
        action: async () => {
          try {
            await SteamService.OpenSteamGameDataFolder(rid, g.appId);
          } catch (e) {
            pushToast({ type: "error", message: mapSteamUserdataI18nError(e, tr), duration: 8000 });
          }
        },
      },
    ];
    if (hasUser) {
      children.push({
        label: tr("Context_Game_CopySettingsFrom"),
        action: async () => {
          try {
            await SteamService.CopySteamGameSettingsFrom(rid, g.appId);
            pushToast({ type: "success", message: tr("Toast_SettingsCopied"), duration: 5000 });
            void refreshGameDataAppSets(steamIds);
          } catch (e) {
            pushToast({ type: "error", message: mapSteamUserdataI18nError(e, tr), duration: 8000 });
          }
        },
      });
    }
    if (hasBackup) {
      children.push({
        label: tr("Context_Game_RestoreSettingsTo"),
        action: async () => {
          try {
            await SteamService.RestoreSteamGameSettingsTo(rid, g.appId);
            pushToast({ type: "success", message: tr("Toast_GameDataRestored"), duration: 5000 });
            void refreshGameDataAppSets(steamIds);
          } catch (e) {
            pushToast({ type: "error", message: mapSteamUserdataI18nError(e, tr), duration: 8000 });
          }
        },
      });
    }
    if (hasUser) {
      children.push({
        label: tr("Context_Game_BackupData"),
        action: async () => {
          try {
            const folder = await SteamService.BackupSteamGameData(rid, g.appId);
            pushToast({ type: "success", message: tr("Toast_GameBackupDone", { folderLocation: folder }), duration: 8000 });
            void refreshGameDataAppSets(steamIds);
          } catch (e) {
            pushToast({ type: "error", message: mapSteamUserdataI18nError(e, tr), duration: 8000 });
          }
        },
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
      action: async () => {
        try {
          await SteamService.LoginAndLaunchGame(rid, -1, g.appId);
          pushToast({
            type: "success",
            message: tr("Toast_StartedGame", { program: g.name }),
            duration: 4000,
          });
        } catch (e) {
          await reportLaunchFailure(e, name);
        }
      },
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
          action: async () => {
            try {
              await SteamService.OpenUserdataFolder(rid);
            } catch (e) {
              pushToast({ type: "error", message: formatToastWithError(get(t)("Toast_LaunchFailed"), e), duration: 8000 });
            }
          },
        },
        shared.changeImage,
      ] as (MenuItemDef | null)[]).filter((x): x is MenuItemDef => x != null),
    },
  ];
}
