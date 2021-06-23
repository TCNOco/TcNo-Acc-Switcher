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

using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Threading.Tasks;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.BattleNet
{
    public partial class Settings
    {
        protected override void OnInitialized()
        {
            AppData.Instance.WindowTitle = "TcNo Account Switcher - BattleNet Settings";
            Globals.DebugWriteLine(@"[Auto:BattleNet\Settings.razor.cs.OnInitializedAsync]");
        }

        #region SETTINGS_GENERAL
        // BUTTON: Pick folder
        public void PickFolder()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:BattleNet\Settings.razor.cs.PickFolder]");
            GeneralInvocableFuncs.ShowModal("find:BattleNet:Battle.net.exe:BattleNetSettings");
        }

        // BUTTON: Reset settings
        public static void ClearSettings()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Origin\Settings.razor.cs.ClearSettings]");
            new Data.Settings.BattleNet().ResetSettings();
            AppData.ActiveNavMan.NavigateTo("/BattleNet?toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Cleared BattleNet switcher settings"));
        }

        // CHECKBOX: Show Overwatch images
        public static void OverwatchToggle()
        {
            GeneralFuncs.ClearFolder("wwwroot\\img\\profiles\\battlenet\\");
        }

        // BUTTON: Clear Forgotten
        public static void ClearForgotten()
        {
            Data.Settings.BattleNet.Instance.IgnoredAccounts.Clear();
            Data.Settings.BattleNet.Instance.SaveAccounts();
        }
        
        // BUTTON: Clear accounts
        public static void ClearAccounts()
        {
            Data.Settings.BattleNet.Instance.Accounts = new List<BattleNetSwitcherBase.BattleNetUser>();
            Data.Settings.BattleNet.Instance.IgnoredAccounts = new List<string>();
            Data.Settings.BattleNet.Instance.SaveAccounts();
        }
        #endregion

        #region SETTINGS_TOOLS
        // BUTTON: Open Folder
        public static void OpenFolder()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:BattleNet\Settings.razor.cs.OpenBattleNetFolder]");
            Process.Start("explorer.exe", new Data.Settings.BattleNet().FolderPath);
        }
        
        
        #endregion
    }
}