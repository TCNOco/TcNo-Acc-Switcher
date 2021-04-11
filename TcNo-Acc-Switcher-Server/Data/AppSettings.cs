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
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Settings;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Data
{
    public class AppSettings
    {
        private static AppSettings _instance = new();
        public AppSettings() { }

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
        private bool _streamerModeEnabled = true;
        [JsonProperty("StreamerModeEnabled", Order = 0)] public bool StreamerModeEnabled { get => _instance._streamerModeEnabled; set => _instance._streamerModeEnabled = value; }

        private int _serverPort = 5000 ;
        [JsonProperty("ServerPort", Order = 1)] public int ServerPort { get => _instance._serverPort; set => _instance._serverPort = value; }

        private Point _windowSize = new() { X = 800, Y = 450 };
        [JsonProperty("WindowSize", Order = 2)] public Point WindowSize { get => _instance._windowSize; set => _instance._windowSize = value; }

        // Variables loaded from other files:
        private Dictionary<string, string> _stylesheet = new()
        {
            { "selectionColor", "#402B00" },
            { "selectionBackground", "#FFAA00" },
            { "contextMenuBackground", "#14151E" },
            { "contextMenuBackground-hover", "#1B2737" },
            { "contextMenuLeftBorder-hover", "#364E6E" },
            { "contextMenuTextColor", "#B0BEC5" },
            { "contextMenuTextColor-hover", "#FFFFFF" },
            { "contextMenuBoxShadow", "none" },
            { "headerbarBackground", "#14151E" },
            { "windowControlsBackground-hover", "rgba(255,255,255,0.1)" },
            { "windowControlsBackground-active", "rgba(255,255,255,0.2)" },
            { "windowControlsCloseBackground", "#E81123" },
            { "windowControlsCloseBackground-active", "#F1707A" },
            { "windowTitleColor", "white"},
            { "footerBackground", "#222"},
            { "footerColor", "#DDD"},
            { "scrollbarTrackBackground", "#1F202D" },
            { "scrollbarThumbBackground", "#515164" },
            { "scrollbarThumbBackground-hover", "#555" },
            { "accountPColor", "#DDD" },
            { "accountColor", "white" },
            { "accountBackground-hover", "#28374E" },
            { "accountBorder-hover", "#2777A4" },
            { "accountBackground-checked", "#274560" },
            { "accountBorder-checked", "#26A0DA" },
            { "accountListBackground", "url(../img/noise.png), linear-gradient(90deg, #2e2f42, #28293A 100%)" },
            { "mainBackground", "#28293A" },
            { "borderedItemBorderColor", "#888" },
            { "borderedItemBorderColor-focus", "#888" },
            { "borderedItemBorderColorBottom-focus", "#FFAA00" },
            { "defaultTextColor", "white" },
            { "linkColor", "#FFAA00" },
            { "linkColor-hover", "#FFDD00" },
            { "linkColor-active", "#CC7700" },
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
            { "listBackground", "#222" },
            { "listColor", "white" },
            { "listColor-checked", "white" },
            { "listBackgroundColor-checked", "FFAA00" },
            { "listTextColor-before", "#FFAA00" },
            { "listTextColor-before-checked", "#945300" },
            { "listTextColor-after", "#3DFF89" },
            { "listTextColor-after-checked", "#945300" },
            { "settingsHeaderColor", "white" },
            { "settingsHeaderHrBorder", "#BBB" },
            { "modalBackground", "#00000055" },
            { "modalFgBackground", "#28293A" },
            { "modalInputBackground", "#212529" },
            { "foundColor", "lime" },
            { "foundBackground", "green" },
            { "notFoundColor", "red" },
            { "notFoundBackground", "darkred" },
            { "limited", "yellow" },
            { "vac", "red" },
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
            { "notification-color-error-lighter", "#17132C" }
        };
        [JsonIgnore] public Dictionary<string, string> Stylesheet { get => _instance._stylesheet; set => _instance._stylesheet = value; }

        // Constants
        [JsonIgnore] public string SettingsFile = "WindowSettings.json";
        [JsonIgnore] public string StylesheetFile = "StyleSettings.json";
        [JsonIgnore] public bool StreamerModeTriggered = false;

        /// <summary>
        /// Check if any streaming software is running. Do let me know if you have a program name that you'd like to expand this list with!
        /// It's basically the program's .exe file, but without ".exe".
        /// </summary>
        /// <returns>True when streaming software is running</returns>
        public bool StreamerModeCheck()
        {
            Globals.DebugWriteLine($@"[Func:Data\AppSettings.StreamerModeCheck]");
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

                switch (p.ProcessName.ToLower())
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
                        Console.WriteLine(p.ProcessName);
                        return true;
                }
            }
            return false;
        }

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
            Globals.DebugWriteLine($@"[Func:Data\AppSettings.SetFromJObject]");
            var curSettings = j.ToObject<AppSettings>();
            if (curSettings == null) return;
            _instance.StreamerModeEnabled = curSettings.StreamerModeEnabled;
        }

        public void LoadFromFile()
        {
            Globals.DebugWriteLine($@"[Func:Data\AppSettings.LoadFromFile]");
            // Main settings
            SetFromJObject(GeneralFuncs.LoadSettings(SettingsFile, GetJObject()));
            // Stylesheet
            if (!File.Exists(StylesheetFile)) SaveStyles();
            //var s = GeneralFuncs.LoadSettings(StylesheetFile, GetStylesJObject()).ToObject<Dictionary<string, string>>();
            var s = GeneralFuncs.LoadSettings(StylesheetFile, GetStylesJObject()).ToObject<Dictionary<string, string>>();
            _instance._stylesheet = s.Count != 0 ? s : _instance._stylesheet;
        }

        public JObject GetJObject() => JObject.FromObject(this);

        [JSInvokable]
        public void SaveSettings(bool reverse = false) => GeneralFuncs.SaveSettings(SettingsFile, GetJObject(), reverse);

        public JObject GetStylesJObject() => JObject.FromObject(_instance._stylesheet);

        [JSInvokable]
        public void SaveStyles(bool reverse = false) => GeneralFuncs.SaveSettings(StylesheetFile, GetStylesJObject(), reverse);
    }
}
