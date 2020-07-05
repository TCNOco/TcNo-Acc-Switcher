using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.ComponentModel;
using System.Diagnostics;
using System.Drawing;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Net;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Data;
using System.Windows.Documents;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using System.Windows.Shapes;
using TcNo_Acc_Switcher_Globals;
using Color = System.Windows.Media.Color;
using Path = System.IO.Path;

namespace TcNo_Acc_Switcher
{
    /// <summary>
    /// Interaction logic for PlatformPicker.xaml
    /// </summary>
    public partial class MainWindow : Window
    {
        private Globals _globals = Globals.LoadExisting(Path.GetDirectoryName(System.Reflection.Assembly.GetEntryAssembly().Location));
        //readonly int steam_version = 3000;
        readonly int steam_version = 3001;
        readonly int steam_trayversion = 1000;
        public MainWindow()
        {
            InitializeComponent();

            //List<Platform> Platforms = new List<Platform>();
            //Platforms.Add(new Platform() { Icon = BitmapToImageSource(Properties.Resources.Origin), Name = "Steam" }); 

            //// DISABLE LISTBOX
            //foreach (Platform pl in Platforms)
            //{
            //    MainViewmodel.Platforms.Add(pl);
            //}
            //ListPlatforms.Items.Add(Platforms[0]);
        }

        public void Process_OptionalUpdate(bool async = false)
        {

            if (_globals.NeedsUpdateCheck_Launch()) Process_Update(async);
        }

        public void Process_Update(bool async = false)
        {
            Thread updateCheckThread = async ? new Thread(AsyncUpdateCheck) : new Thread(UpdateCheck);
            // Check if cleanup needed for update
            if (File.Exists("Update_Complete"))
            {
                File.Delete("Update_Complete");
                ResourceClean();
                if (File.Exists("Restart_SteamTray"))
                {
                    File.Delete("Restart_SteamTray");
                    Start_SteamTray();
                }
                //MessageBoxResult messageBoxResult = MessageBox.Show(new Window { Topmost = true }, Strings.GitHubWhatsNew, Strings.FinishedUpdating,
                //    MessageBoxButton.YesNo, MessageBoxImage.Information, MessageBoxResult.Yes,
                //    MessageBoxOptions.DefaultDesktopOnly);
                MessageBoxResult messageBoxResult = MessageBox.Show(Strings.GitHubWhatsNew, Strings.FinishedUpdating,
                    MessageBoxButton.YesNo, MessageBoxImage.Information, MessageBoxResult.Yes,
                    MessageBoxOptions.DefaultDesktopOnly);
                if (messageBoxResult == MessageBoxResult.Yes)
                {
                    Process.Start(new ProcessStartInfo("https://github.com/TcNobo/TcNo-Acc-Switcher/releases") { UseShellExecute = true });
                }
            }
            else
            {
                if (File.Exists("UpdateFound.txt"))
                    DownloadUpdateDialog();
                else
                    updateCheckThread.Start();
            }

            // Will exit after the update check thread is done
            if (updateCheckThread.IsAlive)
                updateCheckThread.Join();
        }
        private void Start_SteamTray()
        {
            try
            {
                string processName = "Steam\\TcNo Acc Switcher SteamTray.exe";
                if (File.Exists(processName))
                {
                    ProcessStartInfo startInfo = new ProcessStartInfo();
                    startInfo.FileName = System.IO.Path.GetFullPath(processName);
                    startInfo.CreateNoWindow = false;
                    startInfo.UseShellExecute = false;
                    Process.Start(startInfo);
                }
            }
            catch (Exception)
            {
                MessageBox.Show(Strings.ErrTrayProcessStart, Strings.ErrTrayProcessStartHead, MessageBoxButton.OK, MessageBoxImage.Error);
            }
        }

        /// <summary>
        /// Checks for an update, and downloads it.
        /// This downloads and replaces all files from .zip.
        /// Used for updating everything, and not individual files.
        /// </summary>
        void AsyncUpdateCheck()
        {
            UpdateCheck(true);
        }

        void UpdateCheck()
        {
            UpdateCheck(false);
        }
        public void UpdateCheck(bool async = false)
        {
            _globals.LastCheckedNow();
            Globals.Save(_globals);

            try
            {
                var wc = new System.Net.WebClient();
                var webVersion = int.Parse(wc.DownloadString("https://tcno.co/Projects/AccSwitcher/net_version.php").Substring(0, 4));

                // Currently only set to work with Steam, until another update.
                if (webVersion > steam_version)
                {
                    using (FileStream fs = File.Create("UpdateFound.txt"))
                    {
                        byte[] info = new UTF8Encoding(true).GetBytes(Strings.UpdateLastLaunch + DateTime.Now.ToString(CultureInfo.InvariantCulture));
                        fs.Write(info, 0, info.Length);
                    }
                    if (async) this.Dispatcher.Invoke(DownloadUpdateDialog);
                    else DownloadUpdateDialog();
                }
                else
                {
                    if (File.Exists("UpdateFound.txt"))
                        File.Delete("UpdateFound.txt");
                }
            }
            catch (WebException ex)
            {
                MessageBox.Show(Strings.ErrUpdateCheckFail + ex, Strings.ErrUpdateCheckFailHead, MessageBoxButton.OK, MessageBoxImage.Error);
            }

        }
        /// <summary>
        /// Presents option for user to download an update.
        /// </summary>
        public void DownloadUpdateDialog()
        {
            MessageBoxResult messageBoxResult = System.Windows.MessageBox.Show(Strings.UpdateNow, Strings.UpdateFound, System.Windows.MessageBoxButton.YesNo);
            if (messageBoxResult == MessageBoxResult.Yes)
            {
                if (File.Exists("UpdateFound.txt"))
                    File.Delete("UpdateFound.txt");

                // Run updater -- This is the Steam version, so the version update check will be here
                string processName = "TcNo Acc Switcher Updater.exe";
                ProcessStartInfo startInfo = new ProcessStartInfo();
                startInfo.FileName = processName;
                startInfo.CreateNoWindow = false;
                startInfo.UseShellExecute = true;
                Process.Start(startInfo);
                Environment.Exit(1);
            }
        }
        /// <summary>
        /// Cleans up directory after an update
        /// </summary>
        private void ResourceClean()
        {
            string[] delFileNames = new string[] { "7za-license.txt", "7za.exe", "x64.zip", "x32.zip", "upd.7z", "UpdateInformation.txt" };

            foreach (string f in delFileNames)
            {
                if (File.Exists(f))
                    File.Delete(f);
            }
        }

        //public class Platform
        //{
        //    public string Name { get; set; }
        //    public BitmapImage Icon { get; set; }
        //}
        //BitmapImage BitmapToImageSource(Bitmap bitmap)
        //{
        //    using (MemoryStream memory = new MemoryStream())
        //    {
        //        bitmap.Save(memory, System.Drawing.Imaging.ImageFormat.Bmp);
        //        memory.Position = 0;
        //        BitmapImage bitmapimage = new BitmapImage();
        //        bitmapimage.BeginInit();
        //        bitmapimage.StreamSource = memory;
        //        bitmapimage.CacheOption = BitmapCacheOption.OnLoad;
        //        bitmapimage.EndInit();

        //        return bitmapimage;
        //    }
        //}

        // Window interaction
        private void BtnExit(object sender, RoutedEventArgs e)
        {
            Globals.WindowHandling.BtnExit(sender, e, this);
        }
        private void BtnMinimize(object sender, RoutedEventArgs e)
        {
            Globals.WindowHandling.BtnMinimize(sender, e, this);
        }
        private void DragWindow(object sender, MouseButtonEventArgs e)
        {
            Globals.WindowHandling.DragWindow(sender, e, this);
        }

        private void Window_Closing(object sender, CancelEventArgs e)
        {

        }
        private void Window_Closed(object sender, EventArgs e)
        {
            Environment.Exit(1);
        }

        private void PlatformSelect(object sender, SelectionChangedEventArgs e)
        {
            var item = (ListBox)sender;
            var su = (ListBoxItem)item.SelectedItem;
            su.Background = new SolidColorBrush(Color.FromArgb(255, 21, 21, 30));
        }
        private void ListBoxItem_MouseDoubleClick(object sender, MouseButtonEventArgs e)
        {
            MessageBox.Show("Program selected");
        }

        private void ListPlatforms_OnPreviewMouseUp(object sender, MouseButtonEventArgs e)
        {
            foreach (ListBoxItem listPlatformsItem in ListPlatforms.Items)
            {
                listPlatformsItem.Background = new SolidColorBrush() { Color = Color.FromArgb(255, 40, 41, 58) };
            }
        }

        private void PickerSteam(object sender, MouseButtonEventArgs e)
        {
            // Check if Steam is in the default location
            // Otherwise
            // Check if Steam is running, and get the .exe location
            // Otherwise
            // Ask the user where Steam is
            this.Hide();
        }

        private void PickerOrigin(object sender, MouseButtonEventArgs e)
        {
            // Check if Origin is in the default location
            // Otherwise
            // Check if Origin is running, and get the .exe location
            // Otherwise
            // Ask the user where Steam is
            this.Hide();
        }
    }
}
