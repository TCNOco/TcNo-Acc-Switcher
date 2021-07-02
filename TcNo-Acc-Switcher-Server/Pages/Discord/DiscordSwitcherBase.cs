using System.Runtime.Versioning;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Pages.Discord
{
    public class DiscordSwitcherBase
    {
        /// <summary>
        /// JS function handler for swapping to another Discord account.
        /// </summary>
        /// <param name="accName">Requested account's Login Username</param>
        [JSInvokable]
        [SupportedOSPlatform("windows")]
        public static void SwapToDiscord(string accName)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Discord\DiscordSwitcherBase.SwapToDiscord] accName:hidden");
            DiscordSwitcherFuncs.SwapDiscordAccounts(accName);
        }

        /// <summary>
        /// JS function handler for swapping to a new Discord account (No inputs)
        /// </summary>
        [JSInvokable]
        [SupportedOSPlatform("windows")]
        public static void NewLogin_Discord()
        {
            Globals.DebugWriteLine(@"[JSInvoke:Discord\DiscordSwitcherBase.NewLogin_Discord]");
            DiscordSwitcherFuncs.SwapDiscordAccounts();
        }

        [JSInvokable]
        [SupportedOSPlatform("windows")]
        public static void DiscordAddCurrent(string accName)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Discord\DiscordSwitcherBase.DiscordAddCurrent] accName:hidden");
            DiscordSwitcherFuncs.DiscordAddCurrent(accName);
        }
    }
}
