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

namespace TcNo_Acc_Switcher_Server.Pages.Origin
{
    public partial class Settings : ComponentBase
    {
        [Inject]
        public Data.AppData AppData { get; set; }
        private IJSObjectReference _jsModule;
        protected override async Task OnInitializedAsync()
        {
            AppData.WindowTitle = "TcNo Account Switcher - Origin Settings";
            Globals.DebugWriteLine($@"[Auto:Origin\Settings.razor.cs.OnInitializedAsync]");
        }

        #region SETTINGS_GENERAL
        // BUTTON: Pick folder
        public async Task PickFolder()
        {
            Globals.DebugWriteLine($@"[ButtonClicked:Origin\Settings.razor.cs.PickFolder]");
            await JsRuntime.InvokeAsync<string>("ShowModal", "find:Origin:Origin.exe:OriginSettings");
        }

        // BUTTON: Reset settings
        public static void ClearSettings()
        {
            Globals.DebugWriteLine($@"[ButtonClicked:Origin\Settings.razor.cs.ClearSettings]");
            new Data.Settings.Origin().ResetSettings();
            AppData.ActiveNavMan.NavigateTo("/Origin?toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Cleared Origin switcher settings"));
        }
        #endregion

        #region SETTINGS_TOOLS
        // BUTTON: Open Folder
        public static void OpenFolder()
        {
            Globals.DebugWriteLine($@"[ButtonClicked:Origin\Settings.razor.cs.OpenOriginFolder]");
            Process.Start("explorer.exe", new Data.Settings.Origin().FolderPath);
        }
        

        // BUTTON: Advanced Cleaning...
        // Might add later

        #endregion

    }
}
