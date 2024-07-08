// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2024 TroubleChute (Wesley Pyburn)
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
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.Steam
{
    public partial class Settings
    {
        private static readonly Lang Lang = Lang.Instance;

        protected override void OnInitialized()
        {
            AppData.Instance.WindowTitle = Lang["Title_Steam_Settings"];
            Globals.DebugWriteLine(@"[Auto:Steam\Settings.razor.cs.OnInitializedAsync]");
        }

        #region SETTINGS_GENERAL
        // BUTTON: Pick folder
        public void PickFolder()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\Settings.razor.cs.PickFolder]");
            _ = GeneralInvocableFuncs.ShowModal("find:Steam:Steam.exe:SteamSettings");
        }

        // BUTTON: Check account VAC status
        public static void ClearVacStatus()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\Settings.razor.cs.ClearVacStatus]");
            _ = Globals.DeleteFile(Data.Settings.Steam.VacCacheFile)
                ? GeneralInvocableFuncs.ShowToast("success", Lang["Toast_Steam_VacCleared"], renderTo: "toastarea")
                : GeneralInvocableFuncs.ShowToast("error", Lang["Toast_Steam_CantDeleteVacCache"], Lang["Error"],"toastarea");
        }

        // BUTTON: Reset settings
        public static void ClearSettings()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\Settings.razor.cs.ClearSettings]");
            Data.Settings.Steam.ResetSettings();
            AppData.ActiveNavMan.NavigateTo(
                $"/Steam?toast_type=success&toast_title={Uri.EscapeDataString(Lang["Success"])}&toast_message={Uri.EscapeDataString(Lang["Toast_ClearedPlatformSettings", new {platform = "Steam"}])}");
        }

        // BUTTON: Reset images
        public static void ClearImages()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\Settings.razor.cs.ClearImages]");
            SteamSwitcherFuncs.ClearImages();
        }
        #endregion
    }
}
