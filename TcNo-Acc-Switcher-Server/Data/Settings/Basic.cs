// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2024 TroubleChute (Wesley Pyburn)
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
using System.Collections.ObjectModel;
using System.Diagnostics;
using System.IO;
using System.Linq;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using TcNo_Acc_Switcher_Server.Shared;
using TcNo_Acc_Switcher_Server.Shared.Accounts;
using TcNo_Acc_Switcher_Server.Shared.ContextMenu;

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
                        _instance = JsonConvert.DeserializeObject<Basic>(File.ReadAllText(CurrentPlatform.SettingsFile), new JsonSerializerSettings());
                        if (_instance == null)
                        {
                            _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_FailedLoadSettings"]);
                            if (File.Exists(CurrentPlatform.SettingsFile))
                                Globals.CopyFile(CurrentPlatform.SettingsFile, CurrentPlatform.SettingsFile.Replace(".json", ".old.json"));
                            _instance = new Basic { _currentlyModifying = true };
                        }
                        _instance._lastHash = Globals.GetFileMd5(CurrentPlatform.SettingsFile);
                        if (_instance._folderPath.EndsWith(".exe"))
                            _instance._folderPath = Path.GetDirectoryName(_instance._folderPath) ?? string.Join("\\", _instance._folderPath.Split("\\")[..^1]);
                    }
                    else
                    {
                        SaveSettings();
                    }

                    BuildContextMenu();

                    _instance._desktopShortcut = Shortcut.CheckShortcuts(CurrentPlatform.SafeName);
                    AppData.InitializedClasses.Basic = true;

                    _instance._currentlyModifying = false;

                    return _instance;
                }
            }
            set
            {
                lock (LockObj)
                {
                    _instance = value;
                }
            }
        }
        private string _lastHash = "";
        private bool _currentlyModifying;

        public static void SaveSettings() => GeneralFuncs.SaveSettings(CurrentPlatform.SettingsFile, Instance);

        // Variables
        [JsonProperty("FolderPath", Order = 1)] private string _folderPath = "";
        [JsonProperty("Basic_Admin", Order = 2)] private bool _admin;
        [JsonProperty("Basic_TrayAccNumber", Order = 3)] private int _trayAccNumber = 3;
        [JsonProperty("ForgetAccountEnabled", Order = 4)] private bool _forgetAccountEnabled;
        [JsonProperty("ShortcutsJson", Order = 5)] private Dictionary<int, string> _shortcuts = new();
        [JsonProperty("ClosingMethod", Order = 6)] private string _closingMethod = "";
        [JsonProperty("StartingMethod", Order = 7)] private string _startingMethod = "";
        [JsonProperty("AutoStart", Order = 8)] private bool _autoStart = true;
        [JsonProperty("ShowShortNotes", Order = 9)] private bool _showShortNotes = true;
        [JsonProperty("AccountNotes", Order = 10)] private Dictionary<string, string> _accountNotes = new();
        [JsonIgnore] private bool _desktopShortcut;
        [JsonIgnore] private int _lastAccTimestamp = 0;
        [JsonIgnore] private string _lastAccName = "";
        [JsonIgnore] private ObservableCollection<Account> _accounts = new();

        public static int LastAccTimestamp { get => Instance._lastAccTimestamp; set => Instance._lastAccTimestamp = value; }
        public static string LastAccName { get => Instance._lastAccName; set => Instance._lastAccName = value; }


        public static string FolderPath
        {
            get
            {
                if (!string.IsNullOrWhiteSpace(Instance._folderPath)) return Instance._folderPath;
                Instance._folderPath = CurrentPlatform.DefaultFolderPath;

                return Instance._folderPath;
            }
            set => Instance._folderPath = value;
        }

        public static bool Admin { get => Instance._admin; set => Instance._admin = value; }
        public static bool AutoStart { get => Instance._autoStart; set => Instance._autoStart = value; }
        public static int TrayAccNumber { get => Instance._trayAccNumber; set => Instance._trayAccNumber = value; }
        public static bool ForgetAccountEnabled { get => Instance._forgetAccountEnabled; set => Instance._forgetAccountEnabled = value; }
        public static Dictionary<int, string> Shortcuts { get => Instance._shortcuts; set => Instance._shortcuts = value; }
        public static bool ShowShortNotes { get => Instance._showShortNotes; set => Instance._showShortNotes = value; }
        public static Dictionary<string, string> AccountNotes { get => Instance._accountNotes; set => Instance._accountNotes = value; }
        public static ObservableCollection<Account> Accounts { get => Instance._accounts; set => Instance._accounts = value; }
        public static string ClosingMethod
        {
            get
            {
                if (!string.IsNullOrWhiteSpace(Instance._closingMethod)) return Instance._closingMethod;
                Instance._closingMethod = CurrentPlatform.ClosingMethod;

                return Instance._closingMethod;
            }
            set => Instance._closingMethod = value;
        }

        public static string StartingMethod {
            get
            {
                if (!string.IsNullOrWhiteSpace(Instance._startingMethod)) return Instance._startingMethod;
                Instance._startingMethod = CurrentPlatform.StartingMethod;

                return Instance._startingMethod;
            }
            set => Instance._startingMethod = value;
        }
        public static bool DesktopShortcut { get => Instance._desktopShortcut; set => Instance._desktopShortcut = value; }
        public static readonly ObservableCollection<MenuItem> ContextMenuItems = new();
        private static void BuildContextMenu()
        {
            ContextMenuItems.Clear();
            ContextMenuItems.AddRange(new MenuBuilder(
                new []
                {
                    new ("Context_SwapTo", "swapTo(-1, event)"),
                    new ("Context_ChangeName", "showModal('changeUsername')"),
                    new ("Context_CreateShortcut", "createShortcut()"),
                    new ("Context_ChangeImage", "changeImage(event)"),
                    new ("Forget", "forget(event)"),
                    new ("Notes", "showNotes(event)"),
                    BasicStats.PlatformHasAnyGames(CurrentPlatform.SafeName) ?
                        new Tuple<string, object>("Context_ManageGameStats", "ShowGameStatsSetup(event)") : null,
                }).Result());
        }

        public static readonly ObservableCollection<MenuItem> ContextMenuShortcutItems = new MenuBuilder(
            new Tuple<string, object>[]
        {
            new ("Context_RunAdmin", "shortcut('admin')"),
            new ("Context_Hide", "shortcut('hide')"),
        }).Result();

        public static readonly ObservableCollection<MenuItem> ContextMenuPlatformItems = new MenuBuilder(
            new Tuple<string, object>("Context_RunAdmin", "shortcut('admin')")
        ).Result();

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

        public static void SetClosingMethod(string method)
        {
            ClosingMethod = method;
            SaveSettings();
        }
        public static void SetStartingMethod(string method)
        {
            StartingMethod = method;
            SaveSettings();
        }
        public static void OpenFolder(string folder)
        {
            Directory.CreateDirectory(folder); // Create if doesn't exist
            Process.Start("explorer.exe", folder);
            _ = GeneralInvocableFuncs.ShowToast("info", Lang["Toast_PlaceShortcutFiles"], renderTo: "toastarea");
        }

        public static void RunPlatform(string exePath, bool admin, string args, string platName, string startingMethod = "Default")
        {
            _ = Globals.StartProgram(exePath, admin, args, startingMethod)
                ? GeneralInvocableFuncs.ShowToast("info", Lang["Status_StartingPlatform", new {platform = platName}], renderTo: "toastarea")
                : GeneralInvocableFuncs.ShowToast("error", Lang["Toast_StartingPlatformFailed", new {platform = platName}], renderTo: "toastarea");
        }


        public static void RunPlatform(bool admin)
        {
            _ = Globals.StartProgram(Exe(), admin, CurrentPlatform.ExeExtraArgs, CurrentPlatform.StartingMethod)
                ? GeneralInvocableFuncs.ShowToast("info", Lang["Status_StartingPlatform", new {platform = CurrentPlatform.SafeName}], renderTo: "toastarea")
                : GeneralInvocableFuncs.ShowToast("error", Lang["Toast_StartingPlatformFailed", new {platform = CurrentPlatform.SafeName}], renderTo: "toastarea");
        }
        public static void RunPlatform()
        {
            Globals.StartProgram(Exe(), Admin, CurrentPlatform.ExeExtraArgs, CurrentPlatform.StartingMethod);
            _ = GeneralInvocableFuncs.ShowToast("info", Lang["Status_StartingPlatform", new { platform = CurrentPlatform.SafeName }], renderTo: "toastarea");
        }
        public static void RunShortcut(string s, string shortcutFolder = "", bool admin = false, string platform = "")
        {
            AppStats.IncrementGameLaunches(platform);

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
            DesktopShortcut = Shortcut.CheckShortcuts(CurrentPlatform.SafeName);

            SaveSettings();
        }
        #endregion
    }
}
