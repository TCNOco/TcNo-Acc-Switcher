using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Linq;
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
        private IJSObjectReference _jsModule;
        protected override async Task OnInitializedAsync()
        {
            _jsModule = await JsRuntime.InvokeAsync<IJSObjectReference>("import", "./js/steam/settings.js");
            await _jsModule.InvokeAsync<string>("jsLoadSettings");
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
        public static void ClearVacStatus() => SteamSwitcherFuncs.DeleteVacCacheFile();

        // BUTTON: Reset settings
        public static void ClearSettings() => SteamSwitcherFuncs.ResetSettings_Steam();

        // BUTTON: Reset images
        public static void ClearImages() => SteamSwitcherFuncs.ClearImages();
        #endregion

        #region SETTINGS_STEAM_TOOLS
        // Restore forgotten accounts

        // BUTTON: Clear forgotten backups

        // BUTTON: Open Steam Folder
        // - TODO: Also add this to the Right-Click menu, when no Steam account is selected (whitespace).
        public static void OpenSteamFolder() => Process.Start("explorer.exe", SteamSwitcherFuncs.SteamFolder());

        // BUTTON: Advanced Cleaning...
        // - TODO: Handle in button -> Navigate to another page.

        #endregion

    }
}
