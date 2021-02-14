using System;
using System.Collections.Generic;
using System.Linq;
using System.IO;
using System.Net;
using System.Text;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Data;
using System.Windows.Documents;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using TcNo_Acc_Switcher_Globals;

//using System.Windows.Shapes; -- Commented because of clash with System.IO.Path. If causes issues, uncomment.

namespace TcNo_Acc_Switcher_Client
{
    /// <summary>
    /// Interaction logic for More.xaml
    /// </summary>
    public partial class UserDataWindow : Window
    {
        MainWindow _mw;
        public UserDataWindow()
        {
            InitializeComponent();
        }
        public void ShareMainWindow(MainWindow imw) =>
            _mw = imw;
        private void BtnExit(object sender, RoutedEventArgs e) => Globals.WindowHandling.BtnExit(sender, e, this);
        private void BtnMinimize(object sender, RoutedEventArgs e) => Globals.WindowHandling.BtnMinimize(sender, e, this);
        private void DragWindow(object sender, MouseButtonEventArgs e) => Globals.WindowHandling.DragWindow(sender, e, this);
        private void Window_PreviewKeyDown(object sender, KeyEventArgs e)
        {
            if (e.Key == Key.Escape)
                Close();
        }


        public void InitUserDataWindow(string steamId)
        {
            var steamId32 = new Converters.SteamIdConvert(steamId).Id32;
            var userDataFolder = Path.Combine(Settings.GetSteamDirectory(), "userdata", steamId32);  // Contains list of Steam32 IDs
            var screenshotsFolder = Path.Combine(userDataFolder, "760\\remote\\");              // Contains folders of appIDs. */screenshots/ subfolder contains images and a */thumbnails/ folder.

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
                using (var client = new WebClient())
                {
                    client.DownloadFile("https://api.steampowered.com/ISteamApps/GetAppList/v2/", "cache\\SteamMasterAppList.json");
                    // Contains "applist":{ "apps":[ {"appid":<appid>,"name":<name>}, {}... ] }
                }
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
