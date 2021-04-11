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
using System.IO;
using System.Linq;
using System.Security.Cryptography;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.Steam
{
    public partial class AdvancedClearing : ComponentBase
    {
        [Inject]
        protected Data.AppData AppData { get; set; }
        private static readonly Data.Settings.Steam Steam = Data.Settings.Steam.Instance;

        protected override async Task OnInitializedAsync()
        {
            AppData.WindowTitle = "TcNo Account Switcher - Steam Cleaning";
        }


        public const string SteamReturn = "SteamAdvancedClearingAddLine";

        private async void WriteLine(string text)
        {
            await AppData.ActiveIJsRuntime.InvokeVoidAsync(SteamReturn, text);
        }

        private async void NewLine()
        {
            await AppData.ActiveIJsRuntime.InvokeVoidAsync(SteamReturn, "<br />");
        }

        // BUTTON: Kill Steam process
        public void Steam_Close()
        {
            SteamSwitcherFuncs.CloseSteam();
            WriteLine("Closing Steam.");
            NewLine();
        }

        // BUTTON: ..\Steam\Logs
        public void Steam_Clear_Logs()
        {
            GeneralFuncs.ClearFolder(Path.Combine(Steam.FolderPath, "logs\\"), SteamReturn);
            WriteLine("Cleared logs folder.");
            NewLine();
        }

        // BUTTON:..\Steam\*.log
        public void Steam_Clear_Dumps()
        {
            GeneralFuncs.ClearFolder(Path.Combine(Steam.FolderPath, "dumps\\"), SteamReturn);
            WriteLine("Cleared dumps folder.");
            NewLine();
        }

        // BUTTON: %Local%\Steam\htmlcache
        public void Steam_Clear_HtmlCache()
        {
            // HTML Cache - %USERPROFILE%\AppData\Local\Steam\htmlcache
            GeneralFuncs.ClearFolder(Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Steam\\htmlcache"), SteamReturn);
            WriteLine("Cleared HTMLCache.");
            NewLine();
        }

        // BUTTON: ..\Steam\*.log
        public void Steam_Clear_UiLogs()
        {
            // Overlay UI logs -
            //   Steam\GameOverlayUI.exe.log
            //   Steam\GameOverlayRenderer.log
            GeneralFuncs.ClearFilesOfType(Steam.FolderPath, "*.log|*.last", SearchOption.TopDirectoryOnly, SteamReturn);
            WriteLine("Cleared UI Logs.");
            NewLine();
        }

        // BUTTON: ..\Steam\appcache
        public void Steam_Clear_AppCache()
        {
            // App Cache - Steam\appcache
            GeneralFuncs.ClearFilesOfType(Path.Combine(Steam.FolderPath, "appcache"), "*.*", SearchOption.TopDirectoryOnly, SteamReturn);
            WriteLine("Cleared AppCache.");
            NewLine();
        }

        // BUTTON: ..\Steam\appcache\httpcache
        public void Steam_Clear_HttpCache()
        {
            GeneralFuncs.ClearFilesOfType(Path.Combine(Steam.FolderPath, "appcache\\httpcache"), "*.*", SearchOption.AllDirectories, SteamReturn);
            WriteLine("Cleared HTTPCache.");
            NewLine();
        }

        // BUTTON: ..\Steam\depotcache
        public void Steam_Clear_DepotCache()
        {
            GeneralFuncs.ClearFilesOfType(Path.Combine(Steam.FolderPath, "depotcache"), "*.*", SearchOption.TopDirectoryOnly, SteamReturn);
            WriteLine("Cleared DepotCache.");
            NewLine();
        }

        // BUTTON: Forgotten account backups
        public void Steam_Clear_Forgotten()
        {
            SteamSwitcherFuncs.ClearForgotten();
            WriteLine("Cleared forgotten account backups");
            NewLine();
        }

        // BUTTON: ..\Steam\config\config.vdf
        public void Steam_Clear_Config()
        {
            GeneralFuncs.DeleteFile(Path.Combine(Steam.FolderPath, "config\\config.vdf"), jsDest: SteamReturn);
            WriteLine("[ Don't forget to clear forgotten account backups as well ]");
            WriteLine("Cleared config\\config.vdf");
            NewLine();
        }

        // BUTTON: ..\Steam\config\loginusers.vdf
        public void Steam_Clear_LoginUsers()
        {
            GeneralFuncs.DeleteFile(Path.Combine(Steam.FolderPath, "config\\loginusers.vdf"), jsDest: SteamReturn);
            WriteLine("Cleared config\\loginusers.vdf");
            NewLine();
        }

        // BUTTON: ..\Steam\ssfn*
        public void Steam_Clear_Ssfn()
        {
            var d = new DirectoryInfo(Steam.FolderPath);
            var i = 0;
            foreach (var f in d.GetFiles("ssfn*"))
            {
                GeneralFuncs.DeleteFile(fileInfo: f, jsDest: SteamReturn);
                i++;
            }

            WriteLine(i == 0 ? "No SSFN files found." : "Cleared SSFN files.");
            NewLine();
        }

        // BUTTON: HKCU\..\AutoLoginUser
        public void Steam_Clear_AutoLoginUser() => GeneralFuncs.DeleteRegKey(@"Software\Valve\Steam", "AutoLoginuser", SteamReturn);
        // BUTTON: HKCU\..\LastGameNameUsed
        public void Steam_Clear_LastGameNameUsed() => GeneralFuncs.DeleteRegKey(@"Software\Valve\Steam", "LastGameNameUsed", SteamReturn);
        // BUTTON: HKCU\..\PseudoUUID
        public void Steam_Clear_PseudoUUID() => GeneralFuncs.DeleteRegKey(@"Software\Valve\Steam", "PseudoUUID", SteamReturn);
        // BUTTON: HKCU\..\RememberPassword
        public void Steam_Clear_RememberPassword() => GeneralFuncs.DeleteRegKey(@"Software\Valve\Steam", "RememberPassword", SteamReturn);
    }
}
