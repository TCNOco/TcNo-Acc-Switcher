using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Pages.Origin
{
    public class OriginSwitcherBase
    {
        /// <summary>
        /// JS function handler for swapping to another Origin account.
        /// </summary>
        /// <param name="accName">Requested account's Login Username</param>
        /// <param name="state">Requested account's Login state</param>
        [JSInvokable]
        public static void SwapToOrigin(string accName, int state = 0)
        {
            Globals.DebugWriteLine($@"[JSInvoke:Origin\OriginSwitcherBase.SwapToOrigin] accName:{accName}");
            OriginSwitcherFuncs.SwapOriginAccounts(accName, state);
        }
        /// <summary>
        /// JS function handler for swapping to a new Origin account (No inputs)
        /// </summary>
        [JSInvokable]
        public static void NewLogin_Origin()
        {
            Globals.DebugWriteLine(@"[JSInvoke:Origin\OriginSwitcherBase.NewLogin_Origin]");
            OriginSwitcherFuncs.SwapOriginAccounts("", 0);
        }

        [JSInvokable]
        public static void OriginAddCurrent(string accName)
        {
            Globals.DebugWriteLine($@"[JSInvoke:Origin\OriginSwitcherBase.OriginAddCurrent] accName:{accName}");
            OriginSwitcherFuncs.OriginAddCurrent(accName);
        }
    }
}
