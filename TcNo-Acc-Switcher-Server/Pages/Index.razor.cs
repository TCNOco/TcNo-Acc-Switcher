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
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using TcNo_Acc_Switcher_Server.Shared.ContextMenu;
using TcNo_Acc_Switcher_Server.Shared.Modal;
using TcNo_Acc_Switcher_Server.Shared.Toast;
using BasicSettings = TcNo_Acc_Switcher_Server.Data.Settings.Basic;
using SteamSettings = TcNo_Acc_Switcher_Server.Data.Settings.Steam;

namespace TcNo_Acc_Switcher_Server.Pages
{
    public partial class Index
    {
        private static readonly Lang Lang = Lang.Instance;
        [Inject] private AppSettings ASettings { get; set; }

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
            AppData.WindowTitle = "TcNo Account Switcher";
            AppSettings.Platforms.CollectionChanged += (_, _) => InvokeAsync(StateHasChanged);

            // If no platforms are showing:
            if (AppSettings.Platforms.All(x => !x.Enabled))
            {
                NavManager.NavigateTo("Platforms");
            }

            // If just 1 platform is showing, and first launch: Nav into:
            if (AppData.FirstMainMenuVisit && AppSettings.Platforms.Count(x => x.Enabled) == 1)
            {
                AppData.FirstMainMenuVisit = false;
                var onlyPlatform = AppSettings.Platforms.First(x => x.Enabled);
                await Check(onlyPlatform.Name);
            }
            AppData.FirstMainMenuVisit = false;

            if (firstRender)
            {
                await GeneralFuncs.HandleQueries();
                //await AppData.InvokeVoidAsync("initContextMenu");
                await AppData.InvokeVoidAsync("initPlatformListSortable");
                //await AData.InvokeVoidAsync("initAccListSortable");
            }

            AppData.FirstLaunchCheck();

            AppStats.NewNavigation("/");
        }


        /// <summary>
        /// Check can enter platform, before navigating to page.
        /// </summary>
        public async Task Check(string platform)
        {
            Globals.DebugWriteLine($@"[Func:Index.Check] platform={platform}");
            if (platform == "Steam")
            {
                if (!GeneralFuncs.CanKillProcess(SteamSettings.Processes, SteamSettings.ClosingMethod)) return;
                if (!Directory.Exists(SteamSettings.FolderPath) || !File.Exists(SteamSettings.Exe()))
                {
                    AppData.CurrentSwitcher = "Steam";
                    ModalFuncs.ShowUpdatePlatformFolderModal();
                    return;
                }
                if (SteamSwitcherFuncs.SteamSettingsValid()) AppData.NavigateTo("/Steam/");
                else await GeneralInvocableFuncs.ShowModal(Lang["Toast_Steam_CantLocateLoginusers"]);
                return;
            }

            AppData.CurrentSwitcher = platform;
            BasicPlatforms.SetCurrentPlatform(platform);
            if (!GeneralFuncs.CanKillProcess(CurrentPlatform.ExesToEnd, BasicSettings.ClosingMethod)) return;

            if (Directory.Exists(BasicSettings.FolderPath) && File.Exists(BasicSettings.Exe()))
                AppData.NavigateTo("/Basic/");
            else
                ModalFuncs.ShowUpdatePlatformFolderModal();
        }

        #region Platform context menu
        /// <summary>
        /// On context menu click, create shortcut to platform on Desktop.
        /// </summary>
        public void CreatePlatformShortcut()
        {
            var platform = AppSettings.GetPlatform(AppData.SelectedPlatform);
            Shortcut.PlatformDesktopShortcut(Shortcut.Desktop, platform.Name, platform.Identifier, true);

            AData.ShowToastLang(ToastType.Success, "Success", "Toast_ShortcutCreated");
        }

        /// <summary>
        /// On context menu click, Export all account names, stats and more to a CSV.
        /// </summary>
        public async Task ExportAllAccounts()
        {
            if (AppData.IsCurrentlyExportingAccounts)
            {
                AData.ShowToastLang(ToastType.Error, "Error", "Toast_AlreadyProcessing");
                return;
            }

            AppData.IsCurrentlyExportingAccounts = true;

            var exportPath = await GeneralFuncs.ExportAccountList();
            await AppData.InvokeVoidAsync("saveFile", exportPath.Split('\\').Last(), exportPath);
            AppData.IsCurrentlyExportingAccounts = false;
        }

        /// <summary>
        /// Hide a platform from the platforms list. Not giving an item input will use the AppData.SelectedPlatform.
        /// </summary>
        public void HidePlatform(string item = null)
        {
            var platform = item ?? AppData.SelectedPlatform;
            AppSettings.Platforms.First(x => x.Name == platform).SetEnabled(false);
        }

        /// <summary>
        /// Shows the context menu for the requested platform
        /// </summary>
        public static async void PlatformRightClick(MouseEventArgs e, string plat)
        {
            if (e.Button != 2) return;
            AppData.SelectedPlatform = plat;
            await AppData.InvokeVoidAsync("positionAndShowMenu", e, "#AccOrPlatList");
        }
        #endregion
    }
}
