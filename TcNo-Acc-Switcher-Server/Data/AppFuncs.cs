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
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Classes;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Pages.Basic;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using TextCopy;

namespace TcNo_Acc_Switcher_Server.Data
{
    public interface IAppFuncs
    {
        /// <summary>
        /// Swap to the current AppData.SelectedAccountId.
        /// </summary>
        /// <param name="state">Optional profile state for Steam accounts</param>
        Task SwapToAccount(int state = -1);

        /// <summary>
        /// Swaps to an empty account, allowing the user to sign in.
        /// </summary>
        /// <param name="state">Optional profile state for Steam accounts</param>
        Task SwapToNewAccount(int state = -1);

        Task ForgetAccount();
        void LoadNotes();
        Task ExportAllAccounts();

        /// <summary>
        /// Hide a platform from the platforms list. Not giving an item input will use the AppData.SelectedPlatform.
        /// </summary>
        void HidePlatform(string item = null);
        Task CopyText(string text);
    }

    public class AppFuncs : IAppFuncs
    {
        [Inject] private ILang Lang { get; }
        [Inject] private IAppSettings AppSettings { get; }
        [Inject] private IGeneralFuncs GeneralFuncs { get; }
        [Inject] private IAppData AppData { get; }
        [Inject] private ISteam Steam { get; }
        [Inject] private IBasic Basic { get; }
        [Inject] private IModalData ModalData { get; }

        #region Account Management
        /// <summary>
        /// Swap to the current AppData.SelectedAccountId.
        /// </summary>
        /// <param name="state">Optional profile state for Steam accounts</param>
        public async Task SwapToAccount(int state = -1)
        {
            if (!OperatingSystem.IsWindows()) return;

            if (AppData.CurrentSwitcher != "Steam") Basic.SwapBasicAccounts(AppData.SelectedAccountId);

            if (state == -1) state = Steam.OverrideState;
            await Steam.SwapSteamAccounts(AppData.SelectedAccountId, state);
        }

        /// <summary>
        /// Swaps to an empty account, allowing the user to sign in.
        /// </summary>
        /// <param name="state">Optional profile state for Steam accounts</param>
        public async Task SwapToNewAccount(int state = -1)
        {
            if (!OperatingSystem.IsWindows()) return;

            if (AppData.CurrentSwitcher == "Steam") Basic.SwapBasicAccounts();

            if (state == -1) state = Steam.OverrideState;
            await Steam.SwapSteamAccounts("", state);
        }

        public async Task ForgetAccount()
        {
            var skipConfirm = AppData.CurrentSwitcher == "Steam" ? Steam.ForgetAccountEnabled : Basic.ForgetAccountEnabled;
            if (!skipConfirm)
                ModalData.ShowModal("confirm", ExtraArg.ForgetAccount);
            else
            {
                var trayAcc = AppData.SelectedAccountId;
                if (AppData.CurrentSwitcher == "Steam")
                {
                    Steam.SetForgetAcc(true);

                    // Load and remove account that matches SteamID above.
                    var userAccounts = await Steam.GetSteamUsers(Steam.LoginUsersVdf());
                    _ = userAccounts.RemoveAll(x => x.SteamId == AppData.SelectedAccountId);

                    // Save updated loginusers.vdf file
                    await Steam.SaveSteamUsersIntoVdf(userAccounts);
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

                AppData.NotifyDataChanged();

                await GeneralFuncs.ShowToast("success", Lang["Success"], renderTo: "toastarea");
            }
        }

        public void LoadNotes()
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

        public async Task ExportAllAccounts()
        {
            if (AppData.IsCurrentlyExportingAccounts)
            {
                await GeneralFuncs.ShowToast("error", Lang["Toast_AlreadyProcessing"], Lang["Error"], "toastarea");
                return;
            }

            AppData.IsCurrentlyExportingAccounts = true;

            //AppData.SelectedPlatform
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
            AppSettings.SaveSettings();
        }
        #endregion

        #region Clipboard
        [JSInvokable]
        public async Task CopyText(string text) => await ClipboardService.SetTextAsync(text);

        #endregion
    }
}
