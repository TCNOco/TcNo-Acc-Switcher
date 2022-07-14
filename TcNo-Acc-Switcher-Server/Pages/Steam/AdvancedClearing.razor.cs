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
using System.IO;
using System.Runtime.Versioning;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using SteamSettings = TcNo_Acc_Switcher_Server.Data.Settings.Steam;

namespace TcNo_Acc_Switcher_Server.Pages.Steam
{
    public partial class AdvancedClearing
    {
        private static readonly Lang Lang = Lang.Instance;
        [Inject]
        protected AppData AppData { get; set; }

        protected override void OnInitialized()
        {
            Globals.DebugWriteLine(@"[Auto:Steam\AdvancedClearing.razor.cs.OnInitialisedAsync]");
            AppData.WindowTitle = Lang["Title_Steam_Cleaning"];
        }


        public static readonly string SteamReturn = "steamAdvancedClearingAddLine";

        private static async Task WriteLine(string text)
        {
            Globals.DebugWriteLine($@"[Auto:Steam\AdvancedClearing.razor.cs.WriteLine] Line: {text}");
            await AppData.InvokeVoidAsync(SteamReturn, text);
        }

        private static async Task NewLine()
        {
            await AppData.InvokeVoidAsync(SteamReturn, "<br />");
        }

        // BUTTON: Kill Steam process
        public async Task Steam_Close()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Close]");
            await WriteLine(await GeneralFuncs.CloseProcesses(SteamSettings.Processes, SteamSettings.ClosingMethod) ? "Closed Steam." : "ERROR: COULD NOT CLOSE STEAM!");
            await NewLine();
        }

        // BUTTON: ..\Steam\Logs
        public async Task Steam_Clear_Logs()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_Logs]");
            await GeneralFuncs.ClearFolder(Path.Join(SteamSettings.FolderPath, "logs\\"), SteamReturn);
            await WriteLine("Cleared logs folder.");
            await NewLine();
        }

        // BUTTON:..\Steam\*.log
        public async Task Steam_Clear_Dumps()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_Dumps]");
            await GeneralFuncs.ClearFolder(Path.Join(SteamSettings.FolderPath, "dumps\\"), SteamReturn);
            await WriteLine("Cleared dumps folder.");
            await NewLine();
        }

        // BUTTON: %Local%\Steam\htmlcache
        public async Task Steam_Clear_HtmlCache()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_HtmlCache]");
            // HTML Cache - %USERPROFILE%\AppData\Local\Steam\htmlcache
            await GeneralFuncs.ClearFolder(Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Steam\\htmlcache"), SteamReturn);
            await WriteLine("Cleared HTMLCache.");
            await NewLine();
        }

        // BUTTON: ..\Steam\*.log
        public async Task Steam_Clear_UiLogs()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_UiLogs]");
            // Overlay UI logs -
            //   Steam\GameOverlayUI.exe.log
            //   Steam\GameOverlayRenderer.log
            await GeneralFuncs.ClearFilesOfType(SteamSettings.FolderPath, "*.log|*.last", SearchOption.TopDirectoryOnly, SteamReturn);
            await WriteLine("Cleared UI Logs.");
            await NewLine();
        }

        // BUTTON: ..\Steam\appcache
        public async Task Steam_Clear_AppCache()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_AppCache]");
            // App Cache - Steam\appcache
            await GeneralFuncs.ClearFilesOfType(Path.Join(SteamSettings.FolderPath, "appcache"), "*.*", SearchOption.TopDirectoryOnly, SteamReturn);
            await WriteLine("Cleared AppCache.");
            await NewLine();
        }

        // BUTTON: ..\Steam\appcache\httpcache
        public async Task Steam_Clear_HttpCache()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_HttpCache]");
            await GeneralFuncs.ClearFilesOfType(Path.Join(SteamSettings.FolderPath, "appcache\\httpcache"), "*.*", SearchOption.AllDirectories, SteamReturn);
            await WriteLine("Cleared HTTPCache.");
            await NewLine();
        }

        // BUTTON: ..\Steam\depotcache
        public async Task Steam_Clear_DepotCache()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_DepotCache]");
            await GeneralFuncs.ClearFilesOfType(Path.Join(SteamSettings.FolderPath, "depotcache"), "*.*", SearchOption.TopDirectoryOnly, SteamReturn);
            await WriteLine("Cleared DepotCache.");
            await NewLine();
        }

        // BUTTON: ..\Steam\config\config.vdf
        public async Task Steam_Clear_Config()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_Config]");
            await GeneralFuncs.DeleteFile(Path.Join(SteamSettings.FolderPath, "config\\config.vdf"), SteamReturn);
            await WriteLine("Cleared config\\config.vdf");
            await NewLine();
        }

        // BUTTON: ..\Steam\config\loginusers.vdf
        public async Task Steam_Clear_LoginUsers()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_LoginUsers]");
            await GeneralFuncs.DeleteFile(Path.Join(SteamSettings.FolderPath, "config\\loginusers.vdf"), SteamReturn);
            await WriteLine("Cleared config\\loginusers.vdf");
            await NewLine();
        }

        // BUTTON: ..\Steam\ssfn*
        public async Task Steam_Clear_Ssfn()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_Ssfn]");
            var d = new DirectoryInfo(SteamSettings.FolderPath);
            var i = 0;
            foreach (var f in d.GetFiles("ssfn*"))
            {
                await GeneralFuncs.DeleteFile(f, SteamReturn);
                i++;
            }

            await WriteLine(i == 0 ? "No SSFN files found." : "Cleared SSFN files.");
            await NewLine();
        }

        // BUTTON: HKCU\..\AutoLoginUser
        [SupportedOSPlatform("windows")]
        public async Task Steam_Clear_AutoLoginUser()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_AutoLoginUser]");
            await GeneralFuncs.DeleteRegKey(@"Software\Valve\Steam", "AutoLoginuser", SteamReturn);
        }
        // BUTTON: HKCU\..\LastGameNameUsed
        [SupportedOSPlatform("windows")]
        public async Task Steam_Clear_LastGameNameUsed()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_LastGameNameUsed]");
            await GeneralFuncs.DeleteRegKey(@"Software\Valve\Steam", "LastGameNameUsed", SteamReturn);
        }

        // BUTTON: HKCU\..\PseudoUUID
        [SupportedOSPlatform("windows")]
        public async Task Steam_Clear_PseudoUUID()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_PseudoUUID]");
            await GeneralFuncs.DeleteRegKey(@"Software\Valve\Steam", "PseudoUUID", SteamReturn);
        }
        // BUTTON: HKCU\..\RememberPassword
        [SupportedOSPlatform("windows")]
        public async Task Steam_Clear_RememberPassword()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_RememberPassword]");
            await GeneralFuncs.DeleteRegKey(@"Software\Valve\Steam", "RememberPassword", SteamReturn);
        }
    }
}
