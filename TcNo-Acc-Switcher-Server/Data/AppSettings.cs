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
using System.Drawing;
using System.IO;
using System.Linq;
using System.Runtime.Versioning;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Microsoft.Win32;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using SharpScss;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using YamlDotNet.Serialization;
using YamlDotNet.Serialization.NamingConventions;
using Task = System.Threading.Tasks.Task;
using TcNoTask = TcNo_Acc_Switcher_Server.Pages.General.Classes.Task;


namespace TcNo_Acc_Switcher_Server.Data
{
    public class AppSettings
    {
        private static AppSettings _instance = new();

        private static readonly object LockObj = new();

        public static AppSettings Instance
        {
            get
            {
                lock (LockObj)
                {
                    return _instance ??= new AppSettings();
                }
            }
            set => _instance = value;
        }

        private static readonly Lang Lang = Lang.Instance;

        // Variables
        private bool _updateAvailable;
        [JsonIgnore] public bool UpdateAvailable { get => _instance._updateAvailable; set => _instance._updateAvailable = value; }

        private string _lang = "";
        [JsonProperty("Language", Order = 0)] public string Language { get => _instance._lang; set => _instance._lang = value; }

        private bool _streamerModeEnabled = true;
        [JsonProperty("StreamerModeEnabled", Order = 1)] public bool StreamerModeEnabled { get => _instance._streamerModeEnabled; set => _instance._streamerModeEnabled = value; }

        private int _serverPort = 1337;
        [JsonProperty("ServerPort", Order = 2)] public int ServerPort { get => _instance._serverPort; set => _instance._serverPort = value; }

        private Point _windowSize = new() { X = 800, Y = 450 };
        [JsonProperty("WindowSize", Order = 3)] public Point WindowSize { get => _instance._windowSize; set => _instance._windowSize = value; }

        private readonly string _version = Globals.Version;
        [JsonProperty("Version", Order = 4)] public string Version => _instance._version;

        private SortedSet<string> _disabledPlatforms = new();
        [JsonProperty("DisabledPlatforms", Order = 5)]
        public SortedSet<string> DisabledPlatforms { get => _instance._disabledPlatforms; set => _instance._disabledPlatforms = value; }

        private bool _trayMinimizeNotExit;
        [JsonProperty("TrayMinimizeNotExit", Order = 6)]
        public bool TrayMinimizeNotExit { get => _instance._trayMinimizeNotExit; set => _instance._trayMinimizeNotExit = value; }

        private bool _shownMinimizedNotification;
        [JsonProperty("ShownMinimizedNotification", Order = 7)]
        public bool ShownMinimizedNotification { get => _instance._shownMinimizedNotification; set => _instance._shownMinimizedNotification = value; }

        private bool _startCentered;
        [JsonProperty("StartCentered", Order = 8)]
        public bool StartCentered { get => _instance._startCentered; set => _instance._startCentered = value; }

        private string _activeTheme = "Dracula_Cyan";
        [JsonProperty("ActiveTheme")]
        public string ActiveTheme { get => _instance._activeTheme; set => _instance._activeTheme = value; }

        private string _activeBrowser = "WebView";
        [JsonProperty("ActiveBrowser")]
        public string ActiveBrowser { get => _instance._activeBrowser; set => _instance._activeBrowser = value; }

        private bool _desktopShortcut;
        [JsonIgnore] public bool DesktopShortcut { get => _instance._desktopShortcut; set => _instance._desktopShortcut = value; }
        private bool _startMenu;
        [JsonIgnore] public bool StartMenu { get => _instance._startMenu; set => _instance._startMenu = value; }
        private bool _startMenuPlatforms;
        [JsonIgnore] public bool StartMenuPlatforms { get => _instance._startMenuPlatforms; set => _instance._startMenuPlatforms = value; }
        private bool _protocolEnabled;
        [JsonIgnore] public bool ProtocolEnabled { get => _instance._protocolEnabled; set => _instance._protocolEnabled = value; }
        private bool _trayStartup;
        [JsonIgnore] public bool TrayStartup { get => _instance._trayStartup; set => _instance._trayStartup = value; }


        /// <summary>
        /// Some features only work in the 'official browser' such as picking files, this will need to be adjusted for pure-web users, using just the server and another browser.
        /// </summary>
        private bool _usingTcNoBrowser;
        [JsonIgnore] public bool UsingTcNoBrowser { get => _instance._usingTcNoBrowser; set => _instance._usingTcNoBrowser = value; }

        [JsonIgnore]
        public string PlatformContextMenu =>
            Lang == null ? "" : $@"[
              {{""{Lang["Context_HidePlatform"]}"": ""hidePlatform()""}},
              {{""{Lang["Context_CreateShortcut"]}"": ""createPlatformShortcut()""}},
              {{""{Lang["Context_ExportAccList"]}"": ""exportAllAccounts()""}}
            ]";

        [JSInvokable]
        public static async Task HidePlatform(string platform)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Data\AppSettings.HidePlatform]");
            _ = _instance.DisabledPlatforms.Add(platform);
            _instance.SaveSettings();
            await AppData.ReloadPage();
        }

        public static async Task ShowPlatform(string platform)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Data\AppSettings.ShowPlatform]");
            _ = _instance.DisabledPlatforms.Remove(platform);
            _instance.SaveSettings();
            await AppData.ReloadPage();
        }

        private string _stylesheet;
        [JsonIgnore] public string Stylesheet { get => _instance._stylesheet; set => _instance._stylesheet = value; }

        private bool _windowsAccent;
        public bool WindowsAccent { get => _instance._windowsAccent; set => _instance._windowsAccent = value; }

        private string _windowsAccentColor = "";
        [JsonIgnore] public string WindowsAccentColor { get => _instance._windowsAccentColor; set => _instance._windowsAccentColor = value; }

        private (float, float, float) _windowsAccentColorHsl = (0, 0, 0);
        [JsonIgnore] public (float, float, float) WindowsAccentColorHsl { get => _instance._windowsAccentColorHsl; set => _instance._windowsAccentColorHsl = value; }
        [JsonIgnore] public (int, int, int) WindowsAccentColorInt => GetAccentColor();

        // Constants
        [JsonIgnore] public readonly string SettingsFile = "WindowSettings.json";
        [JsonIgnore] private string StylesheetFile => Path.Join("themes", ActiveTheme, "style.css");
        [JsonIgnore] private string StylesheetInfoFile => Path.Join("themes", ActiveTheme, "info.yaml");
        private Dictionary<string, string> _stylesheetInfo;
        [JsonIgnore] public Dictionary<string, string> StylesheetInfo { get => _instance._stylesheetInfo; set => _instance._stylesheetInfo = value; }

        [JsonIgnore] public bool StreamerModeTriggered;

        /// <summary>
        /// Check if any streaming software is running. Do let me know if you have a program name that you'd like to expand this list with!
        /// It's basically the program's .exe file, but without ".exe".
        /// </summary>
        /// <returns>True when streaming software is running</returns>
        public bool StreamerModeCheck()
        {
            Globals.DebugWriteLine(@"[Func:Data\AppSettings.StreamerModeCheck]");
            if (!_streamerModeEnabled) return false; // Don't hide anything if disabled.
            _instance.StreamerModeTriggered = false;
            foreach (var p in Process.GetProcesses())
            {
                //try
                //{
                //    if (p.MainModule == null) continue;
                //}
                //catch (System.ComponentModel.Win32Exception e)
                //{
                //    // This is just something the process can't access.
                //    // Ignore and move on.
                //    continue;
                //}

                switch (p.ProcessName.ToLowerInvariant())
                {
                    case "obs":
                    case "obs32":
                    case "obs64":
                    case "streamlabs obs":
                    case "wirecast":
                    case "xsplit.core":
                    case "xsplit.gamecaster":
                    case "twitchstudio":
                        _instance.StreamerModeTriggered = true;
                        Globals.WriteToLog($"Streamer mode found: {p.ProcessName}");
                        return true;
                }
            }
            return false;
        }

        /// <summary>
        /// Used in JS. Gets whether forget account is enabled (Whether to NOT show prompt, or show it).
        /// </summary>
        /// <returns></returns>
        [JSInvokable]
        public static Task<bool> GetTrayMinimizeNotExit() => Task.FromResult(_instance.TrayMinimizeNotExit);

        /// <summary>
        /// Sets the active browser
        /// </summary>
        public Task SetActiveBrowser(string browser)
        {
            ActiveBrowser = browser;
            _ = GeneralInvocableFuncs.ShowToast("success", Lang["Toast_RestartRequired"], Lang["Notice"], "toastarea");
            return Task.CompletedTask;
        }


        public void ResetSettings()
        {
            _instance.StreamerModeEnabled = true;
            SaveSettings();
        }

        public void SetFromJObject(JObject j)
        {
            Globals.DebugWriteLine(@"[Func:Data\AppSettings.SetFromJObject]");
            var curSettings = j.ToObject<AppSettings>();
            if (curSettings == null) return;
            _instance.StreamerModeEnabled = curSettings.StreamerModeEnabled;
            CheckShortcuts();
        }

        public bool LoadFromFile()
        {
            Globals.DebugWriteLine(@"[Func:Data\AppSettings.LoadFromFile]");
            // Main settings
            if (!File.Exists(SettingsFile)) SaveSettings();
            else SetFromJObject(GeneralFuncs.LoadSettings(SettingsFile, GetJObject()));
            // Stylesheet
            return LoadStylesheetFromFile();
        }

        #region STYLESHEET
        /// <summary>
        /// Returns a block of CSS text to be used on the page. Used to hide or show certain things in certain ways, in components that aren't being added through Blazor.
        /// </summary>
        public string GetCssBlock() => ".streamerCensor { display: " + (_instance.StreamerModeEnabled && _instance.StreamerModeTriggered ? "none!important" : "block") + "}";

        /// <summary>
        /// Swaps in a requested stylesheet, and loads styles from file.
        /// </summary>
        /// <param name="swapTo">Stylesheet name (without .json) to copy and load</param>
        public async Task SwapStylesheet(string swapTo)
        {
            ActiveTheme = swapTo.Replace(" ", "_");
            try
            {
                if (LoadStylesheetFromFile()) await AppData.ReloadPage();
                else _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_LoadStylesheetFailed"],
                    "Stylesheet error", "toastarea");
            }
            catch (Exception)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_LoadStylesheetFailed"],
                    "Stylesheet error", "toastarea");
            }
        }

        /// <summary>
        /// Load stylesheet settings from stylesheet file.
        /// </summary>
        public bool LoadStylesheetFromFile()
        {
#if DEBUG
            if (true) // Always generate file in debug mode.
#else
            if (!File.Exists(StylesheetFile))
#endif
            {
                // Check if SCSS file exists.
                var scss = StylesheetFile.Replace("css", "scss");
                if (File.Exists(scss)) GenCssFromScss(scss);
                else
                {
                    ActiveTheme = "Dracula_Cyan";

                    if (!File.Exists(StylesheetFile))
                    {
                        scss = StylesheetFile.Replace("css", "scss");
                        if (File.Exists(scss)) GenCssFromScss(scss);
                        else throw new Exception(Lang["ThemesNotFound"]);
                    }
                }
            }

            LoadStylesheet();

            return true;
        }

        private void GenCssFromScss(string scss)
        {
            ScssResult convertedScss;
            try
            {
                convertedScss = Scss.ConvertFileToCss(scss, new ScssOptions() { InputFile = scss, OutputFile = StylesheetFile });
            }
            catch (ScssException e)
            {
                Globals.DebugWriteLine("ERROR in CSS: " + e.Message);
                throw;
            }
            // Convert from SCSS to CSS. The arguments are for "exception reporting", according to the SharpScss Git Repo.
            if (File.Exists(StylesheetFile)) File.Delete(StylesheetFile);
            File.WriteAllText(StylesheetFile, convertedScss.Css);

            if (File.Exists(StylesheetFile + ".map")) File.Delete(StylesheetFile + ".map");
            File.WriteAllText(StylesheetFile + ".map", convertedScss.SourceMap);
        }

        private void LoadStylesheet()
        {
            // Load new stylesheet
            var desc = new DeserializerBuilder().WithNamingConvention(HyphenatedNamingConvention.Instance).Build();
            Stylesheet = Globals.ReadAllText(StylesheetFile);

            // Load stylesheet info
            string[] infoText = null;
            if (File.Exists(StylesheetInfoFile)) infoText = Globals.ReadAllLines(StylesheetInfoFile);

            infoText ??= new[] {"name: \"[ERR! info.yml]\"", "accent: \"#00D4FF\""};
            var newSheet = JsonConvert.DeserializeObject<Dictionary<string, string>>(JsonConvert.SerializeObject(desc.Deserialize<object>(string.Join(Environment.NewLine, infoText))));

            StylesheetInfo = newSheet;

            if (OperatingSystem.IsWindows() && WindowsAccent) SetAccentColor();
        }

        /// <summary>
        /// Returns a list of Stylesheets in the Stylesheet folder.
        /// </summary>
        public string[] GetStyleList()
        {
            var themeList = Directory.GetDirectories("themes");
            for (var i = 0; i < themeList.Length; i++)
            {
                var start = themeList[i].LastIndexOf("\\", StringComparison.Ordinal) + 1;
                themeList[i] = themeList[i][start..].Replace('_', ' ');
            }
            return themeList;
        }
        private static (int, int, int) FromRgb(byte R, byte G, byte B)
        {
            // Adapted from: https://stackoverflow.com/a/4794649/5165437
            var _R = (R / 255f);
            var _G = (G / 255f);
            var _B = (B / 255f);

            var _Min = Math.Min(Math.Min(_R, _G), _B);
            var _Max = Math.Max(Math.Max(_R, _G), _B);
            var _Delta = _Max - _Min;

            var H = (float)0;
            var S = (float)0;
            var L = (_Max + _Min) / 2.0f;

            if (_Delta != 0)
            {
                if (L < 0.5f)
                {
                    S = _Delta / (_Max + _Min);
                }
                else
                {
                    S = _Delta / (2.0f - _Max - _Min);
                }


                if (Math.Abs(_R - _Max) < 0.01)
                {
                    H = (_G - _B) / _Delta;
                }
                else if (Math.Abs(_G - _Max) < 0.01)
                {
                    H = 2f + (_B - _R) / _Delta;
                }
                else if (Math.Abs(_B - _Max) < 0.01)
                {
                    H = 4f + (_R - _G) / _Delta;
                }
            }

            // Rounding and formatting for CSS
            H *= 60;
            S *= 100;
            L *= 100;

            return ((int)Math.Round(H), (int)Math.Round(S), (int)Math.Round(L));
        }
        #endregion

        public JObject GetJObject() => JObject.FromObject(this);

        [JSInvokable]
        public void SaveSettings(bool mergeNewIntoOld = false) => GeneralFuncs.SaveSettings(SettingsFile, GetJObject(), mergeNewIntoOld);

        #region SHORTCUTS
        public void CheckShortcuts()
        {
            Globals.DebugWriteLine(@"[Func:Data\AppSettings.CheckShortcuts]");
            _instance._desktopShortcut = File.Exists(Path.Join(Shortcut.Desktop, "TcNo Account Switcher.lnk"));
            _instance._startMenu = File.Exists(Path.Join(Shortcut.StartMenu, "TcNo Account Switcher.lnk"));
            _instance._startMenuPlatforms = Directory.Exists(Path.Join(Shortcut.StartMenu, "Platforms"));
            _instance._trayStartup = TcNoTask.StartWithWindows_Enabled();

            if (OperatingSystem.IsWindows())
                _instance._protocolEnabled = Protocol_IsEnabled();
        }

        public void DesktopShortcut_Toggle()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.DesktopShortcut_Toggle]");
            var s = new Shortcut();
            _ = s.Shortcut_Switcher(Shortcut.Desktop);
            s.ToggleShortcut(!DesktopShortcut);
        }

        public void TrayMinimizeNotExit_Toggle()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.DesktopShortcut_Toggle]");
            if (TrayMinimizeNotExit) return;
            _ = GeneralInvocableFuncs.ShowToast("info", Lang["Toast_TrayPosition"], duration: 15000, renderTo: "toastarea");
            _ = GeneralInvocableFuncs.ShowToast("info", Lang["Toast_TrayHint"], duration: 15000, renderTo: "toastarea");
        }

        /// <summary>
        /// Check for existence of protocol key in registry (tcno:\\)
        /// </summary>
        [SupportedOSPlatform("windows")]
        private static bool Protocol_IsEnabled()
        {
            var key = Registry.ClassesRoot.OpenSubKey(@"tcno");
            return key != null && (key.GetValueNames().Contains("URL Protocol"));
        }

        /// <summary>
        /// Toggle protocol functionality in Windows
        /// </summary>
        [SupportedOSPlatform("windows")]
        public void Protocol_Toggle()
        {
            try
            {
                if (!Protocol_IsEnabled())
                {
                    // Add
                    using var key = Registry.ClassesRoot.CreateSubKey("tcno");
                    key?.SetValue("URL Protocol", "", RegistryValueKind.String);
                    using var defaultKey = Registry.ClassesRoot.CreateSubKey(@"tcno\Shell\Open\Command");
                    defaultKey?.SetValue("", $"\"{Path.Join(Globals.AppDataFolder, "TcNo-Acc-Switcher.exe")}\" \"%1\"", RegistryValueKind.String);
                    _ = GeneralInvocableFuncs.ShowToast("success", Lang["Toast_ProtocolEnabled"], Lang["Toast_ProtocolEnabledTitle"], "toastarea");
                }
                else
                {
                    // Remove
                    Registry.ClassesRoot.DeleteSubKeyTree("tcno");
                    _ = GeneralInvocableFuncs.ShowToast("success", Lang["Toast_ProtocolDisabled"], Lang["Toast_ProtocolDisabledTitle"], "toastarea");
                }
                _instance._protocolEnabled = Protocol_IsEnabled();
            }
            catch (UnauthorizedAccessException)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_RestartAsAdmin"], Lang["Failed"], "toastarea");
                _ = GeneralInvocableFuncs.ShowModal("notice:RestartAsAdmin");
            }
        }

        #region WindowsAccent
        [SupportedOSPlatform("windows")]
        public void WindowsAccent_Toggle()
        {
            if (!WindowsAccent)
                SetAccentColor(true);
            else
            {
                WindowsAccentColor = "";
                _ = AppData.ReloadPage();
            }
        }

        [SupportedOSPlatform("windows")]
        private static void SetAccentColor() => SetAccentColor(false);
        [SupportedOSPlatform("windows")]
        private static void SetAccentColor(bool userInvoked)
        {
            Instance.WindowsAccentColor = GetAccentColorHexString();
            var (r, g, b) = GetAccentColor();
            Instance.WindowsAccentColorHsl = FromRgb(r, g, b);

            if (userInvoked)
                _ = AppData.ReloadPage();
        }

        [SupportedOSPlatform("windows")]
        public static string GetAccentColorHexString()
        {
            byte r, g, b;
            (r, g, b) = GetAccentColor();
            byte[] rgb = { r, g, b };
            return '#' + BitConverter.ToString(rgb).Replace("-", string.Empty);
        }

        [SupportedOSPlatform("windows")]
        public static (byte r, byte g, byte b) GetAccentColor()
        {
            using var dwmKey = Registry.CurrentUser.OpenSubKey(@"Software\Microsoft\Windows\DWM", RegistryKeyPermissionCheck.ReadSubTree);
            const string keyExMsg = "The \"HKCU\\Software\\Microsoft\\Windows\\DWM\" registry key does not exist.";
            if (dwmKey is null) throw new InvalidOperationException(keyExMsg);

            var accentColorObj = dwmKey.GetValue("AccentColor");
            if (accentColorObj is int accentColorDWord)
            {
                return ParseDWordColor(accentColorDWord);
            }
            else
            {
                const string valueExMsg = "The \"HKCU\\Software\\Microsoft\\Windows\\DWM\\AccentColor\" registry key value could not be parsed as an ABGR color.";
                throw new InvalidOperationException(valueExMsg);
            }
        }

        private static (byte r, byte g, byte b) ParseDWordColor(int color)
        {
            byte
                //a = (byte)((color >> 24) & 0xFF),
                b = (byte)((color >> 16) & 0xFF),
                g = (byte)((color >> 8) & 0xFF),
                r = (byte)((color >> 0) & 0xFF);

            return (r, g, b);
        }
        #endregion

        /// <summary>
        /// Create shortcuts in Start Menu
        /// </summary>
        /// <param name="platforms">true creates Platforms folder & drops shortcuts, otherwise only places main program & tray shortcut</param>
        public void StartMenu_Toggle(bool platforms)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.StartMenu_Toggle]");
            if (platforms)
            {
                var platformsFolder = Path.Join(Shortcut.StartMenu, "Platforms");
                if (Directory.Exists(platformsFolder)) GeneralFuncs.RecursiveDelete(new DirectoryInfo(Path.Join(Shortcut.StartMenu, "Platforms")), false);
                else
                {
                    _ = Directory.CreateDirectory(platformsFolder);
                    foreach (var platform in Globals.PlatformList)
                    {
                        CreatePlatformShortcut(platformsFolder, platform, platform.ToLowerInvariant());
                    }
                }

                return;
            }
            // Only create these shortcuts of requested, by setting platforms to false.
            var s = new Shortcut();
            _ = s.Shortcut_Switcher(Shortcut.StartMenu);
            s.ToggleShortcut(!StartMenu, false);

            _ = s.Shortcut_Tray(Shortcut.StartMenu);
            s.ToggleShortcut(!StartMenu, false);
        }
        public void Task_Toggle()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.Task_Toggle]");
            TcNoTask.StartWithWindows_Toggle(!TrayStartup);
        }

        public void StartNow()
        {
            _ = Globals.StartTrayIfNotRunning() switch
            {
                "Started Tray" => GeneralInvocableFuncs.ShowToast("success", Lang["Toast_TrayStarted"], renderTo: "toastarea"),
                "Already running" => GeneralInvocableFuncs.ShowToast("info", Lang["Toast_TrayRunning"], renderTo: "toastarea"),
                "Tray users not found" => GeneralInvocableFuncs.ShowToast("error", Lang["Toast_TrayUsersMissing"], renderTo: "toastarea"),
                _ => GeneralInvocableFuncs.ShowToast("error", Lang["Toast_TrayFail"], renderTo: "toastarea")
            };
        }

        private void CreatePlatformShortcut(string folder, string platformName, string args)
        {
            var s = new Shortcut();
            _ = s.Shortcut_Platform(folder, platformName, args);
            s.ToggleShortcut(!StartMenuPlatforms, false);
        }
        #endregion
    }
}
