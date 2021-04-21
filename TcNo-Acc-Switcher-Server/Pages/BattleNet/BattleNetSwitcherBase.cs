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
using System.Linq;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.Steam;

namespace TcNo_Acc_Switcher_Server.Pages.BattleNet
{
    public class BattleNetSwitcherBase
    {
        /// <summary>
        /// JS function handler for swapping to another Battle.net account.
        /// </summary>
        /// <param name="accName">Requested account's Login Username</param>
        [JSInvokable]
        public static void SwapToBattleNet(string accName = "")
        {
            Globals.DebugWriteLine($@"[JSInvoke:BattleNet\BattleNetSwitcherBase.SwapToBattleNet] accName:{accName}");
            BattleNetSwitcherFuncs.SwapBattleNetAccounts(accName);
        }

        public class BattleNetUser
        {
            [JsonProperty("Email", Order = 0)] public string Email { get; set; }
            [JsonProperty("BattleTag", Order = 1)] public string BTag { get; set; }
            [JsonIgnore] public string ImgUrl { get; set; }
        }
    }
}
