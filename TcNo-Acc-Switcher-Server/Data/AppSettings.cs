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
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using TcNo_Acc_Switcher_Server.Shared.ContextMenu;
using TcNo_Acc_Switcher_Server.Shared.Toast;
using YamlDotNet.Serialization;
using YamlDotNet.Serialization.NamingConventions;

namespace TcNo_Acc_Switcher_Server.Data
{
    public class AppSettings
    {
        [JsonIgnore] private bool _desktopShortcut;
        [JsonIgnore] private bool _startMenu;
        [JsonIgnore] private bool _startMenuPlatforms;
        [JsonIgnore] private bool _protocolEnabled;
        [JsonIgnore] private bool _trayStartup;
        [JsonIgnore] private bool _updateCheckRan;
        [JsonIgnore] private bool _preRenderUpdate;


        private static void InitPlatformsList()
        {
            // Add platforms, if none there.
            if (_platforms.Count == 0)
                _platforms = DefaultPlatforms;

            _platforms.First(x => x.Name == "Steam").SetFromPlatformItem(new PlatformItem("Steam", new List<string> { "s", "steam" }, "steam.exe", true));

            // Load other platforms by initializing BasicPlatforms
            _ = BasicPlatforms.Instance;
        }

        public static bool DiscordRpc {
            get => _discordRpc;
            set
            {
                if (!value) _discordRpcShare = false;
                _discordRpc = value;
            }
        }

        public static bool StatsEnabled
        {
            get => _statsEnabled;
            set
            {
                if (!value) _statsShare = false;
                _statsEnabled = value;
            }
        }

        public static bool StatsShare { get => _statsShare; set => _statsShare = value; }
        public static bool MinimizeOnSwitch { get => _minimizeOnSwitch; set => _minimizeOnSwitch = value; }
        
        public static bool DesktopShortcut { get => _desktopShortcut; set => _desktopShortcut = value; }

        public static bool StartMenu { get => _startMenu; set => _startMenu = value; }

        public static bool StartMenuPlatforms { get => _startMenuPlatforms; set => _startMenuPlatforms = value; }

        public static bool ProtocolEnabled { get => _protocolEnabled; set => _protocolEnabled = value; }
        public static bool TrayStartup { get => _trayStartup; set => _trayStartup = value; }
        private static bool UpdateCheckRan { get =>_updateCheckRan; set => _updateCheckRan = value; }
        public static bool PreRenderUpdate { get =>_preRenderUpdate; set => _preRenderUpdate = value; }

        
        public static bool AlwaysAdmin
        {
            get =>_alwaysAdmin;
            set => _alwaysAdmin = value;
        }
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
        public static Dictionary<string, Dictionary<string, bool>> GloballyHiddenMetrics { get => _globallyHiddenMetrics; set => _globallyHiddenMetrics = value; }

        private string _stylesheet;
        public static string Stylesheet { get => _stylesheet; set => _stylesheet = value; }

        private bool _windowsAccent;
        public static bool WindowsAccent { get => _windowsAccent; set => _windowsAccent = value; }

        private string _windowsAccentColor = "";
        public static string WindowsAccentColor { get => _windowsAccentColor; set => _windowsAccentColor = value; }

        private (float, float, float) _windowsAccentColorHsl = (0, 0, 0);
        public static (float, float, float) WindowsAccentColorHsl { get => _windowsAccentColorHsl; set => _windowsAccentColorHsl = value; }

        [SupportedOSPlatform("windows")]
        public static (int, int, int) WindowsAccentColorInt => GetAccentColor();

        // Constants
        public static readonly string SettingsFile = "WindowSettings.json";
        private static string StylesheetFile => Path.Join("themes", ActiveTheme, "style.css");
        private static string StylesheetInfoFile => Path.Join("themes", ActiveTheme, "info.yaml");
        private Dictionary<string, string> _stylesheetInfo;
        public static Dictionary<string, string> StylesheetInfo { get => _stylesheetInfo; set => _stylesheetInfo = value; }

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
        public void SwapStylesheet(string swapTo)
        {
            _activeTheme = swapTo.Replace(" ", "_");
            try
            {
                if (LoadStylesheetFromFile())
                {
                    AppData.ReloadPage();
                    return;
                }
            }
            catch (Exception)
            {
                //
            }

            AData.ShowToastLang(ToastType.Error, "Error", "Toast_LoadStylesheetFailed");
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
                AData.ShowToastLang(ToastType.Error, "Error", "Toast_LoadStylesheetFailed");
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

        #endregion

        #region SHORTCUTS

        #region WindowsAccent

        [SupportedOSPlatform("windows")]
        public static void SetAccentColor() => SetAccentColor(false);
        [SupportedOSPlatform("windows")]
        public static void SetAccentColor(bool userInvoked)
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
                        AData.ShowToastLang(ToastType.Error, "Toast_UpdateCheckFail", 15000);
                        o["LastUpdateCheckFail"] = DateTime.Now.ToString("yyyy-MM-dd HH:mm:ss.fff");
                        await File.WriteAllTextAsync("WindowSettings.json", o.ToString());
                    }
                    catch (JsonException je)
                    {
                        Globals.WriteToLog("Could not interpret <User Data>\\WindowSettings.json.", je);
                        AData.ShowToastLang(ToastType.Error, "Toast_UserDataLoadFail", 15000);
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
        public void UpdateNow()
        {
            try
            {
                if (Globals.InstalledToProgramFiles() && !Globals.IsAdministrator || !Globals.HasFolderAccess(Globals.AppDataFolder))
                {
                    ModalData.ShowModal("confirm", ModalData.ExtraArg.RestartAsAdmin);
                    return;
                }

                Directory.SetCurrentDirectory(Globals.AppDataFolder);
                // Download latest hash list
                var hashFilePath = Path.Join(Globals.UserDataFolder, "hashes.json");
                Globals.DownloadFile("https://tcno.co/Projects/AccSwitcher/latest/hashes.json", hashFilePath);

                // Verify updater files
                var verifyDictionary = JsonConvert.DeserializeObject<Dictionary<string, string>>(Globals.ReadAllText(hashFilePath));
                if (verifyDictionary == null)
                {
                    AData.ShowToastLang(ToastType.Error, "Toast_UpdateVerifyFail");
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

                AutoStartUpdaterAsAdmin();
            }
            catch (Exception e)
            {
                AData.ShowToastLang(ToastType.Error, "Toast_FailedUpdateCheck");
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
