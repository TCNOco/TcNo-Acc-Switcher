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
using System.Threading.Tasks;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Shared.Toast;

namespace TcNo_Acc_Switcher_Server.Pages.Steam
{
    public partial class Settings
    {
        protected override void OnInitialized()
        {
            AppData.WindowTitle = Lang["Title_Steam_Settings"];
            Globals.DebugWriteLine(@"[Auto:Steam\Settings.razor.cs.OnInitializedAsync]");
        }

        #region SETTINGS_GENERAL
        // BUTTON: Pick folder
        public async Task PickFolder()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\Settings.razor.cs.PickFolder]");
            await GeneralInvocableFuncs.ShowModal("find:Steam:Steam.exe:SteamSettings");
        }

        // BUTTON: Check account VAC status
        public void ClearVacStatus()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\Settings.razor.cs.ClearVacStatus]");
            if (Globals.DeleteFile(Data.Settings.Steam.VacCacheFile))
                AData.ShowToastLang(ToastType.Success, "Toast_Steam_VacCleared");
            else
                AData.ShowToastLang(ToastType.Error, "Error", "Toast_Steam_CantDeleteVacCache");
        }

        // BUTTON: Reset settings
        public void ClearSettings()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\Settings.razor.cs.ClearSettings]");
            Data.Settings.Steam.ResetSettings();
            AppData.NavigateToWithToast("/Steam", "success", Lang["Success"], Lang["Toast_ClearedPlatformSettings", new { platform = "Steam" }]);
        }

        // BUTTON: Reset images
        public static async Task ClearImages()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\Settings.razor.cs.ClearImages]");
            await SteamSwitcherFuncs.ClearImages();
        }
        #endregion
    }
}
