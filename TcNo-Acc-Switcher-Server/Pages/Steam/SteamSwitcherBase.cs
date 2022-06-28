// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2022 TechNobo (Wesley Pyburn)
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

using System.Diagnostics;
using System.IO;
using System.Runtime.Versioning;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Converters;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.Steam
{
    public class SteamSwitcherBase
    {
        private static readonly Lang Lang = Lang.Instance;

        /// <summary>
        /// Converts input SteamID64 into the requested format, then copies it to clipboard.
        /// </summary>
        /// <param name="request">SteamId, SteamId3, SteamId32, SteamId64</param>
        /// <param name="anySteamId">Any format of SteamId to convert</param>
        [JSInvokable]
        public static void CopySteamIdType(string request, string anySteamId)
        {
            Globals.DebugWriteLine($@"[JSInvoke:Steam\SteamSwitcherBase.CopySteamIdType] {anySteamId.Substring(anySteamId.Length - 4, 4)} to: {request}");
            switch (request)
            {
                case "SteamId":
                    GenericFunctions.CopyToClipboard(new SteamIdConvert(anySteamId).Id);
                    break;
                case "SteamId3":
                    GenericFunctions.CopyToClipboard(new SteamIdConvert(anySteamId).Id3);
                    break;
                case "SteamId32":
                    GenericFunctions.CopyToClipboard(new SteamIdConvert(anySteamId).Id32);
                    break;
                case "SteamId64":
                    GenericFunctions.CopyToClipboard(new SteamIdConvert(anySteamId).Id64);
                    break;
            }
        }

        /// <summary>
        /// [Wrapper with fewer arguments]
        /// </summary>
        [JSInvokable]
        [SupportedOSPlatform("windows")]
        public static void SwapToSteamWithReq(string steamId, int request) => SwapToSteam(steamId, request);
        [JSInvokable]
        [SupportedOSPlatform("windows")]
        public static void SwapToSteam(string steamId) => SwapToSteam(steamId, -1);
        /// <summary>
        /// JS function handler for swapping to another Steam account.
        /// </summary>
        /// <param name="steamId">Requested account's SteamID</param>
        /// <param name="ePersonaState">(Optional) Persona State [0: Offline, 1: Online...]</param>
        [SupportedOSPlatform("windows")]
        public static void SwapToSteam(string steamId, int ePersonaState)
        {
            Globals.DebugWriteLine($@"[JSInvoke:Steam\SteamSwitcherBase.SwapToSteam] {(steamId.Length > 0 ? steamId.Substring(steamId.Length - 4, 4) : "")}, ePersonaState: {ePersonaState}");
            // If just double-clicked
            if (ePersonaState == -1) ePersonaState = Data.Settings.Steam.OverrideState;
            SteamSwitcherFuncs.SwapSteamAccounts(steamId, ePersonaState: ePersonaState);
        }

        [JSInvokable]
        public static void SteamOpenUserdata(string steamId)
        {
            var steamId32 = new SteamIdConvert(steamId);
            var folder = Path.Join(Data.Settings.Steam.FolderPath, $"userdata\\{steamId32.Id32}");
            if (Directory.Exists(folder)) _ = Process.Start("explorer.exe", folder);
            else _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_NoFindSteamUserdata"], Lang["Failed"], "toastarea");
        }
        [JSInvokable]
        public static void CopySettingsFrom(string sourceSteamId, string game)
        {
            var destSteamId = SteamSwitcherFuncs.GetCurrentAccountId(true);
            if (!SteamSwitcherFuncs.VerifySteamId(sourceSteamId) || !SteamSwitcherFuncs.VerifySteamId(destSteamId))
            {
                GeneralInvocableFuncs.ShowToast("error", Lang["Toast_NoValidSteamId"], Lang["Failed"],
                    "toastarea"); 
                return;
            }
            if (destSteamId == sourceSteamId) return;
            
            var sourceSteamId32 = new SteamIdConvert(sourceSteamId).Id32;
            var destSteamId32 = new SteamIdConvert(destSteamId).Id32;
            var sourceFolder = Path.Join(Data.Settings.Steam.FolderPath, $"userdata\\{sourceSteamId32}\\{Data.Settings.Steam.AppIds[game]}");
            var destFolder = Path.Join(Data.Settings.Steam.FolderPath, $"userdata\\{destSteamId32}\\{Data.Settings.Steam.AppIds[game]}");
            if (!Directory.Exists(sourceFolder))
            {
                GeneralInvocableFuncs.ShowToast("error", Lang["Toast_NoFindSteamUserdata"], Lang["Failed"],
                    "toastarea");
                return;
            }

            if (Directory.Exists(destFolder))
            {
                SteamSwitcherFuncs.BackupGameDataFolder(destFolder);
                Directory.Delete(destFolder, true);
            }
            Globals.CopyDirectory(sourceFolder, destFolder, true);
            GeneralInvocableFuncs.ShowToast("success", Lang["Toast_SettingsCopied"], Lang["Success"], "toastarea");
        }
        [JSInvokable]
        public static void RestoreSettingsTo(string steamId, string game)
        {
            if (!SteamSwitcherFuncs.VerifySteamId(steamId)) return;
            var steamId32 = new SteamIdConvert(steamId).Id32;
            var backupFolder = Path.Join(Data.Settings.Steam.FolderPath, $"userdata\\{steamId32}\\{Data.Settings.Steam.AppIds[game]}_switcher_backup");
            var folder = backupFolder.Replace("_switcher_backup", "");
            if (!Directory.Exists(backupFolder))
            {
                GeneralInvocableFuncs.ShowToast("error", Lang["Toast_NoFindGameBackup"], Lang["Failed"],
                    "toastarea");
                return;
            }
            if (Directory.Exists(folder)) Directory.Delete(folder, true);
            Globals.CopyDirectory(backupFolder, folder, true);
            Directory.Delete(backupFolder, true);
            GeneralInvocableFuncs.ShowToast("success", Lang["Toast_GameDataRestored"], Lang["Success"], "toastarea");
        }
        [JSInvokable]
        public static void BackupGameData(string steamId, string game)
        {
            if (!SteamSwitcherFuncs.VerifySteamId(steamId)) return;
            var steamId32 = new SteamIdConvert(steamId).Id32;
            var sourceFolder = Path.Join(Data.Settings.Steam.FolderPath, $"userdata\\{steamId32}\\{Data.Settings.Steam.AppIds[game]}");
            var destFolder = sourceFolder + "_switcher_backup";
            if (Directory.Exists(destFolder)) Directory.Delete(destFolder, true);
            SteamSwitcherFuncs.BackupGameDataFolder(sourceFolder);
            GeneralInvocableFuncs.ShowToast("success", Lang["Toast_GameBackupDone"], Lang["Success"], "toastarea");
        }

        /// <summary>
        /// JS function handler for swapping to a new Steam account (No inputs)
        /// </summary>
        [JSInvokable]
        public static void NewLogin_Steam()
        {
            Globals.DebugWriteLine(@"[JSInvoke:Steam\SteamSwitcherBase.NewLogin_Steam]");
            SteamSwitcherFuncs.SwapSteamAccounts();
        }
    }
}
