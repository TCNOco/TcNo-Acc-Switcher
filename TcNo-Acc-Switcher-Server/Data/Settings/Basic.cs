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
                    // This has no self-auto creation as it requires a platform name from the Platform list.
                    return _instance ??= new Basic();
                }
            }
            set => _instance = value;
        }

        // Variables
        private string _folderPath = "";
        private bool _admin;
        private int _trayAccNumber = 3;
        private bool _forgetAccountEnabled;
        private bool _altClose;
        private Dictionary<int, string> _shortcuts = new();
        private bool _desktopShortcut;



        [JsonProperty("FolderPath", Order = 1)]
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

        [JsonProperty("Basic_Admin", Order = 2)] public static bool Admin { get => Instance._admin; set => Instance._admin = value; }
        [JsonProperty("Basic_TrayAccNumber", Order = 3)] public static int TrayAccNumber { get => Instance._trayAccNumber; set => Instance._trayAccNumber = value; }
        [JsonProperty("ForgetAccountEnabled", Order = 4)] public static bool ForgetAccountEnabled { get => Instance._forgetAccountEnabled; set => Instance._forgetAccountEnabled = value; }
        [JsonProperty("AltClose", Order = 5)] public static bool AltClose { get => Instance._altClose; set => Instance._altClose = value; }
        [JsonIgnore] public static Dictionary<int, string> Shortcuts { get => Instance._shortcuts; set => Instance._shortcuts = value; }
        [JsonProperty("ShortcutsJson", Order = 6)]
        static string ShortcutsJson // This HAS to be a string. Shortcuts is an object, and just adds keys instead of replacing it entirely. It doesn't save properly.
        {
            get => JsonConvert.SerializeObject(Instance._shortcuts);
            set => Instance._shortcuts = value == "{}" ? Instance._shortcuts : JsonConvert.DeserializeObject<Dictionary<int, string>>(value);
        }

        [JsonIgnore] public static bool DesktopShortcut { get => Instance._desktopShortcut; set => Instance._desktopShortcut = value; }

        [JsonIgnore] public static readonly string ContextMenuJson = $@"[
				{{""{Lang["Context_SwapTo"]}"": ""swapTo(-1, event)""}},
				{{""{Lang["Context_ChangeName"]}"": ""showModal('changeUsername')""}},
				{{""{Lang["Context_CreateShortcut"]}"": ""createShortcut()""}},
				{{""{Lang["Context_ChangeImage"]}"": ""changeImage(event)""}},
				{{""{Lang["Forget"]}"": ""forget(event)""}}
            ]";

        [JsonIgnore] public static readonly string ContextMenuShortcutJson = $@"[
				{{""{Lang["Context_RunAdmin"]}"": ""shortcut('admin')""}},
				{{""{Lang["Context_Hide"]}"": ""shortcut('hide')""}}
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

        public static void RunPlatform()
        {
            Globals.StartProgram(Exe(), Admin, CurrentPlatform.ExeExtraArgs);
            _ = GeneralInvocableFuncs.ShowToast("info", Lang["Status_StartingPlatform", new { platform = CurrentPlatform.SafeName }], duration: 15000, renderTo: "toastarea");
        }
        public static void RunShortcut(string s, bool admin = false)
        {
            var proc = new Process();
            proc.StartInfo = new ProcessStartInfo
            {
                FileName = Path.GetFullPath(Path.Join(CurrentPlatform.ShortcutFolder, s)),
                UseShellExecute = true,
                Verb = admin ? "runas" : ""
            };

            if (Globals.IsAdministrator && !admin)
            {
                proc.StartInfo.Arguments = proc.StartInfo.FileName;
                proc.StartInfo.FileName = "explorer.exe";
            }

            try
            {
                proc.Start();
                _ = GeneralInvocableFuncs.ShowToast("info", Lang["Status_StartingPlatform", new { platform = CurrentPlatform.RemoveShortcutExt(s) }], duration: 15000, renderTo: "toastarea");
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
                    RunShortcut(shortcut, true);
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
            AltClose = false;
            ShortcutsJson = "{}";

            SaveSettings();
        }
        public static void SetFromJObject(JObject j)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Basic.SetFromJObject]");
            var curSettings = j.ToObject<Basic>();
            if (curSettings == null) return;
            FolderPath = curSettings._folderPath;
            Admin = curSettings._admin;
            TrayAccNumber = curSettings._trayAccNumber;
            DesktopShortcut = Shortcut.CheckShortcuts(CurrentPlatform.FullName);
            AltClose = curSettings._altClose;
            ShortcutsJson = JsonConvert.SerializeObject(curSettings._shortcuts);
        }

        public static void LoadFromFile(string platformFile = "-")
        {
            if (platformFile == "-") platformFile = CurrentPlatform.SettingsFile;
            SetFromJObject(GeneralFuncs.LoadSettings(platformFile, JObject.FromObject(new Basic())));
        }
        public static void SaveSettings(bool mergeNewIntoOld = false) => GeneralFuncs.SaveSettings(CurrentPlatform.SettingsFile, JObject.FromObject(Instance), mergeNewIntoOld);
        #endregion
    }
}
