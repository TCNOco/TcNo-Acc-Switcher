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
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Pages.Basic;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using TextCopy;

namespace TcNo_Acc_Switcher_Server.Data
{
    public class AppFuncs
    {

        #region Account Management
        /// <summary>
        /// Swap to the current AppData.SelectedAccountId.
        /// </summary>
        public void SwapToAccount()
        {
            BasicSwitcherFuncs.SwapBasicAccounts(AppData.SelectedAccountId);
        }

        /// <summary>
        /// Swaps to an empty account, allowing the user to sign in.
        /// </summary>
        public void SwapToNewAccount()
        {
            if (!OperatingSystem.IsWindows()) return;
            BasicSwitcherFuncs.SwapBasicAccounts("");
        }

        public async Task ForgetAccount()
        {
            if (!Basic.ForgetAccountEnabled)
                ModalData.ShowModal("confirm", ModalData.ExtraArg.ForgetAccount);
            else
            {
                var trayAcc = AppData.SelectedAccountId;
                Basic.SetForgetAcc(true);

                // Remove ID from list of ids
                var idsFile = $"LoginCache\\{AppData.CurrentSwitcher}\\ids.json";
                if (File.Exists(idsFile))
                {
                    var allIds = Globals.ReadDict(idsFile).Remove(AppData.SelectedAccountId);
                    await File.WriteAllTextAsync(idsFile, JsonConvert.SerializeObject(allIds));
                }

                // Remove cached files
                Globals.RecursiveDelete($"LoginCache\\{AppData.CurrentSwitcher}\\{AppData.SelectedAccountId}", false);

                // Remove from Steam accounts list
                AppData.BasicAccounts.Remove(AppData.BasicAccounts.First(x => x.AccountId == AppData.SelectedAccountId));

                // Remove from Tray
                Globals.RemoveTrayUserByArg(AppData.CurrentSwitcher, trayAcc);

                // Remove image
                Globals.DeleteFile(Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\{AppData.CurrentSwitcher}\\{Globals.GetCleanFilePath(AppData.SelectedAccountId)}.jpg"));

                AppData.Instance.NotifyDataChanged();

                AppData.Instance.ShowToastLang(ToastType.Success, "Success");
            }
        }

        public static void LoadNotes()
        {
            const string filePath = $"LoginCache\\{AppData.CurrentSwitcherSafe}\\AccountNotes.json";
            if (!File.Exists(filePath)) return;

            var loaded = JsonConvert.DeserializeObject<Dictionary<string, string>>(File.ReadAllText(filePath));
            if (loaded is null) return;

            foreach (var (key, val) in loaded)
            {
                var acc = AppData.BasicAccounts.FirstOrDefault(x => x.AccountId == key);
                if (acc is null) return;
                acc.Note = val;
            }

            ModalData.IsShown = false;
        }
        #endregion

        #region Clipboard
        [JSInvokable]
        public static async Task CopyText(string text) => await ClipboardService.SetTextAsync(text);

        #endregion
    }
}
