using System.Runtime.Versioning;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Pages.Basic
{
    public class BasicSwitcherBase
    {
        [JSInvokable]
        [SupportedOSPlatform("windows")]
        public static async Task BasicAddCurrent(string accName)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Basic\BasicSwitcherBase.BasicAddCurrent] accName:hidden");
            await BasicSwitcherFuncs.BasicAddCurrent(accName);
        }
    }
}