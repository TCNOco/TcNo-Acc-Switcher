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
