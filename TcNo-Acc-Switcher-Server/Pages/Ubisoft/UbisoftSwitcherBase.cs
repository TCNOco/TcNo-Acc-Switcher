using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Pages.Ubisoft
{
    public class UbisoftSwitcherBase
    {
        /// <summary>
        /// [Wrapper with fewer arguments]
        /// </summary>
        [JSInvokable]
        public static void SwapToUbisoft(string userId) => SwapToUbisoft(userId, 0);
        [JSInvokable]
        public static void SwapToUbisoftWithReq(string userId, int request) => SwapToUbisoft(userId, request);
        /// <summary>
        /// JS function handler for swapping to another Ubisoft account.
        /// </summary>
        /// <param name="userId">Requested account's UserId</param>
        /// <param name="state">Requested account's Login state</param>
        public static void SwapToUbisoft(string userId, int state)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Ubisoft\UbisoftSwitcherBase.SwapToUbisoft] userId:hidden");
            UbisoftSwitcherFuncs.SwapUbisoftAccounts(userId, state);
        }

        /// <summary>
        /// JS function handler for swapping to a new Ubisoft account (No inputs)
        /// </summary>
        [JSInvokable]
        public static void NewLogin_Ubisoft()
        {
            Globals.DebugWriteLine(@"[JSInvoke:Ubisoft\UbisoftSwitcherBase.NewSteamLogin]");
            UbisoftSwitcherFuncs.SwapUbisoftAccounts();
        }

        [JSInvokable]
        public static void UbisoftAddCurrent()
        {
            Globals.DebugWriteLine(@"[JSInvoke:Ubisoft\UbisoftSwitcherBase.UbisoftAddCurrent]");
            UbisoftSwitcherFuncs.UbisoftAddCurrent();
        }

        [JSInvokable]
        public static void UbisoftRefreshUsername(string userId)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Ubisoft\UbisoftSwitcherBase.UbisoftRefreshUsername]");
            UbisoftSwitcherFuncs.FindUsername(userId, false);
        }
    }
}
