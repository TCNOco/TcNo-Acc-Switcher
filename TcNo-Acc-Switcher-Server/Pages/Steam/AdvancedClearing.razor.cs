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

        private async void WriteLine(IJSRuntime jsRuntime, string text)
        {
            await jsRuntime.InvokeVoidAsync(SteamReturn, text);
        }

        private async void NewLine(IJSRuntime jsRuntime)
        {
            await jsRuntime.InvokeVoidAsync(SteamReturn, "<br />");
        }

        public void Steam_Close(IJSRuntime jsRuntime)
        {
            SteamSwitcherFuncs.CloseSteam();
            WriteLine(jsRuntime, "Closing Steam.");
            NewLine(jsRuntime);
        }


        public void Steam_Clear_Logs(IJSRuntime jsRuntime)
        {
            GeneralFuncs.ClearFolder(Path.Combine(Steam.FolderPath, "logs\\"), jsRuntime, SteamReturn);
            WriteLine(jsRuntime, "Cleared logs folder.");
            NewLine(jsRuntime);
        }

        public void Steam_Clear_Dumps(IJSRuntime jsRuntime)
        {
            GeneralFuncs.ClearFolder(Path.Combine(Steam.FolderPath, "dumps\\"), jsRuntime, SteamReturn);
            WriteLine(jsRuntime, "Cleared dumps folder.");
            NewLine(jsRuntime);
        }

        public void Steam_Clear_HtmlCache(IJSRuntime jsRuntime)
        {
            // HTML Cache - %USERPROFILE%\AppData\Local\Steam\htmlcache
            GeneralFuncs.ClearFolder(Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Steam\\htmlcache"), jsRuntime, SteamReturn);
            WriteLine(jsRuntime, "Cleared HTMLCache.");
            NewLine(jsRuntime);
        }

        public void Steam_Clear_UiLogs(IJSRuntime jsRuntime)
        {
            // Overlay UI logs -
            //   Steam\GameOverlayUI.exe.log
            //   Steam\GameOverlayRenderer.log
            GeneralFuncs.ClearFilesOfType(Steam.FolderPath, "*.log|*.last", SearchOption.TopDirectoryOnly, jsRuntime, SteamReturn);
            WriteLine(jsRuntime, "Cleared UI Logs.");
            NewLine(jsRuntime);
        }

        public void Steam_Clear_AppCache(IJSRuntime jsRuntime)
        {
            // App Cache - Steam\appcache
            GeneralFuncs.ClearFilesOfType(Path.Combine(Steam.FolderPath, "appcache"), "*.*", SearchOption.TopDirectoryOnly, jsRuntime, SteamReturn);
            WriteLine(jsRuntime, "Cleared AppCache.");
            NewLine(jsRuntime);
        }

        public void Steam_Clear_HttpCache(IJSRuntime jsRuntime)
        {
            GeneralFuncs.ClearFilesOfType(Path.Combine(Steam.FolderPath, "appcache\\httpcache"), "*.*", SearchOption.AllDirectories, jsRuntime, SteamReturn);
            WriteLine(jsRuntime, "Cleared HTTPCache.");
            NewLine(jsRuntime);
        }

        public void Steam_Clear_DepotCache(IJSRuntime jsRuntime)
        {
            GeneralFuncs.ClearFilesOfType(Path.Combine(Steam.FolderPath, "depotcache"), "*.*", SearchOption.TopDirectoryOnly, jsRuntime, SteamReturn);
            WriteLine(jsRuntime, "Cleared DepotCache.");
            NewLine(jsRuntime);
        }

        public void Steam_Clear_Forgotten(IJSRuntime jsRuntime)
        {
            SteamSwitcherFuncs.ClearForgotten(jsRuntime);
            WriteLine(jsRuntime, "Cleared forgotten account backups");
            NewLine(jsRuntime);
        }

        public void Steam_Clear_Config(IJSRuntime jsRuntime)
        {
            GeneralFuncs.DeleteFile(Path.Combine(Steam.FolderPath, "config\\config.vdf"), js: jsRuntime, jsDest: SteamReturn);
            WriteLine(jsRuntime, "[ Don't forget to clear forgotten account backups as well ]");
            WriteLine(jsRuntime, "Cleared config\\config.vdf");
            NewLine(jsRuntime);
        }

        public void Steam_Clear_LoginUsers(IJSRuntime jsRuntime)
        {
            GeneralFuncs.DeleteFile(Path.Combine(Steam.FolderPath, "config\\loginusers.vdf"), js: jsRuntime, jsDest: SteamReturn);
            WriteLine(jsRuntime, "Cleared config\\loginusers.vdf");
            NewLine(jsRuntime);
        }

        public void Steam_Clear_Ssfn(IJSRuntime jsRuntime)
        {
            var d = new DirectoryInfo(Steam.FolderPath);
            var i = 0;
            foreach (var f in d.GetFiles("ssfn*"))
            {
                GeneralFuncs.DeleteFile(fileInfo: f, js: jsRuntime, jsDest: SteamReturn);
                i++;
            }

            WriteLine(jsRuntime, i == 0 ? "No SSFN files found." : "Cleared SSFN files.");
            NewLine(jsRuntime);
        }

        public void Steam_Clear_AutoLoginUser(IJSRuntime jsRuntime) => GeneralFuncs.DeleteRegKey(@"Software\Valve\Steam", "AutoLoginuser", jsRuntime, SteamReturn);
        public void Steam_Clear_LastGameNameUsed(IJSRuntime jsRuntime) => GeneralFuncs.DeleteRegKey(@"Software\Valve\Steam", "LastGameNameUsed", jsRuntime, SteamReturn);
        public void Steam_Clear_PseudoUUID(IJSRuntime jsRuntime) => GeneralFuncs.DeleteRegKey(@"Software\Valve\Steam", "PseudoUUID", jsRuntime, SteamReturn);
        public void Steam_Clear_RememberPassword(IJSRuntime jsRuntime) => GeneralFuncs.DeleteRegKey(@"Software\Valve\Steam", "RememberPassword", jsRuntime, SteamReturn);
    }
}
