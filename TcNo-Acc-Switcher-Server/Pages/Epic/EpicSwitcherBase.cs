using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Pages.Epic
{
    public class EpicSwitcherBase
    {
        /// <summary>
        /// JS function handler for swapping to another Epic account.
        /// </summary>
        /// <param name="accName">Requested account's Login Username</param>
        /// <param name="state">Requested account's Login state</param>
        [JSInvokable]
        public static void SwapToEpic(string accName, int state = 0)
        {
            Globals.DebugWriteLine($@"[JSInvoke:Epic\EpicSwitcherBase.SwapToEpic] accName:{accName}");
            EpicSwitcherFuncs.SwapEpicAccounts(accName, state);
        }

        [JSInvokable]
        public static void EpicAddCurrent(string accName)
        {
            Globals.DebugWriteLine($@"[JSInvoke:Epic\EpicSwitcherBase.EpicAddCurrent] accName:{accName}");
            EpicSwitcherFuncs.EpicAddCurrent(accName);
        }
    }
}
