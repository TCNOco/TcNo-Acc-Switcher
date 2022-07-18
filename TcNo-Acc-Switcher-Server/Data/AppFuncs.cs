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
using TcNo_Acc_Switcher_Server.Data.Settings;
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
        private readonly ILang _lang;
        private readonly IAppSettings _appSettings;
        private readonly IGeneralFuncs _generalFuncs;
        private readonly IAppData _appData;
        private readonly Lazy<ISteam> _lSteam;
        private ISteam Steam => _lSteam.Value;
        private readonly Lazy<IBasic> _lBasic;
        private IBasic Basic => _lBasic.Value;
        private readonly IModalData _modalData;

        public AppFuncs(ILang lang, IAppSettings appSettings, IGeneralFuncs generalFuncs, IAppData appData, IModalData modalData, Lazy<ISteam> steam, Lazy<IBasic> basic)
        {
            _lang = lang;
            _appSettings = appSettings;
            _generalFuncs = generalFuncs;
            _appData = appData;
            _modalData = modalData;
            _lSteam = steam;
            _lBasic = basic;
        }

        #region Account Management
        /// <summary>
        /// Swap to the current AppData.SelectedAccountId.
        /// </summary>
        /// <param name="state">Optional profile state for Steam accounts</param>
        public async Task SwapToAccount(int state = -1)
        {
            if (!OperatingSystem.IsWindows()) return;

            if (_appData.CurrentSwitcher != "Steam") Basic.SwapBasicAccounts(_appData.SelectedAccountId);

            if (state == -1) state = Steam.OverrideState;
            await Steam.SwapSteamAccounts(_appData.SelectedAccountId, state);
        }

        /// <summary>
        /// Swaps to an empty account, allowing the user to sign in.
        /// </summary>
        /// <param name="state">Optional profile state for Steam accounts</param>
        public async Task SwapToNewAccount(int state = -1)
        {
            if (!OperatingSystem.IsWindows()) return;

            if (_appData.CurrentSwitcher == "Steam") Basic.SwapBasicAccounts();

            if (state == -1) state = Steam.OverrideState;
            await Steam.SwapSteamAccounts("", state);
        }

        public async Task ForgetAccount()
        {
            var skipConfirm = _appData.CurrentSwitcher == "Steam" ? Steam.ForgetAccountEnabled : Basic.ForgetAccountEnabled;
            if (!skipConfirm)
                _modalData.ShowModal("confirm", ExtraArg.ForgetAccount);
            else
            {
                var trayAcc = _appData.SelectedAccountId;
                if (_appData.CurrentSwitcher == "Steam")
                {
                    Steam.SetForgetAcc(true);

                    // Load and remove account that matches SteamID above.
                    var userAccounts = await Steam.GetSteamUsers(Steam.LoginUsersVdf());
                    _ = userAccounts.RemoveAll(x => x.SteamId == _appData.SelectedAccountId);

                    // Save updated loginusers.vdf file
                    await Steam.SaveSteamUsersIntoVdf(userAccounts);
                    trayAcc = "+s:" + _appData.SelectedAccountId;

                    // Remove from Steam accounts list
                    _appData.SteamAccounts.Remove(_appData.SteamAccounts.First(x => x.AccountId == _appData.SelectedAccountId));
                }
                else
                {
                    Basic.SetForgetAcc(true);

                    // Remove ID from list of ids
                    var idsFile = $"LoginCache\\{_appData.CurrentSwitcher}\\ids.json";
                    if (File.Exists(idsFile))
                    {
                        var allIds = _generalFuncs.ReadDict(idsFile).Remove(_appData.SelectedAccountId);
                        await File.WriteAllTextAsync(idsFile, JsonConvert.SerializeObject(allIds));
                    }

                    // Remove cached files
                    Globals.RecursiveDelete($"LoginCache\\{_appData.CurrentSwitcher}\\{_appData.SelectedAccountId}", false);

                    // Remove from Steam accounts list
                    _appData.BasicAccounts.Remove(_appData.BasicAccounts.First(x => x.AccountId == _appData.SelectedAccountId));
                }

                // Remove from Tray
                Globals.RemoveTrayUserByArg(_appData.CurrentSwitcher, trayAcc);

                // Remove image
                Globals.DeleteFile(Path.Join(_generalFuncs.WwwRoot(), $"\\img\\profiles\\{_appData.CurrentSwitcher}\\{Globals.GetCleanFilePath(_appData.SelectedAccountId)}.jpg"));

                _appData.NotifyDataChanged();

                await _generalFuncs.ShowToast("success", _lang["Success"], renderTo: "toastarea");
            }
        }

        public void LoadNotes()
        {
            var filePath = _appData.CurrentSwitcher == "Steam"
                ? "LoginCache\\Steam\\AccountNotes.json"
                : $"LoginCache\\{_appData.CurrentSwitcherSafe}\\AccountNotes.json";
            if (!File.Exists(filePath)) return;

            var loaded = JsonConvert.DeserializeObject<Dictionary<string, string>>(File.ReadAllText(filePath));
            if (loaded is null) return;

            if (_appData.CurrentSwitcher == "Steam")
                foreach (var (key, val) in loaded)
                {
                    var acc = _appData.SteamAccounts.FirstOrDefault(x => x.AccountId == key);
                    if (acc is null) return;
                    acc.Note = val;
                }
            else
                foreach (var (key, val) in loaded)
                {
                    var acc = _appData.BasicAccounts.FirstOrDefault(x => x.AccountId == key);
                    if (acc is null) return;
                    acc.Note = val;
                }

            _modalData.IsShown = false;
        }

        public async Task ExportAllAccounts()
        {
            if (_appData.IsCurrentlyExportingAccounts)
            {
                await _generalFuncs.ShowToast("error", _lang["Toast_AlreadyProcessing"], _lang["Error"], "toastarea");
                return;
            }

            _appData.IsCurrentlyExportingAccounts = true;

            //AppData.SelectedPlatform
            var exportPath = await _generalFuncs.ExportAccountList();
            await _appData.InvokeVoidAsync("saveFile", exportPath.Split('\\').Last(), exportPath);
            _appData.IsCurrentlyExportingAccounts = false;
        }

        /// <summary>
        /// Hide a platform from the platforms list. Not giving an item input will use the AppData.SelectedPlatform.
        /// </summary>
        public void HidePlatform(string item = null)
        {
            var platform = item ?? _appData.SelectedPlatform;
            _appSettings.Platforms.First(x => x.Name == platform).SetEnabled(false);
            _appSettings.SaveSettings();
        }
        #endregion

        #region Clipboard
        [JSInvokable]
        public async Task CopyText(string text) => await ClipboardService.SetTextAsync(text);

        #endregion
    }
}
