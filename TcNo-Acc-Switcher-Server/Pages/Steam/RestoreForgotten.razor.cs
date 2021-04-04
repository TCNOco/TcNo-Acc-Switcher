using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.Steam
{
    public partial class RestoreForgotten : ComponentBase
    {
        [Inject]
        public Data.AppData AppData { get; set; }
        private IJSObjectReference _jsModule;
        protected override async Task OnInitializedAsync()
        {
            AppData.WindowTitle = "TcNo Account Switcher - Restore forgotten Steam account";
            _jsModule = await JsRuntime.InvokeAsync<IJSObjectReference>("import", "./js/steam/RestoreForgotten.js");
            await _jsModule.InvokeAsync<string>("jsLoadForgotten");
        }

        /// <summary>
        /// Restores Steam accounts from the Forgotten backup file, back into loginusers.vdf
        /// </summary>
        /// <param name="selectedIds">List of SteamIds for accounts to be restored back into loginusers.vdf from the backup file</param>
        /// <returns>True if accounts were successfully restored</returns>
        [JSInvokable]
        public static Task<bool> Steam_RestoreSelected(string[] selectedIds)
        {
            return Task.FromResult(SteamSwitcherFuncs.RestoreAccounts(selectedIds));
        }
    }
}
