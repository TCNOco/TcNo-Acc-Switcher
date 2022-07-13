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
using System.IO;
using System.Linq;
using System.Net;
using System.Threading.Tasks;
using HtmlAgilityPack;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Pages.BattleNet
{
    public class BattleNetSwitcherBase
    {
        private static readonly Lang Lang = Lang.Instance;

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

            [JsonIgnore] public string ImgUrl { get; set; }
        }
    }
}