// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2021 TechNobo (Wesley Pyburn)
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
using System.Net;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.BattleNet
{
    public class BattleNetSwitcherFuncs
    {
        private static readonly Data.Settings.BattleNet BattleNet = Data.Settings.BattleNet.Instance;
        private static string _battleNetRoaming;

        /// <summary>
        /// Main function for Battle.net Account Switcher. Run on load.
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        public static async Task LoadProfiles()
        {
            Globals.DebugWriteLine(@"[Func:BattleNet\BattleNetSwitcherFuncs.LoadProfiles] Loading BattleNet profiles");
            LoadImportantData();
            BattleNet.LoadAccounts();
            // Check if accounts file exists
            if (!File.Exists(_battleNetRoaming + "\\Battle.net.config"))
            {
                _ = GeneralInvocableFuncs.ShowToast("error", "Could not find Battle.net.config", "toastarea");
                return;
            }

            // Read lines in accounts file
            var file = await File.ReadAllTextAsync(_battleNetRoaming + "\\Battle.net.config").ConfigureAwait(false);
            if (JsonConvert.DeserializeObject(file) is not JObject accountsFile)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", "Could not load accounts file for Blizzard (Battle.net.config file corrupt)", "toastarea");
                return;
            }

            // Verify that there are accounts to iterate over
            var savedAccountsList = accountsFile.SelectToken("Client.SavedAccountNames");
            if (savedAccountsList == null)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", "Could not load accounts file for Blizzard (No accounts found)", "toastarea");
                return;
            }

            foreach (var mail in savedAccountsList.ToString().Split(','))
            {
	            if (string.IsNullOrEmpty(mail) || string.IsNullOrWhiteSpace(mail)) continue; // Ignores blank emails sometimes added: ".com, , asdf@..." 
                try
                {
                    if (BattleNet.Accounts.Count == 0 || BattleNet.Accounts.All(x => x.Email != mail) && !BattleNet.IgnoredAccounts.Contains(mail) && mail != " ")
                        BattleNet.Accounts.Add(new BattleNetSwitcherBase.BattleNetUser { Email = mail });
                }
                catch (NullReferenceException)
                {
                    BattleNet.Accounts.Add(new BattleNetSwitcherBase.BattleNetUser { Email = mail });
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
                        foreach (var acc in from i in savedOrder where BattleNet.Accounts.Any(x => x.Email == i) select BattleNet.Accounts.Single(x => x.Email == i))
                        {
                            BattleNet.Accounts.Remove(acc);
                            BattleNet.Accounts.Insert(index, acc);
                            index++;
                        }
                }
            }

            foreach (var acc in BattleNet.Accounts)
            {
                if (!File.Exists(Path.Join(BattleNet.ImagePath, $"{acc.Email}.png"))) DownloadImage(acc.Email);
                var username = acc.BTag == null ? acc.Email : acc.BTag.Contains("#") ? acc.BTag.Split("#")[0] : acc.BTag;
                var element =
                    $"<div class=\"acc_list_item\"><input type=\"radio\" id=\"{acc.Email}\" Username=\"{username}\" DisplayName=\"{username}\" class=\"acc\" name=\"accounts\" onchange=\"selectedItemChanged()\" />\r\n" +
                    $"<label for=\"{acc.Email}\" class=\"acc\">\r\n" +
                    $"<img src=\"img\\profiles\\battlenet\\{acc.Email}.png?{Globals.GetUnixTime()}\" draggable=\"false\" />\r\n" +
                    $"<h6>{GeneralFuncs.EscapeText(username)}</h6>\r\n";
                if (BattleNet.OverwatchMode && DateTime.Now - acc.LastTimeChecked < TimeSpan.FromDays(1))
                {
                    if (acc.OwTankSr != 0)
                    {
                        element += $"<h6 class=\"battlenetIcoOWTank\"><svg viewBox=\"0 0 60.325 60.325\" draggable=\"false\" class=\"battleNetIcon battlenetIcoOWTank\"><use href=\"img/icons/ico_BattleNetTankIcon.svg#icoBattleNetTank\"></use></svg> {acc.OwTankSr}<sup>SR</sup></h6>\r\n";
                    }
                    if (acc.OwDpsSr != 0)
                    {
                        element += $"<h6 class=\"battlenetIcoOWDamage\"><svg viewBox=\"0 0 60.325 60.325\" draggable=\"false\" class=\"battleNetIcon battlenetIcoOWDamage\"><use href=\"img/icons/ico_BattleNetDamageIcon.svg#icoBattleNetDamage\"></use></svg> {acc.OwDpsSr}<sup>SR</sup></h6>\r\n";
                    }
                    if (acc.OwSupportSr != 0)
                    {
                        element += $"<h6 class=\"battlenetIcoOWSupport\"><svg viewBox=\"0 0 60.325 60.325\" draggable=\"false\" class=\"battleNetIcon battlenetIcoOWSupport\"><use href=\"img/icons/ico_BattleNetSupportIcon.svg#icoBattleNetSupport\"></use></svg> {acc.OwSupportSr}<sup>SR</sup></h6>\r\n";
                    }
                }
                //$"<p>{UnixTimeStampToDateTime(ua.LastLogin)}</p>\r\n</label>";  TODO: Add some sort of "Last logged in" json file
                AppData.InvokeVoidAsync("jQueryAppend", "#acc_list", element + "</div>");
            }

            GenericFunctions.FinaliseAccountList(); // Init context menu & Sorting
            if(BattleNet.OverwatchMode)
                await InitOverwatchMode();
        }


        /// <summary>
        /// Run necessary functions and load data when being launcher without a GUI (From command line for example).
        /// </summary>
        private static void LoadImportantData()
        {
            _battleNetRoaming = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Battle.net");
        }

        public static async Task InitOverwatchMode()
        {
            if (!BattleNet.OverwatchMode) return;
            var accountFetched = false;
            foreach (var acc in BattleNet.Accounts.Where(x => x.BTag != null))
            {
                if (DateTime.Now - acc.LastTimeChecked <= TimeSpan.FromDays(1)) continue;
                accountFetched = acc.FetchRank();
            }

            if (!accountFetched) return;
            await AppData.ReloadPage();
            BattleNet.SaveAccounts();
        }

        public static string DownloadImage(string bTag, string imgUrl = "")
        {
            var dlDir = Path.Join(BattleNet.ImagePath, $"{bTag}.png");
            Directory.CreateDirectory(BattleNet.ImagePath);
            if (imgUrl == "")
            {
                File.Copy("wwwroot\\img\\BattleNetDefault.png", dlDir);
                return dlDir;
            }

            if (File.Exists(dlDir))
            {
                if (GeneralFuncs.GetFileMd5(dlDir) == GeneralFuncs.GetFileMd5("wwwroot\\img\\BattleNetDefault.png")) File.Delete(dlDir);
                GeneralFuncs.DeletedOutdatedFile(dlDir, BattleNet.ImageExpiryTime);
                GeneralFuncs.DeletedInvalidImage(dlDir);
            }
            // Download new copy of the file
            if (File.Exists(dlDir)) return dlDir;
            try
            {
                using var client = new WebClient();
                client.DownloadFile(new Uri(imgUrl), dlDir);
            }
            catch (WebException ex)
            {
                File.Copy("wwwroot\\img\\BattleNetDefault.png", dlDir);
                Globals.WriteToLog("ERROR: Could not connect and download BattleNet profile's image from Steam servers.\nCheck your internet connection.\n\nDetails: " + ex);
            }

            return dlDir;
        }

        /// <summary>
        /// Restart Battle.net with a new account selected.
        /// </summary>
        /// <param name="email">User's account email</param>
        public static async Task SwapBattleNetAccounts(string email)
        {
            Globals.DebugWriteLine(@"[Func:BattleNet\BattleNetSwitcherFuncs.SwapBattleNetAccounts] Swapping to: hidden.");
            LoadImportantData();
            if (BattleNet.Accounts.Count == 0) BattleNet.LoadAccounts();

            AppData.InvokeVoidAsync("updateStatus", "Starting BattleNet");
            if (!CloseBattleNet()) return;
            
            // Load settings into JObject
            var file = await File.ReadAllTextAsync(_battleNetRoaming + "\\Battle.net.config").ConfigureAwait(false);
            if (JsonConvert.DeserializeObject(file) is not JObject jObject)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", "Could not swap accounts (Battle.net.config file corrupt)", "toastarea");
                return;
            }


            // Select the JToken with the Account Emails
            var jToken = jObject.SelectToken("Client.SavedAccountNames");
            BattleNetSwitcherBase.BattleNetUser account;
            if (email != "") // New account
            {
                account = BattleNet.Accounts.First(x => x.Email == email);
                // Set the to be logged in Account to idx 0
                BattleNet.Accounts.Remove(account);
                BattleNet.Accounts.Insert(0, account);

                Globals.AddTrayUser("BattleNet", "+b:" + email, account.BTag ?? email, BattleNet.TrayAccNumber); // Add to Tray list
            }
            else
            {
                account = new BattleNetSwitcherBase.BattleNetUser {Email = " "};
                BattleNet.Accounts.Remove(account);
                BattleNet.Accounts.Insert(0, account);
            }

            // Build the string with the Emails with the Email that's should get logged in at first
            var replaceString = "";
            for (var i = 0; i < BattleNet.Accounts.Count; i++)
            {
                replaceString += BattleNet.Accounts[i].Email;
                if (i < BattleNet.Accounts.Count - 1)
                {
                    replaceString += ",";
                }
            }

            if (account.Email == " ")
            {
                BattleNet.Accounts.Remove(account);
            }
            
            // Replace and write the new Json
            jToken?.Replace(replaceString);
            await File.WriteAllTextAsync(_battleNetRoaming + "\\Battle.net.config", jObject.ToString());

            GeneralFuncs.StartProgram(BattleNet.Exe(), BattleNet.Admin);

            Globals.RefreshTrayArea();
        }

        /// <summary>
        /// Used in JS. Gets whether forget account is enabled (Whether to NOT show prompt, or show it).
        /// </summary>
        /// <returns></returns>
        [JSInvokable]
        public static Task<bool> GetBattleNetForgetAcc() => Task.FromResult(BattleNet.ForgetAccountEnabled);

        /// <summary>
        /// Kills Battle.net processes when run via cmd.exe
        /// </summary>
        public static bool CloseBattleNet()
        {
            Globals.DebugWriteLine(@"[Func:BattleNet\BattleNetSwitcherFuncs.CloseBattleNet]");
            if (!GeneralFuncs.CanKillProcess("Battle.net")) return false;
            Globals.KillProcess("Battle.net");
            return GeneralFuncs.WaitForClose("Battle.net");
        }

        /// <summary>
        /// Changes an accounts name on the TcNo Account Switcher
        /// </summary>
        /// <param name="email">BattleNet email address</param>
        /// <param name="bTag">New name for user</param>
        public static void ChangeBTag(string email, string bTag)
        {
            if (ValidateBTag(bTag))
            {
                BattleNet.Accounts.First(x => x.Email == email).BTag = bTag;
                BattleNet.Accounts.First(x => x.Email == email).LastTimeChecked = new DateTime();
                BattleNet.SaveAccounts();
                Globals.DebugWriteLine(@"[Func:BattleNet\BattleNetSwitcherFuncs.SetBattleTag] accName:hidden, bTag:hidden");
                AppData.ActiveNavMan?.NavigateTo( "/BattleNet/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Changed BattleTag"), true);
            }
            else
                _ = GeneralInvocableFuncs.ShowToast("error", "BattleTag did not match naming policy.");
        }

        public static bool ValidateBTag(string bTag)
        {
            var parts = bTag.Split('#');
            if (parts.Length != 2) return false; // Checks has 2 parts.
            // Checks BTag number length & Checks if BTag numbers part is just numbers
            return parts[1].Length is >= 4 and <= 7 && IntPtr.TryParse(parts[1], out _);
        }

        /// <summary>
        /// Deletes the BattleTag of the Account that belongs to the email and saves it;
        /// </summary>
        /// <param name="email">The email of the account</param>
        [JSInvokable]
        public static void DeleteUsername(string email)
        {
            var account = BattleNet.Accounts.First(x => x.Email == email);
            account.BTag = null;
            account.ImgUrl = null;
            account.LastTimeChecked = new DateTime();
            account.OwDpsSr = 0;
            account.OwSupportSr = 0;
            account.OwTankSr = 0;
            Globals.DebugWriteLine(@"[Func:BattleNet\BattleNetSwitcherFuncs.DeleteBattleTag] accName:hidden");
            BattleNet.SaveAccounts();
            AppData.ActiveNavMan?.NavigateTo("/BattleNet/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Deleted BattleTag"), true);
        }
        
        /// <summary>
        /// Refetch the rank of the account
        /// </summary>
        /// <param name="email">The email of the account</param>
        [JSInvokable]
        public static void RefetchRank(string email)
        {
            Globals.DebugWriteLine(@"[Func:BattleNet\BattleNetSwitcherFuncs.DeleteBattleTag] accName:hidden");
            if (BattleNet.Accounts.First(x => x.Email == email).FetchRank()) AppData.ActiveNavMan?.NavigateTo("/BattleNet/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Fetched Rank"), true);
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
            var account = BattleNet.Accounts.Find(x => x.Email == accName);
            if (account == null) return;
            // Remove image
            var img = Path.Join(BattleNet.ImagePath, $"{account.BTag ?? account.Email}.png");
            if (File.Exists(img)) File.Delete(img);
            // Remove from Tray
            Globals.RemoveTrayUser("BattleNet", account.BTag ?? account.Email); // Add to Tray list
            // Remove from accounts list
            BattleNet.Accounts.Remove(account);
            BattleNet.IgnoredAccounts.Add(account.Email);
            BattleNet.SaveAccounts();

            AppData.ActiveNavMan?.NavigateTo("/BattleNet/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Forgot account"), true);
        }

        /// <summary>
        /// Fires after being confirmed by above function, and actually performs task.
        /// </summary>
        public static void ClearIgnored_Confirmed()
        {
            Globals.DebugWriteLine(@"[Func:BattleNet\BattleNetSwitcherFuncs.ClearForgotten_Confirmed] Confirmation received to clear ignored list.");
            BattleNet.IgnoredAccounts = new List<string>();
            BattleNet.SaveAccounts();
        }

        /// <summary>
        /// Restores a list of accounts, by removing them from the ignore list.
        /// </summary>
        /// <param name="requestedAccs">Array of account emails to remove from ignore list</param>
        /// <returns>Whether it was successful or not</returns>
        public static bool RestoreSelected(string[] requestedAccs)
        {
            if (!File.Exists(BattleNet.IgnoredAccPath)) return false;
            if(BattleNet.IgnoredAccounts.Count == 0) BattleNet.LoadAccounts();
            foreach (var s in requestedAccs)
            {
                Globals.DebugWriteLine(@"[Func:Steam\SteamSwitcherFuncs.RestoreAccounts] Restoring account: hidden");
                if (BattleNet.IgnoredAccounts.Contains(s))
	                BattleNet.IgnoredAccounts.Remove(s);
            }
            BattleNet.SaveAccounts();
            return true;
        }
        #endregion

    }
}
