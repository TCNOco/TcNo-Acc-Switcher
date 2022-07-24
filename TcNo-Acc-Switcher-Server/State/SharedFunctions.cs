using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.State.DataTypes;

namespace TcNo_Acc_Switcher_Server.State;

/// <summary>
/// I did want to avoid something like this, but this seems like the best compromise to having duplicate code.
/// This can be included by both Steam and the BasicPlatform system -- While not relying on them, or injecting them via DI.
/// </summary>
public class SharedFunctions
{
    [Inject] private JSRuntime JsRuntime { get; set; }
    [Inject] private IStatistics Statistics { get; set; }
    [Inject] private Toasts Toasts { get; set; }
    [Inject] private Modals Modals { get; set; }
    [Inject] private Lang Lang { get; set; }

    /// <summary>
    /// Runs jQueryProcessAccListSize, initContextMenu and initAccListSortable - Final init needed for account switcher to work.
    /// </summary>
    public async Task FinaliseAccountList()
    {
        //await JsRuntime.InvokeVoidAsync("jQueryProcessAccListSize");
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


    #region PROCESS_OPERATIONS

    //public static bool CanKillProcess(List<string> procNames) => procNames.Aggregate(true, (current, s) => current & CanKillProcess(s));
    public bool CanKillProcess(List<string> procNames, string closingMethod = "Combined", bool showModal = true)
    {
        var canKillAll = true;
        foreach (var procName in procNames)
        {
            if (procName.StartsWith("SERVICE:") && closingMethod == "TaskKill") continue; // Ignore explicit services when using TaskKill - Admin isn't ALWAYS needed. Eg: Steam.
            canKillAll = canKillAll && CanKillProcess(procName, closingMethod);
        }

        return canKillAll;
    }

    public bool CanKillProcess(string processName, string closingMethod = "Combined", bool showModal = true)
    {
        if (processName.StartsWith("SERVICE:") && closingMethod == "TaskKill") return true; // Ignore explicit services when using TaskKill - Admin isn't ALWAYS needed. Eg: Steam.
        if (processName.StartsWith("SERVICE:")) // Services need admin to close (as far as I understand)
            processName = processName[8..].Split(".exe")[0];


        // Restart self as if can't close admin.
        if (Globals.CanKillProcess(processName)) return true;
        if (showModal)
            Modals.ShowModal("confirm", ExtraArg.RestartAsAdmin);
        return false;
    }

    public async Task<bool> CloseProcesses(string procName, string closingMethod)
    {
        if (!OperatingSystem.IsWindows()) return false;
        Globals.DebugWriteLine(@"Closing: " + procName);
        if (!CanKillProcess(procName, closingMethod)) return false;
        Globals.KillProcess(procName, closingMethod);

        return await WaitForClose(procName);
    }
    public async Task<bool> CloseProcesses(List<string> procNames, string closingMethod)
    {
        if (!OperatingSystem.IsWindows()) return false;
        Globals.DebugWriteLine(@"Closing: " + string.Join(", ", procNames));
        if (!CanKillProcess(procNames, closingMethod)) return false;
        Globals.KillProcess(procNames, closingMethod);

        return await WaitForClose(procNames, closingMethod);
    }

    /// <summary>
    /// Waits for a program to close, and returns true if not running anymore.
    /// </summary>
    /// <param name="procName">Name of process to lookup</param>
    /// <returns>Whether it was closed before this function returns or not.</returns>
    public async Task<bool> WaitForClose(string procName)
    {
        if (!OperatingSystem.IsWindows()) return false;
        var timeout = 0;
        while (Globals.ProcessHelper.IsProcessRunning(procName) && timeout < 10)
        {
            timeout++;
            await JsRuntime.InvokeVoidAsync("updateStatus", Lang["Status_WaitingForClose", new { processName = procName, timeout, timeLimit = "10" }]);
            Thread.Sleep(1000);
        }

        if (timeout == 10)
            Toasts.ShowToastLang(ToastType.Error, "Error", new LangSub("CouldNotCloseX", new { x = procName }));

        return timeout != 10; // Returns true if timeout wasn't reached.
    }
    public async Task<bool> WaitForClose(List<string> procNames, string closingMethod)
    {
        if (!OperatingSystem.IsWindows()) return false;
        var procToClose = new List<string>(); // Make a copy to edit
        foreach (var p in procNames)
        {
            var cur = p;
            if (cur.StartsWith("SERVICE:"))
            {
                if (closingMethod == "TaskKill")
                    continue; // Ignore explicit services when using TaskKill - Admin isn't ALWAYS needed. Eg: Steam.
                cur = cur[8..].Split(".exe")[0]; // Remove "SERVICE:" and ".exe"
            }
            procToClose.Add(cur);
        }

        var timeout = 0;
        var areAnyRunning = false;
        while (timeout < 10) // Gives 10 seconds to verify app is closed.
        {
            var alreadyClosed = new List<string>();
            var appCount = 0;
            timeout++;
            foreach (var p in procToClose)
            {
                var isProcRunning = Globals.ProcessHelper.IsProcessRunning(p);
                if (!isProcRunning)
                    alreadyClosed.Add(p); // Already closed, so remove from list after loop
                areAnyRunning = areAnyRunning || Globals.ProcessHelper.IsProcessRunning(p);
                if (areAnyRunning) appCount++;
            }

            foreach (var p in alreadyClosed)
                procToClose.Remove(p);

            if (procToClose.Count > 0)
                await JsRuntime.InvokeVoidAsync("updateStatus", Lang["Status_WaitingForMultipleClose", new { processName = procToClose[0], count = appCount, timeout, timeLimit = "10" }]);
            if (areAnyRunning)
                Thread.Sleep(1000);
            else
                break;
            areAnyRunning = false;
        }

        if (timeout != 10) return true; // Returns true if timeout wasn't reached.
#pragma warning disable CA1416 // Validate platform compatibility
        var leftOvers = procNames.Where(x => !Globals.ProcessHelper.IsProcessRunning(x));
#pragma warning restore CA1416 // Validate platform compatibility
        Toasts.ShowToastLang(ToastType.Error, "Error", new LangSub("CouldNotCloseX", new { x = string.Join(", ", leftOvers.ToArray()) }));
        return false; // Returns true if timeout wasn't reached.
    }

    #endregion


}