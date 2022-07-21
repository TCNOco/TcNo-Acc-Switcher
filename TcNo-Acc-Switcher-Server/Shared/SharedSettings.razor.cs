using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.State.DataTypes;

namespace TcNo_Acc_Switcher_Server.Shared
{
    public partial class SharedSettings
    {
        public void Button_StartTray()
        {
            var res = NativeFuncs.StartTrayIfNotRunning();
            switch (res)
            {
                case "Started Tray":
                    AppState.Toasts.ShowToastLang(ToastType.Success, "Toast_TrayStarted");
                    break;
                case "Already running":
                    AppState.Toasts.ShowToastLang(ToastType.Info, "Toast_TrayRunning");
                    break;
                case "Tray users not found":
                    AppState.Toasts.ShowToastLang(ToastType.Error, "Toast_TrayUsersMissing");
                    break;
                default:
                    AppState.Toasts.ShowToastLang(ToastType.Error, "Toast_TrayFail");
                    break;
            }
        }

        /// <summary>
        /// Sets the active browser
        /// </summary>
        public void SetActiveBrowser(string browser)
        {
            WindowSettings.ActiveBrowser = browser;
            AppState.Toasts.ShowToastLang(ToastType.Info, "Notice", "Toast_RestartRequired");
        }

        /// <summary>
        /// Toggle whether the program minimizes to the start menu on exit
        /// </summary>
        public void TrayMinimizeNotExit_Toggle()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.TrayMinimizeNotExit_Toggle]");
            if (WindowSettings.TrayMinimizeNotExit) return;
            AppState.Toasts.ShowToastLang(ToastType.Info, "Toast_TrayPosition", 15000);
            AppState.Toasts.ShowToastLang(ToastType.Info, "Toast_TrayHint", 15000);
        }
    }
}
