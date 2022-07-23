using System;
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
public class StaticFuncs
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
        if (!IsTcNoClientApp) fileName = "TcNo-Acc-Switcher-Server_main.exe";
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

    [JSInvokable]
    public static async Task CopyText(string text) => await ClipboardService.SetTextAsync(text);
    #endregion
}