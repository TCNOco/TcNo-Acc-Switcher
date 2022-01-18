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

using System.Collections.Generic;
using System.IO;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;

namespace TcNo_Acc_Switcher_Server.Data.Settings
{
    public class Steam
    {
        private static readonly Lang Lang = Lang.Instance;
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
        private string _folderPath = "C:\\Program Files (x86)\\Steam\\";
        private bool _admin;
        private bool _showSteamId;
        private bool _showVac = true;
        private bool _showLimited = true;
        private bool _showAccUsername = true;
        private bool _trayAccName;
        private int _imageExpiryTime = 7;
        private int _trayAccNumber = 3;
        private int _overrideState = -1;
        private bool _altClose;
        private bool _desktopShortcut;

        [JsonProperty("ForgetAccountEnabled", Order = 0)] public static bool ForgetAccountEnabled { get => Instance._forgetAccountEnabled; set => Instance._forgetAccountEnabled = value; }

        [JsonProperty("FolderPath", Order = 1)] public static string FolderPath { get => Instance._folderPath; set => Instance._folderPath = value; }

        [JsonProperty("Steam_Admin", Order = 3)] public static bool Admin { get => Instance._admin; set => Instance._admin = value; }

        [JsonProperty("Steam_ShowSteamID", Order = 4)] public static bool ShowSteamId { get => Instance._showSteamId; set => Instance._showSteamId = value; }

        [JsonProperty("Steam_ShowVAC", Order = 5)] public static bool ShowVac { get => Instance._showVac; set => Instance._showVac = value; }

        [JsonProperty("Steam_ShowLimited", Order = 6)] public static bool ShowLimited { get => Instance._showLimited; set => Instance._showLimited = value; }

        [JsonProperty("Steam_ShowAccUsername", Order = 7)] public static bool ShowAccUsername { get => Instance._showAccUsername; set => Instance._showAccUsername = value; }

        [JsonProperty("Steam_TrayAccountName", Order = 8)] public static bool TrayAccName { get => Instance._trayAccName; set => Instance._trayAccName = value; }

        [JsonProperty("Steam_ImageExpiryTime", Order = 9)] public static int ImageExpiryTime { get => Instance._imageExpiryTime; set => Instance._imageExpiryTime = value; }

        [JsonProperty("Steam_TrayAccNumber", Order = 10)] public static int TrayAccNumber { get => Instance._trayAccNumber; set => Instance._trayAccNumber = value; }

        [JsonProperty("Steam_OverrideState", Order = 11)] public static int OverrideState { get => Instance._overrideState; set => Instance._overrideState = value; }

        [JsonProperty("AltClose", Order = 13)] public static bool AltClose { get => Instance._altClose; set => Instance._altClose = value; }

        [JsonIgnore] public static bool DesktopShortcut { get => Instance._desktopShortcut; set => Instance._desktopShortcut = value; }

        // Constants
        [JsonIgnore] public static readonly List<string> Processes = new() { "steam.exe", "steamservice.exe", "steamwebhelper.exe", "GameOverlayUI.exe" };
        [JsonIgnore] public static readonly string VacCacheFile = Path.Join(Globals.UserDataFolder, "LoginCache\\Steam\\VACCache\\SteamVACCache.json");
        [JsonIgnore] public static readonly string SettingsFile = "SteamSettings.json";
        [JsonIgnore] public static readonly string ForgottenFile = "SteamForgotten.json";
        [JsonIgnore] public static readonly string SteamImagePath = "wwwroot/img/profiles/steam/";
        [JsonIgnore] public static readonly string SteamImagePathHtml = "img/profiles/steam/";
        [JsonIgnore] public static readonly string ContextMenuJson = $@"[
				{{""{Lang["Context_SwapTo"]}"": ""swapTo(-1, event)""}},
				{{""{Lang["Context_LoginAsSubmenu"]}"": [
					{{""{Lang["Invisible"]}"": ""swapTo(7, event)""}},
					{{""{Lang["Offline"]}"": ""swapTo(0, event)""}},
					{{""{Lang["Online"]}"": ""swapTo(1, event)""}},
					{{""{Lang["Busy"]}"": ""swapTo(2, event)""}},
					{{""{Lang["Away"]}"": ""swapTo(3, event)""}},
					{{""{Lang["Snooze"]}"": ""swapTo(4, event)""}},
					{{""{Lang["LookingToTrade"]}"": ""swapTo(5, event)""}},
					{{""{Lang["LookingToPlay"]}"": ""swapTo(6, event)""}}
				]}},
				{{""{Lang["Context_CopySubmenu"]}"": [
				  {{""{Lang["Context_CopyProfileSubmenu"]}"": [
				    {{""{Lang["Context_CommunityUrl"]}"": ""copy('URL', event)""}},
				    {{""{Lang["Context_CommunityUsername"]}"": ""copy('Line2', event)""}},
				    {{""{Lang["Context_LoginUsername"]}"": ""copy('Username', event)""}}
				  ]}},
				  {{""{Lang["Context_CopySteamIdSubmenu"]}"": [
				    {{""{Lang["Context_Steam_Id"]}"": ""copy('SteamId', event)""}},
				    {{""{Lang["Context_Steam_Id3"]}"": ""copy('SteamId3', event)""}},
				    {{""{Lang["Context_Steam_Id32"]}"": ""copy('SteamId32', event)""}},
				    {{""{Lang["Context_Steam_Id64"]}"": ""copy('id', event)""}}
				  ]}},
				  {{""{Lang["Context_CopyOtherSubmenu"]}"": [
					{{""SteamRep"": ""copy('SteamRep', event)""}},
					{{""SteamID.uk"": ""copy('SteamID.uk', event)""}},
					{{""SteamID.io"": ""copy('SteamID.io', event)""}},
					{{""SteamIDFinder.com"": ""copy('SteamIDFinder.com', event)""}}
				  ]}},
				]}},
				{{""{Lang["Context_CreateShortcutSubmenu"]}"": [
					{{"""": ""createShortcut()""}},
					{{""{Lang["OnlineDefault"]}"": ""createShortcut()""}},
					{{""{Lang["Invisible"]}"": ""createShortcut(':7')""}},
					{{""{Lang["Offline"]}"": ""createShortcut(':0')""}},
					{{""{Lang["Busy"]}"": ""createShortcut(':2')""}},
					{{""{Lang["Away"]}"": ""createShortcut(':3')""}},
					{{""{Lang["Snooze"]}"": ""createShortcut(':4')""}},
					{{""{Lang["LookingToTrade"]}"": ""createShortcut(':5')""}},
					{{""{Lang["LookingToPlay"]}"": ""createShortcut(':6')""}}
				]}},
				{{""{Lang["Context_ChangeImage"]}"": ""changeImage(event)""}},
				{{""{Lang["Context_Steam_OpenUserdata"]}"": ""openUserdata(event)""}},
				{{""{Lang["Forget"]}"": ""forget(event)""}}
            ]";

        public static string StateToString(int state)
        {
            return state switch
            {
                -1 => Lang["NoDefault"],
                0 => Lang["Offline"],
                1 => Lang["Online"],
                2 => Lang["Busy"],
                3 => Lang["Away"],
                4 => Lang["Snooze"],
                5 => Lang["LookingToTrade"],
                6 => Lang["LookingToPlay"],
                7 => Lang["Invisible"],
                _ => ""
            };
        }

        /// <summary>
        /// Default settings for SteamSettings.json
        /// </summary>
        public static void ResetSettings()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.ResetSettings]");
            ForgetAccountEnabled = false;
            FolderPath = "C:\\Program Files (x86)\\Steam\\";
            Admin = false;
            ShowSteamId = false;
            ShowVac = true;
            ShowLimited = true;
            TrayAccName = false;
            ImageExpiryTime = 7;
            TrayAccNumber = 3;
            DesktopShortcut = Shortcut.CheckShortcuts("Steam");
            _ = Shortcut.StartWithWindows_Enabled();
            AltClose = false;

            SaveSettings();
        }

        /// <summary>
        /// Get path of loginusers.vdf, resets & returns "RESET_PATH" if invalid.
        /// </summary>
        /// <returns>(Steam's path)\config\loginusers.vdf</returns>
        public static string LoginUsersVdf()
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
        public static string Exe() => Path.Join(FolderPath, "Steam.exe");

        /// <summary>
        /// Updates the ForgetAccountEnabled bool in Steam settings file
        /// </summary>
        /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
        public static void SetForgetAcc(bool enabled)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.SetForgetAcc]");
            if (ForgetAccountEnabled == enabled) return; // Ignore if already set
            ForgetAccountEnabled = enabled;
            SaveSettings();
        }

        /// <summary>
        /// Returns a block of CSS text to be used on the page. Used to hide or show certain things in certain ways, in components that aren't being added through Blazor.
        /// </summary>
        public static string GetSteamIdCssBlock() => ".steamId { display: " + (_instance._showSteamId ? "block" : "none") + " }";

        #region SETTINGS
        public static void SetFromJObject(JObject j)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.SetFromJObject]");
            var curSettings = j.ToObject<Steam>();
            if (curSettings == null) return;
            ForgetAccountEnabled = curSettings._forgetAccountEnabled;
            FolderPath = curSettings._folderPath;
            Admin = curSettings._admin;
            ShowSteamId = curSettings._showSteamId;
            ShowVac = curSettings._showVac;
            ShowLimited = curSettings._showLimited;
            TrayAccName = curSettings._trayAccName;
            ImageExpiryTime = curSettings._imageExpiryTime;
            TrayAccNumber = curSettings._trayAccNumber;
            DesktopShortcut = Shortcut.CheckShortcuts("Steam");
            AltClose = curSettings._altClose;
            _ = Shortcut.StartWithWindows_Enabled();
        }
        public static void LoadFromFile() => SetFromJObject(GeneralFuncs.LoadSettings(SettingsFile, JObject.FromObject(new Steam())));

        public static void SaveSettings(bool mergeNewIntoOld = false) => GeneralFuncs.SaveSettings(SettingsFile, JObject.FromObject(Instance), mergeNewIntoOld);
        #endregion
    }
}
