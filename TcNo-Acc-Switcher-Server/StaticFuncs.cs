using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Reflection;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using TcNo_Acc_Switcher_Globals;
using TextCopy;

namespace TcNo_Acc_Switcher_Server;

/// <summary>
/// A file containing a ton of static functions that are used everywhere.
/// This file shouldn't require anything else to be loaded in order to work properly.
/// </summary>
public static class StaticFuncs
{
    /// <summary>
    /// Is running with the official window, or just the server in a browser.
    /// </summary>
    public static bool IsTcNoClientApp => Assembly.GetEntryAssembly()?.Location.Contains("TcNo-Acc-Switcher-Client") ?? false;

    /// <summary>
    /// Restart the TcNo Account Switcher as Admin
    /// Launches either the Server or main exe, depending on what's currently running.
    /// </summary>
    public static void RestartAsAdmin(string args = "")
    {
        var fileName = "TcNo-Acc-Switcher_main.exe";
        if (!IsTcNoClientApp)
        {
            fileName = "TcNo-Acc-Switcher-Server.exe_main";
            // Is server app, but could be developing >> No _main just yet.
            if (!File.Exists(Path.Join(Globals.AppDataFolder, fileName)) && File.Exists(Path.Join(Globals.AppDataFolder, "TcNo-Acc-Switcher-Server.exe")))
                fileName = Path.Combine(Globals.AppDataFolder, "TcNo-Acc-Switcher-Server.exe");
        }
        else
        {
            // Is client app, but could be developing >> No _main just yet.
            if (!File.Exists(Path.Join(Globals.AppDataFolder, fileName)) && File.Exists(Path.Join(Globals.AppDataFolder, "TcNo-Acc-Switcher.exe")))
                fileName = Path.Combine(Globals.AppDataFolder, "TcNo-Acc-Switcher.exe");
        }

        var proc = new ProcessStartInfo
        {
            WorkingDirectory = Globals.AppDataFolder,
            FileName = fileName,
            UseShellExecute = true,
            Arguments = args,
            Verb = "runas"
        };
        try
        {
            _ = Process.Start(proc);
            Environment.Exit(0);
        }
        catch (Exception ex)
        {
            Globals.WriteToLog(@"This program must be run as an administrator!" + Environment.NewLine + ex);
            Environment.Exit(0);
        }
    }

    #region FILE_OPERATIONS
    public static string WwwRoot => Path.Join(Globals.UserDataFolder, "\\wwwroot");

    #endregion

    #region Clipboard
    public static void CopyText(string text) => ClipboardService.SetText(text);
    #endregion



    /// <summary>
    /// Saves shortcut order.
    /// </summary>
    public static Action<Dictionary<int, string>> SaveShortcuts;
    [JSInvokable]
    public static void JsSaveShortcuts(Dictionary<int, string> o)
    {
        SaveShortcuts.Invoke(o);
    }

    /// <summary>
    /// Saves platform settings.
    /// </summary>
    public static Action SaveSettings;
    [JSInvokable]
    public static void JsSaveSettings()
    {
        SaveSettings.Invoke();
    }


    /// <summary>
    /// Saves either the platform order, or account order, depending on what this is set to in the page's initializer.
    /// </summary>
    public static Action<string> SaveOrderAction;
    [JSInvokable]
    public static void GiSaveOrder(string s)
    {
        SaveOrderAction.Invoke(s);
    }

    /// <summary>
    /// Opens a link in user's browser through Shell
    /// </summary>
    /// <param name="link">URL string</param>
    [JSInvokable]
    public static void OpenLinkInBrowser(string link)
    {
        Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.OpenLinkInBrowser] link={link}");
        var ps = new ProcessStartInfo(link)
        {
            UseShellExecute = true,
            Verb = "open"
        };
        _ = Process.Start(ps);
    }


    [JSInvokable]
    public static string GiGetCleanFilePath(string f) => Globals.GetCleanFilePath(f);
}