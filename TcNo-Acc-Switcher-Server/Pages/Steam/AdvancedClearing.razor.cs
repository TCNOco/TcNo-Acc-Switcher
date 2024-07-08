// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2024 TroubleChute (Wesley Pyburn)
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
            AppData.Instance.WindowTitle = Lang["Title_Steam_Cleaning"];
        }


        public static readonly string SteamReturn = "steamAdvancedClearingAddLine";

        private static void WriteLine(string text)
        {
            Globals.DebugWriteLine($@"[Auto:Steam\AdvancedClearing.razor.cs.WriteLine] Line: {text}");
            _ = AppData.InvokeVoidAsync(SteamReturn, text);
        }

        private static void NewLine()
        {
            _ = AppData.InvokeVoidAsync(SteamReturn, "<br />");
        }

        // BUTTON: Kill Steam process
        public void Steam_Close()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Close]");
            WriteLine(GeneralFuncs.CloseProcesses(SteamSettings.Processes, SteamSettings.ClosingMethod) ? "Closed Steam." : "ERROR: COULD NOT CLOSE STEAM!");
            NewLine();
        }

        // BUTTON: ..\Steam\Logs
        public void Steam_Clear_Logs()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_Logs]");
            GeneralFuncs.ClearFolder(Path.Join(SteamSettings.FolderPath, "logs\\"), SteamReturn);
            WriteLine("Cleared logs folder.");
            NewLine();
        }

        // BUTTON:..\Steam\*.log
        public void Steam_Clear_Dumps()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_Dumps]");
            GeneralFuncs.ClearFolder(Path.Join(SteamSettings.FolderPath, "dumps\\"), SteamReturn);
            WriteLine("Cleared dumps folder.");
            NewLine();
        }

        // BUTTON: %Local%\Steam\htmlcache
        public void Steam_Clear_HtmlCache()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_HtmlCache]");
            // HTML Cache - %USERPROFILE%\AppData\Local\Steam\htmlcache
            GeneralFuncs.ClearFolder(Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Steam\\htmlcache"), SteamReturn);
            WriteLine("Cleared HTMLCache.");
            NewLine();
        }

        // BUTTON: ..\Steam\*.log
        public void Steam_Clear_UiLogs()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_UiLogs]");
            // Overlay UI logs -
            //   Steam\GameOverlayUI.exe.log
            //   Steam\GameOverlayRenderer.log
            GeneralFuncs.ClearFilesOfType(SteamSettings.FolderPath, "*.log|*.last", SearchOption.TopDirectoryOnly, SteamReturn);
            WriteLine("Cleared UI Logs.");
            NewLine();
        }

        // BUTTON: ..\Steam\appcache
        public void Steam_Clear_AppCache()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_AppCache]");
            // App Cache - Steam\appcache
            GeneralFuncs.ClearFilesOfType(Path.Join(SteamSettings.FolderPath, "appcache"), "*.*", SearchOption.TopDirectoryOnly, SteamReturn);
            WriteLine("Cleared AppCache.");
            NewLine();
        }

        // BUTTON: ..\Steam\appcache\httpcache
        public void Steam_Clear_HttpCache()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_HttpCache]");
            GeneralFuncs.ClearFilesOfType(Path.Join(SteamSettings.FolderPath, "appcache\\httpcache"), "*.*", SearchOption.AllDirectories, SteamReturn);
            WriteLine("Cleared HTTPCache.");
            NewLine();
        }

        // BUTTON: ..\Steam\depotcache
        public void Steam_Clear_DepotCache()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_DepotCache]");
            GeneralFuncs.ClearFilesOfType(Path.Join(SteamSettings.FolderPath, "depotcache"), "*.*", SearchOption.TopDirectoryOnly, SteamReturn);
            WriteLine("Cleared DepotCache.");
            NewLine();
        }

        // BUTTON: ..\Steam\config\config.vdf
        public void Steam_Clear_Config()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_Config]");
            GeneralFuncs.DeleteFile(Path.Join(SteamSettings.FolderPath, "config\\config.vdf"), SteamReturn);
            WriteLine("Cleared config\\config.vdf");
            NewLine();
        }

        // BUTTON: ..\Steam\config\loginusers.vdf
        public void Steam_Clear_LoginUsers()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_LoginUsers]");
            GeneralFuncs.DeleteFile(Path.Join(SteamSettings.FolderPath, "config\\loginusers.vdf"), SteamReturn);
            WriteLine("Cleared config\\loginusers.vdf");
            NewLine();
        }

        // BUTTON: ..\Steam\ssfn*
        public void Steam_Clear_Ssfn()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_Ssfn]");
            var d = new DirectoryInfo(SteamSettings.FolderPath);
            var i = 0;
            foreach (var f in d.GetFiles("ssfn*"))
            {
                GeneralFuncs.DeleteFile(f, SteamReturn);
                i++;
            }

            WriteLine(i == 0 ? "No SSFN files found." : "Cleared SSFN files.");
            NewLine();
        }

        // BUTTON: HKCU\..\AutoLoginUser
        [SupportedOSPlatform("windows")]
        public void Steam_Clear_AutoLoginUser()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_AutoLoginUser]");
            GeneralFuncs.DeleteRegKey(@"Software\Valve\Steam", "AutoLoginuser", SteamReturn);
        }
        // BUTTON: HKCU\..\LastGameNameUsed
        [SupportedOSPlatform("windows")]
        public void Steam_Clear_LastGameNameUsed()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_LastGameNameUsed]");
            GeneralFuncs.DeleteRegKey(@"Software\Valve\Steam", "LastGameNameUsed", SteamReturn);
        }

        // BUTTON: HKCU\..\PseudoUUID
        [SupportedOSPlatform("windows")]
        public void Steam_Clear_PseudoUUID()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_PseudoUUID]");
            GeneralFuncs.DeleteRegKey(@"Software\Valve\Steam", "PseudoUUID", SteamReturn);
        }
        // BUTTON: HKCU\..\RememberPassword
        [SupportedOSPlatform("windows")]
        public void Steam_Clear_RememberPassword()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\AdvancedClearing.razor.cs.Steam_Clear_RememberPassword]");
            GeneralFuncs.DeleteRegKey(@"Software\Valve\Steam", "RememberPassword", SteamReturn);
        }
    }
}
