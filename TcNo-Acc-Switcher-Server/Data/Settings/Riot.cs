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

using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;

namespace TcNo_Acc_Switcher_Server.Data.Settings
{
    public class Riot
    {
        private static Riot _instance = new();

        private static readonly object LockObj = new();
        public static Riot Instance
        {
            get
            {
                lock (LockObj)
                {
                    return _instance ??= new Riot();
                }
            }
            set => _instance = value;
        }

        // Variables
        private bool _admin;
        [JsonProperty("Riot_Admin", Order = 2)] public bool Admin { get => _instance._admin; set => _instance._admin = value; }
        private int _trayAccNumber = 3;
        [JsonProperty("Riot_TrayAccNumber", Order = 3)] public int TrayAccNumber { get => _instance._trayAccNumber; set => _instance._trayAccNumber = value; }
        private bool _forgetAccountEnabled;
        [JsonProperty("ForgetAccountEnabled", Order = 4)] public bool ForgetAccountEnabled { get => _instance._forgetAccountEnabled; set => _instance._forgetAccountEnabled = value; }

        // Game directory Variables
        private string _leagueDir;
        [JsonProperty("LeagueDir", Order = 5)] public string LeagueDir { get => _instance._leagueDir; set => _instance._leagueDir = value; }
        private string _leagueRiotDir;
        [JsonProperty("LeagueRiotDir", Order = 6)] public string LeagueRiotDir { get => _instance._leagueRiotDir; set => _instance._leagueRiotDir = value; }

        private string _runeterraDir;
        [JsonProperty("RuneterraDir", Order = 7)] public string RuneterraDir { get => _instance._runeterraDir; set => _instance._runeterraDir = value; }
        private string _runeterraRiotDir;
        [JsonProperty("RuneterraRiotDir", Order = 8)] public string RuneterraRiotDir { get => _instance._runeterraRiotDir; set => _instance._runeterraRiotDir = value; }

        private string _valorantDir;
        [JsonProperty("ValorantDir", Order = 9)] public string ValorantDir { get => _instance._valorantDir; set => _instance._valorantDir = value; }
        private string _valorantRiotDir;
        [JsonProperty("ValorantRiotDir", Order = 10)] public string ValorantRiotDir { get => _instance._valorantRiotDir; set => _instance._valorantRiotDir = value; }


        private bool _desktopShortcut;
        [JsonIgnore] public bool DesktopShortcut { get => _instance._desktopShortcut; set => _instance._desktopShortcut = value; }
        private bool _initialised;
        [JsonIgnore] public bool Initialised { get => _instance._initialised; set => _instance._initialised = value; }

        // Constants
        [JsonIgnore] public static readonly string SettingsFile = "RiotSettings.json";
        /*
            [JsonIgnore] public string RiotImagePath = "wwwroot/img/profiles/riot/";
            [JsonIgnore] public string RiotImagePathHtml = "img/profiles/riot/";
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
            Globals.DebugWriteLine(@"[Func:Data\Settings\Riot.SetForgetAcc]");
            if (ForgetAccountEnabled == enabled) return; // Ignore if already set
            ForgetAccountEnabled = enabled;
            SaveSettings();
        }

        #region SETTINGS
        /// <summary>
        /// Default settings for RiotSettings.json
        /// </summary>
        public void ResetSettings()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Riot.ResetSettings]");
            _instance.Admin = false;
            _instance.TrayAccNumber = 3;

            // Notice install directories are intentionally left out here.
            // They are just saved for user ease-of-access.

            _instance._desktopShortcut = Shortcut.CheckShortcuts("Riot");

            SaveSettings();
        }
        public void SetFromJObject(JObject j)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Riot.SetFromJObject]");
            var curSettings = j.ToObject<Riot>();
            if (curSettings == null) return;
            _instance.Admin = curSettings.Admin;
            _instance.TrayAccNumber = curSettings.TrayAccNumber;

            // Notice install directories are intentionally left out here.
            // They are just saved for user ease-of-access.

            _instance._desktopShortcut = Shortcut.CheckShortcuts("Riot");
        }
        public void LoadFromFile() => SetFromJObject(GeneralFuncs.LoadSettings(SettingsFile, GetJObject()));
        public JObject GetJObject() => JObject.FromObject(this);
        [JSInvokable]
        public void SaveSettings(bool mergeNewIntoOld = false) => GeneralFuncs.SaveSettings(SettingsFile, GetJObject(), mergeNewIntoOld);
        #endregion
    }
}
