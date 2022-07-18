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
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Microsoft.Win32;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using SharpScss;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Classes;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Shared.ContextMenu;
using YamlDotNet.Serialization;
using YamlDotNet.Serialization.NamingConventions;

namespace TcNo_Acc_Switcher_Server.Data
{
    public interface IAppSettings
    {
        void SaveSettings();
        ObservableCollection<AppSettings.PlatformItem> Platforms { get; set; }
        string Language { get; set; }
        bool Rtl { get; set; }
        bool StreamerModeEnabled { get; set; }
        int ServerPort { get; set; }
        Point WindowSize { get; set; }
        bool AllowTransparency { get; set; }
        bool TrayMinimizeNotExit { get; set; }
        bool ShownMinimizedNotification { get; set; }
        bool StartCentered { get; set; }
        string ActiveTheme { get; set; }
        string ActiveBrowser { get; set; }
        string Background { get; set; }
        bool StatsEnabled { get; set; }
        bool StatsShare { get; set; }
        bool MinimizeOnSwitch { get; set; }
        bool DiscordRpc { get; set; }
        bool DiscordRpcShare { get; set; }
        string PasswordHash { get; set; } // SET should hash password.
        Dictionary<string, Dictionary<string, bool>> GloballyHiddenMetrics { get; set; }
        bool AlwaysAdmin { get; set; }
        string PasswordCurrent { get; set; } // SET should hash password.
        bool DesktopShortcut { get; set; }
        bool StartMenu { get; set; }
        bool StartMenuPlatforms { get; set; }
        bool ProtocolEnabled { get; set; }
        bool TrayStartup { get; set; }
        bool PreRenderUpdate { get; set; }
        ObservableCollection<MenuItem> PlatformContextMenuItems { get; init; }
        string Stylesheet { get; set; }
        bool WindowsAccent { get; set; }
        string WindowsAccentColor { get; set; }
        (float, float, float) WindowsAccentColorHsl { get; set; }
        (int, int, int) WindowsAccentColorInt { get; }
        Dictionary<string, string> StylesheetInfo { get; set; }
        bool StreamerModeTriggered { get; set; }

        /// <summary>
        /// Get platform details from an identifier, or the name.
        /// </summary>
        AppSettings.PlatformItem GetPlatform(string nameOrId);

        /// <summary>
        /// Check if any streaming software is running. Do let me know if you have a program name that you'd like to expand this list with!
        /// It's basically the program's .exe file, but without ".exe".
        /// </summary>
        /// <returns>True when streaming software is running</returns>
        bool StreamerModeCheck();

        /// <summary>
        /// Used in JS. Gets whether forget account is enabled (Whether to NOT show prompt, or show it).
        /// </summary>
        /// <returns></returns>
        Task<bool> GetTrayMinimizeNotExit();

        /// <summary>
        /// Sets the active browser
        /// </summary>
        Task SetActiveBrowser(string browser);

        string TryGetStyle(string key);

        /// <summary>
        /// Returns a block of CSS text to be used on the page. Used to hide or show certain things in certain ways, in components that aren't being added through Blazor.
        /// </summary>
        string GetCssBlock();

        /// <summary>
        /// Swaps in a requested stylesheet, and loads styles from file.
        /// </summary>
        /// <param name="swapTo">Stylesheet name (without .json) to copy and load</param>
        Task SwapStylesheet(string swapTo);

        /// <summary>
        /// Load stylesheet settings from stylesheet file.
        /// </summary>
        bool LoadStylesheetFromFile();
        string[] GetStyleList();

        void CheckShortcuts();
        void DesktopShortcut_Toggle();
        Task TrayMinimizeNotExit_Toggle();

        /// <summary>
        /// Toggle protocol functionality in Windows
        /// </summary>
        Task Protocol_Toggle();

        void WindowsAccent_Toggle();

        /// <summary>
        /// Create shortcuts in Start Menu
        /// </summary>
        /// <param name="platforms">true creates Platforms folder & drops shortcuts, otherwise only places main program & tray shortcut</param>
        void StartMenu_Toggle(bool platforms);

        void AutoStart_Toggle();

        public void StartNow();

        /// <summary>
        /// Checks for an update
        /// </summary>
        void CheckForUpdate();

        public Task UpdateNow();
        public void StartUpdaterAsAdmin(string args = "");
        public void AutoStartUpdaterAsAdmin(string args = "");
    }

    public class AppSettings : IAppSettings
    {
        [Inject] private ILang Lang { get; }
        [Inject] private IAppData AppData { get; }
        [Inject] private IAppFuncs AppFuncs { get; }
        [Inject] private IModalData ModalData { get; }
        [Inject] private IGeneralFuncs GeneralFuncs { get; }
        [Inject] private IShortcut Shortcut { get; }

        private readonly bool _isInit;
        public AppSettings()
        {
            if (!_isInit) return;
            try
            {
                if (File.Exists(SettingsFile)) JsonConvert.PopulateObject(File.ReadAllText(SettingsFile), this);
                //_instance = JsonConvert.DeserializeObject<AppSettings>(File.ReadAllText(SettingsFile), new JsonSerializerSettings());
            }
            catch (Exception e)
            {
                Globals.WriteToLog("Failed to load AppSettings", e);
                _ = GeneralFuncs.ShowToast("error", Lang["Toast_FailedLoadSettings"]);
                if (File.Exists(SettingsFile))
                    Globals.CopyFile(SettingsFile, SettingsFile.Replace(".json", ".old.json"));
            }

            _isInit = true;
            LoadStylesheetFromFile();
            CheckShortcuts();
            InitPlatformsList();
        }

        // From: https://stackoverflow.com/questions/44497878/singleton-how-do-i-instance-a-singleton-for-the-first-time
        //private static readonly Lazy<AppSettings> Lazy = new(() => new AppSettings());
        //public static AppSettings Instance => Lazy.Value;

        //public void SaveSettings() => GeneralFuncs.SaveSettings(SettingsFile, Lazy.Value);
        public void SaveSettings() => GeneralFuncs.SaveSettings(SettingsFile, this);


        public class PlatformItem : IComparable
        {
            public string Name = "";
            public bool Enabled;
            public int DisplayIndex = -1;
            [JsonIgnore] public string SafeName = "";
            [JsonIgnore] public string Identifier = "";
            [JsonIgnore] public string ExeName = "";
            [JsonIgnore] public List<string> PossibleIdentifiers = new(); // Other identifiers that can refer to this platform. (b, bnet, battlenet, etc)
            public int CompareTo(object o)
            {
                var a = this;
                var b = (PlatformItem)o;
                return string.CompareOrdinal(a.Name, b.Name);
            }

            // Needed for JSON serialization/deserialization
            public PlatformItem() { }

            // Used for first init. The rest of the info is added by BasicPlatforms.
            public PlatformItem(string name, bool enabled)
            {
                Name = name;
                SafeName = Globals.GetCleanFilePath(name);
                Enabled = enabled;
            }
            public PlatformItem(string name, List<string> identifiers, string exeName, bool enabled)
            {
                Name = name;
                SafeName = Globals.GetCleanFilePath(name);
                Enabled = enabled;
                DisplayIndex = -1;
                Identifier = identifiers[0];
                PossibleIdentifiers = identifiers;
                ExeName = exeName;
            }

            /// <summary>
            /// Set from a new PlatformItem - Not including Enabled.
            /// </summary>
            public void SetFromPlatformItem(PlatformItem inItem)
            {
                Name = inItem.Name;
                SafeName = Globals.GetCleanFilePath(Name);
                DisplayIndex = inItem.DisplayIndex;
                Identifier = inItem.Identifier;
                ExeName = inItem.ExeName;
            }

            public void SetEnabled(bool enabled)
            {
                Enabled = enabled;
            }
        }

        private static readonly ObservableCollection<PlatformItem> DefaultPlatforms = new()
        {
            new PlatformItem("Discord", true),
            new PlatformItem("Epic Games", true),
            new PlatformItem("Origin", true),
            new PlatformItem("Riot Games", true),
            new PlatformItem("Steam", true),
            new PlatformItem("Ubisoft", true),
        };

        [JsonProperty("Platforms", Order = 7)]
        private ObservableCollection<PlatformItem> _platforms = new();
        public ObservableCollection<PlatformItem> Platforms
        {
            get => _platforms;
            set
            {
                _platforms = value;
                _platforms.Sort();
            }
        }
        /// <summary>
        /// Get platform details from an identifier, or the name.
        /// </summary>
        public PlatformItem GetPlatform(string nameOrId) => Platforms.FirstOrDefault(x => x.Name == nameOrId || x.PossibleIdentifiers.Contains(nameOrId));
        private void InitPlatformsList()
        {
            // Add platforms, if none there.
            if (Platforms.Count == 0)
                Platforms = DefaultPlatforms;

            Platforms.First(x => x.Name == "Steam").SetFromPlatformItem(new PlatformItem("Steam", new List<string> { "s", "steam" }, "steam.exe", true));
        }

        [JsonProperty("Language", Order = 0)] public string Language { get; set; } = "";
        [JsonProperty("Rtl", Order = 1)]  public bool Rtl { get; set; } = CultureInfo.CurrentCulture.TextInfo.IsRightToLeft;
        [JsonProperty("StreamerModeEnabled", Order = 2)]  public bool StreamerModeEnabled { get; set; }
        [JsonProperty("ServerPort", Order = 3)] public int ServerPort { get; set; } = 1337;
        [JsonProperty("WindowSize", Order = 4)] public Point WindowSize { get; set; } = new() { X = 800, Y = 450 };
        [JsonProperty("AllowTransparency", Order = 5)] public bool AllowTransparency { get; set; } = true;
        [JsonProperty("Version", Order = 6)] public string Version = Globals.Version;
        [JsonProperty("TrayMinimizeNotExit", Order = 8)] public bool TrayMinimizeNotExit { get; set; }
        [JsonProperty("ShownMinimizedNotification", Order = 9)] public bool ShownMinimizedNotification { get; set; }
        [JsonProperty("StartCentered", Order = 10)] public bool StartCentered { get; set; }
        [JsonProperty("ActiveTheme", Order = 11)] public string ActiveTheme { get; set; } = "Dracula_Cyan";
        [JsonProperty("ActiveBrowser", Order = 12)] public string ActiveBrowser { get; set; } = "WebView";
        [JsonProperty("Background", Order = 13)] public string Background { get; set; } = "";
        [JsonIgnore] private bool _statsEnabled = true;
        [JsonProperty("CollectStats", Order = 14)]
        public bool StatsEnabled
        {
            get => _statsEnabled;
            set
            {
                if (!value) StatsShare = false;
                _statsEnabled = value;
            }
        }

        [JsonProperty("ShareAnonymousStats", Order = 15)] public bool StatsShare { get; set; } = true;
        [JsonProperty("MinimizeOnSwitch", Order = 16)] public bool MinimizeOnSwitch { get; set; }
        [JsonIgnore] private bool _discordRpc = true;
        [JsonProperty("DiscordRpcEnabled", Order = 17)]
        public bool DiscordRpc
        {
            get => _discordRpc;
            set
            {
                if (!value) DiscordRpcShare = false;
                _discordRpc = value;
            }
        }
        [JsonProperty("DiscordRpcShareTotalSwitches", Order = 18)] public bool DiscordRpcShare { get; set; } = true;
        [JsonProperty("PasswordHash", Order = 19)] public string PasswordHash { get; set; } = ""; // SET should hash password.
        [JsonProperty("GloballyHiddenMetrics", Order = 20)] public Dictionary<string, Dictionary<string, bool>> GloballyHiddenMetrics { get; set; } = new();
        [JsonProperty("AlwaysAdmin", Order = 21)] public bool AlwaysAdmin { get; set; }
        [JsonIgnore] public string PasswordCurrent { get; set; } // SET should hash password.
        [JsonIgnore] public bool DesktopShortcut { get; set; }
        [JsonIgnore] public bool StartMenu { get; set; }
        [JsonIgnore] public bool StartMenuPlatforms { get; set; }
        [JsonIgnore] public bool ProtocolEnabled { get; set; }
        [JsonIgnore] public bool TrayStartup { get; set; }
        [JsonIgnore] private bool UpdateCheckRan { get; set; }
        [JsonIgnore] public bool PreRenderUpdate { get; set; }

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

        public ObservableCollection<MenuItem> PlatformContextMenuItems { get; init; } = new MenuBuilder(
            new Tuple<string, Action>[]
            {
                new ("Context_HidePlatform", () => AppFuncs.HidePlatform()),
                new ("Context_CreateShortcut", async () => await Shortcut.CreatePlatformShortcut()),
                new ("Context_ExportAccList", async () => await AppFuncs.ExportAllAccounts()),
            }).Result();

        [JsonIgnore] public string Stylesheet { get; set; }
        [JsonIgnore] public bool WindowsAccent { get; set; }
        [JsonIgnore] public string WindowsAccentColor { get; set; } = "";
        [JsonIgnore] public (float, float, float) WindowsAccentColorHsl { get; set; } = (0, 0, 0);

        [SupportedOSPlatform("windows")]
        [JsonIgnore] public (int, int, int) WindowsAccentColorInt => GetAccentColor();

        // Constants
        [JsonIgnore] public readonly string SettingsFile = "WindowSettings.json";
        private string StylesheetFile => Path.Join("themes", ActiveTheme, "style.css");
        private string StylesheetInfoFile => Path.Join("themes", ActiveTheme, "info.yaml");
        [JsonIgnore] public Dictionary<string, string> StylesheetInfo { get; set; }

        [JsonIgnore] public bool StreamerModeTriggered { get; set; }

        /// <summary>
        /// Check if any streaming software is running. Do let me know if you have a program name that you'd like to expand this list with!
        /// It's basically the program's .exe file, but without ".exe".
        /// </summary>
        /// <returns>True when streaming software is running</returns>
        public bool StreamerModeCheck()
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
        public Task<bool> GetTrayMinimizeNotExit() => Task.FromResult(TrayMinimizeNotExit);

        /// <summary>
        /// Sets the active browser
        /// </summary>
        public async Task SetActiveBrowser(string browser)
        {
            ActiveBrowser = browser;
            await GeneralFuncs.ShowToast("success", Lang["Toast_RestartRequired"], Lang["Notice"], "toastarea");
        }

        #region STYLESHEET

        public string TryGetStyle(string key)
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
        public string GetCssBlock() => ".streamerCensor { display: " + (StreamerModeEnabled && StreamerModeTriggered ? "none!important" : "block") + "}";

        /// <summary>
        /// Swaps in a requested stylesheet, and loads styles from file.
        /// </summary>
        /// <param name="swapTo">Stylesheet name (without .json) to copy and load</param>
        public async Task SwapStylesheet(string swapTo)
        {
            ActiveTheme = swapTo.Replace(" ", "_");
            try
            {
                if (LoadStylesheetFromFile()) AppData.ReloadPage();
                else await GeneralFuncs.ShowToast("error", Lang["Toast_LoadStylesheetFailed"],
                    "Stylesheet error", "toastarea");
            }
            catch (Exception)
            {
                await GeneralFuncs.ShowToast("error", Lang["Toast_LoadStylesheetFailed"],
                    "Stylesheet error", "toastarea");
            }
        }

        /// <summary>
        /// Load stylesheet settings from stylesheet file.
        /// </summary>
        public bool LoadStylesheetFromFile()
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

        private void GenCssFromScss(string scss)
        {
            ScssResult convertedScss;
            try
            {
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
                    var text = convertedScss.Css;
                    File.WriteAllText(StylesheetFile, text);
                    if (Globals.DeleteFile(StylesheetFile + ".map"))
                        File.WriteAllText(StylesheetFile + ".map", convertedScss.SourceMap);
                }
                else throw new Exception($"Could not delete StylesheetFile: '{StylesheetFile}'");
            }
            catch (Exception ex)
            {
                // Catches generic errors, as well as not being able to overwrite file errors, etc.
                _ = GeneralFuncs.ShowToast("error", Lang["Toast_LoadStylesheetFailed"],
                    "Stylesheet error", "toastarea");
                Globals.WriteToLog($"Could not delete stylesheet file: {StylesheetFile}. Could not refresh stylesheet from scss.", ex);
            }

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

        #endregion

        #region SHORTCUTS
        public void CheckShortcuts()
        {
            Globals.DebugWriteLine(@"[Func:Data\AppSettings.CheckShortcuts]");
            DesktopShortcut = File.Exists(Path.Join(Shortcut.Desktop, "TcNo Account Switcher.lnk"));
            StartMenu = File.Exists(Path.Join(Shortcut.StartMenu, "TcNo Account Switcher.lnk"));
            StartMenuPlatforms = Directory.Exists(Path.Join(Shortcut.StartMenu, "Platforms"));
            TrayStartup = Shortcut.StartWithWindows_Enabled();

            if (OperatingSystem.IsWindows())
                ProtocolEnabled = Protocol_IsEnabled();
        }

        public void DesktopShortcut_Toggle()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.DesktopShortcut_Toggle]");
            var s = new Shortcut();
            _ = s.Shortcut_Switcher(Shortcut.Desktop);
            s.ToggleShortcut(!DesktopShortcut);
        }

        public async Task TrayMinimizeNotExit_Toggle()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.TrayMinimizeNotExit_Toggle]");
            if (TrayMinimizeNotExit) return;
            await GeneralFuncs.ShowToast("info", Lang["Toast_TrayPosition"], duration: 15000, renderTo: "toastarea");
            await GeneralFuncs.ShowToast("info", Lang["Toast_TrayHint"], duration: 15000, renderTo: "toastarea");
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
        public async Task Protocol_Toggle()
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
                    await GeneralFuncs.ShowToast("success", Lang["Toast_ProtocolEnabled"], Lang["Toast_ProtocolEnabledTitle"], "toastarea");
                }
                else
                {
                    // Remove
                    Registry.ClassesRoot.DeleteSubKeyTree("tcno");
                    await GeneralFuncs.ShowToast("success", Lang["Toast_ProtocolDisabled"], Lang["Toast_ProtocolDisabledTitle"], "toastarea");
                }
                ProtocolEnabled = Protocol_IsEnabled();
            }
            catch (UnauthorizedAccessException)
            {
                await GeneralFuncs.ShowToast("error", Lang["Toast_RestartAsAdmin"], Lang["Failed"], "toastarea");
                ModalData.ShowModal("confirm", ExtraArg.RestartAsAdmin);
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
                AppData.ReloadPage();
            }
        }

        [SupportedOSPlatform("windows")]
        private void SetAccentColor() => SetAccentColor(false);
        [SupportedOSPlatform("windows")]
        private void SetAccentColor(bool userInvoked)
        {
            WindowsAccentColor = GetAccentColorHexString();
            var (r, g, b) = GetAccentColor();
            WindowsAccentColorHsl = FromRgb(r, g, b);

            if (userInvoked)
                AppData.ReloadPage();
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
        public void StartMenu_Toggle(bool platforms)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.StartMenu_Toggle]");
            if (platforms)
            {
                var platformsFolder = Path.Join(Shortcut.StartMenu, "Platforms");
                if (Directory.Exists(platformsFolder)) Globals.RecursiveDelete(Path.Join(Shortcut.StartMenu, "Platforms"), false);
                else
                {
                    _ = Directory.CreateDirectory(platformsFolder);
                    foreach (var platform in Platforms)
                    {
                        CreatePlatformShortcut(platformsFolder, platform.Name, platform.SafeName.ToLowerInvariant());
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
        public void AutoStart_Toggle()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.Task_Toggle]");
            Shortcut.StartWithWindows_Toggle(!TrayStartup);
        }

        public void StartNow()
        {
            _ = NativeFuncs.StartTrayIfNotRunning() switch
            {
                "Started Tray" => GeneralFuncs.ShowToast("success", Lang["Toast_TrayStarted"], renderTo: "toastarea"),
                "Already running" => GeneralFuncs.ShowToast("info", Lang["Toast_TrayRunning"], renderTo: "toastarea"),
                "Tray users not found" => GeneralFuncs.ShowToast("error", Lang["Toast_TrayUsersMissing"], renderTo: "toastarea"),
                _ => GeneralFuncs.ShowToast("error", Lang["Toast_TrayFail"], renderTo: "toastarea")
            };
        }

        private void CreatePlatformShortcut(string folder, string platformName, string args)
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
        public async void CheckForUpdate()
        {
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
                    await AppData.InvokeVoidAsync("showUpdateBar");
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
                        await GeneralFuncs.ShowToast("error", Lang["Toast_UpdateCheckFail"], renderTo: "toastarea", duration: 15000);
                        o["LastUpdateCheckFail"] = DateTime.Now.ToString("yyyy-MM-dd HH:mm:ss.fff");
                        await File.WriteAllTextAsync("WindowSettings.json", o.ToString());
                    }
                    catch (JsonException je)
                    {
                        Globals.WriteToLog("Could not interpret <User Data>\\WindowSettings.json.", je);
                        await GeneralFuncs.ShowToast("error", Lang["Toast_UserDataLoadFail"], renderTo: "toastarea", duration: 15000);
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
        public async Task UpdateNow()
        {
            try
            {
                if (Globals.InstalledToProgramFiles() && !Globals.IsAdministrator || !Globals.HasFolderAccess(Globals.AppDataFolder))
                {
                    ModalData.ShowModal("confirm", ExtraArg.RestartAsAdmin);
                    return;
                }

                Directory.SetCurrentDirectory(Globals.AppDataFolder);
                // Download latest hash list
                var hashFilePath = Path.Join(Globals.UserDataFolder, "hashes.json");
                await Globals.DownloadFileAsync("https://tcno.co/Projects/AccSwitcher/latest/hashes.json", hashFilePath);

                // Verify updater files
                var verifyDictionary = JsonConvert.DeserializeObject<Dictionary<string, string>>(Globals.ReadAllText(hashFilePath));
                if (verifyDictionary == null)
                {
                    await GeneralFuncs.ShowToast("error", Lang["Toast_UpdateVerifyFail"]);
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
                    await Globals.DownloadFileAsync("https://tcno.co/Projects/AccSwitcher/latest/" + key.Replace('\\', '/'), key);
                }

                AutoStartUpdaterAsAdmin();
            }
            catch (Exception e)
            {
                await GeneralFuncs.ShowToast("error", Lang["Toast_FailedUpdateCheck"]);
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

        public void StartUpdaterAsAdmin(string args = "")
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
                AppData.NavigateTo("EXIT_APP", true);
            }
            catch (Exception ex)
            {
                Globals.WriteToLog(@"This program must be run as an administrator!" + Environment.NewLine + ex);
                try
                {
                    AppData.NavigateTo("EXIT_APP", true);
                }
                catch (Exception e)
                {
                    Globals.WriteToLog("Could not close application... Just ending the server." + Environment.NewLine + e);
                    Environment.Exit(0);
                }
            }
        }

        public void AutoStartUpdaterAsAdmin(string args = "")
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
                    AppData.NavigateTo("EXIT_APP", true);
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
