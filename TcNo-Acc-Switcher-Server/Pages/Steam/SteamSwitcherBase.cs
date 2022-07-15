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
using System.Threading.Tasks;
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

        public static void SteamOpenUserdata()
        {
            var steamId32 = new SteamIdConvert(AppData.SelectedAccount);
            var folder = Path.Join(Data.Settings.Steam.FolderPath, $"userdata\\{steamId32.Id32}");
            if (Directory.Exists(folder)) _ = Process.Start("explorer.exe", folder);
            else _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_NoFindSteamUserdata"], Lang["Failed"], "toastarea");
        }

        [JSInvokable]
        public static async Task CopySettingsFrom(string sourceSteamId, string gameId)
        {
            var destSteamId = SteamSwitcherFuncs.GetCurrentAccountId(true);
            if (!SteamSwitcherFuncs.VerifySteamId(sourceSteamId) || !SteamSwitcherFuncs.VerifySteamId(destSteamId))
            {
                await GeneralInvocableFuncs.ShowToast("error", Lang["Toast_NoValidSteamId"], Lang["Failed"],
                    "toastarea");
                return;
            }
            if (destSteamId == sourceSteamId)
            {
                await GeneralInvocableFuncs.ShowToast("info", Lang["Toast_SameAccount"], Lang["Failed"],
                    "toastarea");
                return;
            }

            var sourceSteamId32 = new SteamIdConvert(sourceSteamId).Id32;
            var destSteamId32 = new SteamIdConvert(destSteamId).Id32;
            var sourceFolder = Path.Join(Data.Settings.Steam.FolderPath, $"userdata\\{sourceSteamId32}\\{gameId}");
            var destFolder = Path.Join(Data.Settings.Steam.FolderPath, $"userdata\\{destSteamId32}\\{gameId}");
            if (!Directory.Exists(sourceFolder))
            {
                await GeneralInvocableFuncs.ShowToast("error", Lang["Toast_NoFindSteamUserdata"], Lang["Failed"],
                    "toastarea");
                return;
            }

            if (Directory.Exists(destFolder))
            {
                // Backup the account's data you're copying to
                var toAccountBackup = Path.Join(Globals.UserDataFolder, $"Backups\\Steam\\{destSteamId32}\\{gameId}");
                if (Directory.Exists(toAccountBackup)) Directory.Delete(toAccountBackup, true);
                Globals.CopyDirectory(destFolder, toAccountBackup, true);
                Directory.Delete(destFolder, true);
            }
            Globals.CopyDirectory(sourceFolder, destFolder, true);
            await GeneralInvocableFuncs.ShowToast("success", Lang["Toast_SettingsCopied"], Lang["Success"], "toastarea");
        }

        [JSInvokable]
        public static async Task RestoreSettingsTo(string steamId, string gameId)
        {
            if (!SteamSwitcherFuncs.VerifySteamId(steamId)) return;
            var steamId32 = new SteamIdConvert(steamId).Id32;
            var backupFolder = Path.Join(Globals.UserDataFolder, $"Backups\\Steam\\{steamId32}\\{gameId}");

            var folder = Path.Join(Data.Settings.Steam.FolderPath, $"userdata\\{steamId32}\\{gameId}");
            if (!Directory.Exists(backupFolder))
            {
                await GeneralInvocableFuncs.ShowToast("error", Lang["Toast_NoFindGameBackup"], Lang["Failed"],
                    "toastarea");
                return;
            }
            if (Directory.Exists(folder)) Directory.Delete(folder, true);
            Globals.CopyDirectory(backupFolder, folder, true);
            await GeneralInvocableFuncs.ShowToast("success", Lang["Toast_GameDataRestored"], Lang["Success"], "toastarea");
        }

        [JSInvokable]
        public static async Task BackupGameData(string steamId, string gameId)
        {
            var steamId32 = new SteamIdConvert(steamId).Id32;
            var sourceFolder = Path.Join(Data.Settings.Steam.FolderPath, $"userdata\\{steamId32}\\{gameId}");
            if (!SteamSwitcherFuncs.VerifySteamId(steamId) || !Directory.Exists(sourceFolder)) return;
            var destFolder = Path.Join(Globals.UserDataFolder, $"Backups\\Steam\\{steamId32}\\{gameId}");
            if (Directory.Exists(destFolder)) Directory.Delete(destFolder, true);

            Globals.CopyDirectory(sourceFolder, destFolder, true);
            await GeneralInvocableFuncs.ShowToast("success", Lang["Toast_GameBackupDone", new {folderLocation = destFolder }], Lang["Success"], "toastarea");
        }
    }
}
