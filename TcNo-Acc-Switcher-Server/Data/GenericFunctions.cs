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
                case "BattleNet":
                    BattleNet.SaveSettings();
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
            foreach (var element in accList)
            {
                string imgPath;
                if (element is string str)
                {
                    imgPath = GetImgPath(platform, str).Replace("%", "%25");
                    var actualImagePath = Path.Join("wwwroot\\", GetImgPath(platform, str));
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

                    try
                    {
                        var id = str;
                        if (isBasic)
                            str = BasicSwitcherFuncs.GetNameFromId(id);

                        _ = AppData.InvokeVoidAsync("jQueryAppend", "#acc_list",
                            $"<div class=\"acc_list_item\" data-toggle=\"tooltip\"><input type=\"radio\" id=\"{id}\" Username=\"{str}\" DisplayName=\"{str}\" class=\"acc\" name=\"accounts\" onchange=\"selectedItemChanged()\" />\r\n" +
                            $"<label for=\"{id}\" class=\"acc\">\r\n" +
                            $"<img src=\"{imgPath}?{Globals.GetUnixTime()}\" draggable=\"false\" />\r\n" +
                            $"<h6>{str}</h6></div>\r\n");
                        //$"<p>{UnixTimeStampToDateTime(ua.LastLogin)}</p>\r\n</label>";  TODO: Add some sort of "Last logged in" json file
                    }
                    catch (TaskCanceledException e)
                    {
                        Globals.WriteToLog(e.ToString());
                    }

                    continue;
                }

                if (element is not KeyValuePair<string, string> pair) continue;
                {
                    var (key, value) = pair;
                    imgPath = GetImgPath(platform, key);
                    try
                    {
                        _ = AppData.InvokeVoidAsync("jQueryAppend", "#acc_list",
                            $"<div class=\"acc_list_item\"><input type=\"radio\" id=\"{key}\" Username=\"{value}\" DisplayName=\"{value}\" class=\"acc\" name=\"accounts\" onchange=\"selectedItemChanged()\" />\r\n" +
                            $"<label for=\"{key}\" class=\"acc\">\r\n" +
                            $"<img src=\"{imgPath}?{Globals.GetUnixTime()}\" draggable=\"false\" />\r\n" +
                            $"<h6>{value}</h6></div>\r\n");
                        //$"<p>{UnixTimeStampToDateTime(ua.LastLogin)}</p>\r\n</label>";  TODO: Add some sort of "Last logged in" json file
                    }
                    catch (TaskCanceledException e)
                    {
                        Globals.WriteToLog(e.ToString());
                    }
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
            var imgPath = $"\\img\\profiles\\{platform.ToLowerInvariant()}\\{Globals.GetCleanFilePath(user.Replace("#", "-"))}";
            if (File.Exists("wwwroot\\" + imgPath + ".png")) return imgPath + ".png";
            return imgPath + ".jpg";
        }
        #endregion
    }
}
