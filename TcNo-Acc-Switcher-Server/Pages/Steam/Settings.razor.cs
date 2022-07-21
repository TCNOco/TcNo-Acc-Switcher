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
using System.IO;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Shared.Modal;
using TcNo_Acc_Switcher_Server.State;
using TcNo_Acc_Switcher_Server.State.DataTypes;

namespace TcNo_Acc_Switcher_Server.Pages.Steam
{
    public partial class Settings
    {
        [Inject] private Toasts Toasts { get; set; }

        protected override void OnInitialized()
        {
            AppState.WindowState.WindowTitle = Lang["Title_Steam_Settings"];
            Globals.DebugWriteLine(@"[Auto:Steam\Settings.razor.cs.OnInitializedAsync]");
        }

        #region SETTINGS_GENERAL
        // BUTTON: Pick folder
        public void PickFolder()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\Settings.razor.cs.PickFolder]");
            ModalFuncs.ShowUpdatePlatformFolderModal();
        }

        // BUTTON: Check account VAC status
        public void ClearVacStatus()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\Settings.razor.cs.ClearVacStatus]");
            if (Globals.DeleteFile(SteamSettings.VacCacheFile))
                AppState.Toasts.ShowToastLang(ToastType.Success, "Toast_Steam_VacCleared");
            else
                AppState.Toasts.ShowToastLang(ToastType.Error, "Error", "Toast_Steam_CantDeleteVacCache");
        }

        // BUTTON: Reset settings
        public void ClearSettings()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\Settings.razor.cs.ClearSettings]");
            SteamSettings.Reset();
            AppState.Navigation.NavigateToWithToast("/Steam", "success", Lang["Success"], Lang["Toast_ClearedPlatformSettings", new { platform = "Steam" }]);
        }

        // BUTTON: Reset images
        /// <summary>
        /// Clears images folder of contents, to re-download them on next load.
        /// </summary>
        public void ClearImages()
        {
            Globals.DebugWriteLine(@"[ButtonClicked:Steam\Settings.razor.cs.ClearImages]");
            if (!Directory.Exists(SteamSettings.SteamImagePath))
            {
                Toasts.ShowToastLang(ToastType.Error, "Error", "Toast_CantClearImages");
            }
            Globals.DeleteFiles(SteamSettings.SteamImagePath);

            // Reload page, then display notification using a new thread.
            AppState.Navigation.ReloadWithToast("success", Uri.EscapeDataString(Lang["Success"]), Uri.EscapeDataString(Lang["Toast_ClearedImages"]));
        }
        #endregion
    }
}
