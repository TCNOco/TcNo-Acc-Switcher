using System.Linq;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Server.State;
using TcNo_Acc_Switcher_Server.State.Classes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.StateFuncs
{
    public class WindowSettingsFuncs
    {
        [Inject] private IWindowSettings WindowSettings { get; set; }

        /// <summary>
        /// Get platform details from an identifier, or the name.
        /// </summary>
        public PlatformItem GetPlatform(string nameOrId) => WindowSettings.Platforms.FirstOrDefault(x => x.Name == nameOrId || x.PossibleIdentifiers.Contains(nameOrId));
        public void SetActiveBrowser(string browser)
        {
            WindowSettings.ActiveBrowser = browser;
            AData.ShowToastLang(ToastType.Info, "Notice", "Toast_RestartRequired");
        }
    }
}
