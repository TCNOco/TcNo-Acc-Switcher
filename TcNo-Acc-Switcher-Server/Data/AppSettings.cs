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
	    private static readonly Lang Lang = Lang.Instance;

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

        private string _lang = "";
        [JsonProperty("Language", Order = 0)] public string Language { get => _instance._lang; set => _instance._lang = value; }

        private bool _streamerModeEnabled = true;
        [JsonProperty("StreamerModeEnabled", Order = 1)] public bool StreamerModeEnabled { get => _instance._streamerModeEnabled; set => _instance._streamerModeEnabled = value; }

        private int _serverPort = 5000 ;
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

		private string _selectedStylesheet;
        [JsonIgnore] public string SelectedStylesheet { get => _instance._selectedStylesheet; set => _instance._selectedStylesheet = value; }

        [JsonIgnore] public string PlatformContextMenu = $@"[
              {{""{Lang["Context_HidePlatform"]}"": ""hidePlatform()""}},
              {{""{Lang["Context_CreateShortcut"]}"": ""createPlatformShortcut()""}},
              {{""{Lang["Context_ExportAccList"]}"": ""exportAllAccounts()""}}
            ]";
        [JSInvokable]
        public static async System.Threading.Tasks.Task HidePlatform(string platform)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Data\AppSettings.HidePlatform]");
            _instance.DisabledPlatforms.Add(platform);
            _instance.SaveSettings();
            await AppData.ReloadPage();
        }

        public static async System.Threading.Tasks.Task ShowPlatform(string platform)
        {
            Globals.DebugWriteLine(@"[JSInvoke:Data\AppSettings.ShowPlatform]");
            _instance.DisabledPlatforms.Remove(platform);
            _instance.SaveSettings();
            await AppData.ReloadPage();
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
            { "name", "Default" },
            { "selectionColor", "#402B00" },
            { "selectionBackground", "#FFAA00" },
            { "contextMenuBackground", "#14151E" },
            { "contextMenuBackground-hover", "#1B2737" },
            { "contextMenuLeftBorder-hover", "#364E6E" },
            { "contextMenuTextColor", "#B0BEC5" },
            { "contextMenuTextColor-hover", "#FFFFFF" },
            { "contextMenuBoxShadow", "none" },
            { "headerbarBackground", "#14151E" },
            { "headerbarTCNOFill", "white" },
            { "windowControlsBackground", "#14151E" },
            { "windowControlsBackground-hover", "rgba(255,255,255,0.1)" },
            { "windowControlsBackground-active", "rgba(255,255,255,0.2)" },
            { "windowControlsCloseBackground", "#E81123" },
            { "windowControlsCloseBackground-active", "#F1707A" },
            { "windowTitleColor", "white" },
            { "footerBackground", "#222" },
            { "footerColor", "#DDD" },
            { "footerButtonIcon-Fill", "white" },
            { "footerIcoSettings-Fill", "white" },
            { "footerIcoQuestion-Fill", "white" },
            { "scrollbarTrackBackground", "#1F202D" },
            { "scrollbarThumbBackground", "#515164" },
            { "scrollbarThumbBackground-hover", "#555" },
            { "accountListItemWidth", "100px" },
            { "accountListItemHeight", "135px" },
            { "accountBackground-placeholder", "#28374E" },
            { "accountBorder-placeholder", "2px dashed #2777A4" },
            { "accountPColor", "#DDD" },
            { "accountColor", "white" },
            { "accountBackground-hover", "#28374E" },
            { "accountBackground-checked", "#274560" },
            { "accountBorder-hover", "#2777A4" },
            { "accountBorder-checked", "#26A0DA" },
            { "accountListBackground", "url(../img/noise.png), linear-gradient(90deg, #2e2f42, #28293A 100%)" },
            { "mainBackground", "#28293A" },
            { "defaultTextColor", "white" },
            { "linkColor", "#FFAA00" },
            { "linkColor-hover", "#FFDD00" },
            { "linkColor-active", "#CC7700" },
            { "borderedItemBorderColor", "#888" },
            { "borderedItemBorderColor-focus", "#888" },
            { "borderedItemBorderColorBottom-focus", "#FFAA00" },
            { "buttonBackground", "#333" },
            { "buttonBackground-active", "#222" },
            { "buttonBackground-hover", "#444" },
            { "buttonBorder", "#888" },
            { "buttonBorder-hover", "#888" },
            { "buttonBorder-active", "#FFAA00" },
            { "buttonColor", "white" },
            { "checkboxBorder", "white" },
            { "checkboxBorder-checked", "white" },
            { "checkboxBackground", "#28293A" },
            { "checkboxBackground-checked", "#FFAA00" },
            { "inputBackground", "#212529" },
            { "inputColor", "white" },
            { "dropdownBackground", "#333" },
            { "dropdownBorder", "#888" },
            { "dropdownColor", "white" },
            { "dropdownItemBackground-active", "#222" },
            { "dropdownItemBackground-hover", "#444" },
            { "listBackground", "#222" },
            { "listBackgroundColor-checked", "#FFAA00" },
            { "listColor", "white" },
            { "listColor-checked", "white" },
            { "listTextColor-before", "#FFAA00" },
            { "listTextColor-before-checked", "#945300" },
            { "listTextColor-after", "#3DFF89" },
            { "listTextColor-after-checked", "#945300" },
            { "settingsHeaderColor", "white" },
            { "settingsHeaderHrBorder", "#BBB" },
            { "modalBackground", "#00000055" },
            { "modalFgBackground", "#28293A" },
            { "modalInputBackground", "#212529" },
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
            { "notification-color-info", "rgb(3, 169, 244)" },
            { "notification-color-info-light", "rgba(3, 169, 244, .25)" },
            { "notification-color-info-lighter", "#17132C" },
            { "notification-color-success", "rgb(76, 175, 80)" },
            { "notification-color-success-light", "rgba(76, 175, 80, .25)" },
            { "notification-color-success-lighter", "#17132C" },
            { "notification-color-warning", "rgb(255, 152, 0)" },
            { "notification-color-warning-light", "rgba(255, 152, 0, .25)" },
            { "notification-color-warning-lighter", "#17132C" },
            { "notification-color-error", "rgb(244, 67, 54)" },
            { "notification-color-error-light", "rgba(244, 67, 54, .25)" },
            { "notification-color-error-lighter", "#17132C" },
            { "updateBarBackground", "#FFAA00" },
            { "updateBarColor", "black" },
            { "battlenetIcoOWTank-Fill", "white" },
            { "battlenetIcoOWDamage-Fill", "white" },
            { "battlenetIcoOWSupport-Fill", "white" },
            { "limited", "yellow" },
            { "vac", "red" },
            { "riotIcoLeague-Fill", "white" },
            { "riotIcoRuneterra-Fill", "white" },
            { "riotIcoValorant-Fill", "white" },
            { "platformScreenBackground", "var(--accountListBackground)" },
            { "platformBorderColor", "#FFAA00" },
            { "platformFilter-Hover", "brightness(97%) saturate(1.10) contrast(102%)" },
            { "platformFilter-Active", "brightness(95%) saturate(1.10)" },
            { "platformFilterIcon-Hover", "brightness(97%) saturate(1.05) contrast(102%)" },
            { "platformFilterIcon-Active", "brightness(95%) saturate(1.10)" },
            { "platformTransform-Hover", "scale(1.025)" },
            { "platformTransform-Active", "scale(0.98)" },
            { "platformTransform-HoverAnimation-boxShadow-0", "0 0 0 0 rgba(255, 170, 0, 0.7)" },
            { "platformTransform-HoverAnimation-boxShadow-70", "0 0 0 10px rgba(255, 170, 0, 0)" },
            { "platformTransform-HoverAnimation-boxShadow-100", "0 0 0 10px rgba(255, 170, 0, 0)" },
            { "platformTransform-HoverAnimation-transform-0", "scale(1)" },
            { "platformTransform-HoverAnimation-transform-70", "scale(1.02)" },
            { "platformTransform-HoverAnimation-transform-100", "scale(1)" },
            { "icoBattleNetBG", "linear-gradient(0deg, rgba(31,37,48,1) 0%, rgba(31,37,48,1) 0%, rgba(32,33,35,1) 100%, rgba(0,212,255,1) 202123%)" },
            { "icoBattleNetText-Fill", "none" },
            { "icoBattleNetText-Stroke", "#00a5f8" },
            { "icoBattleNetText-StrokeWidth", "8.33px" },
            { "icoBattleNetFG-Fill", "#00a5f8" },
            { "icoBattleNetFG-Stroke", "none" },
            { "icoBattleNetFG-StrokeWidth", "0" },
            { "icoBattleNetGlass-Fill", "rgba(255, 255, 255, 0.02)" },
            { "icoDiscordBG", "linear-gradient(0deg, rgba(31,37,48,1) 0%, rgba(31,37,48,1) 0%, rgba(32,33,35,1) 100%, rgba(0,212,255,1) 202123%)" },
            { "icoDiscordText-Fill", "none" },
            { "icoDiscordText-Stroke", "#00a5f8" },
            { "icoDiscordText-StrokeWidth", "8.33px" },
            { "icoDiscordFG-Fill", "#00a5f8" },
            { "icoDiscordFG-Stroke", "#000" },
            { "icoDiscordFG-StrokeWidth", "0" },
            { "icoDiscordGlass-Fill", "rgba(255, 255, 255, 0.02)" },
            { "icoEpicBG", "linear-gradient(0deg, rgba(31,37,48,1) 0%, rgba(31,37,48,1) 0%, rgba(32,33,35,1) 100%, rgba(0,212,255,1) 202123%)" },
            { "icoEpicText-Fill", "none" },
            { "icoEpicText-Stroke", "#00a5f8" },
            { "icoEpicText-StrokeWidth", "8.33px" },
            { "icoEpicFG-Fill", "#00a5f8" },
            { "icoEpicFG-Stroke", "#00a5f8" },
            { "icoEpicFG-StrokeWidth", "50px" },
            { "icoEpicGlass-Fill", "rgba(255, 255, 255, 0.02)" },
            { "icoOriginBG", "linear-gradient(0deg, rgba(31,37,48,1) 0%, rgba(31,37,48,1) 0%, rgba(32,33,35,1) 100%, rgba(0,212,255,1) 202123%)" },
            { "icoOriginText-Fill", "none" },
            { "icoOriginText-Stroke", "#00a5f8" },
            { "icoOriginText-StrokeWidth", "8.33px" },
            { "icoOriginFG-Fill", "none" },
            { "icoOriginFG-Stroke", "#00a5f8" },
            { "icoOriginFG-StrokeWidth", "12.51px" },
            { "icoOriginGlass-Fill", "rgba(255, 255, 255, 0.02)" },
            { "icoRiotBG", "linear-gradient(0deg, rgba(31,37,48,1) 0%, rgba(31,37,48,1) 0%, rgba(32,33,35,1) 100%, rgba(0,212,255,1) 202123%)" },
            { "icoRiotText-Fill", "none" },
            { "icoRiotText-Stroke", "#00a5f8" },
            { "icoRiotText-StrokeWidth", "8.33px" },
            { "icoRiotFG-Fill", "none" },
            { "icoRiotFG-Stroke", "#00a5f8" },
            { "icoRiotFG-StrokeWidth", "12.51px" },
            { "icoRiotGlass-Fill", "rgba(255, 255, 255, 0.02)" },
            { "icoSteamBG", "linear-gradient(0deg, rgba(31,37,48,1) 0%, rgba(31,37,48,1) 0%, rgba(32,33,35,1) 100%, rgba(0,212,255,1) 202123%)" },
            { "icoSteamText-Fill", "none" },
            { "icoSteamText-Stroke", "#00a5f8" },
            { "icoSteamText-StrokeWidth", "8.33px" },
            { "icoSteamFG-Fill", "none" },
            { "icoSteamFG-Stroke", "#00a5f8" },
            { "icoSteamFG-StrokeWidth", "12.51px" },
            { "icoSteamGlass-Fill", "rgba(255, 255, 255, 0.02)" },
            { "icoUbisoftBG", "linear-gradient(0deg, rgba(31,37,48,1) 0%, rgba(31,37,48,1) 0%, rgba(32,33,35,1) 100%, rgba(0,212,255,1) 202123%)" },
            { "icoUbisoftText-Fill", "none" },
            { "icoUbisoftText-Stroke", "#00a5f8" },
            { "icoUbisoftText-StrokeWidth", "8.33px" },
            { "icoUbisoftFG-Fill", "none" },
            { "icoUbisoftFG-Stroke", "#00a5f8" },
            { "icoUbisoftFG-StrokeWidth", "12.51px" },
            { "icoUbisoftGlass-Fill", "rgba(255, 255, 255, 0.02)" },
            { "ace_editorBackground", "#141414" },
            { "ace_editorColor", "#F8F8F8" },
            { "ace_editor_gutterBackground", "#232323" },
            { "ace_editor_gutterColor", "#E2E2E2" },
            { "ace_editor_printBackground", "#232323" },
            { "ace_editor_cursorColor", "#A7A7A7" },
            { "ace_editor_multiselectBoxShadow", "0 0 3px 0px #141414" },
            { "ace_editor_markerlayer_selectionBackground", "rgba(221, 240, 255, 0.20)" },
            { "ace_editor_markerlayer_selectionBorderRadius", "0px" },
            { "ace_editor_markerlayer_stepBackground", "rgb(102, 82, 0)" },
            { "ace_editor_markerlayer_bracketBorder", "1px solid rgba(255, 255, 255, 0.25)" },
            { "ace_editor_markerlayer_activeLineBackground", "rgba(255, 255, 255, 0.031)" },
            { "ace_editor_markerlayer_selectedWordBorder", "1px solid rgba(221, 240, 255, 0.20)" },
            { "ace_editor_markerlayer_selectedWordBorderRadius", "0px" },
            { "ace_editor_gutterActiveLineBackground", "rgba(255, 255, 255, 0.031)" },
            { "ace_editor_invisibleColor", "rgba(255, 255, 255, 0.25)" },
            { "ace_editor_keywordMetaColor", "#CDA869" },
            { "ace_editor_constantColor", "#CF6A4C" },
            { "ace_editor_constantLanguageColor", "#CF6A4C" },
            { "ace_editor_constantNumericColor", "#CF6A4C" },
            { "ace_editor_constantCharacterColor", "#CF6A4C" },
            { "ace_editor_constantCharacterEscapeColor", "#CF6A4C" },
            { "ace_editor_constantOtherColor", "#CF6A4C" },
            { "ace_editor_languageColor", "#CF6A4C" },
            { "ace_editor_numericColor", "#CF6A4C" },
            { "ace_editor_illegalColor", "#F8F8F8" },
            { "ace_editor_illegalBackground", "rgba(86, 45, 86, 0.75)" },
            { "ace_editor_deprecatedColor", "#D2A8A1" },
            { "ace_editor_deprecatedBackground", "none" },
            { "ace_editor_support", "#9B859D" },
            { "ace_editor_foldBackground", "#AC885B" },
            { "ace_editor_foldBorderColor", "#F8F8F8" },
            { "ace_editor_supportFunctionColor", "#DAD085" },
            { "ace_editor_supportClassColor", "#DAD085" },
            { "ace_editor_supportTypeColor", "#DAD085" },
            { "ace_editor_listColor", "#F9EE98" },
            { "ace_editor_functionTagColor", "#AC885B" },
            { "ace_editor_stringColor", "#8F9D6A" },
            { "ace_editor_regexpColor", "#E9C062" },
            { "ace_editor_commentColor", "#5F5A60" },
            { "ace_editor_variableColor", "#7587A6" },
            { "ace_editor_xmlpeColor", "#494949" },
            { "ace_editor_indentguideBackground", "url(data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAACCAYAAACZgbYnAAAAEklEQVQImWMQERFpYLC1tf0PAAgOAnPnhxyiAAAAAElFTkSuQmCC) right repeat-y" }
        };

        private Dictionary<string, string> _stylesheet;
        [JsonIgnore] public Dictionary<string, string> Stylesheet { get => _instance._stylesheet; set => _instance._stylesheet = value; }

        private bool _windowsAccent;
        public bool WindowsAccent { get => _instance._windowsAccent; set => _instance._windowsAccent = value; }

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
        /// Swaps in a requested stylesheet, and loads styles from file.
        /// </summary>
        /// <param name="swapTo">Stylesheet name (without .json) to copy and load</param>
        public async System.Threading.Tasks.Task SwapStylesheet(string swapTo)
        {
            File.Copy($"themes\\{swapTo.Replace(' ', '_')}.yaml", StylesheetFile, true);
            try
            {
	            if (LoadStylesheetFromFile()) await AppData.ReloadPage();
				else GeneralInvocableFuncs.ShowToast("error", Lang["Toast_LoadStylesheetFailed"],
		            "Stylesheet error", "toastarea");
			}
            catch (Exception)
			{
				GeneralInvocableFuncs.ShowToast("error", Lang["Toast_LoadStylesheetFailed"],
					"Stylesheet error", "toastarea");
			}
		}

        /// <summary>
        /// Load stylesheet settings from stylesheet file.
        /// </summary>
        public bool LoadStylesheetFromFile()
        {
            if (!File.Exists(StylesheetFile))
            {
                if (File.Exists("themes\\Default.yaml"))
	                File.Copy("themes\\Default.yaml", StylesheetFile);
                else if (File.Exists(Path.Join(Globals.AppDataFolder, "themes\\Default.yaml")))
	                File.Copy(Path.Join(Globals.AppDataFolder, "themes\\Default.yaml"), StylesheetFile);
                else
                {
	                throw new Exception(Lang["ThemesNotFound"]);
                }
            }

            try
            {
	            LoadStylesheet();
            }
            catch (YamlDotNet.Core.SyntaxErrorException e)
            {
                File.Copy(StylesheetFile, "StyleSettings_broken.yaml", true);
                if (File.Exists("StyleSettings_ErrorInfo.txt")) File.Delete("StyleSettings_ErrorInfo.txt");
                File.WriteAllText("StyleSettings_ErrorInfo.txt", e.ToString());

	            if (File.Exists("themes\\Default.yaml"))
		            File.Copy("themes\\Default.yaml", StylesheetFile, true);
	            else if (File.Exists(Path.Join(Globals.AppDataFolder, "themes\\Default.yaml")))
		            File.Copy(Path.Join(Globals.AppDataFolder, "themes\\Default.yaml"), StylesheetFile, true);
	            else
	            {
		            throw new Exception(Lang["ThemeSyntaxAnd"] + Environment.NewLine + Lang["ThemesNotFound"] + e);
	            }
	            return false;
            }

            // Get name of current stylesheet
            GetCurrentStylesheet();
	        if (WindowsAccent)
		        SetAccentColor();

	        return true;
        }

        private void LoadStylesheet()
        {
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

        #region SHORTCUTS
        public void CheckShortcuts()
        {
            Globals.DebugWriteLine(@"[Func:Data\AppSettings.CheckShortcuts]");
            _instance._desktopShortcut = File.Exists(Path.Join(Shortcut.Desktop, "TcNo Account Switcher.lnk"));
            _instance._startMenu = File.Exists(Path.Join(Shortcut.StartMenu, "TcNo Account Switcher.lnk"));
            _instance._startMenuPlatforms = Directory.Exists(Path.Join(Shortcut.StartMenu, "Platforms"));
            _instance._trayStartup = Task.StartWithWindows_Enabled();

            if (OperatingSystem.IsWindows())
				_instance._protocolEnabled = Protocol_IsEnabled();
        }

        public void DesktopShortcut_Toggle()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.DesktopShortcut_Toggle]");
            var s = new Shortcut();
            s.Shortcut_Switcher(Shortcut.Desktop);
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
			        GeneralInvocableFuncs.ShowToast("success", Lang["Toast_ProtocolEnabled"], Lang["Toast_ProtocolEnabledTitle"], "toastarea");
		        }
		        else
		        {
			        // Remove
                    Registry.ClassesRoot.DeleteSubKeyTree("tcno");
			        GeneralInvocableFuncs.ShowToast("success", Lang["Toast_ProtocolDisabled"], Lang["Toast_ProtocolDisabledTitle"], "toastarea");
                }
		        _instance._protocolEnabled = Protocol_IsEnabled();
            }
	        catch (UnauthorizedAccessException)
	        {
		        GeneralInvocableFuncs.ShowToast("error", Lang["Toast_RestartAsAdmin"], Lang["Failed"], "toastarea");
                GeneralInvocableFuncs.ShowModal("notice:RestartAsAdmin");
	        }
        }

        #region WindowsAccent
        [SupportedOSPlatform("windows")]
        public void WindowsAccent_Toggle()
        {
	        if (!WindowsAccent)
		        SetAccentColor(true);
	        else
		        GeneralInvocableFuncs.ShowToast("info", Lang["Toast_RestartAfterClose"], Lang["Toast_RestartAfterCloseTitle"], "toastarea");
        }

        [SupportedOSPlatform("windows")]
        private void SetAccentColor() => SetAccentColor(false);
        [SupportedOSPlatform("windows")]
        private static void SetAccentColor(bool userInvoked)
        {
	        var accent = GetAccentColorHexString();
	        _instance._stylesheet["selectionBackground"] = accent;
	        _instance._stylesheet["linkColor"] = accent;
	        _instance._stylesheet["linkColor-hover"] = accent; // TODO: Make this lighter somehow
            _instance._stylesheet["linkColor-active"] = accent; // TODO: Make this darker somehow
            _instance._stylesheet["borderedItemBorderColorBottom-focus"] = accent;
            _instance._stylesheet["buttonBorder-active"] = accent;
            _instance._stylesheet["checkboxBackground-checked"] = accent;
            _instance._stylesheet["listBackgroundColor-checked"] = accent;
	        _instance._stylesheet["listTextColor-before"] = accent;
	        _instance._stylesheet["updateBarBackground"] = accent;
	        _instance._stylesheet["platformBorderColor"] = accent;

	        var accentColorIntString = GetAccentColorIntString();
            _instance._stylesheet["platformTransform-HoverAnimation-boxShadow-0"] = $"0 0 0 0 rgba({accentColorIntString}, 0.7)";
	        _instance._stylesheet["platformTransform-HoverAnimation-boxShadow-70"] = $"0 0 0 10px rgba({accentColorIntString}, 0)";
            _instance._stylesheet["platformTransform-HoverAnimation-boxShadow-100"] = $"0 0 0 10px rgba({accentColorIntString}, 0)";

            if (userInvoked)
                _ = AppData.ReloadPage();
        }

        [SupportedOSPlatform("windows")]
        public static string GetAccentColorHexString()
        {
	        byte r, g, b;
	        (r, g, b) = GetAccentColor();
	        byte[] rgb = {r, g, b};
	        return '#' + BitConverter.ToString(rgb).Replace("-", string.Empty);
        }

        [SupportedOSPlatform("windows")]
        public static string GetAccentColorIntString()
        {
	        byte r, g, b;
	        (r, g, b) = GetAccentColor();
	        return Convert.ToInt32(r) + ", " + Convert.ToInt32(g) + ", " + Convert.ToInt32(b);
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
                    Directory.CreateDirectory(platformsFolder);
                    foreach (var platform in Globals.PlatformList)
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
                "Started Tray" => GeneralInvocableFuncs.ShowToast("success", Lang["Toast_TrayStarted"], renderTo: "toastarea"),
                "Already running" => GeneralInvocableFuncs.ShowToast("info", Lang["Toast_TrayRunning"], renderTo: "toastarea"),
                "Tray users not found" => GeneralInvocableFuncs.ShowToast("error", Lang["Toast_TrayUsersMissing"], renderTo: "toastarea"),
                _ => GeneralInvocableFuncs.ShowToast("error", Lang["Toast_TrayFail"], renderTo: "toastarea")
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
