using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Threading.Tasks;

namespace TcNo_Acc_Switcher_Server.Pages.General.Classes
{
    public class Shortcut
    {
        public string Exe { get; set; }
        public string WorkingDir { get; set; }
        public string IconDir { get; set; }
        public string ShortcutPath { get; set; }

        public string Desc { get; set; }
        public string Args { get; set; }

        public Shortcut()
        {
        }

        public Shortcut(string exe, string workingDir, string iconDir, string shortcutPath, string desc, string args)
        {
            Exe = exe;
            WorkingDir = workingDir;
            IconDir = iconDir;
            ShortcutPath = shortcutPath;
            Desc = desc;
            Args = args;
        }

        public static string Desktop = Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory);
        public static string StartMenu = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.Programs), @"TcNo Account Switcher\");

        // Library class used to see if a shortcut exists.
        public static string ParentDirectory(string dir) => dir.Substring(0, dir.LastIndexOf(Path.DirectorySeparatorChar));
        public bool ShortcutExist() => File.Exists(ShortcutPath);
        public string ShortcutDir() => Path.GetDirectoryName(ShortcutPath) ?? throw new InvalidOperationException();
        public string GetSelfPath() => System.Reflection.Assembly.GetEntryAssembly()?.Location.Replace(".dll", ".exe");



        private void WriteShortcut()
        {
            Directory.CreateDirectory(ShortcutDir());
            if (File.Exists(ShortcutPath)) return;
            if (File.Exists("CreateShortcut.vbs")) File.Delete("CreateShortcut.vbs");

            File.WriteAllLines("CreateShortcut.vbs", new[] {
                "set WshShell = WScript.CreateObject(\"WScript.Shell\")",
                "set oShellLink = WshShell.CreateShortcut(\"" + ShortcutPath + "\")",
                "oShellLink.TargetPath = \"" + Exe + "\"",
                "oShellLink.WindowStyle = 1",
                "oShellLink.IconLocation = \"" + IconDir + "\"",
                "oShellLink.Description = \"" + Desc + "\"",
                "oShellLink.WorkingDirectory = \"" + WorkingDir + "\"",
                "oShellLink.Arguments = \"" + Args + "\"",
                "oShellLink.Save()"
            });

            var vbsProcess = new Process
            {
                StartInfo =
                {
                    FileName = "cscript",
                    Arguments = "//nologo \"" + Path.GetFullPath("CreateShortcut.vbs") + "\"",
                    UseShellExecute = false,
                    RedirectStandardOutput = true,
                    CreateNoWindow = true
                }
            };

            vbsProcess.Start();
            vbsProcess.StandardOutput.ReadToEnd();
            vbsProcess.Close();

            File.Delete("CreateShortcut.vbs");
        }

        public void DeleteShortcut(bool delFolder)
        {
            if (File.Exists(ShortcutPath))
                File.Delete(ShortcutPath);
            if (!delFolder) return;
            if (Directory.GetFiles(ShortcutDir()).Length == 0)
                Directory.Delete(ShortcutDir());
        }

        public void ToggleShortcut(bool shouldExist, bool shouldFolderExist = true)
        {
            if (shouldExist && !ShortcutExist()) WriteShortcut();
            else if (!shouldExist  && ShortcutExist()) DeleteShortcut(!shouldFolderExist);
        }

        #region PROGRAM_SHORTCUTS
        public Shortcut Shortcut_Switcher(string location)
        {
            // Starts the main picker, with the Steam argument.
            Exe = GetSelfPath();
            WorkingDir = Directory.GetCurrentDirectory();
            IconDir = Path.Combine(WorkingDir, "wwwroot\\prog_icons\\steam.ico"); // To change soon
            ShortcutPath = Path.Combine(location, "TcNo Account Switcher.lnk");
            Desc = "TcNo Account Switcher";
            Args = "";
            return this;
        }
        #endregion

        #region STEAM_SHORTCUTS
        // Usage:
        // Shortcut steam = new Shortcut.Shortcut_Steam(Shortcut.Desktop);
        // if (!steam.ShortcutExist()) steam.WriteShortcut();
        // 
        public Shortcut Shortcut_Steam(string location)
        { 
            // Starts the main picker, with the Steam argument.
            Exe = GetSelfPath();
            WorkingDir = Directory.GetCurrentDirectory();
            IconDir = Path.Combine(WorkingDir, "wwwroot\\prog_icons\\steam.ico");
            ShortcutPath = Path.Combine(location, "TcNo Account Switcher - Steam.lnk");
            Desc = "TcNo Account Switcher - Steam";
            Args = "steam";
            return this;
        }
        public Shortcut Shortcut_SteamTray(string location)
        {
            Exe = Path.Combine(ParentDirectory(GetSelfPath()), "TcNo-Acc-Switcher-Tray.exe");
            WorkingDir = Directory.GetCurrentDirectory();
            IconDir = Path.Combine(WorkingDir, "wwwroot\\prog_icons\\steam.ico");
            ShortcutPath = Path.Combine(location, "TcNo Account Switcher - Steam tray.lnk");
            Desc = "TcNo Account Switcher - Steam tray";
            Args = "";
            return this;
        }
        


        #endregion

    }
}
