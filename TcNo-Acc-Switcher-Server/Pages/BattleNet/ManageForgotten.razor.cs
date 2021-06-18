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

// Special thanks to iR3turnZ for contributing to this platform's account switcher
// iR3turnZ: https://github.com/HoeblingerDaniel

using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;

namespace TcNo_Acc_Switcher_Server.Pages.BattleNet
{
    public partial class ManageForgotten
    {
        [Inject]
        public AppData AppData { get; set; }
        private IJSObjectReference _jsModule;
        protected override async void OnInitialized()
        {
            Globals.DebugWriteLine(@"[Auto:BattleNet\ManageForgotten.razor.cs.OnInitializedAsync]");
            AppData.Instance.WindowTitle = "TcNo Account Switcher - Manage ignored BattleNet accounts";
            _jsModule = await _jsRuntime.InvokeAsync<IJSObjectReference>("import", "./js/battlenet/ManageIgnored.js");
            await _jsModule.InvokeAsync<string>("jsLoadIgnored");
        }

        /// <summary>
        /// Restores Steam accounts from the Forgotten backup file, back into loginusers.vdf
        /// </summary>
        /// <param name="selectedIds">List of SteamIds for accounts to be restored back into loginusers.vdf from the backup file</param>
        /// <returns>True if accounts were successfully restored</returns>
        [JSInvokable]
        public static Task<bool> BattleNet_RestoreSelected(string[] selectedIds) => Task.FromResult(BattleNetSwitcherFuncs.RestoreSelected(selectedIds));
    }
}
