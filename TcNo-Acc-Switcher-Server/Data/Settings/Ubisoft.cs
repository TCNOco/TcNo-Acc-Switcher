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
    public class Ubisoft
    {
        private static Ubisoft _instance = new Ubisoft();

        public Ubisoft() { }
        private static readonly object LockObj = new();
        public static Ubisoft Instance
        {
            get
            {
                lock (LockObj)
                {
                    return _instance ??= new Ubisoft();
                }
            }
            set => _instance = value;
        }

        // Variables
        private string _folderPath = "C:\\Program Files (x86)\\Ubisoft\\Ubisoft Game launcher\\";
        [JsonProperty("FolderPath", Order = 1)] public string FolderPath { get => _instance._folderPath; set => _instance._folderPath = value; }
        private Point _windowSize = new() { X = 800, Y = 450 };
        [JsonProperty("WindowSize", Order = 1)] public Point WindowSize { get => _instance._windowSize; set => _instance._windowSize = value; }
        private bool _admin;
        [JsonProperty("Ubisoft_Admin", Order = 2)] public bool Admin { get => _instance._admin; set => _instance._admin = value; }
        private int _trayAccNumber = 3;
        [JsonProperty("Ubisoft_TrayAccNumber", Order = 3)] public int TrayAccNumber { get => _instance._trayAccNumber; set => _instance._trayAccNumber = value; }
        private bool _forgetAccountEnabled;
        [JsonProperty("ForgetAccountEnabled", Order = 4)] public bool ForgetAccountEnabled { get => _instance._forgetAccountEnabled; set => _instance._forgetAccountEnabled = value; }

        private bool _desktopShortcut;
        [JsonIgnore] public bool DesktopShortcut { get => _instance._desktopShortcut; set => _instance._desktopShortcut = value; }
        private bool _startMenu;
        [JsonIgnore] public bool StartMenu { get => _instance._startMenu; set => _instance._startMenu = value; }
        private bool _trayStartup;
        [JsonIgnore] public bool TrayStartup { get => _instance._trayStartup; set => _instance._trayStartup = value; }


        // Constants
        [JsonIgnore] public string SettingsFile = "UbisoftSettings.json";
        [JsonIgnore] public string UbisoftImagePath = "wwwroot/img/profiles/ubi/";
        [JsonIgnore] public string UbisoftImagePathHtml = "img/profiles/ubi/";
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
        /// Updates the ForgetAccountEnabled bool in Steam settings file
        /// </summary>
        /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
        public void UpdateUbisoftForgetAcc(bool enabled)
        {
            Globals.DebugWriteLine($@"[Func:Data\Settings\Ubisoft.UpdateUbisoftForgetAcc]");
            if (ForgetAccountEnabled == enabled) return; // Ignore if already set
            ForgetAccountEnabled = enabled;
            SaveSettings();
        }

        /// <summary>
        /// Default settings for UbisoftSettings.json
        /// </summary>
        public void ResetSettings()
        {
            Globals.DebugWriteLine($@"[Func:Data\Settings\Ubisoft.ResetSettings]");
            _instance.FolderPath = "C:\\Program Files (x86)\\Ubisoft\\Ubisoft Game Launcher\\";
            _instance.WindowSize = new Point() { X = 800, Y = 450 };
            _instance.Admin = false;
            _instance.TrayAccNumber = 3;

            SaveSettings();
        }
        public void SetFromJObject(JObject j)
        {
            Globals.DebugWriteLine($@"[Func:Data\Settings\Ubisoft.SetFromJObject]");
            var curSettings = j.ToObject<Ubisoft>();
            if (curSettings == null) return;
            _instance.FolderPath = curSettings.FolderPath;
            _instance.WindowSize = curSettings.WindowSize;
            _instance.Admin = curSettings.Admin;
            _instance.TrayAccNumber = curSettings.TrayAccNumber;
        }
        public void LoadFromFile() => SetFromJObject(GeneralFuncs.LoadSettings(SettingsFile, GetJObject()));
        public JObject GetJObject() => JObject.FromObject(this);
        [JSInvokable]
        public void SaveSettings(bool mergeNewIntoOld = false) => GeneralFuncs.SaveSettings(SettingsFile, GetJObject(), mergeNewIntoOld);

        /// <summary>
        /// Get Ubisoft.exe path from UbisoftSettings.json 
        /// </summary>
        /// <returns>Ubisoft.exe's path string</returns>
        public string UbisoftExe() => Path.Combine(FolderPath, "upc.exe");


        #region SHORTCUTS
        public void CheckShortcuts()
        {
            Globals.DebugWriteLine($@"[Func:Data\Settings\Ubisoft.CheckShortcuts]");
            _instance._desktopShortcut = File.Exists(Path.Combine(Shortcut.Desktop, "TcNo Account Switcher.lnk"));
            _instance._startMenu = File.Exists(Path.Combine(Shortcut.StartMenu, "TcNo Account Switcher.lnk"));
            _instance._trayStartup = Pages.General.Classes.Task.StartWithWindows_Enabled();
        }

        public void DesktopShortcut_Toggle()
        {
            Globals.DebugWriteLine($@"[Func:Data\Settings\Ubisoft.DesktopShortcut_Toggle]");
            var s = new Shortcut();
            s.Shortcut_Steam(Shortcut.Desktop);
            s.ToggleShortcut(!DesktopShortcut, true);

            s.Shortcut_Switcher(Shortcut.Desktop);
            s.ToggleShortcut(!DesktopShortcut, true);
        }
        public void StartMenu_Toggle()
        {
            Globals.DebugWriteLine($@"[Func:Data\Settings\Ubisoft.StartMenu_Toggle]");
            var s = new Shortcut();
            s.Shortcut_Steam(Shortcut.StartMenu);
            s.ToggleShortcut(!StartMenu, false);

            s.Shortcut_Switcher(Shortcut.StartMenu);
            s.ToggleShortcut(!StartMenu, false);

            s.Shortcut_SteamTray(Shortcut.StartMenu);
            s.ToggleShortcut(!StartMenu, false);
        }
        public void Task_Toggle()
        {
            Globals.DebugWriteLine($@"[Func:Data\Settings\Steam.Task_Toggle]");
            Pages.General.Classes.Task.StartWithWindows_Toggle(!TrayStartup);
        }

        #endregion
    }
}
