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
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;

namespace TcNo_Acc_Switcher_Server.Pages.Riot
{
    public partial class Settings
    {
        private static readonly Lang Lang = Lang.Instance;
        protected override void OnInitialized()
        {
            AppData.Instance.WindowTitle = Lang["Title_Riot_Settings"];
            Globals.DebugWriteLine(@"[Auto:Riot\Settings.razor.cs.OnInitializedAsync]");
        }

        #region SETTINGS_GENERAL
        // BUTTON: Reset settings
        public static void ClearSettings()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Riot\Settings.razor.cs.ClearSettings]");
            new Data.Settings.Riot().ResetSettings();
            AppData.ActiveNavMan.NavigateTo("/Riot?toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeDataString(Lang["Toast_ClearedPlatformSettings", new { platform = "Riot" }]));
        }
        #endregion

        #region SETTINGS_TOOLS

        // BUTTON: Advanced Cleaning...
        // Might add later

        #endregion

    }
}
