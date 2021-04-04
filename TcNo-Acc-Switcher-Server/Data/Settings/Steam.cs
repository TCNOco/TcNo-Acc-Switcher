using System;
using System.Collections.Generic;
using System.Drawing;
using System.IO;
using System.Windows;
using System.Linq;
using System.Threading.Tasks;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Server.Pages.General;

namespace TcNo_Acc_Switcher_Server.Data.Settings
{
    public interface ISteam
    {

    }

    public class Steam
    {
        private static Steam _instance = new Steam();
        public Steam() { }
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
        private Point _windowSize = new() { X = 800, Y = 450 };
        [JsonProperty("WindowSize", Order = 2)] public Point WindowSize { get => _instance._windowSize; set => _instance._windowSize = value; }
        private bool _admin;
        [JsonProperty("Steam_Admin", Order = 3)] public bool Admin { get => _instance._admin; set => _instance._admin = value; }
        private bool _showSteamId;
        [JsonProperty("Steam_ShowSteamID", Order = 4)] public bool ShowSteamId { get => _instance._showSteamId; set => _instance._showSteamId = value; }
        private bool _showVac = true;
        [JsonProperty("Steam_ShowVAC", Order = 5)] public bool ShowVac { get => _instance._showVac; set => _instance._showVac = value; }
        private bool _showLimited = true;
        [JsonProperty("Steam_ShowLimited", Order = 6)] public bool ShowLimited { get => _instance._showLimited; set => _instance._showLimited = value; }
        private bool _desktopShortcut;
        [JsonProperty("Steam_DesktopShortcut", Order = 7)] public bool DesktopShortcut { get => _instance._desktopShortcut; set => _instance._desktopShortcut = value; }
        private bool _startMenu;
        [JsonProperty("Steam_StartMenu", Order = 8)] public bool StartMenu { get => _instance._startMenu; set => _instance._startMenu = value; }
        private bool _trayStartup;
        [JsonProperty("Steam_TrayStartup", Order = 9)] public bool TrayStartup { get => _instance._trayStartup; set => _instance._trayStartup = value; }
        private bool _trayAccName;
        [JsonProperty("Steam_TrayAccountName", Order = 10)] public bool TrayAccName { get => _instance._trayAccName; set => _instance._trayAccName = value; }
        private int _imageExpiryTime = 7;
        [JsonProperty("Steam_ImageExpiryTime", Order = 11)] public int ImageExpiryTime { get => _instance._imageExpiryTime; set => _instance._imageExpiryTime = value; }
        private int _trayAccNumber = 3;
        [JsonProperty("Steam_TrayAccNumber", Order = 12)] public int TrayAccNumber { get => _instance._trayAccNumber; set => _instance._trayAccNumber = value; }
        
        // Constants
        public string VacCacheFile = "profilecache/SteamVACCache.json";
        public string SettingsFile = "SteamSettings.json";
        public string ForgottenFile = "SteamForgotten.json";
        public string SteamImagePath = "wwwroot/img/profiles/steam/";
        public string SteamImagePathHtml = "img/profiles/steam/";

        /// <summary>
        /// Default settings for SteamSettings.json
        /// </summary>
        public void ResetSettings()
        {
            _instance.ForgetAccountEnabled = false;
            _instance.FolderPath = "C:\\Program Files (x86)\\Steam\\";
            _instance.WindowSize = new Point() {X = 800, Y = 450};
            _instance.Admin = false;
            _instance.ShowSteamId = false;
            _instance.ShowVac = true;
            _instance.ShowLimited = true;
            _instance.DesktopShortcut = false; // Replace with check later
            _instance.StartMenu = false; // Replace with check later
            _instance.TrayStartup = false; // Replace with check later
            _instance.TrayAccName = false;
            _instance.ImageExpiryTime = 7;
            _instance.TrayAccNumber = 3;

            SaveSettings();
        }

        public void SetFromJObject(JObject j) {
            var curSettings = j.ToObject<Steam>();
            if (curSettings == null) return;
            _instance.ForgetAccountEnabled = curSettings.ForgetAccountEnabled;
            _instance.FolderPath = curSettings.FolderPath;
            _instance.WindowSize = curSettings.WindowSize;
            _instance.Admin = curSettings.Admin;
            _instance.ShowSteamId = curSettings.ShowSteamId;
            _instance.ShowVac = curSettings.ShowVac;
            _instance.ShowLimited = curSettings.ShowLimited;
            _instance.DesktopShortcut = curSettings.DesktopShortcut;
            _instance.StartMenu = curSettings.StartMenu;
            _instance.TrayStartup = curSettings.TrayStartup;
            _instance.TrayAccName = curSettings.TrayAccName;
            _instance.ImageExpiryTime = curSettings.ImageExpiryTime;
            _instance.TrayAccNumber = curSettings.TrayAccNumber;
        }
        public void LoadFromFile() => SetFromJObject(GeneralFuncs.LoadSettings(SettingsFile));

        public JObject GetJObject() => JObject.FromObject(this);

        public void SaveSettings() => GeneralFuncs.SaveSettings("SteamSettings", GetJObject());

        /// <summary>
        /// Get path of loginusers.vdf, resets & returns "RESET_PATH" if invalid.
        /// </summary>
        /// <returns>(Steam's path)\config\loginuisers.vdf</returns>
        public string LoginUsersVdf()
        {
            var path = Path.Combine(FolderPath, "config\\loginusers.vdf");
            if (File.Exists(path)) return path;
            FolderPath = "";
            SaveSettings();
            return "RESET_PATH";
        }

        /// <summary>
        /// Get Steam.exe path from SteamSettings.json 
        /// </summary>
        /// <returns>Steam.exe's path string</returns>
        public string SteamExe() => Path.Combine(FolderPath, "Steam.exe");

        /// <summary>
        /// Get Steam's config folder
        /// </summary>
        /// <returns>(Steam's Path)\config\</returns>
        public string SteamConfigFolder() => Path.Combine(FolderPath, "config\\");
        
        /// <summary>
        /// Updates the ForgetAccountEnabled bool in Steam settings file
        /// </summary>
        /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
        public void UpdateSteamForgetAcc(bool enabled)
        {
            if (ForgetAccountEnabled == enabled) return; // Ignore if already set
            ForgetAccountEnabled = enabled;
            SaveSettings();
        }

    }
}
