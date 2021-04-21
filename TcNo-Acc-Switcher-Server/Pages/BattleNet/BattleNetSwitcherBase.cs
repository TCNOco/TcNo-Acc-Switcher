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
        public static void SwapToBattleNet(string accName)
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
