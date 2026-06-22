import { pushToast } from "../../stores/toast";
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

type Translate = (key: string, vars?: Record<string, string | number>) => string;

export interface SteamMenuDeps {
  name: string;
  installedGames: { appId: string; name: string }[];
  gameDataBySteamId: Record<string, { userdata: Set<string>; backup: Set<string> }>;
  steamIds: string[];
  refreshGameDataAppSets: (ids: string[]) => Promise<void>;
}

function mapSteamUserdataI18nError(err: unknown, tr: Translate): string {
  return formatWailsError(err, { translateMessage: (key) => tr(key), i18nFirstLineKeys: STEAM_USERDATA_ERR_KEYS });
}

async function clipboardWrite(text: string, tr: Translate): Promise<void> {
  try {
    await navigator.clipboard.writeText(text);
    pushToast({ type: "success", message: tr("Toast_Copied"), duration: 2500 });
  } catch {
    pushToast({ type: "error", message: tr("Toast_CopyFailed"), duration: 4000 });
  }
}

export function createSteamMenuCommands(acc: SteamAccountRow, deps: SteamMenuDeps, tr: Translate) {
  const rid = acc.steamId64;
  const refreshGameData = () => void deps.refreshGameDataAppSets(deps.steamIds);
  const accountDisplay = acc.displayName?.trim() || acc.personaName?.trim() || rid;
  const loginName = (acc.accountName ?? "").trim();

  return {
    async loginAs(personaState: number): Promise<void> {
      try {
        await SteamService.SwapToSteamAccount(rid, personaState, []);
        pushToast({ type: "success", message: tr("Toast_AccountSwitched"), duration: 4000 });
      } catch (e) {
        pushToast({ type: "error", message: formatToastWithError(tr("Toast_SwitchFailed"), e), duration: 8000 });
      }
    },

    async createShortcut(personaState: number, label: string): Promise<void> {
      try {
        const path = await Shortcuts.CreateAccountShortcut("Steam", rid, accountDisplay, String(personaState), label, loginName);
        pushToast({ type: "success", message: `${tr("Toast_ShortcutCreated")}\n${path}`, duration: 6000 });
      } catch (e) {
        pushToast({ type: "error", message: formatToastWithError(tr("Toast_SwitchFailed"), e), duration: 8000 });
      }
    },

    copyCommunityUrl(): void {
      void clipboardWrite(`https://steamcommunity.com/profiles/${rid}`, tr);
    },

    copyCommunityUsername(): void {
      void clipboardWrite(accountDisplay, tr);
    },

    copyLoginUsername(): void {
      void clipboardWrite(loginName || rid, tr);
    },

    async copySteamId(format: "ID64" | "ID3" | "ID32"): Promise<void> {
      try {
        const formats = await SteamService.GetSteamIDFormats(rid);
        void clipboardWrite(formats[format] ?? (format === "ID64" ? rid : ""), tr);
      } catch {
        if (format === "ID64") void clipboardWrite(rid, tr);
      }
    },

    async openGameDataFolder(appId: string): Promise<void> {
      try {
        await SteamService.OpenSteamGameDataFolder(rid, appId);
      } catch (e) {
        pushToast({ type: "error", message: mapSteamUserdataI18nError(e, tr), duration: 8000 });
      }
    },

    async copyGameSettingsFrom(appId: string): Promise<void> {
      try {
        await SteamService.CopySteamGameSettingsFrom(rid, appId);
        pushToast({ type: "success", message: tr("Toast_SettingsCopied"), duration: 5000 });
        refreshGameData();
      } catch (e) {
        pushToast({ type: "error", message: mapSteamUserdataI18nError(e, tr), duration: 8000 });
      }
    },

    async restoreGameSettingsTo(appId: string): Promise<void> {
      try {
        await SteamService.RestoreSteamGameSettingsTo(rid, appId);
        pushToast({ type: "success", message: tr("Toast_GameDataRestored"), duration: 5000 });
        refreshGameData();
      } catch (e) {
        pushToast({ type: "error", message: mapSteamUserdataI18nError(e, tr), duration: 8000 });
      }
    },

    async backupGameData(appId: string): Promise<void> {
      try {
        const folder = await SteamService.BackupSteamGameData(rid, appId);
        pushToast({ type: "success", message: tr("Toast_GameBackupDone", { folderLocation: folder }), duration: 8000 });
        refreshGameData();
      } catch (e) {
        pushToast({ type: "error", message: mapSteamUserdataI18nError(e, tr), duration: 8000 });
      }
    },

    async loginAndLaunchGame(appId: string, gameName: string): Promise<void> {
      try {
        await SteamService.LoginAndLaunchGame(rid, -1, appId);
        pushToast({ type: "success", message: tr("Toast_StartedGame", { program: gameName }), duration: 4000 });
      } catch (e) {
        await reportLaunchFailure(e, deps.name);
      }
    },

    async openUserdataFolder(): Promise<void> {
      try {
        await SteamService.OpenUserdataFolder(rid);
      } catch (e) {
        pushToast({ type: "error", message: formatToastWithError(tr("Toast_LaunchFailed"), e), duration: 8000 });
      }
    },
  };
}
