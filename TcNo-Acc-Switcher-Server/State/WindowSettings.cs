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
using System.Globalization;
using System.IO;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.State
{
    public class WindowSettings : IWindowSettings
    {
        public string Language { get; set; } = "";
        public bool Rtl { get; set; } = CultureInfo.CurrentCulture.TextInfo.IsRightToLeft;
        public bool StreamerModeEnabled { get; set; }
        public int ServerPort { get; set; } = 0;
        public string WindowSize { get; set; } = "";
        public string Version { get; set; } = "";
        public List<object> DisabledPlatforms { get; } = new();
        public bool TrayMinimizeNotExit { get; set; }
        public bool ShownMinimizedNotification { get; set; }
        public bool StartCentered { get; set; }
        public string ActiveTheme { get; set; } = "";
        public string ActiveBrowser { get; set; } = "";
        public string Background { get; set; } = "";
        public List<string> EnabledBasicPlatforms { get; } = new();
        public bool CollectStats { get; set; }
        public bool ShareAnonymousStats { get; set; }
        public bool MinimizeOnSwitch { get; set; }
        public bool DiscordRpcEnabled { get; set; }
        public bool DiscordRpcShareTotalSwitches { get; set; }

        private static string Filename = "WindowSettings.json";
        /// <summary>
        /// Loads WindowSettings from file, if exists. Otherwise default.
        /// </summary>
        public WindowSettings()
        {
            // Load from file, if it exists.
            if (File.Exists(Filename))
                JsonConvert.PopulateObject(File.ReadAllText(Filename), this);
        }

        public void Save()
        {
            try
            {
                // Create folder if it doesn't exist:
                var folder = Path.GetDirectoryName(Filename);
                if (folder != "") _ = Directory.CreateDirectory(folder ?? string.Empty);

                File.WriteAllText(Filename, JsonConvert.SerializeObject(this, Formatting.Indented));
            }
            catch (Exception ex)
            {
                Globals.WriteToLog(ex.ToString());
            }
        }
    }
}
