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

using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Net;
using System.Reflection;
using System.Runtime.Versioning;
using System.Security.Principal;
using System.Threading.Tasks;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.Steam;

using BasicSettings = TcNo_Acc_Switcher_Server.Data.Settings.Basic;
using BattleNetSettings = TcNo_Acc_Switcher_Server.Data.Settings.BattleNet;
using SteamSettings = TcNo_Acc_Switcher_Server.Data.Settings.Steam;

namespace TcNo_Acc_Switcher_Server.Pages
{
    public partial class Index
    {
        private static readonly Lang Lang = Lang.Instance;

        public void Check(string platform)
        {
            Globals.DebugWriteLine($@"[Func:Index.Check] platform={platform}");
            switch (platform)
            {
                case "BattleNet":
                    if (!GeneralFuncs.CanKillProcess(BattleNetSettings.Processes, BattleNetSettings.ClosingMethod)) return;
                    if (Directory.Exists(BattleNetSettings.FolderPath) && File.Exists(BattleNetSettings.Exe())) AppData.ActiveNavMan.NavigateTo("/BattleNet/");
                    else _ = GeneralInvocableFuncs.ShowModal("find:BattleNet:Battle.net.exe:BattleNetSettings");
                    break;

                case "Steam":
                    if (!GeneralFuncs.CanKillProcess(SteamSettings.Processes, SteamSettings.ClosingMethod)) return;
                    if (!Directory.Exists(SteamSettings.FolderPath) || !File.Exists(SteamSettings.Exe()))
                    {
                        _ = GeneralInvocableFuncs.ShowModal("find:Steam:Steam.exe:SteamSettings");
                        return;
                    }
                    if (SteamSwitcherFuncs.SteamSettingsValid()) AppData.ActiveNavMan.NavigateTo("/Steam/");
                    else _ = GeneralInvocableFuncs.ShowModal(Lang["Toast_Steam_CantLocateLoginusers"]);
                    break;

                default:
                    if (BasicPlatforms.PlatformExistsFromShort(platform)) // Is a basic platform!
                    {
                        BasicPlatforms.SetCurrentPlatformFromShort(platform);
                        if (!GeneralFuncs.CanKillProcess(CurrentPlatform.ExesToEnd, BasicSettings.ClosingMethod)) return;

                        if (Directory.Exists(BasicSettings.FolderPath) && File.Exists(BasicSettings.Exe())) AppData.ActiveNavMan.NavigateTo("/Basic/");
                        else _ = GeneralInvocableFuncs.ShowModal($"find:{CurrentPlatform.SafeName}:{CurrentPlatform.ExeName}:{CurrentPlatform.SafeName}");
                    }
                    break;
            }
        }

        private static bool IsAdmin()
        {
            if (!OperatingSystem.IsWindows()) return true;
            // Checks whether program is running as Admin or not
            var securityIdentifier = WindowsIdentity.GetCurrent().Owner;
            return securityIdentifier is not null && securityIdentifier.IsWellKnown(WellKnownSidType.BuiltinAdministratorsSid);
        }

        public static void StartUpdaterAsAdmin(string args = "")
        {
            var exeLocation = Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location) ?? Environment.CurrentDirectory;
            Directory.SetCurrentDirectory(exeLocation);

            var proc = new ProcessStartInfo
            {
                WorkingDirectory = exeLocation,
                FileName = "updater\\TcNo-Acc-Switcher-Updater.exe",
                Arguments = args,
                UseShellExecute = true,
                Verb = "runas"
            };

            try
            {
                _ = Process.Start(proc);
                AppData.ActiveNavMan.NavigateTo("EXIT_APP", true);
            }
            catch (Exception ex)
            {
                Globals.WriteToLog(@"This program must be run as an administrator!" + Environment.NewLine + ex);
                AppData.ActiveNavMan.NavigateTo("EXIT_APP", true);
            }
        }

        public static void AutoStartUpdaterAsAdmin(string args = "")
        {
            // Run updater
            if (Globals.InstalledToProgramFiles() || !Globals.HasFolderAccess(Globals.AppDataFolder))
            {
                StartUpdaterAsAdmin(args);
            }
            else
            {
                _ = Process.Start(new ProcessStartInfo(Path.Join(Globals.AppDataFolder, @"updater\\TcNo-Acc-Switcher-Updater.exe")) { UseShellExecute = true, Arguments = args });
                try
                {
                    AppData.ActiveNavMan.NavigateTo("EXIT_APP", true);
                }
                catch (NullReferenceException)
                {
                    Environment.Exit(0);
                }
            }
        }

        /// <summary>
        /// Verify updater files and start update
        /// </summary>
        public void UpdateNow()
        {
            try
            {
                if (Globals.InstalledToProgramFiles() && !IsAdmin() || !Globals.HasFolderAccess(Globals.AppDataFolder))
                {
                    _ = GeneralInvocableFuncs.ShowModal("notice:RestartAsAdmin");
                    return;
                }

                Directory.SetCurrentDirectory(Globals.AppDataFolder);
                // Download latest hash list
                var hashFilePath = Path.Join(Globals.UserDataFolder, "hashes.json");
                Globals.DownloadFile("https://tcno.co/Projects/AccSwitcher/latest/hashes.json", hashFilePath);

                // Verify updater files
                var verifyDictionary = JsonConvert.DeserializeObject<Dictionary<string, string>>(Globals.ReadAllText(hashFilePath));
                if (verifyDictionary == null)
                {
                    _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_UpdateVerifyFail"]);
                    return;
                }

                var updaterDict = verifyDictionary.Where(pair => pair.Key.StartsWith("updater")).ToDictionary(pair => pair.Key, pair => pair.Value);

                // Download and replace broken files
                Globals.RecursiveDelete("newUpdater", false);
                foreach (var (key, value) in updaterDict)
                {
                    if (key == null) continue;
                    if (File.Exists(key) && value == GeneralFuncs.GetFileMd5(key))
                        continue;
                    Globals.DownloadFile("https://tcno.co/Projects/AccSwitcher/latest/" + key.Replace('\\', '/'),key);
                }

                AutoStartUpdaterAsAdmin();
            }
            catch (Exception e)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_FailedUpdateCheck"]);
                Globals.WriteToLog("Failed to check for updates:" + e);
            }
            Directory.SetCurrentDirectory(Globals.UserDataFolder);
        }
    }
}
