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
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Pages.Basic;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using TcNo_Acc_Switcher_Server.Shared.Accounts;
using TextCopy;

namespace TcNo_Acc_Switcher_Server.Data
{
    public class AppFuncs
    {
        #region Account Management
        /// <summary>
        /// Swap to the current AppData.SelectedAccountId.
        /// </summary>
        /// <param name="state">Optional profile state for Steam accounts</param>
        public static async Task SwapToAccount(int state = -1)
        {
            if (!OperatingSystem.IsWindows()) return;

            if (AppData.CurrentSwitcher != "Steam") BasicSwitcherFuncs.SwapBasicAccounts(AppData.SelectedAccountId);

            if (state == -1) state = Steam.OverrideState;
            await SteamSwitcherFuncs.SwapSteamAccounts(AppData.SelectedAccountId, state);
        }

        /// <summary>
        /// Swaps to an empty account, allowing the user to sign in.
        /// </summary>
        /// <param name="state">Optional profile state for Steam accounts</param>
        public static async Task SwapToNewAccount(int state = -1)
        {
            if (!OperatingSystem.IsWindows()) return;

            if (AppData.CurrentSwitcher == "Steam") BasicSwitcherFuncs.SwapBasicAccounts("");

            if (state == -1) state = Steam.OverrideState;
            await SteamSwitcherFuncs.SwapSteamAccounts("", state);
        }

        public static async Task ShowModal(string modal)
        {
            await AppData.InvokeVoidAsync("showModalOld", modal);
        }

        public static async Task ForgetAccount()
        {
            var skipConfirm = AppData.CurrentSwitcher == "Steam" ? Steam.ForgetAccountEnabled : Basic.ForgetAccountEnabled;
            if (!skipConfirm)
                ModalData.ShowModal("confirm", ModalData.ExtraArg.ForgetAccount);
            else
            {
                var trayAcc = AppData.SelectedAccountId;
                if (AppData.CurrentSwitcher == "Steam")
                {
                    Steam.SetForgetAcc(true);

                    // Load and remove account that matches SteamID above.
                    var userAccounts = await SteamSwitcherFuncs.GetSteamUsers(Steam.LoginUsersVdf());
                    _ = userAccounts.RemoveAll(x => x.SteamId == AppData.SelectedAccountId);

                    // Save updated loginusers.vdf file
                    await SteamSwitcherFuncs.SaveSteamUsersIntoVdf(userAccounts);
                    trayAcc = "+s:" + AppData.SelectedAccountId;

                    // Remove from Steam accounts list
                    AppData.SteamAccounts.Remove(AppData.SteamAccounts.First(x => x.AccountId == AppData.SelectedAccountId));
                }
                else
                {
                    Basic.SetForgetAcc(true);

                    // Remove ID from list of ids
                    var idsFile = $"LoginCache\\{AppData.CurrentSwitcher}\\ids.json";
                    if (File.Exists(idsFile))
                    {
                        var allIds = GeneralFuncs.ReadDict(idsFile).Remove(AppData.SelectedAccountId);
                        await File.WriteAllTextAsync(idsFile, JsonConvert.SerializeObject(allIds));
                    }

                    // Remove cached files
                    Globals.RecursiveDelete($"LoginCache\\{AppData.CurrentSwitcher}\\{AppData.SelectedAccountId}", false);

                    // Remove from Steam accounts list
                    AppData.BasicAccounts.Remove(AppData.BasicAccounts.First(x => x.AccountId == AppData.SelectedAccountId));
                }

                // Remove from Tray
                Globals.RemoveTrayUserByArg(AppData.CurrentSwitcher, trayAcc);

                // Remove image
                Globals.DeleteFile(Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\{AppData.CurrentSwitcher}\\{Globals.GetCleanFilePath(AppData.SelectedAccountId)}.jpg"));

                AppData.Instance.NotifyDataChanged();

                await GeneralInvocableFuncs.ShowToast("success", Lang.Instance["Success"], renderTo: "toastarea");
            }
        }

        public static void LoadNotes()
        {
            var filePath = AppData.CurrentSwitcher == "Steam"
                ? "LoginCache\\Steam\\AccountNotes.json"
                : $"LoginCache\\{AppData.CurrentSwitcherSafe}\\AccountNotes.json";
            if (!File.Exists(filePath)) return;

            var loaded = JsonConvert.DeserializeObject<Dictionary<string, string>>(File.ReadAllText(filePath));
            if (loaded is null) return;

            if (AppData.CurrentSwitcher == "Steam")
                foreach (var (key, val) in loaded)
                {
                    var acc = AppData.SteamAccounts.FirstOrDefault(x => x.AccountId == key);
                    if (acc is null) return;
                    acc.Note = val;
                }
            else
                foreach (var (key, val) in loaded)
                {
                    var acc = AppData.BasicAccounts.FirstOrDefault(x => x.AccountId == key);
                    if (acc is null) return;
                    acc.Note = val;
                }

            ModalData.IsShown = false;
        }

        public static async Task ExportAllAccounts()
        {
            if (AppData.IsCurrentlyExportingAccounts)
            {
                await GeneralInvocableFuncs.ShowToast("error", Lang.Instance["Toast_AlreadyProcessing"], Lang.Instance["Error"], "toastarea");
                return;
            }

            AppData.IsCurrentlyExportingAccounts = true;

            //AppData.SelectedPlatform
            var exportPath = await GeneralFuncs.ExportAccountList();
            await AppData.InvokeVoidAsync("saveFile", exportPath.Split('\\').Last(), exportPath);
            AppData.IsCurrentlyExportingAccounts = false;
        }

        public static async Task CreatePlatformShortcut()
        {
            var platform = AppData.SelectedPlatform;

            Globals.DebugWriteLine(@$"[Func:Pages\General\GeneralInvocableFuncs.GiCreatePlatformShortcut] platform={platform}");
            var platId = platform.ToLowerInvariant();
            platform = BasicPlatforms.PlatformFullName(platform); // If it's a basic platform

            var s = new Shortcut();
            _ = s.Shortcut_Platform(Shortcut.Desktop, platform, platId);
            s.ToggleShortcut(true);

            await GeneralInvocableFuncs.ShowToast("success", Lang.Instance["Toast_ShortcutCreated"], Lang.Instance["Success"], "toastarea");
        }

        public static void HidePlatform(string item = null)
        {
            var platform = item ?? AppData.SelectedPlatform;
            Globals.DebugWriteLine(@"[JSInvoke:Data\AppSettings.HidePlatform]");
            if (BasicPlatforms.PlatformExistsFromShort(platform))
                AppSettings.EnabledBasicPlatforms.Remove(platform);
            else
                AppSettings.DisabledPlatforms.Add(platform);

            if (AppSettings.DisabledPlatforms.Contains(""))
                AppSettings.DisabledPlatforms.Remove("");

            AppSettings.SaveSettings();
            AppSettings.PlatformListNotifyDataChanged();
        }
        #endregion

        #region Clipboard
        [JSInvokable]
        public static async Task CopyText(string text) => await ClipboardService.SetTextAsync(text);

        #endregion
    }
}
