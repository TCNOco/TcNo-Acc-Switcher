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
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.AspNetCore.Components.Web;
using Microsoft.AspNetCore.WebUtilities;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using TcNo_Acc_Switcher_Server.Shared.ContextMenu;
using TcNo_Acc_Switcher_Server.State;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.Pages;

public partial class Index
{
    [Inject] private IStatistics Statistics { get; set; }
    [Inject] private IToasts Toasts { get; set; }
    [Inject] private ITemplatedPlatformState TemplatedPlatformState { get; set; }
    [Inject] private ISteamSettings SteamSettings { get; set; }
    [Inject] private NavigationManager NavigationManager { get; set; }
    [Inject] private IGameStats GameStats { get; set; }
    [Inject] private ISteamState SteamState { get; set; }
    [Inject] private ITemplatedPlatformSettings TemplatedPlatformSettings { get; set; }
    [Inject] private ITemplatedPlatformFuncs TemplatedPlatformFuncs { get; set; }
    [Inject] private ISharedFunctions SharedFunctions { get; set; }

    protected override void OnInitialized()
    {
        _platformContextMenuItems = new MenuBuilder(Lang,
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
            Check(onlyPlatform.Name);
        }
        AppState.WindowState.FirstMainMenuVisit = false;

        if (firstRender)
        {
            await HandleQueries();
            await JsRuntime.InvokeVoidAsync("initPlatformListSortable");
            //await AData.InvokeVoidAsync("initAccListSortable");
        }

        Statistics.NewNavigation("/");
    }

    /// <summary>
    /// For handling queries in URI
    /// </summary>
    public async Task<bool> HandleQueries()
    {
        Globals.DebugWriteLine(@"[JSInvoke:General\GeneralFuncs.HandleQueries]");
        var uri = NavigationManager.ToAbsoluteUri(NavigationManager.Uri);
        // Clear cache reload
        var queries = QueryHelpers.ParseQuery(uri.Query);
        // cacheReload handled in JS

        // Toast
        if (!queries.TryGetValue("toast_type", out var toastType) ||
            !queries.TryGetValue("toast_title", out var toastTitle) ||
            !queries.TryGetValue("toast_message", out var toastMessage)) return true;
        for (var i = 0; i < toastType.Count; i++)
        {
            try
            {
                var type = (ToastType)Enum.Parse(typeof(ToastType), toastType[i]);
                Toasts.ShowToastLang(type, toastTitle[i], toastMessage[i]);
                await JsRuntime.InvokeVoidAsync("removeUrlArgs", "toast_type,toast_title,toast_message");
            }
            catch (TaskCanceledException e)
            {
                Globals.WriteToLog(e.ToString());
            }
        }

        return true;
    }

    /// <summary>
    /// Check can enter platform, before navigating to page.
    /// </summary>
    public void Check(string platform)
    {
        Globals.DebugWriteLine($@"[Func:Index.Check] platform={platform}");
        if (platform == "Steam")
        {
            if (!SharedFunctions.CanKillProcess(SteamSettings.Processes, SteamSettings.ClosingMethod)) return;
            if (!Directory.Exists(SteamSettings.FolderPath) || !File.Exists(SteamSettings.Exe))
            {
                AppState.Switcher.CurrentSwitcher = "Steam";
                Modals.ShowUpdatePlatformFolderModal();
                return;
            }
            if (File.Exists(SteamSettings.LoginUsersVdf))
                NavigationManager.NavigateTo("/Steam/");
            else
                Toasts.ShowToastLang(ToastType.Error, "Toast_Steam_CantLocateLoginusers");
            return;
        }

        AppState.Switcher.CurrentSwitcher = platform;
        TemplatedPlatformState.LoadTemplatedPlatformState(JsRuntime, TemplatedPlatformSettings, TemplatedPlatformFuncs);
        TemplatedPlatformState.SetCurrentPlatform(JsRuntime, TemplatedPlatformSettings, platform);
        if (!SharedFunctions.CanKillProcess(TemplatedPlatformState.CurrentPlatform.ExesToEnd, TemplatedPlatformSettings.ClosingMethod)) return;

        if (Directory.Exists(TemplatedPlatformSettings.FolderPath) && File.Exists(TemplatedPlatformSettings.Exe))
            NavManager.NavigateTo("/Basic/");
        else
            Modals.ShowUpdatePlatformFolderModal();
    }

    #region Platform context menu
    /// <summary>
    /// On context menu click, create shortcut to platform on Desktop.
    /// </summary>
    public void CreatePlatformShortcut()
    {
        var platform = WindowSettings.GetPlatform(AppState.Switcher.SelectedPlatform);
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

        var exportPath = await ExportAccountList();
        await JsRuntime.InvokeVoidAsync("saveFile", exportPath.Split('\\').Last(), exportPath);
        AppState.Switcher.IsCurrentlyExportingAccounts = false;
    }
    public async Task<string> ExportAccountList()
    {
        var platform = WindowSettings.GetPlatform(AppState.Switcher.SelectedPlatform);
        Globals.DebugWriteLine(@$"[Func:Pages\General\GeneralInvocableFuncs.GiExportAccountList] platform={platform}");
        if (!Directory.Exists(Path.Join("LoginCache", platform.SafeName)))
        {
            Toasts.ShowToastLang(ToastType.Error, "Toast_AddAccountsFirstTitle", "Toast_AddAccountsFirst");
            return "";
        }

        var s = CultureInfo.CurrentCulture.TextInfo.ListSeparator; // Different regions use different separators in csv files.

        await GameStats.SetCurrentPlatform(platform.Name);

        List<string> allAccountsTable = new();
        if (platform.Name == "Steam")
        {
            // Add headings and separator for programs like Excel
            allAccountsTable.Add($"SEP={s}");
            allAccountsTable.Add($"Account name:{s}Community name:{s}SteamID:{s}VAC status:{s}Last login:{s}Saved profile image:{s}Stats game:{s}Stat name:{s}Stat value:");

            SteamState.SteamUsers = SteamState.GetSteamUsers(SteamSettings.LoginUsersVdf);
            // Load cached ban info
            SteamState.LoadCachedBanInfo();

            foreach (var su in SteamState.SteamUsers)
            {
                var banInfo = "";
                if (su.Vac && su.Limited) banInfo += "VAC + Limited";
                else banInfo += (su.Vac ? "VAC" : "") + (su.Limited ? "Limited" : "");

                var imagePath = Path.GetFullPath($"{SteamSettings.SteamImagePath + su.SteamId}.jpg");

                allAccountsTable.Add(su.AccName + s +
                                     su.Name + s +
                                     su.SteamId + s +
                                     banInfo + s +
                                     Globals.UnixTimeStampToDateTime(su.LastLogin) + s +
                                     (File.Exists(imagePath) ? imagePath : "Missing from disk") + s +
                                     GameStats.GetSavedStatsString(su.SteamId, s));
            }
        }
        else
        {
            // Add headings and separator for programs like Excel
            allAccountsTable.Add($"SEP={s}");
            // Platform does not have specific details other than usernames saved.
            allAccountsTable.Add($"Account name:{s}Stats game:{s}Stat name:{s}Stat value:");
            foreach (var accDirectory in Directory.GetDirectories(Path.Join("LoginCache", platform.SafeName)))
            {
                allAccountsTable.Add(Path.GetFileName(accDirectory) + s +
                                     GameStats.GetSavedStatsString(accDirectory, s, true));
            }
        }

        var outputFolder = Path.Join("wwwroot", "Exported");
        _ = Directory.CreateDirectory(outputFolder);

        var outputFile = Path.Join(outputFolder, platform + ".csv");
        await File.WriteAllLinesAsync(outputFile, allAccountsTable).ConfigureAwait(false);
        return Path.Join("Exported", platform + ".csv");
    }

    /// <summary>
    /// Hide a platform from the platforms list. Not giving an item input will use the AppData.SelectedPlatform.
    /// </summary>
    public void HidePlatform(string item = null)
    {
        var platform = item ?? AppState.Switcher.SelectedPlatform;
        WindowSettings.Platforms.First(x => x.Name == platform).SetEnabled(false);
    }
    #endregion
}