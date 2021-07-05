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
using System.IO;
using System.Linq;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;

namespace TcNo_Acc_Switcher_Server.Data.Settings
{
    public class Discord
    {
        private static Discord _instance = new();

        private static readonly object LockObj = new();
        public static Discord Instance
        {
            get
            {
                lock (LockObj)
                {
                    return _instance ??= new Discord();
                }
            }
            set => _instance = value;
        }

        // Variables
        private string _folderPath = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Discord");
        [JsonProperty("FolderPath", Order = 1)] public string FolderPath { get => _instance._folderPath; set => _instance._folderPath = value; }
        private bool _admin;
        [JsonProperty("Discord_Admin", Order = 2)] public bool Admin { get => _instance._admin; set => _instance._admin = value; }
        private int _trayAccNumber = 3;
        [JsonProperty("Discord_TrayAccNumber", Order = 3)] public int TrayAccNumber { get => _instance._trayAccNumber; set => _instance._trayAccNumber = value; }
        private bool _forgetAccountEnabled;
        [JsonProperty("ForgetAccountEnabled", Order = 4)] public bool ForgetAccountEnabled { get => _instance._forgetAccountEnabled; set => _instance._forgetAccountEnabled = value; }

        private bool _desktopShortcut;
        [JsonIgnore] public bool DesktopShortcut { get => _instance._desktopShortcut; set => _instance._desktopShortcut = value; }

        private string _password;
        [JsonIgnore] public string Password { get => _instance._password; set => _instance._password = value; }

        // Constants
        [JsonIgnore] public static readonly string SettingsFile = "DiscordSettings.json";
        /*
            [JsonIgnore] public string DiscordImagePath = "wwwroot/img/profiles/discord/";
            [JsonIgnore] public string DiscordImagePathHtml = "img/profiles/discord/";
        */
        [JsonIgnore] public readonly string ContextMenuJson = @"[
              {""Swap to account"": ""swapTo(-1, event)""},
              {""Change switcher name"": ""showModal('changeUsername')""},
              {""Create Desktop Shortcut..."": ""createShortcut()""},
              {""Change image"": ""changeImage(event)""},
              {""Forget"": ""forget(event)""}
            ]";


        /// <summary>
        /// Updates the ForgetAccountEnabled bool in settings file
        /// </summary>
        /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
        public void SetForgetAcc(bool enabled)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Discord.SetForgetAcc]");
            if (ForgetAccountEnabled == enabled) return; // Ignore if already set
            ForgetAccountEnabled = enabled;
            SaveSettings();
        }

        /// <summary>
        /// Get Discord.exe path from DiscordSettings.json 
        /// </summary>
        /// <returns>Discord.exe's path string</returns>
        public string Exe()
        {
            // This one's a little annoying, as it's stored inside a .sql file...
            // I was going to use SQLite... but it's probably quicker to iterate over the folders, and compare them.
            List<int> latest = new() { 0, 0, 0 };
            foreach (var dir in Directory.GetDirectories(Instance.FolderPath))
            {
	            var folder = Path.GetFileName(dir);
                if (folder.StartsWith("app-"))
	            {
		            var sVersionSplit = dir.Split('-')[1].Split('.'); // Removes "app-", then splits the remaining numbers into say: {1, 0, 8050}
		            var current = sVersionSplit.Select(int.Parse).ToList();
		            for (var i = 0; i < current.Count; i++)
		            {
			            if (current[i] > latest[i]) // Newer, so replace with highest new version number
			            {
				            latest = current;
				            break;
			            }

			            if (current[i] != latest[i]) break; // Lower, so break this loop and go to the next version.
		            }
	            }
            }


            var sLatestVersion = "app-" + string.Join('.', latest.Select(x => x.ToString()).ToList());
            return Path.Join(FolderPath, sLatestVersion, "Discord.exe");
        }

        #region SETTINGS
        /// <summary>
        /// Default settings for DiscordSettings.json
        /// </summary>
        public void ResetSettings()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Discord.ResetSettings]");
            _instance.FolderPath = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Discord");
            _instance.Admin = false;
            _instance.TrayAccNumber = 3;
            _instance._desktopShortcut = Shortcut.CheckShortcuts("Discord");

            SaveSettings();
        }
        public void SetFromJObject(JObject j)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Discord.SetFromJObject]");
            var curSettings = j.ToObject<Discord>();
            if (curSettings == null) return;
            _instance.FolderPath = curSettings.FolderPath;
            _instance.Admin = curSettings.Admin;
            _instance.TrayAccNumber = curSettings.TrayAccNumber;
            _instance._desktopShortcut = Shortcut.CheckShortcuts("Discord");
        }
        public void LoadFromFile() => SetFromJObject(GeneralFuncs.LoadSettings(SettingsFile, GetJObject()));
        public JObject GetJObject() => JObject.FromObject(this);
        [JSInvokable]
        public void SaveSettings(bool mergeNewIntoOld = false) => GeneralFuncs.SaveSettings(SettingsFile, GetJObject(), mergeNewIntoOld);
        #endregion
    }
}
