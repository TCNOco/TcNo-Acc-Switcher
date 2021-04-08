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
using System.Linq;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
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

        // Constants
        [JsonIgnore] public string SettingsFile = "WindowSettings.json";
        [JsonIgnore] public bool StreamerModeTriggered = false;

        /// <summary>
        /// Check if any streaming software is running. Do let me know if you have a program name that you'd like to expand this list with!
        /// It's basically the program's .exe file, but without ".exe".
        /// </summary>
        /// <returns>True when streaming software is running</returns>
        public bool StreamerModeCheck()
        {
            if (!_streamerModeEnabled) return false; // Don't hide anything if disabled.
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

                switch (p.ProcessName)
                {
                    case "obs":
                    case "obs64":
                        _instance.StreamerModeTriggered = true;
                        return true;
                }
            }
            return false;
        }

        /// <summary>
        /// Returns a block of CSS text to be used on the page. Used to hide or show certain things in certain ways, in components that aren't being added through Blazor.
        /// </summary>
        public string GetCssBlock() => ".streamerCensor { display: " + (_instance.StreamerModeEnabled && _instance.StreamerModeTriggered ? "none" : "block") + "!important }";

        public void ResetSettings()
        {
            _instance.StreamerModeEnabled = true;

            SaveSettings();
        }

        public void SetFromJObject(JObject j)
        {
            var curSettings = j.ToObject<AppSettings>();
            if (curSettings == null) return;
            _instance.StreamerModeEnabled = curSettings.StreamerModeEnabled;
        }
        public void LoadFromFile() => SetFromJObject(GeneralFuncs.LoadSettings(SettingsFile));

        public JObject GetJObject() => JObject.FromObject(this);

        [JSInvokable]
        public void SaveSettings(bool reverse = false) => GeneralFuncs.SaveSettings(SettingsFile, GetJObject(), reverse);
    }
}
