using System;
using System.Diagnostics;
using System.IO;
using System.Threading.Tasks;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Data.Settings;

internal static class BasicFuncs
{
    public static async Task OpenFolder(IGeneralFuncs generalFuncs, string folder)
    {
        Directory.CreateDirectory(folder); // Create if doesn't exist
        Process.Start("explorer.exe", folder);
        await generalFuncs.ShowToastLangVars("info", "Toast_PlaceShortcutFiles", renderTo: "toastarea");
    }

    public static void RunPlatform(IGeneralFuncs generalFuncs, string exePath, bool admin, string args,
        string platName, string startingMethod = "Default")
    {
        _ = Globals.StartProgram(exePath, admin, args, startingMethod)
            ? generalFuncs.ShowToastLangVars("info",
                new LangItem("Status_StartingPlatform", new {platform = platName}), renderTo: "toastarea")
            : generalFuncs.ShowToastLangVars("error",
                new LangItem("Toast_StartingPlatformFailed", new {platform = platName}), renderTo: "toastarea");
    }

    public static async Task RunShortcut(IGeneralFuncs generalFuncs, ICurrentPlatform currentPlatform,
        IAppStats appStats, string s, string shortcutFolder = "", bool admin = false, string platform = "")
    {
        appStats.IncrementGameLaunches(platform);

        if (shortcutFolder == "")
            shortcutFolder = currentPlatform.ShortcutFolder;
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
                await generalFuncs.ShowToastLangVars("warning", "Toast_UrlAdminErr", duration: 15000,
                    renderTo: "toastarea");
        }
        else if (Globals.IsAdministrator && !admin)
        {
            proc.StartInfo.Arguments = proc.StartInfo.FileName;
            proc.StartInfo.FileName = "explorer.exe";
        }

        try
        {
            proc.Start();
            await generalFuncs.ShowToastLangVars("info",
                new LangItem("Status_StartingPlatform", new {platform = PlatformFuncs.RemoveShortcutExt(s)}),
                renderTo: "toastarea");
        }
        catch (Exception e)
        {
            // Cancelled by user, or another error.
            Globals.WriteToLog($"Tried to start \"{s}\" but failed.", e);
            await generalFuncs.ShowToastLangVars("error", "Status_FailedLog", duration: 15000,
                renderTo: "toastarea");
        }
    }
}