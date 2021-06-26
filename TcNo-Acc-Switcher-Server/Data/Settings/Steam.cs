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

using System.IO;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;

namespace TcNo_Acc_Switcher_Server.Data.Settings
{
    public interface ISteam
    {

    }

    public class Steam
    {
        private static Steam _instance = new();

        private static readonly object LockObj = new();
        public static Steam Instance
        {
            get
            {
                lock (LockObj)
                {
                    return _instance ??= new Steam();
                }
            }
            set => _instance = value;
        }

        // Variables
        private bool _forgetAccountEnabled;
        [JsonProperty("ForgetAccountEnabled", Order = 0)] public bool ForgetAccountEnabled { get => _instance._forgetAccountEnabled; set => _instance._forgetAccountEnabled = value; }
        private string _folderPath = "C:\\Program Files (x86)\\Steam\\";
        [JsonProperty("FolderPath", Order = 1)] public string FolderPath { get => _instance._folderPath; set => _instance._folderPath = value; }
        private bool _admin;
        [JsonProperty("Steam_Admin", Order = 3)] public bool Admin { get => _instance._admin; set => _instance._admin = value; }
        private bool _showSteamId;
        [JsonProperty("Steam_ShowSteamID", Order = 4)] public bool ShowSteamId { get => _instance._showSteamId; set => _instance._showSteamId = value; }
        private bool _showVac = true;
        [JsonProperty("Steam_ShowVAC", Order = 5)] public bool ShowVac { get => _instance._showVac; set => _instance._showVac = value; }
        private bool _showLimited = true;
        [JsonProperty("Steam_ShowLimited", Order = 6)] public bool ShowLimited { get => _instance._showLimited; set => _instance._showLimited = value; }
        private bool _showAccUsername = true;
        [JsonProperty("Steam_ShowAccUsername", Order = 7)] public bool ShowAccUsername { get => _instance._showAccUsername; set => _instance._showAccUsername = value; }
        private bool _trayAccName;
        [JsonProperty("Steam_TrayAccountName", Order = 8)] public bool TrayAccName { get => _instance._trayAccName; set => _instance._trayAccName = value; }
        private int _imageExpiryTime = 7;
        [JsonProperty("Steam_ImageExpiryTime", Order = 9)] public int ImageExpiryTime { get => _instance._imageExpiryTime; set => _instance._imageExpiryTime = value; }
        private int _trayAccNumber = 3;
        [JsonProperty("Steam_TrayAccNumber", Order = 10)] public int TrayAccNumber { get => _instance._trayAccNumber; set => _instance._trayAccNumber = value; }

        private bool _desktopShortcut;
        [JsonIgnore] public bool DesktopShortcut { get => _instance._desktopShortcut; set => _instance._desktopShortcut = value; }

        // Constants
        [JsonIgnore] public readonly string VacCacheFile = Path.Join(Globals.UserDataFolder, "LoginCache\\Steam\\VACCache\\SteamVACCache.json");
        [JsonIgnore] public static readonly string SettingsFile = "SteamSettings.json";
        [JsonIgnore] public readonly string ForgottenFile = "SteamForgotten.json";
        [JsonIgnore] public readonly string SteamImagePath = "wwwroot/img/profiles/steam/";
        [JsonIgnore] public readonly string SteamImagePathHtml = "img/profiles/steam/";
        [JsonIgnore] public readonly string ContextMenuJson = @"[
              {""Swap to account"": ""swapTo(-1, event)""},
              {""Login as..."": [
                {""Invisible"": ""swapTo(7, event)""},
                {""Offline"": ""swapTo(0, event)""},
                {""Online"": ""swapTo(1, event)""},
                {""Busy"": ""swapTo(2, event)""},
                {""Away"": ""swapTo(3, event)""},
                {""Snooze"": ""swapTo(4, event)""},
                {""Looking to Trade"": ""swapTo(5, event)""},
                {""Looking to Play"": ""swapTo(6, event)""}
              ]},
			  {""Copy..."": [
	              {""Copy Profile..."": [
	                {""Community URL"": ""copy('URL', event)""},
	                {""Community Username"": ""copy('Line2', event)""},
	                {""Login username"": ""copy('Username', event)""}
	              ]},
	              {""Copy SteamID..."": [
	                {""SteamID [STEAM0:~]"": ""copy('SteamId', event)""},
	                {""SteamID3 [U:1:~]"": ""copy('SteamId3', event)""},
	                {""SteamID32"": ""copy('SteamId32', event)""},
	                {""SteamID64 7656~"": ""copy('id', event)""}
	              ]},
	              {""Copy other..."": [
	                {""SteamRep"": ""copy('SteamRep', event)""},
	                {""SteamID.uk"": ""copy('SteamID.uk', event)""},
	                {""SteamID.io"": ""copy('SteamID.io', event)""},
	                {""SteamRep"": ""copy('SteamIDFinder.com', event)""}
	              ]},
			  ]},
              {""Create Desktop Shortcut..."": [
                {"""": ""createShortcut()""},
                {""Online (Default)"": ""createShortcut()""},
                {""Invisible"": ""createShortcut(':7')""},
                {""Offline"": ""createShortcut(':0')""},
                {""Busy"": ""createShortcut(':2')""},
                {""Away"": ""createShortcut(':3')""},
                {""Snooze"": ""createShortcut(':4')""},
                {""Looking to Trade"": ""createShortcut(':5')""},
                {""Looking to Play"": ""createShortcut(':6')""}
              ]},
              {""Change image"": ""changeImage(event)""},
              {""Open userdata"": ""openUserdata(event)""},
              {""Forget"": ""forget(event)""}
            ]";

        /// <summary>
        /// Default settings for SteamSettings.json
        /// </summary>
        public void ResetSettings()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.ResetSettings]");
            _instance.ForgetAccountEnabled = false;
            _instance.FolderPath = "C:\\Program Files (x86)\\Steam\\";
            _instance.Admin = false;
            _instance.ShowSteamId = false;
            _instance.ShowVac = true;
            _instance.ShowLimited = true;
            _instance.TrayAccName = false;
            _instance.ImageExpiryTime = 7;
            _instance.TrayAccNumber = 3;
            _instance._desktopShortcut = Shortcut.CheckShortcuts("Steam");
            Task.StartWithWindows_Enabled();

            SaveSettings();
        }

        /// <summary>
        /// Get path of loginusers.vdf, resets & returns "RESET_PATH" if invalid.
        /// </summary>
        /// <returns>(Steam's path)\config\loginusers.vdf</returns>
        public string LoginUsersVdf()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.LoginUsersVdf]");
            var path = Path.Join(FolderPath, "config\\loginusers.vdf");
            if (File.Exists(path)) return path;
            FolderPath = "";
            SaveSettings();
            return "RESET_PATH";
        }

        /// <summary>
        /// Get Steam.exe path from SteamSettings.json 
        /// </summary>
        /// <returns>Steam.exe's path string</returns>
        public string Exe() => Path.Join(FolderPath, "Steam.exe");

        /// <summary>
        /// Updates the ForgetAccountEnabled bool in Steam settings file
        /// </summary>
        /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
        public void SetForgetAcc(bool enabled)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.SetForgetAcc]");
            if (ForgetAccountEnabled == enabled) return; // Ignore if already set
            ForgetAccountEnabled = enabled;
            SaveSettings();
        }

        /// <summary>
        /// Returns a block of CSS text to be used on the page. Used to hide or show certain things in certain ways, in components that aren't being added through Blazor.
        /// </summary>
        public string GetSteamIdCssBlock() => ".steamId { display: " + (_instance._showSteamId ? "block" : "none") + " }";

        #region SETTINGS
        public void SetFromJObject(JObject j)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.SetFromJObject]");
            var curSettings = j.ToObject<Steam>();
            if (curSettings == null) return;
            _instance.ForgetAccountEnabled = curSettings.ForgetAccountEnabled;
            _instance.FolderPath = curSettings.FolderPath;
            _instance.Admin = curSettings.Admin;
            _instance.ShowSteamId = curSettings.ShowSteamId;
            _instance.ShowVac = curSettings.ShowVac;
            _instance.ShowLimited = curSettings.ShowLimited;
            _instance.TrayAccName = curSettings.TrayAccName;
            _instance.ImageExpiryTime = curSettings.ImageExpiryTime;
            _instance.TrayAccNumber = curSettings.TrayAccNumber;
            _instance._desktopShortcut = Shortcut.CheckShortcuts("Steam");
            Task.StartWithWindows_Enabled();
        }
        public void LoadFromFile() => SetFromJObject(GeneralFuncs.LoadSettings(SettingsFile, GetJObject()));

        public JObject GetJObject() => JObject.FromObject(this);

        [JSInvokable]
        public void SaveSettings(bool mergeNewIntoOld = false) => GeneralFuncs.SaveSettings(SettingsFile, GetJObject(), mergeNewIntoOld);
        #endregion
    }
}
