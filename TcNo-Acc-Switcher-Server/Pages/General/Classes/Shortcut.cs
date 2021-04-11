// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2021 TechNobo (Wesley Pyburn)
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using TcNo_Acc_Switcher_Globals;

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

        public static string Desktop = Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory);
        public static string StartMenu = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.Programs), @"TcNo Account Switcher\");

        // Library class used to see if a shortcut exists.
        public static string ParentDirectory(string dir) => dir.Substring(0, dir.LastIndexOf(Path.DirectorySeparatorChar));
        public bool ShortcutExist() => File.Exists(ShortcutPath);
        public string ShortcutDir() => Path.GetDirectoryName(ShortcutPath) ?? throw new InvalidOperationException();
        public string GetSelfPath() => System.Reflection.Assembly.GetEntryAssembly()?.Location.Replace(".dll", ".exe");


        /// <summary>
        /// Write Shortcut using WShell. Not too sure how else to do this, and it works.
        /// Probably will find another way of doing this at some stage.
        /// </summary>
        private void WriteShortcut()
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.WriteShortcut]");
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

        /// <summary>
        /// Delete shortcut if it exists
        /// </summary>
        /// <param name="delFolder">(Optional) Whether to delete parent folder if it's enpty</param>
        public void DeleteShortcut(bool delFolder)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.DeleteShortcut] delFolder={delFolder}");
            if (File.Exists(ShortcutPath))
                File.Delete(ShortcutPath);
            if (!delFolder) return;
            if (Directory.GetFiles(ShortcutDir()).Length == 0)
                Directory.Delete(ShortcutDir());
        }

        /// <summary>
        /// Toggles whether a shortcut exists (Creates/Deletes depending on state)
        /// </summary>
        /// <param name="shouldExist">Whether the shortcut SHOULD exist</param>
        /// <param name="shouldFolderExist">Whether the shortcut ALREADY Exists</param>
        public void ToggleShortcut(bool shouldExist, bool shouldFolderExist = true)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.ToggleShortcut] shouldExist={shouldExist}, shouldFolderExist={shouldFolderExist}");
            if (shouldExist && !ShortcutExist()) WriteShortcut();
            else if (!shouldExist  && ShortcutExist()) DeleteShortcut(!shouldFolderExist);
        }

        /// <summary>
        /// Write shortcut to file if doesn't already exist
        /// </summary>
        public void TryWrite()
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.TryWrite]");
            if (!ShortcutExist()) WriteShortcut();
        }

        #region PROGRAM_SHORTCUTS
        /// <summary>
        /// Fills in necessary info to create a shortcut to the TcNo Account Switcher
        /// </summary>
        /// <param name="location"></param>
        /// <returns></returns>
        public Shortcut Shortcut_Switcher(string location)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.Shortcut_Switcher] location={location}");
            // Starts the main picker, with the Steam argument.
            Exe = GetSelfPath();
            WorkingDir = Directory.GetCurrentDirectory();
            IconDir = Path.Combine(WorkingDir, "wwwroot\\prog_icons\\steam.ico"); // To change soon
            ShortcutPath = Path.Combine(location, "TcNo Account Switcher.lnk");
            Desc = "TcNo Account Switcher";
            Args = "";
            return this;
        }

        /// <summary>
        /// Creates an icon file with multiple sizes, and combines the BG and FG images.
        /// </summary>
        /// <param name="bgImg">Background image, platform</param>
        /// <param name="fgImg">Foreground image, user image</param>
        /// <param name="iconName">Filename, unique so stored without being overwritten</param>
        public void CreateCombinedIcon(string bgImg, string fgImg, string iconName)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.CreateCombinedIcon] bgImg={bgImg}, fgImg={fgImg.Substring(fgImg.Length - 6, 6)}, iconName=hidden");
            IconFactory.CreateIcon(bgImg, fgImg, ref iconName);
            IconDir = Path.GetFullPath(iconName);
        }
        #endregion

        #region STEAM_SHORTCUTS
        // Usage:
        // var s = new Shortcut();
        // s.Shortcut_Steam(Shortcut.Desktop);
        // s.ToggleShortcut(!DesktopShortcut, true);

        /// <summary>
        /// Sets up Steam shortcut
        /// </summary>
        /// <param name="location">Place to put shortcut</param>
        /// <param name="shortcutName">(Optional) Full name for shortcut FILE</param>
        /// <param name="descAdd">(Optional) Additional description to add to "TcNo Account Switcher - Steam</param>
        /// <param name="args">(Optional) Arguments to add, default "steam" to open Steam page of switcher</param>
        /// <returns></returns>
        public Shortcut Shortcut_Steam(string location, string shortcutName = "TcNo Account Switcher - Steam.lnk", string descAdd = "", string args = "steam")
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.Shortcut_Steam] location={location}, shortcutName={shortcutName}, descAdd={descAdd}, args={args}");
            // Starts the main picker, with the Steam argument.
            Exe = GetSelfPath();
            WorkingDir = Directory.GetCurrentDirectory();
            IconDir = Path.Combine(WorkingDir, "wwwroot\\prog_icons\\steam.ico");
            ShortcutPath = Path.Combine(location, shortcutName);
            Desc = "TcNo Account Switcher - Steam" + descAdd != "" ? descAdd : "";
            Args = args;
            return this;
        }
        /// <summary>
        /// Sets up a Steam Tray shortcut
        /// </summary>
        /// <param name="location">Place to put shortcut</param>
        /// <returns></returns>
        public Shortcut Shortcut_SteamTray(string location)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.Shortcut_SteamTray] location={location}");
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
