using System.Runtime.Versioning;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Pages.Basic
{
    public class BasicSwitcherBase
    {
        /// <summary>
        /// JS function handler for swapping to another Basic account.
        /// </summary>
        /// <param name="accName">Requested account's Login Username</param>
        [JSInvokable]
        [SupportedOSPlatform("windows")]
        public static void SwapToBasic(string accName)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Basic\BasicSwitcherBase.SwapToBasic] accName:hidden");
            BasicSwitcherFuncs.SwapBasicAccounts(accName);
        }

        /// <summary>
        /// JS function handler for swapping to a new Basic account (No inputs)
        /// </summary>
        [JSInvokable]
        [SupportedOSPlatform("windows")]
        public static void NewLoginBasic()
        {
            Globals.DebugWriteLine(@"[JSInvoke:Basic\BasicSwitcherBase.NewLoginBasic]");
            BasicSwitcherFuncs.SwapBasicAccounts();
        }

        [JSInvokable]
        [SupportedOSPlatform("windows")]
        public static void BasicAddCurrent(string accName)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Basic\BasicSwitcherBase.BasicAddCurrent] accName:hidden");
            _ = BasicSwitcherFuncs.BasicAddCurrent(accName);
        }
    }
}