using System.Runtime.Versioning;
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
        [JSInvokable]
        [SupportedOSPlatform("windows")]
        public static void SwapToEpic(string accName)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Epic\EpicSwitcherBase.SwapToEpic] accName:hidden");
            EpicSwitcherFuncs.SwapEpicAccounts(accName);
        }

        /// <summary>
        /// JS function handler for swapping to a new Epic account (No inputs)
        /// </summary>
        [JSInvokable]
        [SupportedOSPlatform("windows")]
        public static void NewLogin_Epic()
        {
            Globals.DebugWriteLine(@"[JSInvoke:Epic\EpicSwitcherBase.NewLogin_Epic]");
            EpicSwitcherFuncs.SwapEpicAccounts();
        }

        [JSInvokable]
        [SupportedOSPlatform("windows")]
        public static void EpicAddCurrent(string accName)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Epic\EpicSwitcherBase.EpicAddCurrent] accName:hidden");
            EpicSwitcherFuncs.EpicAddCurrent(accName);
        }
    }
}
