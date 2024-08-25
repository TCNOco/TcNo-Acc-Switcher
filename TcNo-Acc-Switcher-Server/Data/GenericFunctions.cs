// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2024 TroubleChute (Wesley Pyburn)
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
using System.Collections;
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

namespace TcNo_Acc_Switcher_Server.Data
{
    public class GenericFunctions
    {
        private static readonly Lang Lang = Lang.Instance;

        [JSInvokable]
        public static void CopyToClipboard(string toCopy)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Data\GenericFunctions.CopyToClipboard] toCopy=hidden");
            WindowsClipboard.SetText(toCopy);
        }


        /// <summary>
        /// Save settings with Ctrl+S Hot key
        /// </summary>
        [JSInvokable]
        public static void GiCtrlS(string platform)
        {
            AppSettings.SaveSettings();
            switch (platform)
            {
                case "Steam":
                    Steam.SaveSettings();
                    break;
                case "Basic":
                    Basic.SaveSettings();
                    break;
            }
            _ = GeneralInvocableFuncs.ShowToast("success", Lang["Saved"], renderTo:"toastarea");
        }

        #region ACCOUNT SWITCHER SHARED FUNCTIONS
        public static bool GenericLoadAccounts(string name, bool isBasic = false)
        {
            var localCachePath = Path.Join(Globals.UserDataFolder, $"LoginCache\\{name}\\");
            if (!Directory.Exists(localCachePath)) return false;
            if (!ListAccountsFromFolder(localCachePath, out var accList)) return false;

            // Order
            accList = OrderAccounts(accList, $"{localCachePath}\\order.json");

            InsertAccounts(accList, name, isBasic);
            AppStats.SetAccountCount(CurrentPlatform.SafeName, accList.Count);
            return true;
        }

        /// <summary>
        /// Gets a list of 'account names' from cache folder provided
        /// </summary>
        /// <param name="folder">Cache folder containing accounts</param>
        /// <param name="accList">List of account strings</param>
        /// <returns>Whether the directory exists and successfully added listed names</returns>
        public static bool ListAccountsFromFolder(string folder, out List<string> accList)
        {
            accList = new List<string>();

            if (!Directory.Exists(folder)) return false;
            var idsFile = Path.Join(folder, "ids.json");
            accList = File.Exists(idsFile)
                ? GeneralFuncs.ReadDict(idsFile).Keys.ToList()
                : (from f in Directory.GetDirectories(folder)
                    where !f.EndsWith("Shortcuts")
                    let lastSlash = f.LastIndexOf("\\", StringComparison.Ordinal) + 1
                    select f[lastSlash..]).ToList();

            return true;
        }

        /// <summary>
        /// Orders a list of strings by order specific in jsonOrderFile
        /// </summary>
        /// <param name="accList">List of strings to sort</param>
        /// <param name="jsonOrderFile">JSON file containing list order</param>
        /// <returns></returns>
        public static List<string> OrderAccounts(List<string> accList, string jsonOrderFile)
        {
            // Order
            if (!File.Exists(jsonOrderFile)) return accList;
            var savedOrder = JsonConvert.DeserializeObject<List<string>>(Globals.ReadAllText(jsonOrderFile));
            if (savedOrder == null) return accList;
            var index = 0;
            if (savedOrder is not { Count: > 0 }) return accList;
            foreach (var acc in from i in savedOrder where accList.Any(x => x == i) select accList.Single(x => x == i))
            {
                _ = accList.Remove(acc);
                accList.Insert(Math.Min(index, accList.Count), acc);
                index++;
            }
            return accList;
        }

        /// <summary>
        /// Runs jQueryProcessAccListSize, initContextMenu and initAccListSortable - Final init needed for account switcher to work.
        /// </summary>
        public static void FinaliseAccountList()
        {
            _ = AppData.InvokeVoidAsync("jQueryProcessAccListSize");
            _ = AppData.InvokeVoidAsync("initContextMenu");
            _ = AppData.InvokeVoidAsync("initAccListSortable");
        }

        /// <summary>
        /// Iterate through account list and insert into platforms account screen
        /// </summary>
        /// <param name="accList">Account list</param>
        /// <param name="platform">Platform name</param>
        /// <param name="isBasic">(Unused for now) To use Basic platform account's ID as accId)</param>
        public static void InsertAccounts(IEnumerable accList, string platform, bool isBasic = false)
        {
            if (isBasic)
                BasicSwitcherFuncs.LoadAccountIds();

            Basic.Accounts.Clear();

            foreach (var element in accList)
            {
                var account = new Shared.Accounts.Account
                {
                    Platform = CurrentPlatform.SafeName
                };

                if (element is string str)
                {
                    account.AccountId = str;
                    account.DisplayName = isBasic ? BasicSwitcherFuncs.GetNameFromId(str) : str;

                    // Handle account image
                    account.ImagePath = GetImgPath(platform, str).Replace("%", "%25");
                    var actualImagePath = Path.Join("wwwroot", GetImgPath(platform, str));

                    if (!File.Exists(actualImagePath))
                    {
                        // Make sure the directory exists
                        Directory.CreateDirectory(Path.GetDirectoryName(actualImagePath)!);
                        var defaultPng = $"wwwroot\\img\\platform\\{platform}Default.png";
                        const string defaultFallback = "wwwroot\\img\\BasicDefault.png";
                        if (File.Exists(defaultPng))
                            File.Copy(defaultPng, actualImagePath, true);
                        else if (File.Exists(defaultFallback))
                            File.Copy(defaultFallback, actualImagePath, true);
                    }

                    // Handle game stats (if any enabled and collected.)
                    account.UserStats = BasicStats.GetUserStatsAllGamesMarkup(CurrentPlatform.FullName, str);

                    Basic.Accounts.Add(account);
                    continue;
                }

                // TODO: I have no idea what this was for... But the continue skips the section here, right? Or at least there doesn't need to be brackets around it? Lost in my own code here... Whoops.
                if (element is not KeyValuePair<string, string> pair) continue;
                {
                    // Handle account image
                    var (key, value) = pair;
                    account.ImagePath = GetImgPath(platform, key);

                    // Handle game stats (if any enabled and collected.)
                    account.UserStats = BasicStats.GetUserStatsAllGamesMarkup(CurrentPlatform.FullName, key);

                    account.AccountId = key;
                    account.DisplayName = value;

                    Basic.Accounts.Add(account);
                }
            }
            FinaliseAccountList(); // Init context menu & Sorting
        }

        /// <summary>
        /// Finds if file exists as .jpg or .png
        /// </summary>
        /// <param name="platform">Platform name</param>
        /// <param name="user">Username/ID for use in image name</param>
        /// <returns>Image path</returns>
        private static string GetImgPath(string platform, string user)
        {
            var imgPath = $"img\\profiles\\{platform.ToLowerInvariant()}\\{Globals.GetCleanFilePath(user.Replace("#", "-"))}";
            if (File.Exists("wwwroot\\" + imgPath + ".png")) return imgPath + ".png";
            if (File.Exists("wwwroot\\" + imgPath + ".jpg")) return imgPath + ".jpg";
            return "\\img\\BasicDefault.png";
        }
        #endregion
    }
}
