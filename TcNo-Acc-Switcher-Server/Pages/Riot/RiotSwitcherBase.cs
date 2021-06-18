using System.Runtime.Versioning;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Pages.Riot
{
    public class RiotSwitcherBase
    {
        /// <summary>
        /// JS function handler for swapping to another Riot account.
        /// </summary>
        /// <param name="accName">Requested account's Login Username</param>
        [JSInvokable]
        [SupportedOSPlatform("windows")]
        public static void SwapToRiot(string accName)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Riot\RiotSwitcherBase.SwapToRiot] accName:hidden");
            RiotSwitcherFuncs.SwapRiotAccounts(accName);
        }

        /// <summary>
        /// JS function handler for swapping to a new Riot account (No inputs)
        /// </summary>
        [JSInvokable]
        [SupportedOSPlatform("windows")]
        public static void NewLogin_Riot()
        {
            Globals.DebugWriteLine(@"[JSInvoke:Riot\RiotSwitcherBase.NewLogin_Riot]");
            RiotSwitcherFuncs.SwapRiotAccounts("");
        }

        [JSInvokable]
        [SupportedOSPlatform("windows")]
        public static void RiotAddCurrent(string accName)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Riot\RiotSwitcherBase.RiotAddCurrent] accName:hidden");
            RiotSwitcherFuncs.RiotAddCurrent(accName);
        }
    }
}