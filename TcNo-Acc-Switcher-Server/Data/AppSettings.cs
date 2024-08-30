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
using System.Drawing;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Net;
using System.Reflection;
using System.Runtime.Versioning;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Microsoft.Win32;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using SharpScss;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using TcNo_Acc_Switcher_Server.Shared;
using TcNo_Acc_Switcher_Server.Shared.ContextMenu;
using YamlDotNet.Serialization;
using YamlDotNet.Serialization.NamingConventions;

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
                        _instance = JsonConvert.DeserializeObject<AppSettings>(File.ReadAllText(SettingsFile), new JsonSerializerSettings());
                        if (_instance == null)
                        {
                            _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_FailedLoadSettings"]);
                            if (File.Exists(SettingsFile))
                                Globals.CopyFile(SettingsFile, SettingsFile.Replace(".json", ".old.json"));
                            _instance = new AppSettings { _currentlyModifying = true };
                        }
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
            set
            {
                lock (LockObj)
                {
                    _instance = value;
                }
            }
        }

        private static readonly Lang Lang = Lang.Instance;
        private string _lastHash = "";
        private bool _currentlyModifying;
        public static void SaveSettings() => GeneralFuncs.SaveSettings(SettingsFile, Instance);

        // Variables
        [JsonProperty("Language", Order = 0)] private string _lang = "";
        [JsonProperty("Rtl", Order = 1)] private bool _rtl = CultureInfo.CurrentCulture.TextInfo.IsRightToLeft;
        [JsonProperty("StreamerModeEnabled", Order = 2)] private bool _streamerModeEnabled = true;
        [JsonProperty("ServerPort", Order = 3)] private int _serverPort = 1337;
        [JsonProperty("WindowSize", Order = 4)] private Point _windowSize = new() { X = 800, Y = 450 };
        [JsonProperty("AllowTransparency", Order = 5)] private bool _allowTransparency = true;
        [JsonProperty("Version", Order = 6)] private string _version = Globals.Version;
        [JsonProperty("DisabledPlatforms", Order = 7)] private SortedSet<string> _disabledPlatforms = new();
        [JsonProperty("TrayMinimizeNotExit", Order = 8)] private bool _trayMinimizeNotExit;
        [JsonProperty("ShownMinimizedNotification", Order = 9)] private bool _shownMinimizedNotification;
        [JsonProperty("StartCentered", Order = 10)] private bool _startCentered;
        [JsonProperty("ActiveTheme", Order = 11)] private string _activeTheme = "Dracula_Cyan";
        [JsonProperty("ActiveBrowser", Order = 12)] private string _activeBrowser = "WebView";
        [JsonProperty("Background", Order = 13)] private string _background = "";
        [JsonProperty("EnabledBasicPlatforms", Order = 14)] private HashSet<string> _enabledBasicPlatforms;
        [JsonProperty("CollectStats", Order = 15)] private bool _statsEnabled = true;
        [JsonProperty("ShareAnonymousStats", Order = 16)] private bool _statsShare = true;
        [JsonProperty("MinimizeOnSwitch", Order = 17)] private bool _minimizeOnSwitch;
        [JsonProperty("DiscordRpcEnabled", Order = 18)] private bool _discordRpc = true;
        [JsonProperty("DiscordRpcShareTotalSwitches", Order = 19)] private bool _discordRpcShare = false;
        [JsonProperty("PasswordHash", Order = 20)] private string _passwordHash = "";
        [JsonProperty("GloballyHiddenMetrics", Order = 21)] private Dictionary<string, Dictionary<string, bool>> _globallyHiddenMetrics = new();
        [JsonProperty("WindowsAccent", Order = 22)] private bool _windowsAccent;
        [JsonProperty("AutoUpdatePlatforms", Order = 23)] private bool _autoUpdatePlatforms = true;
        [JsonProperty("CheckForUpdates", Order = 24)] private bool _checkForUpdates = true;
        [JsonProperty("OfflineMode", Order = 25)] private bool _offlineMode = false;
        [JsonIgnore] private bool _desktopShortcut;
        [JsonIgnore] private bool _startMenu;
        [JsonIgnore] private bool _startMenuPlatforms;
        [JsonIgnore] private bool _protocolEnabled;
        [JsonIgnore] private bool _trayStartup;
        [JsonIgnore] private bool _updateCheckRan;
        [JsonIgnore] private bool _preRenderUpdate;
        [JsonIgnore] private string _passwordCurrent;

        public static string Language { get => Instance._lang; set => Instance._lang = value; }
        public static bool Rtl { get => Instance._rtl; set => Instance._rtl = value; }
        public static bool StreamerModeEnabled { get => Instance._streamerModeEnabled; set => Instance._streamerModeEnabled = value; }
        public static int ServerPort { get => Instance._serverPort; set => Instance._serverPort = value; }
        public static Point WindowSize { get => Instance._windowSize; set => Instance._windowSize = value; }
        public static bool AllowTransparency { get => Instance._allowTransparency; set => Instance._allowTransparency = value; }
        public static string Version { get => Instance._version; set => Instance._version = value; }

        public static SortedSet<string> DisabledPlatforms { get => Instance._disabledPlatforms; set => Instance._disabledPlatforms = value; }

        public static bool TrayMinimizeNotExit { get => Instance._trayMinimizeNotExit; set => Instance._trayMinimizeNotExit = value; }
        public static bool ShownMinimizedNotification { get => Instance._shownMinimizedNotification; set => Instance._shownMinimizedNotification = value; }
        public static bool StartCentered { get => Instance._startCentered; set => Instance._startCentered = value; }
        public static string ActiveTheme { get => Instance._activeTheme; set => Instance._activeTheme = value; }
        public static string ActiveBrowser { get => Instance._activeBrowser; set => Instance._activeBrowser = value; }
        public static string Background { get => Instance._background; set => Instance._background = value; }

        public static HashSet<string> EnabledBasicPlatforms
        {
            get =>
                Instance._enabledBasicPlatforms ??
                (Instance._enabledBasicPlatforms = new HashSet<string> {"o", "u", "e", "r", "d"});
            set => Instance._enabledBasicPlatforms = value;
        }

        public static bool DiscordRpc { get => Instance._discordRpc; set
        {
            if (!value) Instance._discordRpcShare = false;
            Instance._discordRpc = value;
        }
    }
        public static bool DiscordRpcShare { get => Instance._discordRpcShare; set => Instance._discordRpcShare = value; }
        public static string PasswordHash { get => Instance._passwordHash; set => Instance._passwordHash = value; } // SET should hash password.
        public static string PasswordCurrent { get => Instance._passwordCurrent; set => Instance._passwordCurrent = value; } // SET should hash password.

        public static bool StatsEnabled
        {
            get => Instance._statsEnabled;
            set
            {
                if (!value) Instance._statsShare = false;
                Instance._statsEnabled = value;
            }
        }

        public static bool StatsShare { get => Instance._statsShare; set => Instance._statsShare = value; }
        public static bool MinimizeOnSwitch { get => Instance._minimizeOnSwitch; set => Instance._minimizeOnSwitch = value; }
        public static bool OfflineMode { get => Instance._offlineMode; set => Instance._offlineMode = value; }
        public static bool DesktopShortcut { get => Instance._desktopShortcut; set => Instance._desktopShortcut = value; }

        public static bool StartMenu { get => Instance._startMenu; set => Instance._startMenu = value; }

        public static bool StartMenuPlatforms { get => Instance._startMenuPlatforms; set => Instance._startMenuPlatforms = value; }

        public static bool ProtocolEnabled { get => Instance._protocolEnabled; set => Instance._protocolEnabled = value; }
        public static bool TrayStartup { get => Instance._trayStartup; set => Instance._trayStartup = value; }
        private static bool UpdateCheckRan { get =>Instance._updateCheckRan; set => Instance._updateCheckRan = value; }
        public static bool PreRenderUpdate { get =>Instance._preRenderUpdate; set => Instance._preRenderUpdate = value; }
        public class GameSetting
        {
            public string SettingId { get; set; } = "";
            public bool Checked { get; set; }
        }

        /// <summary>
        /// For BasicStats // Game statistics collection and showing
        /// Keys for metrics on this list are not shown for any account.
        /// List of all games:[Settings:Hidden metric] metric keys.
        /// </summary>
        public static Dictionary<string, Dictionary<string, bool>> GloballyHiddenMetrics { get => Instance._globallyHiddenMetrics; set => Instance._globallyHiddenMetrics = value; }

        public static readonly ObservableCollection<MenuItem> PlatformContextMenuItems = new MenuBuilder(
            new Tuple<string, object>[]
            {
                new ("Context_HidePlatform", "hidePlatform()"),
                new ("Context_CreateShortcut", "createPlatformShortcut()"),
                new ("Context_ExportAccList", "exportAllAccounts()"),
            }).Result();

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

            if (DisabledPlatforms.Contains(""))
                DisabledPlatforms.Remove("");

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
            _ = DisabledPlatforms.Remove(platform);

            SaveSettings();
            await AppData.ReloadPage();
        }

        private string _stylesheet;
        public static string Stylesheet { get => Instance._stylesheet; set => Instance._stylesheet = value; }
        public static bool WindowsAccent { get => Instance._windowsAccent; set => Instance._windowsAccent = value; }
        public static bool AutoUpdatePlatforms { get => Instance._autoUpdatePlatforms; set => Instance._autoUpdatePlatforms = value; }
        public static bool CheckForUpdates { get => Instance._checkForUpdates; set => Instance._checkForUpdates = value; }

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

        #region STYLESHEET

        public static string TryGetStyle(string key)
        {
            try
            {
                return StylesheetInfo[key];
            }
            catch (Exception ex)
            {
                // Try load default theme for values
                var fallback = Path.Join("themes", "Dracula_Cyan", "info.yaml");
                if (File.Exists(fallback))
                {
                    var desc = new DeserializerBuilder().WithNamingConvention(HyphenatedNamingConvention.Instance).Build();
                    var fallbackSheet = JsonConvert.DeserializeObject<Dictionary<string, string>>(JsonConvert.SerializeObject(desc.Deserialize<object>(string.Join(Environment.NewLine, Globals.ReadAllLines(fallback)))));
                    try
                    {
                        if (fallbackSheet is not null) return fallbackSheet[key];
                        Globals.WriteToLog("fallback stylesheet was null!");
                        return "";
                    }
                    catch (Exception exFallback)
                    {
                        Globals.WriteToLog($"Could not find {key} in fallback stylesheet Dracula_Cyan", exFallback);
                        return "";
                    }
                }

                Globals.WriteToLog($"Could not find {key} in stylesheet, and fallback Dracula_Cyan does not exist", ex);
                return "";
            }
        }

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
            catch (Exception e)
            {
                Globals.WriteToLog("Failed to load stylesheet.", e);
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

            try
            {
                LoadStylesheet();
            }
            catch (FileNotFoundException ex)
            {
                // Check if CEF issue, and download if missing.
                if (!ex.ToString().Contains("YamlDotNet")) throw;
                AutoStartUpdaterAsAdmin("verify");
                Environment.Exit(1);
                throw;
            }

            return true;
        }

        private static void GenCssFromScss(string scss)
        {
            ScssResult convertedScss;
            try
            {
                if (AppSettings.OfflineMode)
                {
                    // Copy scss file
                    var offlineScss = Path.Join(Globals.UserDataFolder, $"offline_{scss}");
                    Directory.CreateDirectory(Path.GetDirectoryName(offlineScss));
                    if (File.Exists(offlineScss)) File.Delete(offlineScss);
                    var text = File.ReadAllText(scss);
                    text = Globals.RemoveHttpImports(text);
                    File.WriteAllText(offlineScss, text);
                    scss = offlineScss;
                }

                convertedScss = Scss.ConvertFileToCss(scss, new ScssOptions { InputFile = scss, OutputFile = StylesheetFile });
            }
            catch (ScssException e)
            {
                Globals.DebugWriteLine("ERROR in CSS: " + e.Message);
                throw;
            }

            // Convert from SCSS to CSS. The arguments are for "exception reporting", according to the SharpScss Git Repo.
            try
            {
                if (Globals.DeleteFile(StylesheetFile))
                {
                    File.WriteAllText(StylesheetFile, convertedScss.Css);
                    if (Globals.DeleteFile(StylesheetFile + ".map"))
                        File.WriteAllText(StylesheetFile + ".map", convertedScss.SourceMap);
                }
                else throw new Exception($"Could not delete StylesheetFile: '{StylesheetFile}'");
            }
            catch (Exception ex)
            {
                // Catches generic errors, as well as not being able to overwrite file errors, etc.
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_LoadStylesheetFailed"],
                    "Stylesheet error", "toastarea");
                Globals.WriteToLog($"Could not delete stylesheet file: {StylesheetFile}. Could not refresh stylesheet from scss.", ex);
            }

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
        private static (int, int, int) FromRgb(byte r, byte g, byte b)
        {
            // Adapted from: https://stackoverflow.com/a/4794649/5165437
            var rf = (r / 255f);
            var gf = (g / 255f);
            var bf = (b / 255f);

            var min = Math.Min(Math.Min(rf, gf), bf);
            var max = Math.Max(Math.Max(rf, gf), bf);
            var delta = max - min;

            var h = (float)0;
            var s = (float)0;
            var l = (max + min) / 2.0f;

            if (delta != 0)
            {
                if (l < 0.5f)
                    s = delta / (max + min);
                else
                    s = delta / (2.0f - max - min);

                if (Math.Abs(rf - max) < 0.01)
                    h = (gf - bf) / delta;
                else if (Math.Abs(gf - max) < 0.01)
                    h = 2f + (bf - rf) / delta;
                else if (Math.Abs(bf - max) < 0.01)
                    h = 4f + (rf - gf) / delta;
            }

            // Rounding and formatting for CSS
            h *= 60;
            s *= 100;
            l *= 100;

            return ((int)Math.Round(h), (int)Math.Round(s), (int)Math.Round(l));
        }

        [JSInvokable]
        public static async Task SetBackground(string path)
        {
            Background = $"{path}";

            if (File.Exists(path) && path != "")
            {
                Directory.CreateDirectory(Path.Join(Globals.UserDataFolder, "wwwroot\\img\\custom\\"));
                Globals.CopyFile(path, Path.Join(Globals.UserDataFolder, "wwwroot\\img\\custom\\background" + Path.GetExtension(path)));
                Background = $"img/custom/background{Path.GetExtension(path)}";
                SaveSettings();
            }
            await AppData.CacheReloadPage();
        }

        [JSInvokable]
        public static void SetSwitcherPassword(string pass)
        {
            PasswordHash = Globals.GetSha256HashString(pass);
            SaveSettings();
            _ = GeneralInvocableFuncs.ShowToast("success", Lang["Toast_PasswordChanged"], renderTo: "toastarea");
        }

        [JSInvokable]
        public static async Task SetUserData(string path)
        {
            // Verify this is different.
            var diOriginal = new DirectoryInfo(Globals.AppDataFolder);
            var diNew = new DirectoryInfo(path);
            if (diOriginal.FullName == diNew.FullName) return;

            Background = $"{path}";

            if (Directory.Exists(path) && path != "")
            {
                await File.WriteAllTextAsync(Path.Join(Globals.AppDataFolder, "userdata_path.txt"), path);
            }

            bool folderEmpty;
            if (Directory.Exists(path))
                folderEmpty = Globals.IsDirectoryEmpty(path);
            else
            {
                folderEmpty = true;
                Directory.CreateDirectory(path);
            }


            if (folderEmpty)
            {
                _ = GeneralInvocableFuncs.ShowToast("info", Lang["Toast_DataLocationCopying"], renderTo: "toastarea");
                if (!Globals.CopyFilesRecursive(Globals.AppDataFolder, path))
                    _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_FileCopyFail"], renderTo: "toastarea");
            }
            else
                _ = GeneralInvocableFuncs.ShowToast("info", Lang["Toast_DataLocationNotCopying"], renderTo: "toastarea");

            _ = GeneralInvocableFuncs.ShowToast("info", Lang["Toast_DataLocationSet"], renderTo: "toastarea");
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
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.TrayMinimizeNotExit_Toggle]");
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
            SaveSettings();
        }

        [SupportedOSPlatform("windows")]
        private static void SetAccentColor() => SetAccentColor(false);
        [SupportedOSPlatform("windows")]
        private static void SetAccentColor(bool userInvoked)
        {
            WindowsAccentColor = GetAccentColorHexString();
            var (r, g, b) = GetAccentColor();
            WindowsAccentColorHsl = FromRgb(r, g, b);
            
            SaveSettings();
            if (userInvoked)
                _ = AppData.ReloadPage();
        }

        [SupportedOSPlatform("windows")]
        public static string GetAccentColorHexString()
        {
            var (r, g, b) = GetAccentColor();
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

            const string valueExMsg = "The \"HKCU\\Software\\Microsoft\\Windows\\DWM\\AccentColor\" registry key value could not be parsed as an ABGR color.";
            throw new InvalidOperationException(valueExMsg);
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


        #region Updater

        /// <summary>
        /// Checks for an update
        /// </summary>
        public static void CheckForUpdate()
        {
            if (OfflineMode) return;
            if (UpdateCheckRan) return;
            UpdateCheckRan = true;

            try
            {
#if DEBUG
                var latestVersion = Globals.DownloadString("https://tcno.co/Projects/AccSwitcher/api?debug&v=" + Globals.Version);
#else
                var latestVersion = Globals.DownloadString("https://tcno.co/Projects/AccSwitcher/api?v=" + Globals.Version);
#endif
                if (CheckLatest(latestVersion)) return;
                // Show notification
                try
                {
                    _ = AppData.InvokeVoidAsync("showUpdateBar");
                }
                catch (Exception)
                {
                    PreRenderUpdate = true;
                }
            }
            catch (Exception e) when (e is WebException or AggregateException)
            {
                if (File.Exists("WindowSettings.json"))
                {
                    try
                    {
                        var o = JObject.Parse(Globals.ReadAllText("WindowSettings.json"));
                        if (o.ContainsKey("LastUpdateCheckFail"))
                        {
                            if (!(DateTime.TryParseExact((string)o["LastUpdateCheckFail"], "yyyy-MM-dd HH:mm:ss.fff",
                                      CultureInfo.InvariantCulture, DateTimeStyles.None, out var timediff) &&
                                  DateTime.Now.Subtract(timediff).Days >= 1)) return;
                        }

                        // Has not shown error today
                        _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_UpdateCheckFail"], renderTo: "toastarea", duration: 15000);
                        o["LastUpdateCheckFail"] = DateTime.Now.ToString("yyyy-MM-dd HH:mm:ss.fff");
                        File.WriteAllText("WindowSettings.json", o.ToString());
                    }
                    catch (JsonException je)
                    {
                        Globals.WriteToLog("Could not interpret <User Data>\\WindowSettings.json.", je);
                        _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_UserDataLoadFail"], renderTo: "toastarea", duration: 15000);
                        File.Move("WindowSettings.json", "WindowSettings.bak.json", true);
                    }
                }
                Globals.WriteToLog(@"Could not reach https://tcno.co/ to check for updates.", e);
            }
        }

        /// <summary>
        /// Verify updater files and start update
        /// </summary>
        [JSInvokable]
        public static void UpdateNow()
        {
            if (AppSettings.OfflineMode) return;
            try
            {
                if (Globals.InstalledToProgramFiles() && !Globals.IsAdministrator || !Globals.HasFolderAccess(Globals.AppDataFolder))
                {
                    _ = GeneralInvocableFuncs.ShowModal("notice:RestartAsAdmin");
                    return;
                }

                _ = GeneralInvocableFuncs.ShowToast("info", Lang["Toast_UpdatesDownloading"], renderTo: "toastarea");

                Directory.SetCurrentDirectory(Globals.AppDataFolder);
                // Download latest hash list
                var hashFilePath = Path.Join(Globals.UserDataFolder, "hashes.json");
                Globals.DownloadFile("https://tcno.co/Projects/AccSwitcher/latest/hashes.json", hashFilePath);

                // Verify updater files
                var verifyDictionary = JsonConvert.DeserializeObject<Dictionary<string, string>>(Globals.ReadAllText(hashFilePath));
                if (verifyDictionary == null)
                {
                    _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_UpdateVerifyFail"], renderTo: "toastarea");
                    return;
                }

                var updaterDict = verifyDictionary.Where(pair => pair.Key.StartsWith("updater")).ToDictionary(pair => pair.Key, pair => pair.Value);

                // Download and replace broken files
                Globals.RecursiveDelete("newUpdater", false);
                foreach (var (key, value) in updaterDict)
                {
                    if (key == null) continue;
                    if (File.Exists(key) && value == GeneralFuncs.GetFileMd5(key))
                        continue;
                    Globals.DownloadFile("https://tcno.co/Projects/AccSwitcher/latest/" + key.Replace('\\', '/'), key);
                }

                _ = GeneralInvocableFuncs.ShowToast("success", Lang["Toast_UpdatesDownloaded"], renderTo: "toastarea");
                AppData.UpdatePending = true;
            }
            catch (Exception e)
            {
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_FailedUpdateCheck"]);
                Globals.WriteToLog("Failed to check for updates:" + e);
            }
            Directory.SetCurrentDirectory(Globals.UserDataFolder);
        }

        /// <summary>
        /// Checks whether the program version is equal to or newer than the servers
        /// </summary>
        /// <param name="latest">Latest version provided by server</param>
        /// <returns>True when the program is up-to-date or ahead</returns>
        private static bool CheckLatest(string latest)
        {
            latest = latest.Replace("\r", "").Replace("\n", "");
            if (DateTime.TryParseExact(latest, "yyyy-MM-dd_mm", null, DateTimeStyles.None, out var latestDate))
            {
                if (DateTime.TryParseExact(Globals.Version, "yyyy-MM-dd_mm", null, DateTimeStyles.None, out var currentDate))
                {
                    if (latestDate.Equals(currentDate) || currentDate.Subtract(latestDate) > TimeSpan.Zero) return true;
                }
                else
                    Globals.WriteToLog($"Unable to convert '{latest}' to a date and time.");
            }
            else
                Globals.WriteToLog($"Unable to convert '{latest}' to a date and time.");
            return false;
        }

        public static void StartUpdaterAsAdmin(string args = "")
        {
            var exeLocation = Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location) ?? Environment.CurrentDirectory;
            Directory.SetCurrentDirectory(exeLocation);

            var proc = new ProcessStartInfo
            {
                WorkingDirectory = exeLocation,
                FileName = "updater\\TcNo-Acc-Switcher-Updater.exe",
                Arguments = args,
                UseShellExecute = true,
                Verb = "runas"
            };

            try
            {
                _ = Process.Start(proc);
                AppData.ActiveNavMan.NavigateTo("EXIT_APP", true);
            }
            catch (Exception ex)
            {
                Globals.WriteToLog(@"This program must be run as an administrator!" + Environment.NewLine + ex);
                try
                {
                    AppData.ActiveNavMan.NavigateTo("EXIT_APP", true);
                }
                catch (Exception e)
                {
                    Globals.WriteToLog("Could not close application... Just ending the server." + Environment.NewLine + e);
                    Environment.Exit(0);
                }
            }
        }

        public static void AutoStartUpdaterAsAdmin(string args = "")
        {
            // Run updater
            if (Globals.InstalledToProgramFiles() || !Globals.HasFolderAccess(Globals.AppDataFolder))
            {
                StartUpdaterAsAdmin(args);
            }
            else
            {
                _ = Process.Start(new ProcessStartInfo(Path.Join(Globals.AppDataFolder, @"updater\\TcNo-Acc-Switcher-Updater.exe")) { UseShellExecute = true, Arguments = args });
                try
                {
                    AppData.ActiveNavMan.NavigateTo("EXIT_APP", true);
                }
                catch (NullReferenceException)
                {
                    Environment.Exit(0);
                }
            }
        }
        #endregion
    }
}
