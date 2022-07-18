// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2022 TechNobo (Wesley Pyburn)
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
using System.Drawing;
using System.Drawing.Imaging;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Runtime.Versioning;
using System.Threading.Tasks;
using ImageMagick;
using Microsoft.AspNetCore.Components;
using SkiaSharp;
using Svg.Skia;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Data.Classes
{
    public class Shortcut
    {
        public string Exe { get; set; }
        public string WorkingDir { get; set; }
        public string IconDir { get; set; }
        public string ShortcutPath { get; set; }
        public bool ShortcutExist() => File.Exists(ShortcutPath);
        public string ShortcutDir() => Path.GetDirectoryName(ShortcutPath) ?? throw new InvalidOperationException();

        public string Desc { get; set; }
        public string Args { get; set; }


        /// <summary>
        /// Write Shortcut using WShell. Not too sure how else to do this, and it works.
        /// Probably will find another way of doing this at some stage.
        /// </summary>
        private void WriteShortcut()
        {
            Globals.DebugWriteLine(@"[Func:General\Classes\Shortcut.WriteShortcut]");
            _ = Directory.CreateDirectory(ShortcutDir());
            if (File.Exists(ShortcutPath)) return;
            Globals.DeleteFile("CreateShortcut.vbs");

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

            _ = vbsProcess.Start();
            _ = vbsProcess.StandardOutput.ReadToEnd();
            vbsProcess.Close();

            Globals.DeleteFile("CreateShortcut.vbs");
        }

        /// <summary>
        /// Delete shortcut if it exists
        /// </summary>
        /// <param name="delFolder">(Optional) Whether to delete parent folder if it's empty</param>
        public void DeleteShortcut(bool delFolder)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.DeleteShortcut] delFolder={delFolder}");
            Globals.DeleteFile(ShortcutPath);
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
            switch (shouldExist)
            {
                case true when !ShortcutExist():
                    WriteShortcut();
                    break;
                case false when ShortcutExist():
                    DeleteShortcut(!shouldFolderExist);
                    break;
            }
        }

        /// <summary>
        /// Write shortcut to file if doesn't already exist
        /// </summary>
        public void TryWrite()
        {
            Globals.DebugWriteLine(@"[Func:General\Classes\Shortcut.TryWrite]");
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
            Exe = ShortcutFuncs.GetSelfPath();
            WorkingDir = Directory.GetCurrentDirectory();
            IconDir = Path.Join(WorkingDir, "wwwroot\\prog_icons\\program.ico"); // To change soon
            ShortcutPath = Path.Join(location, "TcNo Account Switcher.lnk");
            Desc = "TcNo Account Switcher";
            Args = "";
            return this;
        }

        /// <summary>
        /// Sets up a Steam Tray shortcut
        /// </summary>
        /// <param name="location">Place to put shortcut</param>
        /// <returns></returns>
        public Shortcut Shortcut_Tray(string location)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.Shortcut_Tray] location={location}");
            Exe = Path.Join(ShortcutFuncs.ParentDirectory(ShortcutFuncs.GetSelfPath()), "TcNo-Acc-Switcher-Tray.exe");
            WorkingDir = Directory.GetCurrentDirectory();
            IconDir = Path.Join(WorkingDir, "wwwroot\\prog_icons\\program.ico");
            ShortcutPath = Path.Join(location, "TcNo Account Switcher - Tray.lnk");
            Desc = "TcNo Account Switcher - Tray";
            Args = "";
            return this;
        }

        /// <summary>
        /// Creates an icon file with multiple sizes, and combines the BG and FG images.
        /// </summary>
        /// <param name="generalFuncs"></param>
        /// <param name="appSettings"></param>
        /// <param name="bgImg">Background image, platform</param>
        /// <param name="fgImg">Foreground image, user image</param>
        /// <param name="iconName">Filename, unique so stored without being overwritten</param>
        [SupportedOSPlatform("windows")]
        public async Task CreateCombinedIcon(IGeneralFuncs generalFuncs, IAppSettings appSettings, string bgImg, string fgImg, string iconName)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.CreateCombinedIcon] bgImg={bgImg}, fgImg={fgImg.Substring(fgImg.Length - 6, 6)}, iconName=hidden");
            try
            {
                ShortcutFuncs.CreateIcon(generalFuncs, appSettings, bgImg, fgImg, ref iconName);
                IconDir = Path.GetFullPath(iconName);
            }
            catch (Exception e)
            {
                Globals.WriteToLog($"Failed to CreateIcon! '{bgImg}', '{fgImg}, '{iconName}'", e);
                await generalFuncs.ShowToastLangVars("error", "Toast_FailedCreateIcon");
            }
        }
        #endregion

        #region SPECIFIC_SHORTCUTS
        // Usage:
        // var s = new Shortcut();
        // s.Shortcut_Platform(Shortcut.Desktop, platformName = "Steam", args = "steam");
        // s.ToggleShortcut(!DesktopShortcut, true);

        /// <summary>
        /// Sets up a platform specific shortcut
        /// </summary>
        /// <param name="location">Place to put shortcut</param>
        /// <param name="platformName">(Optional) Name of the platform this shortcut is for (eg. steam)</param>
        /// <param name="args">(Optional) Arguments to add, default "steam" to open Steam page of switcher</param>
        /// <param name="descAdd">(Optional) Additional description to add to "TcNo Account Switcher - Steam</param>
        /// <param name="platformNameIsFullName">Whether the platformName is the fill name, or just to be appended</param>
        /// <returns></returns>
        public Shortcut Shortcut_Platform(string location, string platformName = "Steam", string args = "steam", string descAdd = "", bool platformNameIsFullName = false)
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Shortcut.Shortcut_Platform] location={location}, platformName={platformName}, descAdd={descAdd}, args={args}");
            // Starts the main picker, with the platform argument, eg: "steam", "origin".
            Exe = ShortcutFuncs.GetSelfPath();
            WorkingDir = Directory.GetCurrentDirectory();
            IconDir = Path.Join(WorkingDir, "wwwroot\\prog_icons\\program.ico"); // TODO: May add platform specific icons here at some point.
            ShortcutPath = Path.Join(location, (platformNameIsFullName ? platformName : $"{platformName} - TcNo Account Switcher") + ".lnk");
            Desc = $"TcNo Account Switcher - {platformName}" + descAdd != "" ? descAdd : "";
            Args = args;
            return this;
        }
        #endregion
    }
}
