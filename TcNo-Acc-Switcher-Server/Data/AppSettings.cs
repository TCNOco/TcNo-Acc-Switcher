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
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using YamlDotNet.Serialization;
using YamlDotNet.Serialization.NamingConventions;
using Task = TcNo_Acc_Switcher_Server.Pages.General.Classes.Task;

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

        // Variables
        private bool _updateAvailable;
        [JsonIgnore] public bool UpdateAvailable { get => _instance._updateAvailable; set => _instance._updateAvailable = value; }

        private bool _streamerModeEnabled = true;
        [JsonProperty("StreamerModeEnabled", Order = 1)] public bool StreamerModeEnabled { get => _instance._streamerModeEnabled; set => _instance._streamerModeEnabled = value; }

        private int _serverPort = 5000 ;
        [JsonProperty("ServerPort", Order = 2)] public int ServerPort { get => _instance._serverPort; set => _instance._serverPort = value; }

        private Point _windowSize = new() { X = 800, Y = 450 };
        [JsonProperty("WindowSize", Order = 3)] public Point WindowSize { get => _instance._windowSize; set => _instance._windowSize = value; }

        private SortedSet<string> _disabledPlatforms = new() {};

        [JsonProperty("DisabledPlatforms", Order = 4)]
        public SortedSet<string> DisabledPlatforms { get => _instance._disabledPlatforms; set => _instance._disabledPlatforms = value; }

        private bool _trayMinimizeNotExit;

        [JsonProperty("TrayMinimizeNotExit", Order = 4)]
        public bool TrayMinimizeNotExit
        {
            get => _instance._trayMinimizeNotExit;
            set
            {
                if (value)
                {
                    _ = GeneralInvocableFuncs.ShowToast("info", "On clicking the Exit button: I'll be on the Windows Tray! (Right of Start Bar)", duration: 15000, renderTo: "toastarea");
                    _ = GeneralInvocableFuncs.ShowToast("info", "Hint: Ctrl+Click the 'X' to close me completely, or via the Tray > 'Exit'", duration: 15000, renderTo: "toastarea");
                }
                _instance._trayMinimizeNotExit = value; 

            }
        }

        private bool _trayMinimizeLessMem;
        [JsonProperty("TrayMinimizeLessMem", Order = 4)] public bool TrayMinimizeLessMem { get => _instance._trayMinimizeLessMem; set => _instance._trayMinimizeLessMem = value; }


        private bool _desktopShortcut;
        [JsonIgnore] public bool DesktopShortcut { get => _instance._desktopShortcut; set => _instance._desktopShortcut = value; }
        private bool _startMenu;
        [JsonIgnore] public bool StartMenu { get => _instance._startMenu; set => _instance._startMenu = value; }
        private bool _startMenuPlatforms;
        [JsonIgnore] public bool StartMenuPlatforms { get => _instance._startMenuPlatforms; set => _instance._startMenuPlatforms = value; }
        private bool _trayStartup;
        [JsonIgnore] public bool TrayStartup { get => _instance._trayStartup; set => _instance._trayStartup = value; }

        private bool _currentlyElevated;
        [JsonIgnore] public bool CurrentlyElevated { get => _instance._currentlyElevated; set => _instance._currentlyElevated = value; }

        private string _selectedStylesheet;
        [JsonIgnore] public string SelectedStylesheet { get => _instance._selectedStylesheet; set => _instance._selectedStylesheet = value; }

        [JsonIgnore] public static readonly string[] PlatformList = { "Steam", "Origin", "Ubisoft", "BattleNet", "Epic", "Riot" };

        [JsonIgnore] public string PlatformContextMenu = @"[
              {""Hide platform"": ""hidePlatform()""},
              {""Create Desktop Shortcut"": ""createPlatformShortcut()""}
            ]";
        [JSInvokable]
        public static async System.Threading.Tasks.Task HidePlatform(string platform)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Data\AppSettings.HidePlatform]");
            _instance.DisabledPlatforms.Add(platform);
            _instance.SaveSettings();
            await AppData.ActiveIJsRuntime.InvokeVoidAsync("location.reload");
        }

        public static async System.Threading.Tasks.Task ShowPlatform(string platform)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Data\AppSettings.ShowPlatform]");
            _instance.DisabledPlatforms.Remove(platform);
            _instance.SaveSettings();
            await AppData.ActiveIJsRuntime.InvokeVoidAsync("location.reload");
        }


        // Variables loaded from other files:
        // To create this: Take a standard YAML stylesheet and convert it to JSON,
        // Then replace:
        // "  " with "  { "
        // ",\r\n" with " },\r\n"
        // ": " with ", "
        // THEN DON'T FORGET TO ADD NEW ENTRIES INTO Shared\DynamicStylesheet.razor!
        private readonly Dictionary<string, string> _defaultStylesheet = new()
        {
            { "name", "Dracula Cyan" },
            { "selectionColor", "#FFFFFF" },
            { "selectionBackground", "#274560" },
            { "contextMenuBackground", "#253340" },
            { "contextMenuBackground-hover", "#1B2737" },
            { "contextMenuLeftBorder-hover", "#8BE9FD" },
            { "contextMenuTextColor", "#FFFFFF" },
            { "contextMenuTextColor-hover", "#FFFFFF" },
            { "contextMenuBoxShadow", "10px 10px 5px -3px rgb(0 0 0 / 30%)" },
            { "headerbarBackground", "#253340" },
            { "headerbarTCNOFill", "white" },
            { "windowControlsBackground", "#14151E" },
            { "windowControlsBackground-hover", "rgba(255,255,255,0.2)" },
            { "windowControlsBackground-active", "rgba(255,255,255,0.1)" },
            { "windowControlsCloseBackground", "#E81123" },
            { "windowControlsCloseBackground-active", "#F1707A" },
            { "windowTitleColor", "white" },
            { "footerBackground", "#252A2E" },
            { "footerColor", "#DDD" },
            { "footerIcoSettings-Fill", "white" },
            { "footerIcoQuestion-Fill", "white" },
            { "scrollbarTrackBackground", "#1E2934" },
            { "scrollbarThumbBackground", "#8BE9FD" },
            { "scrollbarThumbBackground-hover", "#8BE9FD" },
            { "accountListItemWidth", "100px" },
            { "accountListItemHeight", "135px" },
            { "accountBackground-placeholder", "#13222f" },
            { "accountBorder-placeholder", "2px dashed #039CBF" },
            { "accountPColor", "#DDD" },
            { "accountColor", "white" },
            { "accountBackground-hover", "#13222f" },
            { "accountBackground-checked", "#274560" },
            { "accountBorder-hover", "#039CBF" },
            { "accountBorder-checked", "#8BE9FD" },
            { "accountListBackground", "linear-gradient(90deg, #11181d, #0E1419 100%)" },
            { "mainBackground", "#0E1419" },
            { "defaultTextColor", "white" },
            { "linkColor", "#8BE9FD" },
            { "linkColor-hover", "#8BE9FD" },
            { "linkColor-active", "#8BE9FD" },
            { "borderedItemBorderColor", "#888" },
            { "borderedItemBorderColor-focus", "#888" },
            { "borderedItemBorderColorBottom-focus", "#8BE9FD" },
            { "buttonBackground", "#274560" },
            { "buttonBackground-hover", "#28374E" },
            { "buttonBackground-active", "#28374E" },
            { "buttonBorder", "#274560" },
            { "buttonBorder-hover", "#274560" },
            { "buttonBorder-active", "#8BE9FD" },
            { "buttonColor", "white" },
            { "checkboxBorder", "#6272A4" },
            { "checkboxBorder-checked", "#6272A4" },
            { "checkboxBackground", "#253340" },
            { "checkboxBackground-checked", "#8BE9FD" },
            { "inputBackground", "#070A0D" },
            { "inputColor", "white" },
            { "dropdownBackground", "#274560" },
            { "dropdownBorder", "#274560" },
            { "dropdownColor", "white" },
            { "dropdownItemBackground-hover", "#28374E" },
            { "dropdownItemBackground-active", "#28374E" },
            { "listBackground", "#0E1419" },
            { "listBackgroundColor-checked", "#FFAA00" },
            { "listColor", "white" },
            { "listColor-checked", "white" },
            { "listTextColor-before", "#8BE9FD" },
            { "listTextColor-before-checked", "#8BE9FD" },
            { "listTextColor-after", "#8BE9FD" },
            { "listTextColor-after-checked", "#8BE9FD" },
            { "settingsHeaderColor", "white" },
            { "settingsHeaderHrBorder", "#BBB" },
            { "modalBackground", "#00000055" },
            { "modalFgBackground", "#0E1419" },
            { "modalInputBackground", "#070A0D" },
            { "modalTCNOFill", "white" },
            { "modalIcoDiscord-Fill", "white" },
            { "modalIcoGitHub-Fill", "white" },
            { "modalIcoNetworking-Fill", "white" },
            { "modalIcoDoc-Fill", "white" },
            { "modalFoundColor", "lime" },
            { "modalFoundBackground", "green" },
            { "modalNotFoundColor", "red" },
            { "modalNotFoundBackground", "darkred" },
            { "notification-color-dark-text", "white" },
            { "notification-color-dark-border", "rgb(20, 20, 20)" },
            { "notification-color-info", "rgb(139, 133, 253)" },
            { "notification-color-info-light", "rgba(139, 133, 253, .25)" },
            { "notification-color-info-lighter", "#274560" },
            { "notification-color-success", "rgb(80 250 123)" },
            { "notification-color-success-light", "rgba(80 250 123, .25)" },
            { "notification-color-success-lighter", "#274560" },
            { "notification-color-warning", "rgb(255 184 108)" },
            { "notification-color-warning-light", "rgba(255 184 108, .25)" },
            { "notification-color-warning-lighter", "#274560" },
            { "notification-color-error", "rgb(255 85 85)" },
            { "notification-color-error-light", "rgba(255 85 85, .25)" },
            { "notification-color-error-lighter", "#274560" },
            { "updateBarBackground", "#8BE9FD" },
            { "updateBarColor", "black" },
            { "battlenetIcoOWTank-Fill", "white" },
            { "battlenetIcoOWDamage-Fill", "white" },
            { "battlenetIcoOWSupport-Fill", "white" },
            { "limited", "yellow" },
            { "vac", "rgb(255 85 85)" },
            { "riotIcoLeague-Fill", "rgb(255, 215, 85)" },
            { "riotIcoRuneterra-Fill", "rgb(255, 215, 85)" },
            { "riotIcoValorant-Fill", "rgb(255, 85, 85)" },
            { "platformScreenBackground", "var(--accountListBackground)" },
            { "platformBorderColor", "#8BE9FD" },
            { "platformFilter-Hover", "brightness(97%) saturate(1.10) contrast(102%)" },
            { "platformFilter-Active", "brightness(95%) saturate(1.05)" },
            { "platformFilterIcon-Hover", "brightness(97%) saturate(1.05) contrast(102%)" },
            { "platformFilterIcon-Active", "brightness(95%) saturate(1.05)" },
            { "platformTransform-Hover", "scale(1.025)" },
            { "platformTransform-Active", "scale(0.98)" },
            { "platformTransform-HoverAnimation-boxShadow-0", "0 0 0 0 rgba(139, 233, 253, 0.7)" },
            { "platformTransform-HoverAnimation-boxShadow-70", "0 0 0 10px rgba(139, 233, 253, 0)" },
            { "platformTransform-HoverAnimation-boxShadow-100", "0 0 0 10px rgba(139, 233, 253, 0)" },
            { "platformTransform-HoverAnimation-transform-0", "scale(1)" },
            { "platformTransform-HoverAnimation-transform-70", "scale(1.02)" },
            { "platformTransform-HoverAnimation-transform-100", "scale(1)" },
            { "icoBattleNetBG", "linear-gradient(0deg, rgba(37,51,64,1) 0%, rgba(37,51,64,1) 0%, rgba(32,33,35,1) 100%, rgba(0,212,255,1) 202123%)" },
            { "icoBattleNetText-Fill", "none" },
            { "icoBattleNetText-Stroke", "#00a5f8" },
            { "icoBattleNetText-StrokeWidth", "8.33px" },
            { "icoBattleNetFG-Fill", "#00a5f8" },
            { "icoBattleNetFG-Stroke", "none" },
            { "icoBattleNetFG-StrokeWidth", "0" },
            { "icoBattleNetGlass-Fill", "rgba(255, 255, 255, 0.02)" },
            { "icoEpicBG", "linear-gradient(0deg, rgba(37,51,64,1) 0%, rgba(37,51,64,1) 0%, rgba(32,33,35,1) 100%, rgba(0,212,255,1) 202123%)" },
            { "icoEpicText-Fill", "none" },
            { "icoEpicText-Stroke", "#00a5f8" },
            { "icoEpicText-StrokeWidth", "8.33px" },
            { "icoEpicFG-Fill", "#00a5f8" },
            { "icoEpicFG-Stroke", "#00a5f8" },
            { "icoEpicFG-StrokeWidth", "50px" },
            { "icoEpicGlass-Fill", "rgba(255, 255, 255, 0.02)" },
            { "icoOriginBG", "linear-gradient(0deg, rgba(37,51,64,1) 0%, rgba(37,51,64,1) 0%, rgba(32,33,35,1) 100%, rgba(0,212,255,1) 202123%)" },
            { "icoOriginText-Fill", "none" },
            { "icoOriginText-Stroke", "#00a5f8" },
            { "icoOriginText-StrokeWidth", "8.33px" },
            { "icoOriginFG-Fill", "none" },
            { "icoOriginFG-Stroke", "#00a5f8" },
            { "icoOriginFG-StrokeWidth", "12.51px" },
            { "icoOriginGlass-Fill", "rgba(255, 255, 255, 0.02)" },
            { "icoRiotBG", "linear-gradient(0deg, rgba(37,51,64,1) 0%, rgba(37,51,64,1) 0%, rgba(32,33,35,1) 100%, rgba(0,212,255,1) 202123%)" },
            { "icoRiotText-Fill", "none" },
            { "icoRiotText-Stroke", "#00a5f8" },
            { "icoRiotText-StrokeWidth", "8.33px" },
            { "icoRiotFG-Fill", "none" },
            { "icoRiotFG-Stroke", "#00a5f8" },
            { "icoRiotFG-StrokeWidth", "12.51px" },
            { "icoRiotGlass-Fill", "rgba(255, 255, 255, 0.02)" },
            { "icoSteamBG", "linear-gradient(0deg, rgba(37,51,64,1) 0%, rgba(37,51,64,1) 0%, rgba(32,33,35,1) 100%, rgba(0,212,255,1) 202123%)" },
            { "icoSteamText-Fill", "none" },
            { "icoSteamText-Stroke", "#00a5f8" },
            { "icoSteamText-StrokeWidth", "8.33px" },
            { "icoSteamFG-Fill", "none" },
            { "icoSteamFG-Stroke", "#00a5f8" },
            { "icoSteamFG-StrokeWidth", "12.51px" },
            { "icoSteamGlass-Fill", "rgba(255, 255, 255, 0.02)" },
            { "icoUbisoftBG", "linear-gradient(0deg, rgba(37,51,64,1) 0%, rgba(37,51,64,1) 0%, rgba(32,33,35,1) 100%, rgba(0,212,255,1) 202123%)" },
            { "icoUbisoftText-Fill", "none" },
            { "icoUbisoftText-Stroke", "#00a5f8" },
            { "icoUbisoftText-StrokeWidth", "8.33px" },
            { "icoUbisoftFG-Fill", "none" },
            { "icoUbisoftFG-Stroke", "#00a5f8" },
            { "icoUbisoftFG-StrokeWidth", "12.51px" },
            { "icoUbisoftGlass-Fill", "rgba(255, 255, 255, 0.02)" }
        };

        private Dictionary<string, string> _stylesheet;
        [JsonIgnore] public Dictionary<string, string> Stylesheet { get => _instance._stylesheet; set => _instance._stylesheet = value; }

        // Constants
        [JsonIgnore] public readonly string SettingsFile = "WindowSettings.json";
        [JsonIgnore] public readonly string StylesheetFile = "StyleSettings.yaml";
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
        public static Task<bool> GetTrayMinimizeNotExit() => System.Threading.Tasks.Task.FromResult(_instance.TrayMinimizeNotExit);

        /// <summary>
        /// Returns a block of CSS text to be used on the page. Used to hide or show certain things in certain ways, in components that aren't being added through Blazor.
        /// </summary>
        public string GetCssBlock() => ".streamerCensor { display: " + (_instance.StreamerModeEnabled && _instance.StreamerModeTriggered ? "none!important" : "block") + "}";

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

        public void LoadFromFile()
        {
            Globals.DebugWriteLine(@"[Func:Data\AppSettings.LoadFromFile]");
            // Main settings
            if (!File.Exists(SettingsFile)) SaveSettings();
            else SetFromJObject(GeneralFuncs.LoadSettings(SettingsFile, GetJObject()));
            // Stylesheet
            LoadStylesheetFromFile();
        }

        #region STYLESHEET
        /// <summary>
        /// Swaps in a requested stylesheet, and loads styles from file.
        /// </summary>
        /// <param name="swapTo">Stylesheet name (without .json) to copy and load</param>
        public async System.Threading.Tasks.Task SwapStylesheet(string swapTo)
        {
            File.Copy($"themes\\{swapTo.Replace(' ', '_')}.yaml", StylesheetFile, true);
            LoadStylesheetFromFile();
            await AppData.ActiveIJsRuntime.InvokeVoidAsync("location.reload");
        }

        /// <summary>
        /// Load stylesheet settings from stylesheet file.
        /// </summary>
        public void LoadStylesheetFromFile()
        {
            if (!File.Exists(StylesheetFile)) File.Copy("themes\\Default.yaml", StylesheetFile);
            // Load new stylesheet
            var desc = new DeserializerBuilder().WithNamingConvention(HyphenatedNamingConvention.Instance).Build();
            var attempts = 0;
            var text = File.ReadAllLines(StylesheetFile);
            while (attempts <= text.Length)
            {
                try
                {
                    var dict = JsonConvert.DeserializeObject<Dictionary<string, string>>(JsonConvert.SerializeObject(desc.Deserialize<object>(string.Join(Environment.NewLine, text))));
                    // Load default values, and copy in new values (Just in case some are missing)
                    _instance._stylesheet = _instance._defaultStylesheet;
                    var unchangedKeys = new List<string>(_instance._defaultStylesheet.Keys);
                    if (dict != null)
                        foreach (var (key, val) in dict)
                        {
                            _instance._stylesheet[key] = val;
                            unchangedKeys.Remove(key);
                        }
                    // Add new keys to file:
                    List<string> newKeys = new();
                    foreach (var key in unchangedKeys)
                    {
                        var value = _instance._stylesheet[key];
                        if (value.Contains("#")) value = $"\"{value}\"";
                        newKeys.Add(key + ": " + value);
                    }
                    // Write new lines to file
                    if (newKeys.Count > 0)
                    {
                        newKeys.Insert(0, Environment.NewLine + "# -- NEW ITEMS [See default themes to see where they go] -- #");
                        File.AppendAllLines(StylesheetFile, newKeys);
                    }
                    break;
                }
                catch (YamlDotNet.Core.SemanticErrorException e)
                {
                    // Check lines for common mistakes:
                    var line = e.End.Line - 1;
                    var currentLine = text[line];
                    var foundIssue = false;
                    // - Check for leading or trailing spaces:
                    if (currentLine[0] == ' ' || currentLine[^1] == ' ')
                    {
                        currentLine = currentLine.Trim();
                        foundIssue = true;
                    }

                    // - Check if there is a colon and a space (required for it to work), add space if no space.
                    if (currentLine.Contains(':') && currentLine[currentLine.IndexOf(':') + 1] != ' ')
                    {
                        currentLine = currentLine.Insert(currentLine.IndexOf(':') + 1, " ");
                        foundIssue = true;
                    }

                    if (!foundIssue)
                    {
                        // ReSharper disable once RedundantAssignment
                        attempts++;

                        // Comment out line:
                        currentLine = $"# -- ERROR -- # {currentLine}";
                    }

                    // Replace line and save new file
                    if (text[line] != currentLine)
                    {
                        text[line] = currentLine;
                        File.WriteAllLines(StylesheetFile, text);
                    }

                    if (attempts == text.Length)
                    {
                        // All 10 fix attempts have failed -> Copy in default file.
                        if (File.Exists("themes\\Default.yaml"))
                        {
                            // Check if haven't already copied default file into the stylesheet file
                            var curThemeHash = GeneralFuncs.GetFileMd5(StylesheetFile);
                            var defaultThemeHash = GeneralFuncs.GetFileMd5("themes\\Default.yaml");
                            if (curThemeHash != defaultThemeHash)
                            {
                                File.Copy("themes\\Default.yaml", StylesheetFile, true);
                                attempts = text.Length - 1; // One last attempt -> This time loads default settings now in that file.
                            }
                        }
                    }
                }
            }

            // Get name of current stylesheet
            GetCurrentStylesheet();
        }

        /// <summary>
        /// Returns a list of Stylesheets in the Stylesheet folder.
        /// </summary>
        /// <returns></returns>
        public string[] GetStyleList()
        {
            var list = Directory.GetFiles("themes");
            for (var i = 0; i < list.Length; i++)
            {
                var start = list[i].LastIndexOf("\\", StringComparison.Ordinal) + 1;
                var end = list[i].IndexOf(".yaml", StringComparison.OrdinalIgnoreCase);
                if (end == -1) end = 0;
                list[i] = list[i][start..end].Replace('_', ' ');
            }
            return list;
        }

        /// <summary>
        /// Gets the active stylesheet name
        /// </summary>
        public void GetCurrentStylesheet() => _instance._selectedStylesheet = _instance._stylesheet["name"];
        #endregion

        public JObject GetJObject() => JObject.FromObject(this);

        [JSInvokable]
        public void SaveSettings(bool mergeNewIntoOld = false) => GeneralFuncs.SaveSettings(SettingsFile, GetJObject(), mergeNewIntoOld);

        public JObject GetStylesJObject() => JObject.FromObject(_instance._stylesheet);

        #region SHORTCUTS
        public void CheckShortcuts()
        {
            Globals.DebugWriteLine(@"[Func:Data\AppSettings.CheckShortcuts]");
            _instance._desktopShortcut = File.Exists(Path.Join(Shortcut.Desktop, "TcNo Account Switcher.lnk"));
            _instance._startMenu = File.Exists(Path.Join(Shortcut.StartMenu, "TcNo Account Switcher.lnk"));
            _instance._startMenuPlatforms = Directory.Exists(Path.Join(Shortcut.StartMenu, "Platforms"));
            _instance._trayStartup = Task.StartWithWindows_Enabled();
        }

        public void DesktopShortcut_Toggle()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.DesktopShortcut_Toggle]");
            var s = new Shortcut();
            s.Shortcut_Switcher(Shortcut.Desktop);
            s.ToggleShortcut(!DesktopShortcut);
        }
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
                    Directory.CreateDirectory(platformsFolder);
                    foreach (var platform in PlatformList)
                    {
                        CreatePlatformShortcut(platformsFolder, platform, platform.ToLowerInvariant());
                    }
                }

                return;
            }
            // Only create these shortcuts of requested, by setting platforms to false.
            var s = new Shortcut();
            s.Shortcut_Switcher(Shortcut.StartMenu);
            s.ToggleShortcut(!StartMenu, false);

            s.Shortcut_Tray(Shortcut.StartMenu);
            s.ToggleShortcut(!StartMenu, false);
        }
        public void Task_Toggle()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.Task_Toggle]");
            Task.StartWithWindows_Toggle(!TrayStartup);
        }

        public void StartNow()
        {
            _ = Globals.StartTrayIfNotRunning() switch
            {
                "Started Tray" => GeneralInvocableFuncs.ShowToast("success", "Tray started!", renderTo: "toastarea"),
                "Already running" => GeneralInvocableFuncs.ShowToast("info", "Tray already open", renderTo: "toastarea"),
                "Tray users not found" => GeneralInvocableFuncs.ShowToast("error", "No tray users saved", renderTo: "toastarea"),
                _ => GeneralInvocableFuncs.ShowToast("error", "Could not start tray application!", renderTo: "toastarea")
            };
        }

        private void CreatePlatformShortcut(string folder, string platformName, string args)
        {
            var s = new Shortcut();
            s.Shortcut_Platform(folder, platformName, args);
            s.ToggleShortcut(!StartMenuPlatforms, false);
        }
        #endregion
    }
}
