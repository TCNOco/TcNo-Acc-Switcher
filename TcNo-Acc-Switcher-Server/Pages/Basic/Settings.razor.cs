// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2022 TechNobo (Wesley Pyburn)
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
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.Basic
{
    public partial class Settings
    {
        private static readonly Lang Lang = Lang.Instance;
        [Inject]
        public AppData AppData { get; set; }
        protected override void OnInitialized()
        {
            AppData.Instance.WindowTitle = Lang["Title_Template_Settings", new { platformName = CurrentPlatform.Instance.FullName }];
            Globals.DebugWriteLine(@"[Auto:Basic\Settings.razor.cs.OnInitializedAsync]");
        }

        #region SETTINGS_GENERAL
        // BUTTON: Pick folder
        public void PickFolder()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Basic\Settings.razor.cs.PickFolder]");
            _ = GeneralInvocableFuncs.ShowModal("find:Basic:BasicGamesLauncher.exe:BasicSettings");
        }

        // BUTTON: Reset settings
        public static void ClearSettings()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Basic\Settings.razor.cs.ClearSettings]");
            new Data.Settings.Basic().ResetSettings();
            AppData.ActiveNavMan.NavigateTo("/Basic?toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeDataString(Lang["Toast_ClearedPlatformSettings", new { platform = "Basic" }]));
        }
        #endregion

        #region SETTINGS_TOOLS
        // BUTTON: Open Folder
        public static void OpenFolder()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Basic\Settings.razor.cs.OpenBasicFolder]");
            _ = Process.Start("explorer.exe", new Data.Settings.Basic().FolderPath);
        }


        // BUTTON: Advanced Cleaning...
        // Might add later

        #endregion

    }
}
