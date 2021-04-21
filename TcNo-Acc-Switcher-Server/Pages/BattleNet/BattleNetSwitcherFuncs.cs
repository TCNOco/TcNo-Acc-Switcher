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
        private static string _battleNetRoaming = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "Battle.net");
        private static string _battleNetProgramData = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData), "Battle.net");
        private static List<BattleNetSwitcherBase.BattleNetUser> _accounts = new();
        
        /// <summary>
        /// Main function for Battle.net Account Switcher. Run on load.
        /// Prepares HTML Elements string for insertion into the account switcher GUI.
        /// </summary>
        /// <returns>Whether account loading is successful, or a path reset is needed (invalid dir saved)</returns>
        public static async void LoadProfiles()
        {
            Globals.DebugWriteLine($@"[Func:BattleNet\BattleNetSwitcherFuncs.LoadProfiles] Loading BattleNet profiles");
            _accounts = new List<BattleNetSwitcherBase.BattleNetUser>();
            LoadAccounts(ref _accounts);

            foreach (var acc in _accounts)
            {
                var element =
                    $"<input type=\"radio\" id=\"{acc.Email}\" class=\"acc\" name=\"accounts\" onchange=\"SelectedItemChanged()\" />\r\n" +
                    $"<label for=\"{acc.Email}\" class=\"acc\">\r\n" +
                    $"<img src=\"\\img\\BattleNetDefault.png\" draggable=\"false\" />\r\n" +
                    $"<h6>{acc.BTag ?? acc.Email}</h6>\r\n";
                //$"<p>{UnixTimeStampToDateTime(ua.LastLogin)}</p>\r\n</label>";  TODO: Add some sort of "Last logged in" json file
                await AppData.ActiveIJsRuntime.InvokeVoidAsync("jQueryAppend", new object[] { "#acc_list", element });
            }
            await AppData.ActiveIJsRuntime.InvokeVoidAsync("initContextMenu");
        }

        private static void LoadAccounts(ref List<BattleNetSwitcherBase.BattleNetUser> _accounts)
        {
            BattleNet.LoadIgnoredAccounts();
            var file = File.ReadAllText(_battleNetRoaming + "\\Battle.net.config");
            foreach (var mail in (JsonConvert.DeserializeObject(file) as JObject)?.SelectToken("Client.SavedAccountNames")?.ToString()?.Split(','))
            {
                if (BattleNet.IgnoredAccounts.Count(x => x.Key == mail) == 0 && mail != "TempUsername") // If not on IgnoredAccounts list
                    _accounts.Add(new BattleNetSwitcherBase.BattleNetUser() { Email = mail, BTag = BattleNet.BTags.ContainsKey(mail) ? BattleNet.BTags[mail] : null });
            }
        }


        /// <summary>
        /// Restart Battle.net with a new account selected.
        /// </summary>
        /// <param name="email">User's account email</param>
        public static void SwapBattleNetAccounts(string email)
        {
            Globals.DebugWriteLine($@"[Func:BattleNet\BattleNetSwitcherFuncs.SwapBattleNetAccounts] Swapping to: {email}.");
            if (_accounts.Count == 0) LoadAccounts(ref _accounts);

            AppData.ActiveIJsRuntime.InvokeVoidAsync("updateStatus", "Starting BattleNet");
            if (!CloseBattleNet()) return;

            // Load settings into JObject
            var file = File.ReadAllText(_battleNetRoaming + "\\Battle.net.config");
            var jObject = JsonConvert.DeserializeObject(file) as JObject;

            // Select the JToken with the Account Emails
            var jToken = jObject?.SelectToken("Client.SavedAccountNames");

            var replaceString = "";

            if (email != "") // New account
            {
                var account = _accounts.First(x => x.Email == email);
                // Set the to be logged in Account to idx 0
                _accounts.Remove(account);
                _accounts.Insert(0, account);

                Globals.AddTrayUser("BattleNet", "+b:" + email, account.BTag ?? email); // Add to Tray list
            }
            else
            {
                var account = new BattleNetSwitcherBase.BattleNetUser() {Email = "TempUsername"};
                _accounts.Remove(account);
                _accounts.Insert(0, account);
            }

            // Build the string with the Emails with the Email that's should get logged in at first
            for (var i = 0; i < _accounts.Count; i++)
            {
                replaceString += _accounts[i].Email;
                if (i < _accounts.Count - 1)
                {
                    replaceString += ",";
                }
            }
            
            // Replace and write the new Json
            jToken?.Replace(replaceString);
            File.WriteAllText(_battleNetRoaming + "\\Battle.net.config", jObject?.ToString());

            GeneralFuncs.StartProgram(BattleNet.Exe(), BattleNet.Admin);
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
        /// <param name="id">BattleNet email address</param>
        /// <param name="newName">New name for user</param>
        public static void ChangeUsername(string id, string newName)
        {
            Globals.DebugWriteLine($@"[Func:BattleNet\BattleNetSwitcherFuncs.SetBattleTag] id:{id}, newName:{newName}");
            BattleNet.BTags.Remove(id);
            BattleNet.BTags.Add(id, newName);
            BattleNet.SaveSettings();
            AppData.ActiveNavMan?.NavigateTo("/BattleNet/?cacheReload&toast_type=success&toast_title=Success&toast_message=" + Uri.EscapeUriString("Changed username"), true);
        }

        #region IGNORING_ACCOUNTS
        /// <summary>
        /// Adds an account to the ignore list, so it won't show on the account switcher list anymore.
        /// </summary>
        /// <param name="accName">Email address of Blizzard account to ignore.</param>
        public static void ForgetAccount(string accName)
        {
            Globals.DebugWriteLine($@"[Func:BattleNet\BattleNetSwitcherFuncs.ForgetAccount] accName:{accName}");
            BattleNet.AddIgnoredAccount(accName);
            BattleNet.BTags.Remove(accName);
            BattleNet.SaveSettings();
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
            if (File.Exists(BattleNet.IgnoredAccPath)) File.WriteAllText(BattleNet.IgnoredAccPath, "{}");
        }

        /// <summary>
        /// Restores a list of accounts, by removing them from the ignore list.
        /// </summary>
        /// <param name="requestedAccs">Array of account emails to remove from ignore list</param>
        /// <returns>Whether it was successful or not</returns>
        public static bool RestoreSelected(string[] requestedAccs)
        {
            if (!File.Exists(BattleNet.IgnoredAccPath)) return false;
            var ignoredAccounts = JsonConvert.DeserializeObject<Dictionary<string, string>>(File.ReadAllText(BattleNet.IgnoredAccPath));
            foreach (var s in requestedAccs)
            {
                Globals.DebugWriteLine($@"[Func:Steam\SteamSwitcherFuncs.RestoreAccounts] Restoring account: {s}");
                ignoredAccounts?.Remove(s);
            }
            File.WriteAllText(BattleNet.IgnoredAccPath, JsonConvert.SerializeObject(ignoredAccounts));
            return true;
        }
        #endregion

    }
}
