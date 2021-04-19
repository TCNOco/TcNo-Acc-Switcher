using System;
using System.Collections.Generic;
using System.Drawing;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;

namespace TcNo_Acc_Switcher_Server.Data.Settings
{
    public class Origin
    {
        private static Origin _instance = new Origin();

        public Origin() { }
        private static readonly object LockObj = new();
        public static Origin Instance
        {
            get
            {
                lock (LockObj)
                {
                    return _instance ??= new Origin();
                }
            }
            set => _instance = value;
        }

        // Variables
        private string _folderPath = "C:\\Program Files (x86)\\Origin\\";
        [JsonProperty("FolderPath", Order = 1)] public string FolderPath { get => _instance._folderPath; set => _instance._folderPath = value; }
        private Point _windowSize = new() { X = 800, Y = 450 };
        [JsonProperty("WindowSize", Order = 1)] public Point WindowSize { get => _instance._windowSize; set => _instance._windowSize = value; }
        private bool _admin;
        [JsonProperty("Origin_Admin", Order = 2)] public bool Admin { get => _instance._admin; set => _instance._admin = value; }
        private int _trayAccNumber = 3;
        [JsonProperty("Origin_TrayAccNumber", Order = 3)] public int TrayAccNumber { get => _instance._trayAccNumber; set => _instance._trayAccNumber = value; }
        private bool _forgetAccountEnabled;
        [JsonProperty("ForgetAccountEnabled", Order = 4)] public bool ForgetAccountEnabled { get => _instance._forgetAccountEnabled; set => _instance._forgetAccountEnabled = value; }

        private bool _desktopShortcut;
        [JsonIgnore] public bool DesktopShortcut { get => _instance._desktopShortcut; set => _instance._desktopShortcut = value; }

        // Constants
        [JsonIgnore] public string SettingsFile = "OriginSettings.json";
        [JsonIgnore] public string OriginImagePath = "wwwroot/img/profiles/origin/";
        [JsonIgnore] public string OriginImagePathHtml = "img/profiles/origin/";
        //[JsonIgnore] public string ContextMenuJson = @"[
        //      {""Swap to account"": ""SwapTo(-1, event)""},
        //      {""Login as..."": [
        //        {""Invisible"": ""SwapTo(10, event)""},
        //      ]},
        //      {""Create Desktop Shortcut"": ""CreateShortcut()""},
        //      {""Forget"": ""forget(event)""}
        //    ]";
        [JsonIgnore] public string ContextMenuJson = @"[
              {""Swap to account"": ""SwapTo(-1, event)""},
              {""Forget"": ""forget(event)""}
            ]";


        /// <summary>
        /// Updates the ForgetAccountEnabled bool in settings file
        /// </summary>
        /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
        public void UpdateOriginForgetAcc(bool enabled)
        {
            Globals.DebugWriteLine($@"[Func:Data\Settings\Origin.UpdateOriginForgetAcc]");
            if (ForgetAccountEnabled == enabled) return; // Ignore if already set
            ForgetAccountEnabled = enabled;
            SaveSettings();
        }

        /// <summary>
        /// Get Origin.exe path from OriginSettings.json 
        /// </summary>
        /// <returns>Origin.exe's path string</returns>
        public string Exe() => Path.Combine(FolderPath, "Origin.exe");


        #region SETTINGS
        /// <summary>
        /// Default settings for OriginSettings.json
        /// </summary>
        public void ResetSettings()
        {
            Globals.DebugWriteLine($@"[Func:Data\Settings\Origin.ResetSettings]");
            _instance.FolderPath = "C:\\Program Files (x86)\\Origin\\";
            _instance.WindowSize = new Point() { X = 800, Y = 450 };
            _instance.Admin = false;
            _instance.TrayAccNumber = 3;

            CheckShortcuts();

            SaveSettings();
        }
        public void SetFromJObject(JObject j)
        {
            Globals.DebugWriteLine($@"[Func:Data\Settings\Origin.SetFromJObject]");
            var curSettings = j.ToObject<Origin>();
            if (curSettings == null) return;
            _instance.FolderPath = curSettings.FolderPath;
            _instance.WindowSize = curSettings.WindowSize;
            _instance.Admin = curSettings.Admin;
            _instance.TrayAccNumber = curSettings.TrayAccNumber;

            CheckShortcuts();
        }
        public void LoadFromFile() => SetFromJObject(GeneralFuncs.LoadSettings(SettingsFile, GetJObject()));
        public JObject GetJObject() => JObject.FromObject(this);
        [JSInvokable]
        public void SaveSettings(bool mergeNewIntoOld = false) => GeneralFuncs.SaveSettings(SettingsFile, GetJObject(), mergeNewIntoOld);
        #endregion

        #region SHORTCUTS
        public void CheckShortcuts()
        {
            Globals.DebugWriteLine($@"[Func:Data\Settings\Origin.CheckShortcuts]");
            _instance._desktopShortcut = File.Exists(Path.Combine(Shortcut.Desktop, "Origin - TcNo Account Switcher.lnk"));
            AppSettings.Instance.CheckShortcuts();
        }

        public void DesktopShortcut_Toggle()
        {
            Globals.DebugWriteLine($@"[Func:Data\Settings\Origin.DesktopShortcut_Toggle]");
            var s = new Shortcut();
            s.Shortcut_Platform(Shortcut.Desktop, "Origin", "origin");
            s.ToggleShortcut(!DesktopShortcut, true);
        }
        #endregion
    }
}
