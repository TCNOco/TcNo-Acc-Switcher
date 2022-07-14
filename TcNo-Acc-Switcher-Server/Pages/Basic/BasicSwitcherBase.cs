﻿using System.Runtime.Versioning;
using System.Threading.Tasks;
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
        public static void NewLogin_Basic()
        {
            Globals.DebugWriteLine(@"[JSInvoke:Basic\BasicSwitcherBase.NewLogin_Basic]");
            BasicSwitcherFuncs.SwapBasicAccounts();
        }

        [JSInvokable]
        [SupportedOSPlatform("windows")]
        public static async Task BasicAddCurrent(string accName)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Basic\BasicSwitcherBase.BasicAddCurrent] accName:hidden");
            await BasicSwitcherFuncs.BasicAddCurrent(accName);
        }
    }
}