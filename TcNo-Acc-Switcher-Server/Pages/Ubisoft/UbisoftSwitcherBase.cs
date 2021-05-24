using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Pages.Ubisoft
{
    public class UbisoftSwitcherBase
    {
        /// <summary>
        /// JS function handler for swapping to another Ubisoft account.
        /// </summary>
        /// <param name="userId">Requested account's UserId</param>
        /// <param name="state">Requested account's Login state</param>
        [JSInvokable]
        public static void SwapToUbisoft(string userId, int state = 0)
        {
            Globals.DebugWriteLine($@"[JSInvoke:Ubisoft\UbisoftSwitcherBase.SwapToUbisoft] userId:{userId}");
            UbisoftSwitcherFuncs.SwapUbisoftAccounts(userId, state);
        }

        [JSInvokable]
        public static void UbisoftAddCurrent()
        {
            Globals.DebugWriteLine($@"[JSInvoke:Ubisoft\UbisoftSwitcherBase.UbisoftAddCurrent]");
            UbisoftSwitcherFuncs.UbisoftAddCurrent();
        }

        [JSInvokable]
        public static void UbisoftRefreshUsername(string userId)
        {
            Globals.DebugWriteLine($@"[JSInvoke:Ubisoft\UbisoftSwitcherBase.UbisoftRefreshUsername]");
            UbisoftSwitcherFuncs.FindUsername(userId, false);
        }
    }
}
