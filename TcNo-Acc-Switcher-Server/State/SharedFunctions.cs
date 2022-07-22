using System;
using System.Diagnostics;
using System.IO;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.State;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace State
{
    /// <summary>
    /// I did want to avoid something like this, but this seems like the best compromise to having duplicate code.
    /// This can be included by both Steam and the BasicPlatform system -- While not relying on them, or injecting them via DI.
    /// </summary>
    public class SharedFunctions
    {
        [Inject] private JSRuntime JsRuntime { get; set; }
        [Inject] private IAppState AppState { get; set; }
        [Inject] private IStatistics Statistics { get; set; }
        [Inject] private Toasts Toasts { get; set; }

        /// <summary>
        /// Runs jQueryProcessAccListSize, initContextMenu and initAccListSortable - Final init needed for account switcher to work.
        /// </summary>
        public async Task FinaliseAccountList()
        {
            //await AppData.InvokeVoidAsync("jQueryProcessAccListSize");
            await JsRuntime.InvokeVoidAsync("initContextMenu");
            await JsRuntime.InvokeVoidAsync("initAccListSortable");
        }

        public void RunShortcut(string s, string shortcutFolder, string platform = "", bool admin = false)
        {
            Statistics.IncrementGameLaunches(platform);

            var proc = new Process();
            proc.StartInfo = new ProcessStartInfo
            {
                FileName = Path.GetFullPath(Path.Join(shortcutFolder, s)),
                UseShellExecute = true,
                Verb = admin ? "runas" : ""
            };

            if (s.EndsWith(".url"))
            {
                // These can not be run as admin...
                proc.StartInfo.UseShellExecute = false;
                proc.StartInfo.WindowStyle = ProcessWindowStyle.Hidden;
                proc.StartInfo.Arguments = $"/C \"{proc.StartInfo.FileName}\"";
                proc.StartInfo.FileName = "cmd.exe";
                proc.StartInfo.CreateNoWindow = true;
                proc.StartInfo.RedirectStandardInput = true;
                proc.StartInfo.RedirectStandardOutput = true;
                if (admin)
                    Toasts.ShowToastLang(ToastType.Warning, "Toast_UrlAdminErr", 15000);
            }
            else if (Globals.IsAdministrator && !admin)
            {
                proc.StartInfo.Arguments = proc.StartInfo.FileName;
                proc.StartInfo.FileName = "explorer.exe";
            }

            try
            {
                proc.Start();
                Toasts.ShowToastLang(ToastType.Info, new LangSub("Status_StartingPlatform", new { platform = Globals.RemoveShortcutExt(s) }));
            }
            catch (Exception e)
            {
                // Cancelled by user, or another error.
                Globals.WriteToLog($"Tried to start \"{s}\" but failed.", e);
                Toasts.ShowToastLang(ToastType.Error, "Status_FailedLog", duration: 15000);
            }
        }
    }
}
