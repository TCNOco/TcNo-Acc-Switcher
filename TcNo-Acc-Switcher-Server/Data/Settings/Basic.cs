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

        [JsonProperty("FolderPath", Order = 1)]
        public string FolderPath
        {
            get
            {
                if (_instance._folderPath != "") return _instance._folderPath;
                _instance._folderPath = CurrentPlatform.Instance.DefaultFolderPath;

                return _instance._folderPath;
            }
            set => _instance._folderPath = value;
        }

        private bool _admin;
        [JsonProperty("Basic_Admin", Order = 2)] public bool Admin { get => _instance._admin; set => _instance._admin = value; }
        private int _trayAccNumber = 3;
        [JsonProperty("Basic_TrayAccNumber", Order = 3)] public int TrayAccNumber { get => _instance._trayAccNumber; set => _instance._trayAccNumber = value; }
        private bool _forgetAccountEnabled;
        [JsonProperty("ForgetAccountEnabled", Order = 4)] public bool ForgetAccountEnabled { get => _instance._forgetAccountEnabled; set => _instance._forgetAccountEnabled = value; }
        private bool _altClose;
        [JsonProperty("AltClose", Order = 5)] public bool AltClose { get => _instance._altClose; set => _instance._altClose = value; }
        private Dictionary<int, string> _shortcuts = new();
        [JsonIgnore] public Dictionary<int, string> Shortcuts { get => _instance._shortcuts; set => _instance._shortcuts = value; }
        [JsonProperty("ShortcutsJson", Order = 6)]
        string ShortcutsJson // This HAS to be a string. Shortcuts is an object, and just adds keys instead of replacing it entirely. It doesn't save properly.
        {
            get => JsonConvert.SerializeObject(_instance._shortcuts);
            set => _instance._shortcuts = value == "{}" ? _instance._shortcuts : JsonConvert.DeserializeObject<Dictionary<int, string>>(value);
        }
        //[JsonProperty("Shortcuts", Order = 6)]
        //List<object> ShortcutsJson
        //{
        //    get => _instance._shortcuts.Cast<object>().ToList();
        //    set
        //    {
        //        if (value.Count == 0) return;
        //        var newList = value.Cast<KeyValuePair<int, string>>().ToList();
        //        _instance._shortcuts = newList.ToDictionary(x => x.Key, x => x.Value);
        //    }
        //}


        private bool _desktopShortcut;
        [JsonIgnore] public bool DesktopShortcut { get => _instance._desktopShortcut; set => _instance._desktopShortcut = value; }

        [JsonIgnore] public readonly string ContextMenuJson = $@"[
				{{""{Lang["Context_SwapTo"]}"": ""swapTo(-1, event)""}},
				{{""{Lang["Context_ChangeName"]}"": ""showModal('changeUsername')""}},
				{{""{Lang["Context_CreateShortcut"]}"": ""createShortcut()""}},
				{{""{Lang["Context_ChangeImage"]}"": ""changeImage(event)""}},
				{{""{Lang["Forget"]}"": ""forget(event)""}}
            ]";

        [JsonIgnore] public readonly string ContextMenuShortcutJson = $@"[
				{{""{Lang["Context_RunAdmin"]}"": ""shortcut('admin')""}},
				{{""{Lang["Context_Hide"]}"": ""shortcut('hide')""}}
            ]";

        /// <summary>
        /// Updates the ForgetAccountEnabled bool in settings file
        /// </summary>
        /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
        public void SetForgetAcc(bool enabled)
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
        public string Exe() => Path.Join(FolderPath, CurrentPlatform.Instance.ExeName);

        [JSInvokable]
        public static void SaveShortcutOrder(Dictionary<int, string> o)
        {
            Instance.Shortcuts = o;
            Instance.SaveSettings();
        }

        public static void RunPlatform()
        {
            Globals.StartProgram(Data.Settings.Basic.Instance.Exe(), Data.Settings.Basic.Instance.Admin,
                CurrentPlatform.Instance.ExeExtraArgs);
            _ = GeneralInvocableFuncs.ShowToast("info", Lang["Status_StartingPlatform", new { platform = CurrentPlatform.Instance.SafeName }], duration: 15000, renderTo: "toastarea");
        }
        public static void RunShortcut(string s, bool admin = false)
        {
            var proc = new Process();
            proc.StartInfo = new ProcessStartInfo
            {
                FileName = Path.GetFullPath(Path.Join(CurrentPlatform.Instance.ShortcutFolder, s)),
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
            if (!Instance.Shortcuts.ContainsValue(shortcut)) return;
            switch (action)
            {
                case "hide":
                {
                    // Remove shortcut from folder, and list.
                    Instance.Shortcuts.Remove(Instance.Shortcuts.First(e => e.Value == shortcut).Key);
                    var f = Path.Join(CurrentPlatform.Instance.ShortcutFolder, shortcut);
                    if (File.Exists(f)) File.Move(f, f.Replace(".lnk", "_ignored.lnk").Replace(".url", "_ignored.url"));

                    // Save.
                    Instance.SaveSettings();
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
        public void ResetSettings()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Basic.ResetSettings]");
            _instance.FolderPath = CurrentPlatform.Instance.DefaultFolderPath;
            _instance.Admin = false;
            _instance.TrayAccNumber = 3;
            _instance._desktopShortcut = Shortcut.CheckShortcuts(CurrentPlatform.Instance.FullName);
            _instance._altClose = false;
            ShortcutsJson = "{}";

            SaveSettings();
        }
        public void SetFromJObject(JObject j)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Basic.SetFromJObject]");
            var curSettings = j.ToObject<Basic>();
            if (curSettings == null) return;
            _instance.FolderPath = curSettings.FolderPath;
            _instance.Admin = curSettings.Admin;
            _instance.TrayAccNumber = curSettings.TrayAccNumber;
            _instance._desktopShortcut = Shortcut.CheckShortcuts(CurrentPlatform.Instance.FullName);
            _instance._altClose = curSettings.AltClose;
            ShortcutsJson = curSettings.ShortcutsJson;
        }

        public void LoadFromFile(string platformFile = "-")
        {
            if (platformFile == "-") platformFile = CurrentPlatform.Instance.SettingsFile;
            SetFromJObject(GeneralFuncs.LoadSettings(platformFile, GetJObject()));
        }
        public JObject GetJObject() => JObject.FromObject(this);
        [JSInvokable]
        public void SaveSettings(bool mergeNewIntoOld = false) => GeneralFuncs.SaveSettings(CurrentPlatform.Instance.SettingsFile, GetJObject(), mergeNewIntoOld);
        #endregion
    }
}
