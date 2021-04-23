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
using System.Diagnostics;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.Ubisoft
{
    public partial class Settings : ComponentBase
    {
        [Inject]
        public Data.AppData AppData { get; set; }
        private IJSObjectReference _jsModule;
        protected override async Task OnInitializedAsync()
        {
            AppData.WindowTitle = "TcNo Account Switcher - Ubisoft Settings";
            Globals.DebugWriteLine($@"[Auto:Ubisoft\Settings.razor.cs.OnInitializedAsync]");
        }

        #region SETTINGS_GENERAL
        // BUTTON: Pick folder
        public async Task PickFolder()
        {
            Globals.DebugWriteLine($@"[ButtonClicked:Ubisoft\Settings.razor.cs.PickFolder]");
            await JsRuntime.InvokeAsync<string>("ShowModal", "find:Ubisoft:upc.exe:UbisoftSettings");
        }
        
        // BUTTON: Reset settings
        public static void ClearSettings()
        {
            Globals.DebugWriteLine($@"[ButtonClicked:Ubisoft\Settings.razor.cs.ClearSettings]");
            new Data.Settings.Ubisoft().ResetSettings();
            AppData.ActiveNavMan.NavigateTo("/Ubisoft?toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Cleared Ubisoft switcher settings"));
        }
        #endregion

        #region SETTINGS_TOOLS
        // BUTTON: Open Folder
        public static void OpenFolder()
        {
            Globals.DebugWriteLine($@"[ButtonClicked:Ubisoft\Settings.razor.cs.OpenUbisoftFolder]");
            Process.Start("explorer.exe", new Data.Settings.Ubisoft().FolderPath);
        }

        // BUTTON: RefreshImages
        public static void RefreshImages()
        {
            var allIds = UbisoftSwitcherFuncs.ReadAllIds();
            foreach (var (userId, username) in allIds)
            {
                UbisoftSwitcherFuncs.ImportAvatar(userId);
            }
            _ = GeneralInvocableFuncs.ShowToast("success", "Refreshed images", "Done", "toastarea");
        }


        // BUTTON: Advanced Cleaning...
        // Might add later

        #endregion

    }
}
