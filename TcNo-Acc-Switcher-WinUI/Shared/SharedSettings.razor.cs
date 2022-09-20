using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher.State.DataTypes;
using TcNo_Acc_Switcher.State.Interfaces;

namespace TcNo_Acc_Switcher.Shared;

public partial class SharedSettings
{
    [Inject] private IToasts Toasts { get; set; }

    public void Button_StartTray()
    {
        var res = NativeFuncs.StartTrayIfNotRunning();
        switch (res)
        {
            case "Started Tray":
                Toasts.ShowToastLang(ToastType.Success, "Toast_TrayStarted");
                break;
            case "Already running":
                Toasts.ShowToastLang(ToastType.Info, "Toast_TrayRunning");
                break;
            case "Tray users not found":
                Toasts.ShowToastLang(ToastType.Error, "Toast_TrayUsersMissing");
                break;
            default:
                Toasts.ShowToastLang(ToastType.Error, "Toast_TrayFail");
                break;
        }
    }

    /// <summary>
    /// Sets the active browser
    /// </summary>
    public void SetActiveBrowser(string browser)
    {
        WindowSettings.ActiveBrowser = browser;
        Toasts.ShowToastLang(ToastType.Info, "Notice", "Toast_RestartRequired");
    }
}