using System.Runtime.Versioning;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;

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
        public static void BasicAddCurrent(string accName)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Basic\BasicSwitcherBase.BasicAddCurrent] accName:hidden");
            if (string.IsNullOrWhiteSpace(accName)) // Reject if username is empty.
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang.Instance["Toast_NameEmpty"], Lang.Instance["Failed"], "toastarea");
                return;
            }

            try
            {
                _ = BasicSwitcherFuncs.BasicAddCurrent(accName);
            }
            catch (System.IO.DirectoryNotFoundException io)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang.Instance["Toast_RestartAsAdmin"], Lang.Instance["Failed"], "toastarea");
                Globals.WriteToLog($"Failed to BasicAddCurrent. DirectoryNotFoundException", io);
            }
        }
    }
}