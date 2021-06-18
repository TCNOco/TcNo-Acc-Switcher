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
using System.IO;
using System.Linq;
using System.Net;
using System.Threading.Tasks;
using HtmlAgilityPack;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.BattleNet
{
    public class BattleNetSwitcherBase
    {
        /// <summary>
        /// JS function handler for swapping to another Battle.net account.
        /// </summary>
        /// <param name="accName">Requested account's Login Username</param>
        [JSInvokable]
        public static async Task SwapToBattleNet(string accName = "")
        {
            Globals.DebugWriteLine(@"[JSInvoke:BattleNet\BattleNetSwitcherBase.SwapToBattleNet] accName:hidden");
            await BattleNetSwitcherFuncs.SwapBattleNetAccounts(accName);
        }
        /// <summary>
        /// JS function handler for swapping to a new BattleNet account (No inputs)
        /// </summary>
        [JSInvokable]
        public static async Task NewLogin_BattleNet()
        {
            Globals.DebugWriteLine(@"[JSInvoke:BattleNet\BattleNetSwitcherBase.NewLogin_BattleNet]");
            await BattleNetSwitcherFuncs.SwapBattleNetAccounts("");
        }


        public class BattleNetUser
        {
            [JsonProperty("Email", Order = 0)] public string Email { get; set; }
            [JsonProperty("BattleTag", Order = 1)] public string BTag { get; set; }
            [JsonProperty("OwTankSr", Order = 2)] public int OwTankSr { get; set; }
            [JsonProperty("OwDpsSr", Order = 3)] public int OwDpsSr { get; set; }
            [JsonProperty("OwSupportSr", Order = 4)] public int OwSupportSr { get; set; }
            [JsonProperty("LastTimeChecked", Order = 5)] public DateTime LastTimeChecked { get; set; }
            [JsonIgnore] public string ImgUrl { get; set; }


            public bool FetchRank()
            {
                if (!BattleNetSwitcherFuncs.ValidateBTag(BTag)) return false;
                var split = BTag.Split("#");
                var req = WebRequest.Create($"https://playoverwatch.com/en-us/career/pc/{split[0]}-{split[1]}/");
                req.Method = "GET";
            
                var doc = new HtmlDocument();
                var responseStream = req.GetResponse().GetResponseStream();
                if (responseStream == null)
                {
                    _ = GeneralInvocableFuncs.ShowToast("error", $"Error trying to get stats for {BTag}", renderTo: "toastarea");
                    return false;
                }
                using (var reader = new StreamReader(responseStream))
                {
                    doc.LoadHtml(reader.ReadToEnd());
                }
                
                // If the PlayOverwatch site is overloaded
                if (doc.DocumentNode.SelectSingleNode("/html/body/section[1]/section/div/h1") != null)
                {
                    _ = GeneralInvocableFuncs.ShowToast("error", $"Error trying to get stats for {BTag}", renderTo: "toastarea");
                    return false;
                }

                // If the Profile is private
                if (doc.DocumentNode.SelectSingleNode(
                    "/html/body/section[1]/div[1]/section/div/div/div/div/div[1]/p")?.InnerHtml == "Private Profile")
                {
                    ImgUrl = doc.DocumentNode
                        .SelectSingleNode("/html/body/section[1]/div[1]/section/div/div/div/div/div[2]/img")
                        .Attributes["src"].Value;
                    _ = GeneralInvocableFuncs.ShowToast("warning", $"{BTag}'s profile is private", renderTo: "toastarea");
                    BattleNetSwitcherFuncs.DownloadImage(Email, ImgUrl);
                    LastTimeChecked = DateTime.Now - TimeSpan.FromMinutes(1435); // 23 Hours 55 Minutes
                    return false;
                }
                
                // If BattleTag is invalid
                if (doc.DocumentNode.SelectSingleNode(
                    "/html/body/section[1]/section/div/h1")?.InnerHtml == "PROFILE NOT FOUND")
                {
                    _ = GeneralInvocableFuncs.ShowToast("error", $"{BTag} was not found", renderTo: "toastarea");
                    return false;
                }
                
                LastTimeChecked = DateTime.Now;
                ImgUrl = doc.DocumentNode
                    .SelectSingleNode("/html/body/section[1]/div[1]/section/div/div[2]/div/div/div[2]/img")
                    .Attributes["src"].Value;
                BattleNetSwitcherFuncs.DownloadImage(Email, ImgUrl);

                var ranks = doc.DocumentNode.SelectNodes("/html/body/section[1]/div[1]/section/div/div[2]/div/div/div[2]/div/div[3]");
                foreach (var node in ranks.Elements())
                {
                    if (!int.TryParse(node.LastChild.LastChild.InnerHtml, out var sr))
                    {
                        continue;
                    }
                    switch (node.LastChild.FirstChild.Attributes["data-ow-tooltip-text"].Value.Split(" ").First())
                    {
                        case "Tank":
                            OwTankSr = sr;
                            break;
                        case "Damage":
                            OwDpsSr = sr;
                            break;
                        case "Support":
                            OwSupportSr = sr;
                            break;
                        default:
                            continue;
                    }
                }
                return true;
            }
        }
    }
}
