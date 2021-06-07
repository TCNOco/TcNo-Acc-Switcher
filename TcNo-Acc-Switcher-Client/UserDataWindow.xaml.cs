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
using System.Net;

//using System.Windows.Shapes; -- Commented because of clash with System.IO.Path. If causes issues, un-comment.

namespace TcNo_Acc_Switcher_Client
{
    /// <summary>
    /// Interaction logic for More.xaml
    /// </summary>
    public partial class UserDataWindow
    {
        public UserDataWindow()
        {
            InitializeComponent();
        }

        public void InitUserDataWindow(string steamId)
        {
            //var steamId32 = new TcNo_Acc_Switcher_Server.Converters.SteamIdConvert(steamId).Id32;
            //var userDataFolder = Path.Join(TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.SteamFolder(), "userdata", steamId32);  // Contains list of Steam32 IDs
            //var screenshotsFolder = Path.Join(userDataFolder, "760\\remote\\");              // Contains folders of appIDs. */screenshots/ sub-folder contains images and a */thumbnails/ folder.

            // GetAppNames() <== for screen shot folder
            // Add list of app names and numbers to screenshots list, for user:
            // <AppId> - <AppName>
            // => Button to open screen shot folder for that game

            // List of apps
            // GetAppNames() <== for userData Folder
            // Button to open Local or Remote folder
            // -- Usually local. Remote is cloud saved stuff (?)
            // -- Usually /local/cfg, or something of those sorts.
            // Button to copy local folder to another account for that app ==> Copy say CS:GO binds and settings to another account's CS:GO
        }

        // var MasterAppList variable -- So it's only loaded once.
        // var LocalAppList variable -- So bit AppList not always loaded.
        private void LoadLocalAppList()
        {
            Directory.CreateDirectory("cache");
            //if (!File.Exists("cache\\SteamLocalAppList.json"))
            // Save empty JSON list to file, just to create it.
            // else
            // Load LocalAppList into variable.
        }
        private void LoadAppList(bool overwrite = false)
        {
            Directory.CreateDirectory("cache");

            if (!File.Exists("cache\\SteamMasterAppList.json") || overwrite)
            {
                using var client = new WebClient();
                client.DownloadFile("https://api.steampowered.com/ISteamApps/GetAppList/v2/", "cache\\SteamMasterAppList.json");
                // Contains "applist":{ "apps":[ {"appid":<appid>,"name":<name>}, {}... ] }
            }
            else
            {

            }
        }

        private void GetAppNames(string[] appId)
        {
            // Check and get names from LocalAppList.

            // If not all app names found: 

            LoadAppList();
            // foreach appId (not cached) => Get app name from appId
            // Add to LocalAppList
            // Save LocalAppList

            // return LocalAppList
        }
        // Complete game ID list: https://api.steampowered.com/ISteamApps/GetAppList/v2/
    }
}
