// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2021 TechNobo (Wesley Pyburn)
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
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.Steam;

namespace TcNo_Acc_Switcher_Server.Pages
{
    public partial class Index
    {
        public async void Check(string platform)
        {
            Globals.DebugWriteLine($@"[Func:Index.Check] platform={platform}");

            switch (platform)
            {
                case "BattleNet":
                    if (!GeneralFuncs.CanKillProcess("Battle.net")) return;

                    if (Directory.Exists(_battleNet.FolderPath) && File.Exists(_battleNet.Exe())) _navManager.NavigateTo("/BattleNet/");
                    else await GeneralInvocableFuncs.ShowModal("find:BattleNet:Battle.net.exe:BattleNetSettings");
                    break;
                case "Epic":
                    if (!GeneralFuncs.CanKillProcess("EpicGamesLauncher.exe")) return;

                    if (Directory.Exists(_epic.FolderPath) && File.Exists(_epic.Exe())) _navManager.NavigateTo("/Epic/");
                    else await GeneralInvocableFuncs.ShowModal("find:Epic:EpicGamesLauncher.exe:EpicSettings");
                    break;
                case "Origin":
                    if (!GeneralFuncs.CanKillProcess("Origin")) return;

                    if (Directory.Exists(_origin.FolderPath) && File.Exists(_origin.Exe())) _navManager.NavigateTo("/Origin/");
                    else await GeneralInvocableFuncs.ShowModal("find:Origin:Origin.exe:OriginSettings");
                    break;
                case "Riot":
                    if (!Riot.RiotSwitcherFuncs.CanCloseRiot()) return;

                    if (Directory.Exists(Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Riot Games\\Riot Client\\Data"))) _navManager.NavigateTo("/Riot/");
                    else await GeneralInvocableFuncs.ShowModal("find:Riot:RiotClientPrivateSettings.yaml:RiotSettings");
                    break;
                case "Steam":
                    if (!GeneralFuncs.CanKillProcess("steam")) return;

                    if (!Directory.Exists(_steam.FolderPath) || !File.Exists(_steam.Exe()))
                    {
                        await GeneralInvocableFuncs.ShowModal("find:Steam:Steam.exe:SteamSettings");
                        return;
                    }

                    if (SteamSwitcherFuncs.SteamSettingsValid()) _navManager.NavigateTo("/Steam/");
                    else await GeneralInvocableFuncs.ShowModal("Cannot locate '.../Steam/config/loginusers.vdf'. Try signing into an account first.");
                    break;
                case "Ubisoft":
                    if (!GeneralFuncs.CanKillProcess("upc")) return;

                    if (Directory.Exists(_ubisoft.FolderPath) && File.Exists(_ubisoft.Exe())) _navManager.NavigateTo("/Ubisoft/");
                    else await GeneralInvocableFuncs.ShowModal("find:Ubisoft:upc.exe:UbisoftSettings");
                    break;
            }
        }
        
        /// <summary>
        /// Verify updater files and start update
        /// </summary>
        public void UpdateNow()
        {
            try
            {
                // Download latest hash list
                const string hashFilePath = "hashes.json";
                var client = new WebClient();
                client.DownloadFile(new Uri("https://tcno.co/Projects/AccSwitcher/latest/hashes.json"), hashFilePath);

                // Verify updater files
                var verifyDictionary = JsonConvert.DeserializeObject<Dictionary<string, string>>(File.ReadAllText(hashFilePath));
                if (verifyDictionary == null)
                {
                    _ = GeneralInvocableFuncs.ShowToast("error",
                        "Can verify updater files. Download latest version and replace files in your directory.");
                    return;
                }

                var updaterDict = verifyDictionary.Where(pair => pair.Key.StartsWith("updater")).ToDictionary(pair => pair.Key, pair => pair.Value);

                // Download and replace broken files
                if (Directory.Exists("newUpdater")) GeneralFuncs.RecursiveDelete(new DirectoryInfo("newUpdater"), false);
                foreach (var (key, value) in updaterDict)
                {
                    if (File.Exists(key))
                        if (value == GeneralFuncs.GetFileMd5(key))
                            continue;
                    var uri = new Uri("https://tcno.co/Projects/AccSwitcher/latest/" + key.Replace('\\', '/'));
                    client.DownloadFile(uri, key);
                }

                // Run updater
                Process.Start(new ProcessStartInfo(@"updater\\TcNo-Acc-Switcher-Updater.exe") { UseShellExecute = true });
                Process.GetCurrentProcess().Kill();
            }
            catch (Exception e)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", "Failed to check for updates.");
                Globals.DebugWriteLine("Failed to check for updates:" + e);
            }
        }
    }
}
