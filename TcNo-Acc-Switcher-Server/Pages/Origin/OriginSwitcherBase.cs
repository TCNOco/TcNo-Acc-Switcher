using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Pages.Origin
{
    public class OriginSwitcherBase
    {
        /// <summary>
        /// [Wrapper with fewer arguments]
        /// </summary>
        [JSInvokable]
        public static void SwapToOrigin(string accName) => SwapToOrigin(accName, 0);
        [JSInvokable]
        public static void SwapToOriginWithReq(string accName, int request) => SwapToOrigin(accName, request);
        /// <summary>
        /// JS function handler for swapping to another Origin account.
        /// </summary>
        /// <param name="accName">Requested account's Login Username</param>
        /// <param name="state">Requested account's Login state</param>
        public static void SwapToOrigin(string accName, int state)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Origin\OriginSwitcherBase.SwapToOrigin] accName:hidden");
            OriginSwitcherFuncs.SwapOriginAccounts(accName, state);
        }
        /// <summary>
        /// JS function handler for swapping to a new Origin account (No inputs)
        /// </summary>
        [JSInvokable]
        public static void NewLogin_Origin()
        {
            Globals.DebugWriteLine(@"[JSInvoke:Origin\OriginSwitcherBase.NewLogin_Origin]");
            OriginSwitcherFuncs.SwapOriginAccounts();
        }

        [JSInvokable]
        public static void OriginAddCurrent(string accName)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Origin\OriginSwitcherBase.OriginAddCurrent] accName:hidden");
            OriginSwitcherFuncs.OriginAddCurrent(accName);
        }
    }
}
