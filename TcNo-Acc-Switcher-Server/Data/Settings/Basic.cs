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
using System.IO;
using System.Linq;
using System.Reflection;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using SteamKit2.GC.Artifact.Internal;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;

namespace TcNo_Acc_Switcher_Server.Data.Settings
{
    public class Basic
    {
        private static readonly Lang Lang = Lang.Instance;
        private static Basic _instance = new();

        private static readonly object LockObj = new();
        public static Basic Instance
        {
            get
            {
                lock (LockObj)
                {
                    // Load settings if have changed, or not set
                    if (_instance is { _currentlyModifying: true }) return _instance;
                    if (_instance != new Basic() && Globals.GetFileMd5(CurrentPlatform.SettingsFile) == _instance._lastHash) return _instance;

                    _instance = new Basic { _currentlyModifying = true };

                    if (File.Exists(CurrentPlatform.SettingsFile))
                    {
                        _instance = JsonConvert.DeserializeObject<Basic>(File.ReadAllText(CurrentPlatform.SettingsFile), new JsonSerializerSettings() { });
                        _instance._lastHash = Globals.GetFileMd5(CurrentPlatform.SettingsFile);
                    }
                    else
                    {
                        SaveSettings();
                    }

                    _instance._desktopShortcut = Shortcut.CheckShortcuts(CurrentPlatform.FullName);
                    _instance._currentlyModifying = false;

                    return _instance;
                }
            }
            set => _instance = value;
        }
        private string _lastHash = "";
        private bool _currentlyModifying = false;

        public static void SaveSettings() => GeneralFuncs.SaveSettings(CurrentPlatform.SettingsFile, Instance);

        // Variables
        [JsonProperty("FolderPath", Order = 1)] private string _folderPath = "";
        [JsonProperty("Basic_Admin", Order = 2)] private bool _admin;
        [JsonProperty("Basic_TrayAccNumber", Order = 3)] private int _trayAccNumber = 3;
        [JsonProperty("ForgetAccountEnabled", Order = 4)] private bool _forgetAccountEnabled;
        [JsonProperty("ShortcutsJson", Order = 5)] private Dictionary<int, string> _shortcuts = new();
        [JsonIgnore] private bool _desktopShortcut;


        public static string FolderPath
        {
            get
            {
                if (!string.IsNullOrEmpty(Instance._folderPath)) return Instance._folderPath;
                Instance._folderPath = CurrentPlatform.DefaultFolderPath;

                return Instance._folderPath;
            }
            set => Instance._folderPath = value;
        }

        public static bool Admin { get => Instance._admin; set => Instance._admin = value; }
        public static int TrayAccNumber { get => Instance._trayAccNumber; set => Instance._trayAccNumber = value; }
        public static bool ForgetAccountEnabled { get => Instance._forgetAccountEnabled; set => Instance._forgetAccountEnabled = value; }
        public static Dictionary<int, string> Shortcuts { get => Instance._shortcuts; set => Instance._shortcuts = value; }

        public static bool DesktopShortcut { get => Instance._desktopShortcut; set => Instance._desktopShortcut = value; }

        public static readonly string ContextMenuJson = $@"[
				{{""{Lang["Context_SwapTo"]}"": ""swapTo(-1, event)""}},
				{{""{Lang["Context_ChangeName"]}"": ""showModal('changeUsername')""}},
				{{""{Lang["Context_CreateShortcut"]}"": ""createShortcut()""}},
				{{""{Lang["Context_ChangeImage"]}"": ""changeImage(event)""}},
				{{""{Lang["Forget"]}"": ""forget(event)""}}
            ]";

        public static readonly string ContextMenuShortcutJson = $@"[
				{{""{Lang["Context_RunAdmin"]}"": ""shortcut('admin')""}},
				{{""{Lang["Context_Hide"]}"": ""shortcut('hide')""}}
            ]";

        public static readonly string ContextMenuPlatformJson = $@"[
				{{""{Lang["Context_RunAdmin"]}"": ""shortcut('admin')""}}
            ]";

        /// <summary>
        /// Updates the ForgetAccountEnabled bool in settings file
        /// </summary>
        /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
        public static void SetForgetAcc(bool enabled)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Basic.SetForgetAcc]");
            if (ForgetAccountEnabled == enabled) return; // Ignore if already set
            ForgetAccountEnabled = enabled;
            SaveSettings();
        }

        /// <summary>
        /// Get Basic.exe path from BasicSettings.json
        /// </summary>
        /// <returns>Basic.exe's path string</returns>
        public static string Exe() => Path.Join(FolderPath, CurrentPlatform.ExeName);

        [JSInvokable]
        public static void SaveShortcutOrder(Dictionary<int, string> o)
        {
            Shortcuts = o;
            SaveSettings();
        }
        public static void OpenFolder(string folder)
        {
            Directory.CreateDirectory(folder); // Create if doesn't exist
            Process.Start("explorer.exe", folder);
            _ = GeneralInvocableFuncs.ShowToast("info", Lang["Toast_PlaceShortcutFiles"], renderTo: "toastarea");
        }

        public static void RunPlatform(string exePath, bool admin, string args, string platName)
        {
            if (Globals.StartProgram(exePath, admin, args))
                _ = GeneralInvocableFuncs.ShowToast("info", Lang["Status_StartingPlatform", new { platform = platName }], renderTo: "toastarea");
            else
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_StartingPlatformFailed", new { platform = platName }], renderTo: "toastarea");
        }


        public static void RunPlatform(bool admin)
        {
            if (Globals.StartProgram(Exe(), admin, CurrentPlatform.ExeExtraArgs))
                _ = GeneralInvocableFuncs.ShowToast("info", Lang["Status_StartingPlatform", new { platform = CurrentPlatform.SafeName }], renderTo: "toastarea");
            else
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_StartingPlatformFailed", new { platform = CurrentPlatform.SafeName }], renderTo: "toastarea");
        }
        public static void RunPlatform()
        {
            Globals.StartProgram(Exe(), Admin, CurrentPlatform.ExeExtraArgs);
            _ = GeneralInvocableFuncs.ShowToast("info", Lang["Status_StartingPlatform", new { platform = CurrentPlatform.SafeName }], renderTo: "toastarea");
        }
        public static void RunShortcut(string s, string shortcutFolder = "", bool admin = false)
        {
            if (shortcutFolder == "")
                shortcutFolder = CurrentPlatform.ShortcutFolder;
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
                if (admin) _ = GeneralInvocableFuncs.ShowToast("warning", Lang["Toast_UrlAdminErr"], duration: 15000, renderTo: "toastarea");
            }
            else if (Globals.IsAdministrator && !admin)
            {
                proc.StartInfo.Arguments = proc.StartInfo.FileName;
                proc.StartInfo.FileName = "explorer.exe";
            }

            try
            {
                proc.Start();
                _ = GeneralInvocableFuncs.ShowToast("info", Lang["Status_StartingPlatform", new { platform = PlatformFuncs.RemoveShortcutExt(s) }], renderTo: "toastarea");
            }
            catch (Exception e)
            {
                // Cancelled by user, or another error.
                Globals.WriteToLog($"Tried to start \"{s}\" but failed.", e);
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Status_FailedLog"], duration: 15000, renderTo: "toastarea");
            }
        }

        [JSInvokable]
        public static void HandleShortcutAction(string shortcut, string action)
        {
            if (shortcut == "btnStartPlat") // Start platform requested
            {
                RunPlatform(action == "admin");
                return;
            }

            if (!Shortcuts.ContainsValue(shortcut)) return;

            switch (action)
            {
                case "hide":
                {
                    // Remove shortcut from folder, and list.
                    Shortcuts.Remove(Shortcuts.First(e => e.Value == shortcut).Key);
                    var f = Path.Join(CurrentPlatform.ShortcutFolder, shortcut);
                    if (File.Exists(f)) File.Move(f, f.Replace(".lnk", "_ignored.lnk").Replace(".url", "_ignored.url"));

                    // Save.
                    SaveSettings();
                    break;
                }
                case "admin":
                    RunShortcut(shortcut, admin: true);
                    break;
            }
        }

        #region SETTINGS
        /// <summary>
        /// </summary>
        public static void ResetSettings()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Basic.ResetSettings]");
            FolderPath = CurrentPlatform.DefaultFolderPath;
            Admin = false;
            TrayAccNumber = 3;
            DesktopShortcut = Shortcut.CheckShortcuts(CurrentPlatform.FullName);

            SaveSettings();
        }
        #endregion
    }
}
