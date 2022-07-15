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
using Microsoft.JSInterop;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Pages.Basic;
using TcNo_Acc_Switcher_Server.Pages.General;
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
                await ShowModal($"confirm:AcceptForget{AppData.CurrentSwitcher}Acc:{AppData.SelectedAccountId}");
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
                    Steam.Accounts.Remove(Steam.Accounts.First(x => x.AccountId == AppData.SelectedAccountId));
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
                    Basic.Accounts.Remove(Basic.Accounts.First(x => x.AccountId == AppData.SelectedAccountId));
                }

                // Remove from Tray
                Globals.RemoveTrayUserByArg(AppData.CurrentSwitcher, trayAcc);

                // Remove image
                Globals.DeleteFile(Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\{AppData.CurrentSwitcher}\\{Globals.GetCleanFilePath(AppData.SelectedAccountId)}.jpg"));

                AppData.Instance.NotifyDataChanged();

                await GeneralInvocableFuncs.ShowToast("success", Lang.Instance["Success"], renderTo: "toastarea");
            }
        }

        #endregion

        #region Clipboard
        [JSInvokable]
        public static async Task CopyText(string text) => await ClipboardService.SetTextAsync(text);

        #endregion
    }
}
