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
using System.Drawing;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Globalization;
using System.IO;
using System.Linq;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.State.Classes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State
{
    public class WindowSettings : IWindowSettings
    {
        public string Language { get; set; } = "";
        public bool Rtl { get; set; } = CultureInfo.CurrentCulture.TextInfo.IsRightToLeft;
        public bool StreamerModeEnabled { get; set; } = true;
        public int ServerPort { get; set; } = 0;
        public Point WindowSize { get; set; } = new() { X = 800, Y = 450 };
        public bool AllowTransparency { get; set; } = true;
        public string Version { get; set; } = Globals.Version;
        public List<object> DisabledPlatforms { get; } = new();
        public bool TrayMinimizeNotExit { get; set; } = false;
        public bool ShownMinimizedNotification { get; set; } = false;
        public bool StartCentered { get; set; } = false;
        public string ActiveTheme { get; set; } = "Dracula_Cyan";
        public string ActiveBrowser { get; set; } = "WebView";
        public string Background { get; set; } = "";
        public List<string> EnabledBasicPlatforms { get; } = new();
        private bool _collectStats = true;

        public bool CollectStats
        {
            get => _collectStats;
            set
            {
                _collectStats = value;
                if (!value) ShareAnonymousStats = false;
            }
        }

        public bool ShareAnonymousStats { get; set; } = true;
        public bool MinimizeOnSwitch { get; set; } = false;
        private bool _discordRpcEnabled = true;

        public bool DiscordRpcEnabled
        {
            get => _discordRpcEnabled;
            set
            {
                _discordRpcEnabled = value;
                if (!value) DiscordRpcShareTotalSwitches = false;
            }
        }

        public bool DiscordRpcShareTotalSwitches { get; set; } = true;
        public string PasswordHash { get; set; } = "";
        /// <summary>
        /// For BasicStats // Game statistics collection and showing
        /// Keys for metrics on this list are not shown for any account.
        /// List of all games:[Settings:Hidden metric] metric keys.
        /// </summary>
        public Dictionary<string, Dictionary<string, bool>> GloballyHiddenMetrics { get; set; } = new();
        public bool AlwaysAdmin { get; set; } = false;


        public ObservableCollection<PlatformItem> Platforms { get; set; } = new()
        {
            new PlatformItem("Discord", true),
            new PlatformItem("Epic Games", true),
            new PlatformItem("Origin", true),
            new PlatformItem("Riot Games", true),
            new PlatformItem("Steam", true),
            new PlatformItem("Ubisoft", true),
        };

        private static string Filename = "WindowSettings.json";
        /// <summary>
        /// Loads WindowSettings from file, if exists. Otherwise default.
        /// A toast for errors can not be displayed here as this needs to be loaded before the language instance.
        /// </summary>
        public WindowSettings()
        {
            Globals.LoadSettings(Filename, this, false);

            Platforms.CollectionChanged += (_, _) => Platforms.Sort();
        }
        public void Save() => Globals.SaveJsonFile(Filename, this, false);
    }
}
