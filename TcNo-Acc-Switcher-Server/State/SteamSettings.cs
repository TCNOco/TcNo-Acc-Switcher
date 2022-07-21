using System;
using System.Collections.Generic;
using System.IO;
using System.Reflection;
using Microsoft.JSInterop;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;

namespace TcNo_Acc_Switcher_Server.State
{
    public class SteamSettings
    {
        public bool ForgetAccountEnabled { get; set; }
        public string FolderPath { get; set; } = "C:\\Program Files (x86)\\Steam\\";
        public bool Admin { get; set; }
        public bool ShowSteamId { get; set; }
        public bool ShowVac { get; set; } = true;
        public bool ShowLimited { get; set; } = true;
        public bool ShowLastLogin { get; set; } = true;
        public bool ShowAccUsername { get; set; } = true;
        public string TrayAccountName { get; set; }
        public int ImageExpiryTime { get; set; } = 7;
        public int TrayAccNumber { get; set; } = 3;
        public int OverrideState { get; set; } = -1;
        public string SteamWebApiKey { get; set; } = "";
        public Dictionary<int, string> Shortcuts { get; set; } = new();
        public string ClosingMethod { get; set; } = "TaskKill";
        public string StartingMethod { get; set; } = "Default";
        public bool AutoStart { get; set; } = true;
        public bool ShowShortNotes { get; set; } = true;
        public bool StartSilent { get; set; }
        public Dictionary<string, string> CustomAccountNames { get; set; } = new();

        [JsonIgnore] public string LoginUsersVdf;
        public const string SteamImagePath = "wwwroot/img/profiles/steam/";
        public const string SteamImagePathHtml = "img/profiles/steam/";
        public readonly string[] Processes = { "steam.exe", "SERVICE:steamservice.exe", "steamwebhelper.exe", "GameOverlayUI.exe" };
        public readonly string VacCacheFile = Path.Join(Globals.UserDataFolder, "LoginCache\\Steam\\VACCache\\SteamVACCache.json");


        public static readonly string Filename = "SteamSettings.json";
        public SteamSettings()
        {
            Globals.LoadSettings(Filename, this, false);
            LoginUsersVdf = Path.Join(FolderPath, "config\\loginusers.vdf");
        }
        public void Save() => Globals.SaveJsonFile(Filename, this, false);

        public void Reset()
        {
            var type = GetType();
            var properties = type.GetProperties();
            foreach (var t in properties)
                t.SetValue(this, null);
            Save();
        }


        /// <summary>
        /// Get Steam.exe path from SteamSettings.json
        /// </summary>
        /// <returns>Steam.exe's path string</returns>
        public string Exe => Path.Join(FolderPath, "Steam.exe");

        [JSInvokable]
        public void SaveShortcutOrderSteam(Dictionary<int, string> o)
        {
            Shortcuts = o;
            Save();
        }

        public void SetClosingMethod(string method)
        {
            ClosingMethod = method;
            Save();
        }
        public void SetStartingMethod(string method)
        {
            StartingMethod = method;
            Save();
        }

        /// <summary>
        /// Updates the ForgetAccountEnabled bool in Steam settings file
        /// </summary>
        /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
        public void SetForgetAcc(bool enabled)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.SetForgetAcc]");
            if (ForgetAccountEnabled == enabled) return; // Ignore if already set
            ForgetAccountEnabled = enabled;
            Save();
        }

        /// <summary>
        /// Returns a block of CSS text to be used on the page. Used to hide or show certain things in certain ways, in components that aren't being added through Blazor.
        /// </summary>
        public string GetSteamIdCssBlock() => ".steamId { display: " + (ShowSteamId ? "block" : "none") + " } .lastLogin { display: " + (ShowLastLogin ? "block" : "none") + " }";
    }
}
