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
using System.Linq;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.AspNetCore.Components.Web;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using TcNo_Acc_Switcher_Server.Shared.ContextMenu;
using TcNo_Acc_Switcher_Server.Shared.Modal;
using TcNo_Acc_Switcher_Server.State;
using TcNo_Acc_Switcher_Server.State.DataTypes;

namespace TcNo_Acc_Switcher_Server.Pages
{
    public partial class Index
    {
        [Inject] private Toasts Toasts { get; set; }
        [Inject] private NewLang Lang { get; set; }
        [Inject] private BasicSettings BasicSettings { get; set; }
        [Inject] private SteamSettings SteamSettings { get; set; }
        [Inject] private NavigationManager NavigationManager { get; set; }

        protected override void OnInitialized()
        {
            _platformContextMenuItems = new MenuBuilder(
                new Tuple<string, object>[]
                {
                    new ("Context_HidePlatform", new Action(() => HidePlatform())),
                    new ("Context_CreateShortcut", new Action(CreatePlatformShortcut)),
                    new ("Context_ExportAccList", new Action(async () => await ExportAllAccounts())),
                }).Result();
        }

        protected override async Task OnAfterRenderAsync(bool firstRender)
        {
            AppState.WindowState.WindowTitle = "TcNo Account Switcher";
            WindowSettings.Platforms.CollectionChanged += (_, _) => InvokeAsync(StateHasChanged);

            // If no platforms are showing:
            if (WindowSettings.Platforms.All(x => !x.Enabled))
            {
                NavManager.NavigateTo("Platforms");
            }

            // If just 1 platform is showing, and first launch: Nav into:
            if (AppState.WindowState.FirstMainMenuVisit && WindowSettings.Platforms.Count(x => x.Enabled) == 1)
            {
                AppState.WindowState.FirstMainMenuVisit = false;
                var onlyPlatform = WindowSettings.Platforms.First(x => x.Enabled);
                await Check(onlyPlatform.Name);
            }
            AppState.WindowState.FirstMainMenuVisit = false;

            if (firstRender)
            {
                await GeneralFuncs.HandleQueries();
                //await AppData.InvokeVoidAsync("initContextMenu");
                await JsRuntime.InvokeVoidAsync("initPlatformListSortable");
                //await AData.InvokeVoidAsync("initAccListSortable");
            }

            AppStats.NewNavigation("/");
        }


        /// <summary>
        /// Check can enter platform, before navigating to page.
        /// </summary>
        public void Check(string platform)
        {
            Globals.DebugWriteLine($@"[Func:Index.Check] platform={platform}");
            if (platform == "Steam")
            {
                if (!GeneralFuncs.CanKillProcess(SteamSettings.Processes, SteamSettings.ClosingMethod)) return;
                if (!Directory.Exists(SteamSettings.FolderPath) || !File.Exists(SteamSettings.Exe))
                {
                    AppState.Switcher.CurrentSwitcher = "Steam";
                    ModalFuncs.ShowUpdatePlatformFolderModal();
                    return;
                }
                if (File.Exists(SteamSettings.LoginUsersVdf))
                    NavigationManager.NavigateTo("/Steam/");
                else
                    Toasts.ShowToastLang(ToastType.Error, "Toast_Steam_CantLocateLoginusers");
                return;
            }

            AppState.Switcher.CurrentSwitcher = platform;
            BasicPlatforms.SetCurrentPlatform(platform);
            if (!GeneralFuncs.CanKillProcess(CurrentPlatform.ExesToEnd, BasicSettings.ClosingMethod)) return;

            if (Directory.Exists(BasicSettings.FolderPath) && File.Exists(BasicSettings.Exe))
                NavManager.NavigateTo("/Basic/");
            else
                ModalFuncs.ShowUpdatePlatformFolderModal();
        }

        #region Platform context menu
        /// <summary>
        /// On context menu click, create shortcut to platform on Desktop.
        /// </summary>
        public void CreatePlatformShortcut()
        {
            var platform = WindowSettingsFuncs.GetPlatform(AppData.SelectedPlatform);
            Shortcut.PlatformDesktopShortcut(Shortcut.Desktop, platform.Name, platform.Identifier, true);

            Toasts.ShowToastLang(ToastType.Success, "Success", "Toast_ShortcutCreated");
        }

        /// <summary>
        /// On context menu click, Export all account names, stats and more to a CSV.
        /// </summary>
        public async Task ExportAllAccounts()
        {
            if (AppState.Switcher.IsCurrentlyExportingAccounts)
            {
                Toasts.ShowToastLang(ToastType.Error, "Error", "Toast_AlreadyProcessing");
                return;
            }

            AppState.Switcher.IsCurrentlyExportingAccounts = true;

            var exportPath = await GeneralFuncs.ExportAccountList();
            await JsRuntime.InvokeVoidAsync("saveFile", exportPath.Split('\\').Last(), exportPath);
            AppState.Switcher.IsCurrentlyExportingAccounts = false;
        }

        /// <summary>
        /// Hide a platform from the platforms list. Not giving an item input will use the AppData.SelectedPlatform.
        /// </summary>
        public void HidePlatform(string item = null)
        {
            var platform = item ?? AppState.Switcher.SelectedPlatform;
            WindowSettings.Platforms.First(x => x.Name == platform).SetEnabled(false);
        }

        /// <summary>
        /// Shows the context menu for the requested platform
        /// </summary>
        public async void PlatformRightClick(MouseEventArgs e, string plat)
        {
            if (e.Button != 2) return;
            AppState.Switcher.SelectedPlatform = plat;
            await JsRuntime.InvokeVoidAsync("positionAndShowMenu", e, "#AccOrPlatList");
        }
        #endregion
    }
}
