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
using System.Linq;
using System.Text.Encodings.Web;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.Steam
{
    public partial class Settings : ComponentBase
    {
        [Inject]
        public IJSRuntime JsRuntime { get; set; }
        [Inject]
        public Data.AppData AppData { get; set; }
        private IJSObjectReference _jsModule;
        protected override async Task OnInitializedAsync()
        {
            AppData.WindowTitle = "TcNo Account Switcher - Steam Settings";
            //_jsModule = await JsRuntime.InvokeAsync<IJSObjectReference>("import", "./js/steam/settings.js");
            //await _jsModule.InvokeAsync<string>("jsLoadSettings");
        }
        public async Task ClickSaveSettings()
        {
            await _jsModule.InvokeAsync<string>("jsSaveSettings");
            NavManager.NavigateTo("/Steam");
        }
        
        #region SETTINGS_GENERAL
        // BUTTON: Pick Steam folder
        public async Task PickSteamFolder()
        {
            await JsRuntime.InvokeAsync<string>("ShowModal", "find:Steam:Steam.exe:SteamSettings");
        }

        // BUTTON: Check account VAC status
        public static async void ClearVacStatus(IJSRuntime Js)
        {
            if (SteamSwitcherFuncs.DeleteVacCacheFile())
                await GeneralInvocableFuncs.ShowToast(Js, "success", "VAC status for accounts was cleared", renderTo: "toastarea");
            else
                await GeneralInvocableFuncs.ShowToast(Js, "error", "Could not delete 'profilecache/SteamVACCache.json'", "Error", "toastarea");
        }

        // BUTTON: Reset settings
        public static void ClearSettings(NavigationManager navManager)
        {
            new Data.Settings.Steam().ResetSettings();
            navManager.NavigateTo("/Steam?toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Cleared Steam switcher settings"));
        }

        // BUTTON: Reset images
        public static void ClearImages(IJSRuntime js) => SteamSwitcherFuncs.ClearImages(js); // ADD A TOAST TO THIS
        #endregion

        #region SETTINGS_STEAM_TOOLS
        // Restore forgotten accounts

        // BUTTON: Clear forgotten backups

        // BUTTON: Open Steam Folder
        // - TODO: Also add this to the Right-Click menu, when no Steam account is selected (whitespace).
        public static void OpenSteamFolder() => Process.Start("explorer.exe", new Data.Settings.Steam().FolderPath);

        // BUTTON: Advanced Cleaning...
        // Handled on page

        #endregion

    }
}
