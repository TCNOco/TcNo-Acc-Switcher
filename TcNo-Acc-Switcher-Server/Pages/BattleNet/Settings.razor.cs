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

// Special thanks to iR3turnZ for contributing to this platform's account switcher
// iR3turnZ: https://github.com/HoeblingerDaniel

using System;
using System.Collections.Generic;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.BattleNet
{
    public partial class Settings
    {
        private static readonly Lang Lang = Lang.Instance;
        protected override void OnInitialized()
        {
            AppData.Instance.WindowTitle = Lang["Title_BNet_Settings"];
            Globals.DebugWriteLine(@"[Auto:BattleNet\Settings.razor.cs.OnInitializedAsync]");
        }

        #region SETTINGS_GENERAL
        // BUTTON: Pick folder
        public void PickFolder()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:BattleNet\Settings.razor.cs.PickFolder]");
            _ = GeneralInvocableFuncs.ShowModal("find:BattleNet:Battle.net.exe:BattleNetSettings");
        }

        // BUTTON: Reset settings
        public static void ClearSettings()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:BattleNet\Settings.razor.cs.ClearSettings]");
            Data.Settings.BattleNet.ResetSettings();
            AppData.ActiveNavMan.NavigateTo(
                $"/BattleNet?toast_type=success&toast_title={Uri.EscapeDataString(Lang["Success"])}&toast_message={Uri.EscapeDataString(Lang["Toast_ClearedPlatformSettings", new {platform = "BattleNet"}])}");
        }

        // CHECKBOX: Show Overwatch images
        public static void OverwatchToggle()
        {
            GeneralFuncs.ClearFolder("wwwroot\\img\\profiles\\battlenet\\");
        }

        // BUTTON: Clear Forgotten
        public static void ClearForgotten()
        {
            Data.Settings.BattleNet.IgnoredAccounts.Clear();
            Data.Settings.BattleNet.SaveAccounts();
        }

        // BUTTON: Clear accounts
        public static void ClearAccounts()
        {
            Data.Settings.BattleNet.Accounts = new List<BattleNetSwitcherBase.BattleNetUser>();
            Data.Settings.BattleNet.IgnoredAccounts = new List<string>();
            Data.Settings.BattleNet.SaveAccounts();
        }
        #endregion
    }
}