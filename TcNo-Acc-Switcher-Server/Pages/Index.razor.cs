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
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using Task = System.Threading.Tasks.Task;

namespace TcNo_Acc_Switcher_Server.Pages
{
    public partial class Index : ComponentBase
    {
        public async void CheckSteam()
        {
            Globals.DebugWriteLine($@"[Func:Index.CheckSteam]");

            if (!GeneralFuncs.CanKillProcess("steam")) return;

            Steam.LoadFromFile();
            if (!Directory.Exists(Steam.FolderPath) || !File.Exists(Steam.Exe()))
            {
                await GeneralInvocableFuncs.ShowModal("find:Steam:Steam.exe:SteamSettings");
                return;
            }
            
            if (SteamSwitcherFuncs.SteamSettingsValid())
                NavManager.NavigateTo("/Steam/");
            else
                await GeneralInvocableFuncs.ShowModal("Cannot locate '.../Steam/config/loginusers.vdf'. Try signing into an account first.");
        }
        public async void CheckOrigin()
        {
            Globals.DebugWriteLine($@"[Func:Index.CheckOrigin]");

            if (!GeneralFuncs.CanKillProcess("Origin")) return;

            Origin.LoadFromFile();
            if (Directory.Exists(Origin.FolderPath) && File.Exists(Origin.Exe()))
                NavManager.NavigateTo("/Origin/");
            else
                await GeneralInvocableFuncs.ShowModal("find:Origin:Origin.exe:OriginSettings");
        }
        public async void CheckUbisoft()
        {
            Globals.DebugWriteLine($@"[Func:Index.CheckUbisoft]");

            if (!GeneralFuncs.CanKillProcess("upc")) return;

            Ubisoft.LoadFromFile();
            if (Directory.Exists(Ubisoft.FolderPath) && File.Exists(Ubisoft.Exe()))
                NavManager.NavigateTo("/Ubisoft/");
            else
                await GeneralInvocableFuncs.ShowModal("find:Ubisoft:upc.exe:UbisoftSettings");
        }

        public async void CheckBattleNet()
        {
            Globals.DebugWriteLine($@"[Func:Index.CheckBattleNet]");

            if (!GeneralFuncs.CanKillProcess("Battle.net")) return;

            BattleNet.LoadFromFile();
            if (Directory.Exists(BattleNet.FolderPath) && File.Exists(BattleNet.Exe()))
                NavManager.NavigateTo("/BattleNet/");
            else
                await GeneralInvocableFuncs.ShowModal("find:BattleNet:Battle.net.exe:BattleNetSettings");
        }

        public async void CheckEpic()
        {
            Globals.DebugWriteLine($@"[Func:Index.CheckEpic]");

            if (!GeneralFuncs.CanKillProcess("EpicGamesLauncher.exe")) return;

            Epic.LoadFromFile();
            if (Directory.Exists(Epic.FolderPath) && File.Exists(Epic.Exe()))
                NavManager.NavigateTo("/Epic/");
            else
                await GeneralInvocableFuncs.ShowModal("find:Epic:EpicGamesLauncher.exe:EpicSettings");
        }

        public void UpdateNow()
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
    }
}
