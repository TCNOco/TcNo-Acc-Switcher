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
using System.Globalization;
using System.IO;
using System.Linq;
using System.Runtime.Versioning;
using System.Text.Json.Serialization;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.AspNetCore.Http;
using Microsoft.JSInterop;
using Microsoft.Win32;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using SharpScss;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using YamlDotNet.Serialization;
using YamlDotNet.Serialization.NamingConventions;
using Task = System.Threading.Tasks.Task;


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
                    // Load settings if have changed, or not set
                    if (_instance is {_currentlyModifying: true}) return _instance;
                    if (_instance != new AppSettings() && Globals.GetFileMd5(SettingsFile) == _instance._lastHash) return _instance;

                    _instance = new AppSettings { _currentlyModifying = true };

                    if (File.Exists(SettingsFile))
                    {
                        _instance = JsonConvert.DeserializeObject<AppSettings>(File.ReadAllText(SettingsFile), new JsonSerializerSettings() { });
                        _instance._lastHash = Globals.GetFileMd5(SettingsFile);
                    }else
                    {
                        SaveSettings();
                    }

                    LoadStylesheetFromFile();
                    CheckShortcuts();

                    _instance._currentlyModifying = false;

                    return _instance;
                }
            }
            set => _instance = value;
        }

        private static readonly Lang Lang = Lang.Instance;
        private string _lastHash = "";
        private bool _currentlyModifying = false;
        public static void SaveSettings() => GeneralFuncs.SaveSettings(SettingsFile, Instance);

        // Variables
        private bool _updateAvailable;
        [JsonProperty("Language", Order = 0)] private string _lang = "";
        [JsonProperty("Rtl", Order = 1)] private bool _rtl = CultureInfo.CurrentCulture.TextInfo.IsRightToLeft;
        [JsonProperty("StreamerModeEnabled", Order = 2)] private bool _streamerModeEnabled = true;
        [JsonProperty("ServerPort", Order = 3)] private int _serverPort = 1337;
        [JsonProperty("WindowSize", Order = 4)] private Point _windowSize = new() { X = 800, Y = 450 };
        [JsonProperty("Version", Order = 5)] private readonly string _version = Globals.Version;
        [JsonProperty("DisabledPlatforms", Order = 6)] private SortedSet<string> _disabledPlatforms = new();
        [JsonProperty("TrayMinimizeNotExit", Order = 7)] private bool _trayMinimizeNotExit;
        [JsonProperty("ShownMinimizedNotification", Order = 8)] private bool _shownMinimizedNotification;
        [JsonProperty("StartCentered", Order = 9)] private bool _startCentered;
        [JsonProperty("ActiveTheme", Order = 10)] private string _activeTheme = "Dracula_Cyan";
        [JsonProperty("ActiveBrowser", Order = 11)] private string _activeBrowser = "WebView";
        [JsonProperty("Background", Order = 12)] private string _background = "";
        [JsonProperty("EnabledBasicPlatforms", Order = 13)] private HashSet<string> _enabledBasicPlatforms = new();
        [Newtonsoft.Json.JsonIgnore] private bool _desktopShortcut;
        [Newtonsoft.Json.JsonIgnore] private bool _startMenu;
        [Newtonsoft.Json.JsonIgnore] private bool _startMenuPlatforms;
        [Newtonsoft.Json.JsonIgnore] private bool _protocolEnabled;
        [Newtonsoft.Json.JsonIgnore] private bool _trayStartup;

        public static bool UpdateAvailable { get => Instance._updateAvailable; set => Instance._updateAvailable = value; }
        public static string Language { get => Instance._lang; set => Instance._lang = value; }
        public static bool Rtl { get => Instance._rtl; set => Instance._rtl = value; }
        public static bool StreamerModeEnabled { get => Instance._streamerModeEnabled; set => Instance._streamerModeEnabled = value; }
        public static int ServerPort { get => Instance._serverPort; set => Instance._serverPort = value; }
        public static Point WindowSize { get => Instance._windowSize; set => Instance._windowSize = value; }
        public static string Version => Instance._version;
        public static SortedSet<string> DisabledPlatforms { get => Instance._disabledPlatforms; set => Instance._disabledPlatforms = value; }
        public static bool TrayMinimizeNotExit { get => Instance._trayMinimizeNotExit; set => Instance._trayMinimizeNotExit = value; }
        public static bool ShownMinimizedNotification { get => Instance._shownMinimizedNotification; set => Instance._shownMinimizedNotification = value; }
        public static bool StartCentered { get => Instance._startCentered; set => Instance._startCentered = value; }
        public static string ActiveTheme { get => Instance._activeTheme; set => Instance._activeTheme = value; }
        public static string ActiveBrowser { get => Instance._activeBrowser; set => Instance._activeBrowser = value; }
        public static string Background { get => Instance._background; set => Instance._background = value; }

        public static HashSet<string> EnabledBasicPlatforms
        {
            get => Instance._enabledBasicPlatforms;
            set => Instance._enabledBasicPlatforms = value;
        }

        public static List<string> EnabledBasicPlatformSorted()
        {
            var enabled = EnabledBasicPlatforms.ToList();
            enabled.Sort(StringComparer.InvariantCultureIgnoreCase);
            return enabled;
        }

        public static bool DesktopShortcut { get => Instance._desktopShortcut; set => Instance._desktopShortcut = value; }

        public static bool StartMenu { get => Instance._startMenu; set => Instance._startMenu = value; }

        public static bool StartMenuPlatforms { get => Instance._startMenuPlatforms; set => Instance._startMenuPlatforms = value; }

        public static bool ProtocolEnabled { get => Instance._protocolEnabled; set => Instance._protocolEnabled = value; }
        public static bool TrayStartup { get => Instance._trayStartup; set => Instance._trayStartup = value; }

        [Newtonsoft.Json.JsonIgnore]
        public static string PlatformContextMenu =>
            Lang == null ? "" : $@"[
              {{""{Lang["Context_HidePlatform"]}"": ""hidePlatform()""}},
              {{""{Lang["Context_CreateShortcut"]}"": ""createPlatformShortcut()""}},
              {{""{Lang["Context_ExportAccList"]}"": ""exportAllAccounts()""}}
            ]";

        [JSInvokable]
        public static async Task HidePlatform(string platform)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Data\AppSettings.HidePlatform]");
            if (BasicPlatforms.PlatformExistsFromShort(platform))
            {
                EnabledBasicPlatforms.Remove(platform);
            }
            else
                _ = DisabledPlatforms.Add(platform);
            SaveSettings();
            await AppData.ReloadPage();
        }

        public static async Task ShowPlatform(string platform)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Data\AppSettings.ShowPlatform]");
            if (BasicPlatforms.PlatformExistsFromShort(platform))
            {
                if (!EnabledBasicPlatforms.Contains(platform))
                    EnabledBasicPlatforms.Add(platform);
            }
            else
                _ = DisabledPlatforms.Remove(platform);
            SaveSettings();
            await AppData.ReloadPage();
        }

        private string _stylesheet;
        public static string Stylesheet { get => Instance._stylesheet; set => Instance._stylesheet = value; }

        private bool _windowsAccent;
        public static bool WindowsAccent { get => Instance._windowsAccent; set => Instance._windowsAccent = value; }

        private string _windowsAccentColor = "";
        public static string WindowsAccentColor { get => Instance._windowsAccentColor; set => Instance._windowsAccentColor = value; }

        private (float, float, float) _windowsAccentColorHsl = (0, 0, 0);
        public static (float, float, float) WindowsAccentColorHsl { get => Instance._windowsAccentColorHsl; set => Instance._windowsAccentColorHsl = value; }

        [SupportedOSPlatform("windows")]
        public static (int, int, int) WindowsAccentColorInt => GetAccentColor();

        // Constants
        public static readonly string SettingsFile = "WindowSettings.json";
        private static string StylesheetFile => Path.Join("themes", ActiveTheme, "style.css");
        private static string StylesheetInfoFile => Path.Join("themes", ActiveTheme, "info.yaml");
        private Dictionary<string, string> _stylesheetInfo;
        public static Dictionary<string, string> StylesheetInfo { get => Instance._stylesheetInfo; set => Instance._stylesheetInfo = value; }

        public static bool StreamerModeTriggered;

        /// <summary>
        /// Check if any streaming software is running. Do let me know if you have a program name that you'd like to expand this list with!
        /// It's basically the program's .exe file, but without ".exe".
        /// </summary>
        /// <returns>True when streaming software is running</returns>
        public static bool StreamerModeCheck()
        {
            Globals.DebugWriteLine(@"[Func:Data\AppSettings.StreamerModeCheck]");
            if (!StreamerModeEnabled) return false; // Don't hide anything if disabled.
            StreamerModeTriggered = false;
            foreach (var p in Process.GetProcesses())
            {
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
                        StreamerModeTriggered = true;
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
        public static Task<bool> GetTrayMinimizeNotExit() => Task.FromResult(TrayMinimizeNotExit);

        /// <summary>
        /// Sets the active browser
        /// </summary>
        public static Task SetActiveBrowser(string browser)
        {
            ActiveBrowser = browser;
            _ = GeneralInvocableFuncs.ShowToast("success", Lang["Toast_RestartRequired"], Lang["Notice"], "toastarea");
            return Task.CompletedTask;
        }


        public static void ResetSettings()
        {
            StreamerModeEnabled = true;
            SaveSettings();
        }

        #region STYLESHEET
        /// <summary>
        /// Returns a block of CSS text to be used on the page. Used to hide or show certain things in certain ways, in components that aren't being added through Blazor.
        /// </summary>
        public static string GetCssBlock() => ".streamerCensor { display: " + (StreamerModeEnabled && StreamerModeTriggered ? "none!important" : "block") + "}";

        /// <summary>
        /// Swaps in a requested stylesheet, and loads styles from file.
        /// </summary>
        /// <param name="swapTo">Stylesheet name (without .json) to copy and load</param>
        public static async Task SwapStylesheet(string swapTo)
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
        public static bool LoadStylesheetFromFile()
        {
            // This is the first function that's called, and sometimes fails if this is not reset after being changed previously.
            Directory.SetCurrentDirectory(Globals.UserDataFolder);
#if DEBUG
            if (true) // Always generate file in debug mode.
#else
            if (!File.Exists(StylesheetFile))
#endif
            {
                // Check if SCSS file exists.
                var temp = Directory.GetCurrentDirectory();
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

        private static void GenCssFromScss(string scss)
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
            Globals.DeleteFile(StylesheetFile);
            File.WriteAllText(StylesheetFile, convertedScss.Css);

            Globals.DeleteFile(StylesheetFile + ".map");
            File.WriteAllText(StylesheetFile + ".map", convertedScss.SourceMap);
        }

        private static void LoadStylesheet()
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
        public static string[] GetStyleList()
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

        [JSInvokable]
        public static async Task SetBackground(string path)
        {
            Background = $"{path}";

            if (File.Exists(path) && path != "")
            {
                Directory.CreateDirectory(Path.Join(Globals.UserDataFolder, "wwwroot\\img\\custom\\"));
                File.Copy(path, Path.Join(Globals.UserDataFolder, "wwwroot\\img\\custom\\background" + Path.GetExtension(path)), true);
                Background = $"img/custom/background{Path.GetExtension(path)}";
                SaveSettings();
            }
            await AppData.CacheReloadPage();
        }

        #endregion

        #region SHORTCUTS
        public static void CheckShortcuts()
        {
            Globals.DebugWriteLine(@"[Func:Data\AppSettings.CheckShortcuts]");
            DesktopShortcut = File.Exists(Path.Join(Shortcut.Desktop, "TcNo Account Switcher.lnk"));
            StartMenu = File.Exists(Path.Join(Shortcut.StartMenu, "TcNo Account Switcher.lnk"));
            StartMenuPlatforms = Directory.Exists(Path.Join(Shortcut.StartMenu, "Platforms"));
            TrayStartup = Shortcut.StartWithWindows_Enabled();

            if (OperatingSystem.IsWindows())
                ProtocolEnabled = Protocol_IsEnabled();
        }

        public static void DesktopShortcut_Toggle()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.DesktopShortcut_Toggle]");
            var s = new Shortcut();
            _ = s.Shortcut_Switcher(Shortcut.Desktop);
            s.ToggleShortcut(!DesktopShortcut);
        }

        public static void TrayMinimizeNotExit_Toggle()
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
        public static void Protocol_Toggle()
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
                ProtocolEnabled = Protocol_IsEnabled();
            }
            catch (UnauthorizedAccessException)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_RestartAsAdmin"], Lang["Failed"], "toastarea");
                _ = GeneralInvocableFuncs.ShowModal("notice:RestartAsAdmin");
            }
        }

        #region WindowsAccent
        [SupportedOSPlatform("windows")]
        public static void WindowsAccent_Toggle()
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
            WindowsAccentColor = GetAccentColorHexString();
            var (r, g, b) = GetAccentColor();
            WindowsAccentColorHsl = FromRgb(r, g, b);

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
        public static void StartMenu_Toggle(bool platforms)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.StartMenu_Toggle]");
            if (platforms)
            {
                var platformsFolder = Path.Join(Shortcut.StartMenu, "Platforms");
                if (Directory.Exists(platformsFolder)) Globals.RecursiveDelete(Path.Join(Shortcut.StartMenu, "Platforms"), false);
                else
                {
                    _ = Directory.CreateDirectory(platformsFolder);
                    foreach (var platform in AppData.Instance.PlatformList)
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
        public static void AutoStart_Toggle()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.Task_Toggle]");
            Shortcut.StartWithWindows_Toggle(!TrayStartup);
        }

        public static void StartNow()
        {
            _ = NativeFuncs.StartTrayIfNotRunning() switch
            {
                "Started Tray" => GeneralInvocableFuncs.ShowToast("success", Lang["Toast_TrayStarted"], renderTo: "toastarea"),
                "Already running" => GeneralInvocableFuncs.ShowToast("info", Lang["Toast_TrayRunning"], renderTo: "toastarea"),
                "Tray users not found" => GeneralInvocableFuncs.ShowToast("error", Lang["Toast_TrayUsersMissing"], renderTo: "toastarea"),
                _ => GeneralInvocableFuncs.ShowToast("error", Lang["Toast_TrayFail"], renderTo: "toastarea")
            };
        }

        private static void CreatePlatformShortcut(string folder, string platformName, string args)
        {
            var s = new Shortcut();
            _ = s.Shortcut_Platform(folder, platformName, args);
            s.ToggleShortcut(!StartMenuPlatforms, false);
        }
        #endregion
    }
}
