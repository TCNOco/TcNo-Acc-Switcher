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
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using System.Xml;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.BattleNet;
using Formatting = System.Xml.Formatting;

namespace TcNo_Acc_Switcher_Server.Pages.BattleNet
{
    public class BattleNetSwitcherFuncs
    {
        private static readonly Data.Settings.BattleNet BattleNet = Data.Settings.BattleNet.Instance;
        private static string _battleNetRoaming;
        private static int IconSize = 15;
        private static string DamageIcon = 
            $"<svg width=\"{IconSize}\" height=\"{IconSize}\" version=\"1.1\" viewBox=\"0 0 60.325 60.325\" xmlns=\"http://www.w3.org/2000/svg\"><path d=\"m36.451 58.73v-9.6006h-12.577v9.6006zm0-12.997v-34.224c0-5.2977-5.0459-9.8967-6.2886-9.8967s-6.2886 4.599-6.2886 9.8967v34.224zm18.777 12.997v-9.6006h-12.577v9.6006zm0-12.997v-34.224c0-5.2977-5.0459-9.8967-6.2886-9.8967s-6.2886 4.599-6.2886 9.8967v34.224zm-37.553 12.997v-9.6006h-12.577v9.6006zm0-12.997v-34.224c0-5.2977-5.0459-9.8967-6.2886-9.8967s-6.2886 4.599-6.2886 9.8967v34.224z\"/></svg>";
        private static string TankIcon =
            $"<svg width=\"{IconSize}\" height=\"{IconSize}\" version=\"1.1\" viewBox=\"0 0 60.325 60.325\" xmlns=\"http://www.w3.org/2000/svg\"><path d=\"m5.4398 34.59v-24.069c0-3.8588 8.0447-8.9157 24.723-8.9157 16.678 0 24.723 5.0568 24.723 8.9157v24.069c0 5.821-19.817 24.133-24.723 24.133-4.9053 0-24.723-18.312-24.723-24.133z\"/></svg>";
        private static string SupportIcon = 
            $"<svg width=\"{IconSize}\" height=\"{IconSize}\" version=\"1.1\" viewBox=\"0 0 60.325 60.325\" xmlns=\"http://www.w3.org/2000/svg\"><path d=\"m40.777 54.38c0 1.8962-1.6187 4.3473-3.6536 4.3473h-13.922c-2.0349 0-3.6536-2.4511-3.6536-4.3473v-13.597h-13.597c-1.8962 0-4.3473-1.6187-4.3473-3.6536v-13.922c0-2.0349 2.4511-3.6536 4.3473-3.6536h13.597v-13.597c0-1.8962 1.6187-4.3473 3.6536-4.3473h13.922c2.0349 0 3.6536 2.4511 3.6536 4.3473v13.597h13.597c1.8962 0 4.3473 1.6187 4.3473 3.6536v13.922c0 2.0349-2.4511 3.6536-4.3473 3.6536h-13.597z\"/></svg>";



        /// <summary>
        /// Main function for Battle.net Account Switcher. Run on load.
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        public static async void LoadProfiles()
        {
            Globals.DebugWriteLine($@"[Func:BattleNet\BattleNetSwitcherFuncs.LoadProfiles] Loading BattleNet profiles");
            _battleNetRoaming = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Battle.net");
            BattleNet.LoadAccounts();
            var file = await File.ReadAllTextAsync(_battleNetRoaming + "\\Battle.net.config");
            foreach (var mail in (JsonConvert.DeserializeObject(file) as JObject)?.SelectToken("Client.SavedAccountNames")?.ToString()?.Split(','))
            {
                if (BattleNet.Accounts.All(x => x.Email != mail) && BattleNet.IgnoredAccounts.All(x => x.Email != mail) && mail != " ")
                {
                    BattleNet.Accounts.Add(new BattleNetSwitcherBase.BattleNetUser(){Email = mail});
                }
            }

            // Order
            if (File.Exists("LoginCache\\BattleNet\\order.json"))
            {
                var savedOrder = JsonConvert.DeserializeObject<List<string>>(await File.ReadAllTextAsync("LoginCache\\BattleNet\\order.json"));
                var index = 0;
                if (savedOrder != null && savedOrder.Count > 0)
                    foreach (var acc in from i in savedOrder where BattleNet.Accounts.Any(x => x.Email == i) select BattleNet.Accounts.Single(x => x.Email == i))
                    {
                        BattleNet.Accounts.Remove(acc);
                        BattleNet.Accounts.Insert(index, acc);
                        index++;
                    }
            }

            foreach (var acc in BattleNet.Accounts)
            {
                var element =
                    $"<div class=\"acc_list_item\"><input type=\"radio\" id=\"{acc.Email}\" class=\"acc\" name=\"accounts\" onchange=\"SelectedItemChanged()\" />\r\n" +
                    $"<label for=\"{acc.Email}\" class=\"acc\">\r\n" +
                    $"<img src=\"{(acc.BTag == null || !BattleNet.OverwatchMode ||acc.ImgUrl == null ? "img/BattleNetDefault.png" : acc.ImgUrl)}\" draggable=\"false\" />\r\n" +
                    $"<h6>{(acc.BTag == null ? acc.Email : (acc.BTag.Contains("#") ? acc.BTag.Split("#")[0] : acc.BTag))}</h6>\r\n";
                if ( BattleNet.OverwatchMode && DateTime.Now - acc.LastTimeChecked < TimeSpan.FromDays(1))
                {
                    if (acc.OwTankSr != 0)
                    {
                        element += $"<h6>{TankIcon} {acc.OwTankSr}<sup>SR</sup></h6>\r\n";
                    }
                    if (acc.OwDpsSr != 0)
                    {
                        element += $"<h6>{DamageIcon} {acc.OwDpsSr}<sup>SR</sup></h6>\r\n";
                    }
                    if (acc.OwSupportSr != 0)
                    {
                        element += $"<h6>{SupportIcon} {acc.OwSupportSr}<sup>SR</sup></h6>\r\n";
                    }
                }
                //$"<p>{UnixTimeStampToDateTime(ua.LastLogin)}</p>\r\n</label>";  TODO: Add some sort of "Last logged in" json file
                await AppData.ActiveIJsRuntime.InvokeVoidAsync("jQueryAppend", new object[] { "#acc_list", element += "</div>" });
            }
            await AppData.ActiveIJsRuntime.InvokeVoidAsync("initContextMenu");
            await AppData.ActiveIJsRuntime.InvokeVoidAsync("initAccListSortable");
            if(BattleNet.OverwatchMode)
                InitOverwatchMode();
        }

        public static void InitOverwatchMode()
        {
            if (!BattleNet.OverwatchMode) return;
            var accountFetched = false;
            foreach (var acc in BattleNet.Accounts.Where(x => x.BTag != null))
            {
                if (DateTime.Now - acc.LastTimeChecked <= TimeSpan.FromDays(1)) continue;
                accountFetched = acc.FetchRank();
            }

            if (!accountFetched) return;
            _ = AppData.ActiveIJsRuntime.InvokeVoidAsync("location.reload");
            BattleNet.SaveAccounts();
        }
        
        /// <summary>
        /// Restart Battle.net with a new account selected.
        /// </summary>
        /// <param name="email">User's account email</param>
        public static void SwapBattleNetAccounts(string email)
        {
            Globals.DebugWriteLine($@"[Func:BattleNet\BattleNetSwitcherFuncs.SwapBattleNetAccounts] Swapping to: {email}.");
            if (BattleNet.Accounts.Count == 0) BattleNet.LoadAccounts();

            AppData.ActiveIJsRuntime.InvokeVoidAsync("updateStatus", "Starting BattleNet");
            if (!CloseBattleNet()) return;
            
            // Load settings into JObject
            var file = File.ReadAllText(_battleNetRoaming + "\\Battle.net.config");
            var jObject = JsonConvert.DeserializeObject(file) as JObject;

            // Select the JToken with the Account Emails
            var jToken = jObject?.SelectToken("Client.SavedAccountNames");
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
                account = new BattleNetSwitcherBase.BattleNetUser() {Email = " "};
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
            File.WriteAllText(_battleNetRoaming + "\\Battle.net.config", jObject?.ToString());

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
            Globals.DebugWriteLine($@"[Func:BattleNet\BattleNetSwitcherFuncs.CloseBattleNet]");
            if (!GeneralFuncs.CanKillProcess("Battle.net")) return false;
            Globals.KillProcess("Battle.net");
            return true;
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
                Globals.DebugWriteLine($@"[Func:BattleNet\BattleNetSwitcherFuncs.SetBattleTag] accName:{email}, bTag:{bTag}");
                AppData.ActiveNavMan?.NavigateTo( "/BattleNet/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Changed BattleTag"), true);
            }
            else
                _ = GeneralInvocableFuncs.ShowToast("error", "BattleTag did not match naming policy.");
        }

        public static bool ValidateBTag(string bTag)
        {
            var parts = bTag.Split('#');
            if (parts.Length != 2) return false; // Checks has 2 parts.
            if (!(parts[1].Length >= 4 && parts[1].Length <= 7)) return false; // Checks BTag number length
            return IntPtr.TryParse(parts[1], out _); // Checks if BTag numbers part is just numbers
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
            Globals.DebugWriteLine($@"[Func:BattleNet\BattleNetSwitcherFuncs.DeleteBattleTag] accName:{email}");
            BattleNet.SaveAccounts();
            AppData.ActiveNavMan?.NavigateTo("/BattleNet/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Deleted BattleTag"), true);
        }
        
        /// <summary>
        /// Refetches the rank of the account
        /// </summary>
        /// <param name="email">The email of the account</param>
        [JSInvokable]
        public static void RefetchRank(string email)
        {
            Globals.DebugWriteLine($@"[Func:BattleNet\BattleNetSwitcherFuncs.DeleteBattleTag] accName:{email}");
            if (BattleNet.Accounts.First(x => x.Email == email).FetchRank()) AppData.ActiveNavMan?.NavigateTo("/BattleNet/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Refetched Rank"), true);
        }
        
        
        #region FORGETTING_ACCOUNTS
        
        /// <summary>
        /// Adds an account to the ignore list, so it won't show on the account switcher list anymore.
        /// </summary>
        /// <param name="accName">Email address of Blizzard account to ignore.</param>
        public static void ForgetAccount(string accName)
        {
            Globals.DebugWriteLine($@"[Func:BattleNet\BattleNetSwitcherFuncs.ForgetAccount] accName:{accName}");
            var account = BattleNet.Accounts.Find(x => x.Email == accName);
            BattleNet.Accounts.Remove(account);
            BattleNet.IgnoredAccounts.Add(account);
            BattleNet.SaveAccounts();
            AppData.ActiveNavMan?.NavigateTo("/BattleNet/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Forgot account"), true);
        }

        /// <summary>
        /// Clears list of ignored accounts
        /// </summary>
        public static async void ClearIgnored()
        {
            Globals.DebugWriteLine($@"[Func:BattleNet\BattleNetSwitcherFuncs.ClearForgotten] Clearing ignored list.");
            await GeneralInvocableFuncs.ShowModal("confirm:ClearBattleNetIgnored:" + "Are you sure you want to clear backups of forgotten accounts?".Replace(' ', '_'));
            // Confirmed in GeneralInvocableFuncs.GiConfirmAction for rest of function
        }

        /// <summary>
        /// Fires after being confirmed by above function, and actually performs task.
        /// </summary>
        public static void ClearIgnored_Confirmed()
        {
            Globals.DebugWriteLine($@"[Func:BattleNet\BattleNetSwitcherFuncs.ClearForgotten_Confirmed] Confirmation received to clear ignored list.");
            BattleNet.IgnoredAccounts = new List<BattleNetSwitcherBase.BattleNetUser>();
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
                Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.RestoreAccounts] Restoring account: {s}");
                BattleNet.IgnoredAccounts?.Remove(BattleNet.Accounts.Find(x=> x.Email == s));
            }
            BattleNet.SaveAccounts();
            return true;
        }
        #endregion

    }
}
