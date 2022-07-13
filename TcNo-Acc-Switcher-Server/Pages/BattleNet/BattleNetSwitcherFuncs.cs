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

// Special thanks to iR3turnZ for contributing to this platform's account switcher
// iR3turnZ: https://github.com/HoeblingerDaniel

using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Shared.Accounts;
using BattleNetSettings = TcNo_Acc_Switcher_Server.Data.Settings.BattleNet;

namespace TcNo_Acc_Switcher_Server.Pages.BattleNet
{
    public class BattleNetSwitcherFuncs
    {
        private static readonly Lang Lang = Lang.Instance;

        private static string _battleNetRoaming;

        /// <summary>
        /// Main function for Battle.net Account Switcher. Run on load.
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        public static async Task LoadProfiles()
        {
            Globals.DebugWriteLine(@"[Func:BattleNet\BattleNetSwitcherFuncs.LoadProfiles] Loading BattleNet profiles");

            LoadImportantData();
            BattleNetSettings.LoadAccounts();

            // Check if accounts file exists
            if (!File.Exists(_battleNetRoaming + "\\Battle.net.config"))
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_BNet_CantLoadNotFound"], "toastarea");
                return;
            }

            // Read lines in accounts file
            var file = await File.ReadAllTextAsync(_battleNetRoaming + "\\Battle.net.config").ConfigureAwait(false);
            if (JsonConvert.DeserializeObject(file) is not JObject accountsFile)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_BNet_CantLoadConfigCorrupt"], "toastarea");
                return;
            }

            // Verify that there are accounts to iterate over
            AppData.BNetAccountsList = accountsFile.SelectToken("Client.SavedAccountNames");
            if (AppData.BNetAccountsList == null)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_BNet_CantLoadEmpty"], "toastarea");
                return;
            }

            foreach (var mail in AppData.BNetAccountsList.ToString().Split(','))
            {
                if (string.IsNullOrEmpty(mail) || string.IsNullOrWhiteSpace(mail)) continue; // Ignores blank emails sometimes added: ".com, , asdf@..."
                try
                {
                    if (BattleNetSettings.BNetAccounts.Count == 0 || BattleNetSettings.BNetAccounts.All(x => x.Email != mail) && !BattleNetSettings.IgnoredAccounts.Contains(mail) && mail != " ")
                        BattleNetSettings.BNetAccounts.Add(new BattleNetSwitcherBase.BattleNetUser { Email = mail });
                }
                catch (NullReferenceException)
                {
                    BattleNetSettings.BNetAccounts.Add(new BattleNetSwitcherBase.BattleNetUser { Email = mail });
                }
            }

            // Order
            if (File.Exists("LoginCache\\BattleNet\\order.json"))
            {
                var savedOrder = JsonConvert.DeserializeObject<List<string>>(await File.ReadAllTextAsync("LoginCache\\BattleNet\\order.json").ConfigureAwait(false));
                if (savedOrder != null)
                {
                    var index = 0;
                    if (savedOrder is { Count: > 0 })
                        foreach (var acc in from i in savedOrder where BattleNetSettings.BNetAccounts.Any(x => x.Email == i) select BattleNetSettings.BNetAccounts.Single(x => x.Email == i))
                        {
                            _ = BattleNetSettings.BNetAccounts.Remove(acc);
                            BattleNetSettings.BNetAccounts.Insert(Math.Min(index, BattleNetSettings.BNetAccounts.Count), acc);
                            index++;
                        }
                }
            }

            BattleNetSettings.Accounts.Clear();

            foreach (var acc in BattleNetSettings.BNetAccounts)
            {
                if (!File.Exists(Path.Join(BattleNetSettings.ImagePath, $"{acc.Email}.jpg"))) _ = DownloadImage(acc.Email);
                var username = acc.Email;

                // Temporary: Rename old BattleNet PFPs from png to jpg for new scheme:
                var imagePath = Path.Join(GeneralFuncs.WwwRoot(), $"img\\profiles\\battlenet\\{acc.Email}");
                if (File.Exists(imagePath + ".png"))
                {
                    Globals.CopyFile(imagePath + ".png", imagePath + ".jpg");
                    Globals.DeleteFile(imagePath + ".png");
                }

                // Prepare account
                var account = new Account
                {
                    Platform = "BattleNet",
                    AccountId = acc.Email,
                    DisplayName = GeneralFuncs.EscapeText(username),
                    ImagePath = $"img\\profiles\\battlenet\\{acc.Email}.jpg",
                    UserStats = BasicStats.GetUserStatsAllGamesMarkup("BattleNet", acc.Email)
                };
                BattleNetSettings.Accounts.Add(account);
            }

            GenericFunctions.FinaliseAccountList(); // Init context menu & Sorting

            AppStats.SetAccountCount("BattleNet", BattleNetSettings.BNetAccounts.Count);
            BattleNetSettings.SaveAccounts();
        }

        public static string GetCurrentAccountId()
        {
            // 30 second window - For when changing accounts
            if (BattleNetSettings.LastAccName != "" && BattleNetSettings.LastAccTimestamp - Globals.GetUnixTimeInt() < 30)
                return BattleNetSettings.LastAccName;

            try
            {
                return AppData.BNetAccountsList == null ? "" : AppData.BNetAccountsList.ToString().Split(',')[0];
            }
            catch (Exception)
            {
                //
            }

            return "";
        }

        // private static SavedAccountsList

        /// <summary>
        /// Run necessary functions and load data when being launcher without a GUI (From command line for example).
        /// </summary>
        private static void LoadImportantData()
        {
            _battleNetRoaming = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Battle.net");
        }

        public static string DownloadImage(string bTag, string imgUrl = "")
        {
            var dlDir = Path.Join(BattleNetSettings.ImagePath, $"{bTag}.jpg");
            _ = Directory.CreateDirectory(BattleNetSettings.ImagePath);
            if (imgUrl == "")
            {
                Globals.CopyFile("wwwroot\\img\\BattleNetDefault.png", dlDir);
                return dlDir;
            }

            if (File.Exists(dlDir))
            {
                if (GeneralFuncs.GetFileMd5(dlDir) == GeneralFuncs.GetFileMd5("wwwroot\\img\\BattleNetDefault.png")) Globals.DeleteFile(dlDir);
                _ = GeneralFuncs.DeletedOutdatedFile(dlDir, BattleNetSettings.ImageExpiryTime);
                _ = GeneralFuncs.DeletedInvalidImage(dlDir);
            }
            // Download new copy of the file
            if (File.Exists(dlDir)) return dlDir;
            try
            {
                Globals.DownloadFile(imgUrl, dlDir);
            }
            catch (Exception ex)
            {
                Globals.CopyFile("wwwroot\\img\\BattleNetDefault.png", dlDir);
                Globals.WriteToLog("ERROR: Could not connect and download BattleNet profile's image from Steam servers.\nCheck your internet connection.\n\nDetails: " + ex);
            }

            return dlDir;
        }

        /// <summary>
        /// Restart Battle.net with a new account selected.
        /// </summary>
        /// <param name="email">User's account email</param>
        /// <param name="args">Starting arguments</param>
        public static async Task SwapBattleNetAccounts(string email, string args = "")
        {
            Globals.DebugWriteLine(@"[Func:BattleNet\BattleNetSwitcherFuncs.SwapBattleNetAccounts] Swapping to: hidden.");
            LoadImportantData();
            if (BattleNetSettings.BNetAccounts.Count == 0) BattleNetSettings.LoadAccounts();
            BattleNetSettings.SaveAccounts();

            _ = AppData.InvokeVoidAsync("updateStatus", Lang["Status_ClosingPlatform", new { platform = "BattleNet" }]);
            if (!GeneralFuncs.CloseProcesses(BattleNetSettings.Processes, BattleNetSettings.ClosingMethod))
            {
                if (Globals.IsAdministrator)
                    _ = AppData.InvokeVoidAsync("updateStatus", Lang["Status_ClosingPlatformFailed", new { platform = "BattleNet" }]);
                else
                {
                    _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_RestartAsAdmin"], Lang["Failed"], "toastarea");
                    _ = GeneralInvocableFuncs.ShowModal("notice:RestartAsAdmin");
                }
                return;
            }

            // Load settings into JObject
            var file = await File.ReadAllTextAsync(_battleNetRoaming + "\\Battle.net.config").ConfigureAwait(false);
            if (JsonConvert.DeserializeObject(file) is not JObject jObject)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_BNet_CantSwapAccounts"], "toastarea");
                return;
            }


            // Select the JToken with the Account Emails
            var jToken = jObject.SelectToken("Client.SavedAccountNames");
            BattleNetSwitcherBase.BattleNetUser account;
            if (email != "") // New account
            {
                account = BattleNetSettings.BNetAccounts.First(x => x.Email == email);
                // Set the to be logged in Account to idx 0
                _ = BattleNetSettings.BNetAccounts.Remove(account);
                BattleNetSettings.BNetAccounts.Insert(0, account);

                Globals.AddTrayUser("BattleNet", "+b:" + email, email, BattleNetSettings.TrayAccNumber); // Add to Tray list
            }
            else
            {
                account = new BattleNetSwitcherBase.BattleNetUser { Email = " " };
                _ = BattleNetSettings.BNetAccounts.Remove(account);
                BattleNetSettings.BNetAccounts.Insert(0, account);
            }

            // Build the string with the Emails with the Email that's should get logged in at first
            var replaceString = "";
            for (var i = 0; i < BattleNetSettings.BNetAccounts.Count; i++)
            {
                replaceString += BattleNetSettings.BNetAccounts[i].Email;
                if (i < BattleNetSettings.BNetAccounts.Count - 1)
                {
                    replaceString += ",";
                }
            }

            if (account.Email == " ")
            {
                _ = BattleNetSettings.BNetAccounts.Remove(account);
            }

            // Replace and write the new Json
            jToken?.Replace(replaceString);
            await File.WriteAllTextAsync(_battleNetRoaming + "\\Battle.net.config", jObject.ToString());

            if (BattleNetSettings.AutoStart)
                Data.Settings.Basic.RunPlatform(BattleNetSettings.Exe(), BattleNetSettings.Admin, args, "BattleNet", BattleNetSettings.StartingMethod);

            if (BattleNetSettings.AutoStart && AppSettings.MinimizeOnSwitch) _ = AppData.InvokeVoidAsync("hideWindow");

            NativeFuncs.RefreshTrayArea();
            _ = AppData.InvokeVoidAsync("updateStatus", Lang["Done"]);
            AppStats.IncrementSwitches("BattleNet");

            try
            {
                BattleNetSettings.LastAccName = email;
                BattleNetSettings.LastAccTimestamp = Globals.GetUnixTimeInt();
                if (BattleNetSettings.LastAccName != "") _ = AppData.InvokeVoidAsync("highlightCurrentAccount", BattleNetSettings.LastAccName);
            }
            catch (Exception)
            {
                //
            }
        }

        /// <summary>
        /// Used in JS. Gets whether forget account is enabled (Whether to NOT show prompt, or show it).
        /// </summary>
        /// <returns></returns>
        [JSInvokable]
        public static Task<bool> GetBattleNetForgetAcc() => Task.FromResult(BattleNetSettings.ForgetAccountEnabled);

        [JSInvokable]
        public static Task<string> GetBattleNetNotes(string id) => Task.FromResult(BattleNetSettings.AccountNotes.GetValueOrDefault(id) ?? "");

        [JSInvokable]
        public static void SetBattleNetNotes(string id, string note)
        {
            BattleNetSettings.AccountNotes[id] = note;
            BattleNetSettings.SaveSettings();
        }

        #region FORGETTING_ACCOUNTS

        /// <summary>
        /// Adds an account to the ignore list, so it won't show on the account switcher list anymore.
        /// </summary>
        /// <param name="accName">Email address of Battlenet account to ignore.</param>
        public static void ForgetAccount(string accName)
        {
            Globals.DebugWriteLine(@"[Func:BattleNet\BattleNetSwitcherFuncs.ForgetAccount] accName:hidden");
            // Get user account
            var account = BattleNetSettings.BNetAccounts.Find(x => x.Email == accName);
            if (account == null) return;
            // Remove image
            var img = Path.Join(BattleNetSettings.ImagePath, $"{account.Email}.jpg");
            Globals.DeleteFile(img);
            // Remove from Tray
            Globals.RemoveTrayUser("BattleNet", account.Email); // Add to Tray list
            // Remove from accounts list
            _ = BattleNetSettings.BNetAccounts.Remove(account);
            BattleNetSettings.IgnoredAccounts.Add(account.Email);
            BattleNetSettings.SaveAccounts();

            AppData.ActiveNavMan?.NavigateTo(
                $"/BattleNet/?cacheReload&toast_type=success&toast_title={Uri.EscapeDataString(Lang["Success"])}&toast_message={Uri.EscapeDataString(Lang["Toast_ForgotAccount"])}", true);
        }

        /// <summary>
        /// Fires after being confirmed by above function, and actually performs task.
        /// </summary>
        public static void ClearIgnored_Confirmed()
        {
            Globals.DebugWriteLine(@"[Func:BattleNet\BattleNetSwitcherFuncs.ClearIgnored_Confirmed] Confirmation received to clear ignored list.");
            BattleNetSettings.IgnoredAccounts = new List<string>();
            BattleNetSettings.SaveAccounts();
        }

        /// <summary>
        /// Restores a list of accounts, by removing them from the ignore list.
        /// </summary>
        /// <param name="requestedAccs">Array of account emails to remove from ignore list</param>
        /// <returns>Whether it was successful or not</returns>
        public static bool RestoreSelected(string[] requestedAccs)
        {
            if (!File.Exists(BattleNetSettings.IgnoredAccPath)) return false;
            if (BattleNetSettings.IgnoredAccounts.Count == 0) BattleNetSettings.LoadAccounts();
            foreach (var s in requestedAccs)
            {
                Globals.DebugWriteLine(@"[Func:Steam\SteamSwitcherFuncs.RestoreAccounts] Restoring account: hidden");
                if (BattleNetSettings.IgnoredAccounts.Contains(s))
                    _ = BattleNetSettings.IgnoredAccounts.Remove(s);
            }
            BattleNetSettings.SaveAccounts();
            return true;
        }
        #endregion

    }
}
