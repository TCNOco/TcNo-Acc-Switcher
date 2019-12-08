using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.ComponentModel;
using System.Diagnostics;
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
using System.Windows.Media.Animation;
using System.Windows.Media.Imaging;
using System.Windows.Navigation;
//using System.Windows.Shapes; -- Commented because of clash with System.IO.Path. If causes issues, uncomment.
using System.Xml;

// For registry keys
using Microsoft.Win32;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;

namespace TCNO_Acc_Switcher_CSharp_WPF
{
    /// <summary>
    /// Interaction logic for MainWindow.xaml
    /// </summary>
    public static class getChild
    {
        public static T GetChildOfType<T>(this DependencyObject depObj)
        where T : DependencyObject
        {
            if (depObj == null) return null;

            for (int i = 0; i < VisualTreeHelper.GetChildrenCount(depObj); i++)
            {
                var child = VisualTreeHelper.GetChild(depObj, i);

                var result = (child as T) ?? GetChildOfType<T>(child);
                if (result != null) return result;
            }
            return null;
        }
    }
    public partial class MainWindow : Window
    {
        List<Steamuser> userAccounts = new List<Steamuser>();
        List<string> fLoginUsersLines = new List<string>();
        MainWindowViewModel MainViewmodel = new MainWindowViewModel();

        //int version = 1;
        int version = 2003;
        

        // Settings will load later. Just defined here.
        UserSettings persistentSettings = new UserSettings();
        SolidColorBrush vacRedBrush = (SolidColorBrush)(new BrushConverter().ConvertFromString("#FFFF293A"));


        public MainWindow()
        {
            /* TODO:
             */
            if (File.Exists(Path.Combine("Resources", "7za.exe")))
            {
                new DirectoryInfo("Resources").Delete(true);
                File.Delete("UpdateInformation.txt");
                File.Delete("x64.zip");
                File.Delete("x32.zip");
                bool deleted = false;
                while (!deleted)
                {
                    try
                    {
                        File.Delete("TcNo-Acc-Switcher-Updater.exe");
                        File.Delete("TcNo-Acc-Switcher-Updater.dll");
                        File.Delete("TcNo-Acc-Switcher-Updater.runtimeconfig.json");
                        deleted = true;
                    }
                    catch (Exception)
                    {
                        Thread.Sleep(500);
                    }
                }
                // Because closing a messagebox before the window shows causes it to crash for some reason...
                MessageBoxResult messageBoxResult = System.Windows.MessageBox.Show("Open GitHub to see what's new?", "Finished updating.", System.Windows.MessageBoxButton.YesNo);
                if (messageBoxResult == MessageBoxResult.Yes)
                {
                    Process.Start(new ProcessStartInfo("https://github.com/TcNobo/TcNo-Acc-Switcher/releases") { UseShellExecute = true });
                }
            }
            else
            {
                if (File.Exists("UpdateFound.txt"))
                    downloadUpdateDialog();
                else
                {
                    Thread updateCheckThread = new Thread(updateCheck);
                    updateCheckThread.Start();
                }
            }
            MainViewmodel.ProgramVersion = "Version: " + version.ToString();

            if (File.Exists("DeleteImagesOnStart"))
            {
                File.Delete("DeleteImagesOnStart");
                new DirectoryInfo("Images").Delete(true);
            }

            this.DataContext = MainViewmodel;
            InitializeSettings(); // Load user settings
            InitializeComponent();
            updateFromSettings(); // Update components

            // Create image folder
            Directory.CreateDirectory("images");

            // Collect Steam Account basic info from Steam file
            lblStatus.Content = "Status: Collecting Steam Accounts";
            getSteamAccounts();

            // Check if profile images exist, otherwise queue
            List<Steamuser> ImagesToDownload = new List<Steamuser>();
            foreach (Steamuser su in userAccounts)
            {
                if (!File.Exists(su.ImgURL))
                {
                    ImagesToDownload.Add(su);
                }
                else
                {
                    su.ImgURL = Path.GetFullPath(su.ImgURL);
                    MainViewmodel.SteamUsers.Add(su);
                }
            }

            if (File.Exists("Users.json") && persistentSettings.ShowVACStatus)
                loadVacInformation();

            if (ImagesToDownload.Count > 0)
            {
                Thread t = new Thread(new ParameterizedThreadStart(DownloadImages));
                t.Start(ImagesToDownload);
                lblStatus.Content = "Status: Starting image download.";
            }
            else
            {
                lblStatus.Content = "Status: Ready";
            }
        }
        void downloadUpdateDialog()
        {
            MessageBoxResult messageBoxResult = System.Windows.MessageBox.Show("Update now?", "Update was found", System.Windows.MessageBoxButton.YesNo);
            if (messageBoxResult == MessageBoxResult.Yes)
            {
                if (File.Exists("UpdateFound.txt"))
                    File.Delete("UpdateFound.txt");

                // Extract embedded files
                if (!Directory.Exists("Resources"))
                    Directory.CreateDirectory("Resources");
                File.WriteAllBytes(Path.Join("Resources", "7za.exe"), Properties.Resources._7za);
                File.WriteAllText(Path.Join("Resources", "7za-license.txt"), Properties.Resources.License);
                string  zPath = Path.Combine("Resources", "7za.exe"),
                        updzip = "upd.7z",
                        ePath = Path.GetDirectoryName(System.Reflection.Assembly.GetEntryAssembly().Location);
#if X64
                string arch = "x64";
                File.WriteAllBytes(updzip, Properties.Resources.update64);
#else
                string arch = "x32";
                File.WriteAllBytes(updzip, Properties.Resources.update32);
#endif

                ProcessStartInfo pro = new ProcessStartInfo();
                pro.WindowStyle = ProcessWindowStyle.Hidden;
                pro.FileName = zPath;
                pro.Arguments = string.Format("x \"{0}\" -y -o\"{1}\"", updzip, ePath);
                pro.UseShellExecute = false;
                pro.RedirectStandardOutput = true;
                pro.CreateNoWindow = true;
                Process x = Process.Start(pro);
                x.WaitForExit();

                File.Delete("UpdateInformation.txt");
                using (FileStream fs = File.Create("UpdateInformation.txt"))
                {
                    byte[] info = new UTF8Encoding(true).GetBytes(System.AppDomain.CurrentDomain.FriendlyName + ".exe|" + arch + "|" + version.ToString());
                    fs.Write(info, 0, info.Length);
                }

                // Run update.exe
                string processName = "TcNo-Acc-Switcher-Updater.exe";
                ProcessStartInfo startInfo = new ProcessStartInfo();
                startInfo.FileName = processName;
                startInfo.CreateNoWindow = false;
                startInfo.UseShellExecute = true;
                Process.Start(startInfo);
                Environment.Exit(1);
            }
        }
        void updateCheck()
        {
            System.Net.WebClient wc = new System.Net.WebClient();
            int webVersion = int.Parse(wc.DownloadString("https://tcno.co/Projects/AccSwitcher/version.php").Substring(0,4));
            
            if (webVersion > version)
            {
                using (FileStream fs = File.Create("UpdateFound.txt"))
                {
                    byte[] info = new UTF8Encoding(true).GetBytes("An update was found last launch" + DateTime.Now.ToString());
                    fs.Write(info, 0, info.Length);
                }
                this.Dispatcher.Invoke(() =>
                {
                    downloadUpdateDialog();
                });
            }
            else
            {
                if (File.Exists("UpdateFound.txt"))
                    File.Delete("UpdateFound.txt");
            }
        }
        bool vacBanned = false;
        void DownloadImages(object oin)
        {
            List<Steamuser> ImagesToDownload = (List<Steamuser>)oin;

            int totalUsers = ImagesToDownload.Count();
            int currentUser = 0;
            // DISABLE LISTBOX
            foreach (Steamuser su in ImagesToDownload)
            {
                currentUser++;
                this.Dispatcher.Invoke(() =>
                {
                    lblStatus.Content = $"Status: Downloading profile image: {currentUser.ToString()}/{totalUsers}";
                });
                string imageURL = getUserImageURL(su.SteamID);

                if (!string.IsNullOrEmpty(imageURL))
                {
                    using (WebClient client = new WebClient())
                    {
                        client.DownloadFile(new Uri(imageURL), su.ImgURL);
                    }
                }
                else
                {
                    File.WriteAllBytes(su.ImgURL, Properties.Resources.QuestionMark);
                }
                su.ImgURL = Path.GetFullPath(su.ImgURL);
                if (persistentSettings.ShowVACStatus)
                    su.vacStatus = vacBanned ? vacRedBrush : Brushes.Transparent;

                this.Dispatcher.Invoke(() =>
                {
                    MainViewmodel.SteamUsers.Add(su);
                });
            }
            this.Dispatcher.Invoke(() =>
            {
                saveVacInformation();
                lblStatus.Content = "Status: Ready";
            });
            // ENABLE LISTBOX
        }
        public void Window_SizeUpdated(object sender, RoutedEventArgs e)
        {
            //if (resultsTab.IsSelected)
            //{
            //    Grid.SetRowSpan(dataGrid1, 2);
            //    Grid.SetRowSpan(dataGrid2, 2);
            //}
        }
        void InitializeSettings()
        {
            if (!File.Exists("settings.json"))
                saveSettings();
            else
                loadSettings();
            bool validSteamFound = (File.Exists(persistentSettings.SteamEXE()));
            //bool validSteamFound = false; // Testing
            validSteamFound = setAndCheckSteamFolder(false);
            if (!validSteamFound)
            {
                MessageBox.Show("You are required to pick a Steam directory for this program to work. Please check you have it installed and run this program again");
                Environment.Exit(1);
                // this.Close() won't work, because the main window hasn't appeared just yet. Still needs to be populated with Steam Accounts.
            }
        }
        bool setAndCheckSteamFolder(bool manual)
        {
            if (!manual)
            {
                MainViewmodel.SteamNotFound = true;
                string ProgramFiles = "C:\\Program Files\\Steam\\Steam.exe",
                       ProgramFiles86 = "C:\\Program Files (x86)\\Steam\\Steam.exe";
                bool exists = File.Exists(ProgramFiles),
                     exists86 = File.Exists(ProgramFiles86);

                if (exists86)
                    persistentSettings.SteamFolder = Directory.GetParent(ProgramFiles86).ToString();
                else if (exists)
                    persistentSettings.SteamFolder = Directory.GetParent(ProgramFiles).ToString();

                if (exists86 || exists)
                {
                    saveSettings();
                    return (File.Exists(persistentSettings.SteamEXE()));
                }
            }
            else
            {
                MainViewmodel.SteamNotFound = false;
            }

            SteamFolderInput getInputFolderDialog = new SteamFolderInput();
            getInputFolderDialog.DataContext = MainViewmodel;
            getInputFolderDialog.ShowDialog();
            if (!String.IsNullOrEmpty(MainViewmodel.InputFolderDialogResponse))
            {
                persistentSettings.SteamFolder = MainViewmodel.InputFolderDialogResponse;
                saveSettings();
                return (File.Exists(persistentSettings.SteamEXE()));
            }
            else
                return false;
        }
        void loadSettings()
        {
            JsonSerializer serializer = new JsonSerializer() { NullValueHandling = NullValueHandling.Ignore };
            using (StreamReader sr = new StreamReader(@"settings.json"))
            {
                // persistentSettings = JsonConvert.DeserializeObject<UserSettings>(sr.ReadToEnd()); -- Entirely replaces, instead of merging. New variables won't have values.
                // Using a JSON Union Merge means that settings that are missing will have default values, set at the top of this file.
                JObject jCurrent = JObject.Parse(JsonConvert.SerializeObject(persistentSettings));

                jCurrent.Merge(JObject.Parse(sr.ReadToEnd()), new JsonMergeSettings
                {
                    MergeArrayHandling = MergeArrayHandling.Union
                });
                persistentSettings = jCurrent.ToObject<UserSettings>();
            }
        }
        void updateFromSettings()
        {
            MainViewmodel.StartAsAdmin = persistentSettings.StartAsAdmin;
            MainViewmodel.ShowSteamID = persistentSettings.ShowSteamID;
            MainViewmodel.ShowVACStatus = persistentSettings.ShowVACStatus;
            MainViewmodel.InputFolderDialogResponse = persistentSettings.SteamFolder;
            this.Width = persistentSettings.WindowSize.Width;
            this.Height = persistentSettings.WindowSize.Height;
            ShowSteamIDHidden.IsChecked = persistentSettings.ShowSteamID;
            toggleVACStatus(persistentSettings.ShowVACStatus);
        }
        void saveOtherVarsToSettings()
        {
            persistentSettings.WindowSize = new Size(this.Width, this.Height);
            persistentSettings.StartAsAdmin = MainViewmodel.StartAsAdmin;
            persistentSettings.ShowVACStatus = MainViewmodel.ShowVACStatus;
            persistentSettings.SteamFolder = MainViewmodel.InputFolderDialogResponse;
        }
        void saveSettings()
        {
            if (!Double.IsNaN(this.Height))
            {
                // Verifies that the program has started properly. Can be any property to do with the window. Just using Width.
                saveOtherVarsToSettings();
                JsonSerializer serializer = new JsonSerializer() { NullValueHandling = NullValueHandling.Ignore };

                using (StreamWriter sw = new StreamWriter(@"settings.json"))
                using (JsonWriter writer = new JsonTextWriter(sw))
                {
                    serializer.Serialize(writer, persistentSettings);
                }
            }
        }
        public static string UnixTimeStampToDateTime(string unixTimeStampString)
        {
            double unixTimeStamp = Convert.ToDouble(unixTimeStampString);
            // Unix timestamp is seconds past epoch
            var localDateTimeOffset = DateTimeOffset.FromUnixTimeSeconds(long.Parse(unixTimeStampString)).DateTime.ToLocalTime();
            return localDateTimeOffset.ToString("dd/MM/yyyy hh:mm:ss");
        }
        void getSteamAccounts()
        {
            string line, lineNoQuot;
            string username = "", steamID = "", rememberAccount = "", personaName = "", timestamp = "";

            System.IO.StreamReader file = new System.IO.StreamReader(persistentSettings.LoginusersVDF());
            while ((line = file.ReadLine()) != null)
            {
                fLoginUsersLines.Add(line);
                line = line.Replace("\t", "");
                lineNoQuot = line.Replace("\"","");

                if (lineNoQuot.All(char.IsDigit) && string.IsNullOrEmpty(steamID)) // Line is SteamID and steamID is empty >> New user.
                {
                    steamID = lineNoQuot;
                } 
                else if (lineNoQuot.All(char.IsDigit) && !string.IsNullOrEmpty(steamID)) // If steamID isn't empty, save account details, empty temp vars for collection.
                {
                    userAccounts.Add(new Steamuser() { Name = personaName, AccName = username, SteamID = steamID, ImgURL = Path.Combine("images", $"{steamID}.jpg"), lastLogin = UnixTimeStampToDateTime(timestamp) });
                    username = "";
                    rememberAccount = "";
                    personaName = "";
                    timestamp = "";
                    steamID = lineNoQuot;
                }
                else if (line.Contains("AccountName"))
                {
                    username = lineNoQuot.Substring(11, lineNoQuot.Length-11);
                } else if (line.Contains("RememberPassword"))
                {
                    rememberAccount = lineNoQuot.Substring(lineNoQuot.Length - 1);
                }
                else if (line.Contains("PersonaName"))
                {
                    personaName = lineNoQuot.Substring(11, lineNoQuot.Length - 11);
                }
                else if (line.Contains("Timestamp"))
                {
                    timestamp = lineNoQuot.Substring(9, lineNoQuot.Length - 9);
                }

                System.Console.WriteLine(line);
            }
            // While loop adds account when new one started. Will not include the last one, so that's done here.
            userAccounts.Add(new Steamuser() { Name = personaName, AccName = username, SteamID = steamID, ImgURL = Path.Combine("images", $"{steamID}.jpg"), lastLogin = UnixTimeStampToDateTime(timestamp) });

            file.Close();
        }
        string getUserImageURL(string steamID)
        {
            string imageURL = "";
            XmlDocument profileXML = new XmlDocument();
            profileXML.Load($"https://steamcommunity.com/profiles/{steamID}?xml=1");
            try
            {
                imageURL = profileXML.DocumentElement.SelectNodes("/profile/avatarFull")[0].InnerText;
                bool isVAC = profileXML.DocumentElement.SelectNodes("/profile/vacBanned")[0].InnerText == "1" ? true : false;
                bool isLimited = profileXML.DocumentElement.SelectNodes("/profile/isLimitedAccount")[0].InnerText == "1" ? true : false;
                vacBanned = isVAC || isLimited;
            }
            catch (NullReferenceException) // User has not set up their account, or does not have an image.
            {
                imageURL = "";
            }
            return imageURL;
        }
        private void SteamUserSelect(object sender, SelectionChangedEventArgs e)
        {
            if (!listAccounts.IsLoaded) return;
            var item = (ListBox)sender;
            var su = (Steamuser)item.SelectedItem;
            lblStatus.Content = "Selected " + su.Name;
            HeaderInstruction.Content = "2. Press Login";
            btnLogin.IsEnabled = true;
            btnLogin.Background = new SolidColorBrush(MainViewmodel.SelectedSteamUser != null ? DarkGreen : DefaultGray);
        }
        //private void SteamUserUnselect(object sender, RoutedEventArgs e)
        //{
        //    if (!listAccounts.IsLoaded) return;
        //    //MessageBox.Show("You have selected a ListBoxItem!");
        //    MessageBox.Show(MainViewmodel.SelectedSteamUser.Name);
        //}
        private void mostRecentUpdate()
        {
            // -----------------------------------
            // ----- Manage "loginusers.vdf" -----
            // -----------------------------------
            Byte[] info;
            using (FileStream fs = File.Open(persistentSettings.LoginusersVDF(), FileMode.Truncate, FileAccess.Write, FileShare.None))
            {
                lblStatus.Content = "Status: Editing loginusers.vdf";
                string lineNoQuot;
                bool userIDMatch = false;
                string outline = "", SelectedSteamID = MainViewmodel.SelectedSteamUser.SteamID;
                foreach (string curline in fLoginUsersLines)
                {
                    outline = curline;

                    lineNoQuot = curline;
                    lineNoQuot = lineNoQuot.Replace("\t", "").Replace("\"", "");

                    if (lineNoQuot.All(char.IsDigit)) // Check if line is JUST digits -> SteamID
                    {
                        userIDMatch = false;
                        if (lineNoQuot == SelectedSteamID)
                        {
                            // Most recent ID matches! Set this account to active.
                            userIDMatch = true;
                        }
                    }
                    else if (curline.Contains("mostrecent"))
                    {
                        // Set every mostrecent to 0, unless it's the one you want to switch to.
                        if (userIDMatch)
                        {
                            outline = "\t\t\"mostrecent\"\t\t\"1\"";
                        }
                        else
                        {
                            outline = "\t\t\"mostrecent\"\t\t\"0\"";
                        }
                    }
                    info = new UTF8Encoding(true).GetBytes(outline + "\n");
                    fs.Write(info, 0, info.Length);
                }
            }

            // -----------------------------------
            // --------- Manage registry ---------
            // -----------------------------------
            /*
            ------------ Structure ------------
            HKEY_CURRENT_USER\Software\Valve\Steam\
                --> AutoLoginUser = username
                --> RememberPassword = 1
            */
            lblStatus.Content = "Status: Editing Steam's Registry keys";
            using (RegistryKey key = Registry.CurrentUser.CreateSubKey(@"Software\Valve\Steam"))
            {
                key.SetValue("AutoLoginUser", MainViewmodel.SelectedSteamUser.AccName);
                key.SetValue("RememberPassword", 1);
            }
        }
        private void closeSteam()
        {
            // This is what Administrator permissions are required for.
            System.Diagnostics.Process process = new System.Diagnostics.Process();
            System.Diagnostics.ProcessStartInfo startInfo = new System.Diagnostics.ProcessStartInfo();
            startInfo.WindowStyle = System.Diagnostics.ProcessWindowStyle.Hidden;
            startInfo.FileName = "cmd.exe";
            startInfo.Arguments = "/C TASKKILL /F /T /IM steam*";
            process.StartInfo = startInfo;
            process.Start();
            process.WaitForExit();
        }

        public class Steamuser
        {
            public string Name { get; set; }
            public string SteamID { get; set; }
            public string ImgURL { get; set; }
            public string lastLogin { get; set; }
            public string AccName { get; set; }
            public System.Windows.Media.Brush vacStatus { get; set; }
        }
        public class UserSettings
        {
            //UserSettings defaultSettings = new UserSettings { StartAsAdmin = false, SteamFolder = "C:\\Program Files (x86)\\Steam\\", ShowSteamID = false, WindowSize = new Size(773, 420) };
            public bool StartAsAdmin { get; set; } = false;
            public bool ShowSteamID { get; set; } = false;
            public bool ShowVACStatus { get; set; } = true;
            public string SteamFolder { get; set; } = "C:\\Program Files (x86)\\Steam\\";
            public Size WindowSize { get; set; } = new Size(773, 420);
            public string LoginusersVDF()
            {
                return Path.Combine(SteamFolder, "config\\loginusers.vdf");
            }
            public string SteamEXE()
            {
                return Path.Combine(SteamFolder, "Steam.exe");
            }
        }

        public class MainWindowViewModel : INotifyPropertyChanged 
        {
            public MainWindowViewModel()
            {
                SteamUsers = new ObservableCollection<Steamuser>();
                InputFolderDialogResponse = "";
                SteamNotFound = new bool();
                StartAsAdmin = new bool();
                ShowSteamID = new bool();
                ShowVACStatus = new bool();
                vacStatus = Brushes.Black;
                ProgramVersion = "";
            }

            public ObservableCollection<Steamuser> SteamUsers { get; private set; }

            private Steamuser _SelectedSteamUser;
            public Steamuser SelectedSteamUser
            {
                get { return _SelectedSteamUser; }
                set
                {
                    _SelectedSteamUser = value;
                }
            }
            private string _InputFolderDialogResponse;
            public string InputFolderDialogResponse
            {
                get { return _InputFolderDialogResponse; }
                set
                {
                    _InputFolderDialogResponse = value;
                }
            }
            private bool _ShowSteamID;
            public bool ShowSteamID
            {
                get
                {
                    return _ShowSteamID;
                }
                set
                {
                    _ShowSteamID = value;
                }
            }
            private bool _ShowVACStatus;
            public bool ShowVACStatus
            {
                get
                {
                    return _ShowVACStatus;
                }
                set
                {
                    _ShowVACStatus = value;
                }
            }
            private bool _StartAsAdmin;
            public bool StartAsAdmin
            {
                get
                {
                    return _StartAsAdmin;
                }
                set
                {
                    _StartAsAdmin = value;
                }
            }
            private bool _SteamNotFound;
            public bool SteamNotFound
            {
                get
                {
                    return _SteamNotFound;
                }
                set
                {
                    _SteamNotFound = value;
                }
            }
            private string _ProgramVersion;
            public string ProgramVersion
            {
                get
                {
                    return _ProgramVersion;
                }
                set
                {
                    _ProgramVersion = value;
                }
            } 
            private System.Windows.Media.Brush _vacStatus;
            public System.Windows.Media.Brush vacStatus
            {
                get
                {
                    return _vacStatus;
                }
                set
                {
                    _vacStatus = value;
                }
            }

            public event PropertyChangedEventHandler PropertyChanged;

            protected void NotifyPropertyChanged(String info)
            {
                if (PropertyChanged != null)
                {
                    PropertyChanged(this, new PropertyChangedEventArgs(info));
                }
            }
        }

        private void LoginMouseDown(object sender, MouseButtonEventArgs e)
        {
            LoginSelected();
        }
        private void ListBoxItem_MouseDoubleClick(object sender, MouseButtonEventArgs e)
        {
            LoginSelected();
        }
        private void LoginSelected()
        {
            saveSettings();
            LoginButtonAnimation("#0c0c0c", "#333333", 2000);

            lblStatus.Content = "Logging into: " + MainViewmodel.SelectedSteamUser.Name;
            btnLogin.IsEnabled = false;

            MainViewmodel.SelectedSteamUser.lastLogin = DateTime.Now.ToString("dd/MM/yyyy hh:mm:ss");
            listAccounts.Items.Refresh();

            lblStatus.Content = "Status: Closing Steam";
            closeSteam();
            mostRecentUpdate();

            if (persistentSettings.StartAsAdmin)
                Process.Start(persistentSettings.SteamEXE());
            else
                Process.Start(new ProcessStartInfo("explorer.exe", persistentSettings.SteamEXE()));
            lblStatus.Content = "Status: Started Steam";
            btnLogin.IsEnabled = true;
        }

        private void LoginButtonAnimation(string colFrom, string colTo, int len)
        {
            ColorAnimation animation;
            animation = new ColorAnimation();
            animation.From = (Color)(ColorConverter.ConvertFromString(colFrom));
            animation.To = (Color)(ColorConverter.ConvertFromString(colTo));
            animation.Duration = new Duration(TimeSpan.FromMilliseconds(len));

            btnLogin.Background = new SolidColorBrush(Colors.Orange);
            btnLogin.Background.BeginAnimation(SolidColorBrush.ColorProperty, animation);
        }

        Color DarkGreen = (Color)(ColorConverter.ConvertFromString("#053305"));
        Color DefaultGray = (Color)(ColorConverter.ConvertFromString("#333333"));
        private void btnLogin_MouseEnter(object sender, MouseEventArgs e)
        {
            btnLogin.Background = new SolidColorBrush(MainViewmodel.SelectedSteamUser != null ? Colors.Green : DefaultGray);
        }

        private void btnLogin_MouseLeave(object sender, MouseEventArgs e)
        {
            btnLogin.Background = new SolidColorBrush(MainViewmodel.SelectedSteamUser !=null ? DarkGreen : DefaultGray);
        }
        private void btnExit(object sender, RoutedEventArgs e)
        {
            this.Close();
        }

        private void btnMinimize(object sender, RoutedEventArgs e)
        {
            this.WindowState = WindowState.Minimized;
        }

        private void dragWindow(object sender, MouseButtonEventArgs e)
        {
            this.DragMove();
        }

        private void Window_Closing(object sender, CancelEventArgs e)
        {
            if (!Double.IsNaN(this.Height)) // Verifies that the program has started properly. Can be any property to do with the window. Just using Width.
                saveSettings();
        }

        private void chkShowSettings_MouseEnter(object sender, MouseEventArgs e)
        {
            getChild.GetChildOfType<Border>(chkShowSettings).Background = new SolidColorBrush((Color)(ColorConverter.ConvertFromString("#444444")));
            getChild.GetChildOfType<Border>(chkShowSettings).BorderBrush = new SolidColorBrush((Color)(ColorConverter.ConvertFromString("#0685d1")));
        }

        private void chkShowSettings_MouseLeave(object sender, MouseEventArgs e)
        {
            getChild.GetChildOfType<Border>(chkShowSettings).Background = new SolidColorBrush((Color)(ColorConverter.ConvertFromString("#333333")));
            getChild.GetChildOfType<Border>(chkShowSettings).BorderBrush = new SolidColorBrush(Colors.Gray);
        }
        public void ResetSettings()
        {
            persistentSettings = new UserSettings();
            updateFromSettings();
            setAndCheckSteamFolder(false);
            listAccounts.Items.Refresh();

        }
        public void PickSteamFolder()
        {
            bool validSteamFound = (File.Exists(persistentSettings.SteamEXE()));
            string OldLocation = persistentSettings.SteamFolder;

            validSteamFound = setAndCheckSteamFolder(true);
            if (!validSteamFound)
            {
                persistentSettings.SteamFolder = OldLocation;
                MainViewmodel.InputFolderDialogResponse = OldLocation;
                MessageBox.Show("Steam location not chosen. Resetting to old value: " + OldLocation);
            }
        }
        public void ResetImages()
        {
                File.Create("DeleteImagesOnStart");
                MessageBox.Show("The program will now close. Once opened, new images will download.");
                this.Close();
        }
        public bool VACCheckRunning = false;
        public void CheckVac()
        {
            if (!VACCheckRunning)
            {
                VACCheckRunning = true;
                lblStatus.Content = "Status: Checking VAC status for each account.";

                foreach (Steamuser su in MainViewmodel.SteamUsers)
                {
                    su.vacStatus = Brushes.Transparent;
                }
                listAccounts.Items.Refresh();


                Thread t = new Thread(new ParameterizedThreadStart(checkVacForeach));
                t.Start(MainViewmodel.SteamUsers);
            }
        }

        private void Window_Closed(object sender, EventArgs e)
        {
            Environment.Exit(1);
        }

        private void btnShowInfo_MouseEnter(object sender, MouseEventArgs e)
        {
            getChild.GetChildOfType<Border>(btnShowInfo).Background = new SolidColorBrush((Color)(ColorConverter.ConvertFromString("#444444")));
            getChild.GetChildOfType<Border>(btnShowInfo).BorderBrush = new SolidColorBrush((Color)(ColorConverter.ConvertFromString("#0685d1")));
        }

        private void btnShowInfo_MouseLeave(object sender, MouseEventArgs e)
        {
            getChild.GetChildOfType<Border>(btnShowInfo).Background = new SolidColorBrush((Color)(ColorConverter.ConvertFromString("#333333")));
            getChild.GetChildOfType<Border>(btnShowInfo).BorderBrush = new SolidColorBrush(Colors.Gray);
        }

        private void btnShowInfo_Click(object sender, RoutedEventArgs e)
        {
            InfoWindow infoWindow = new InfoWindow();
            infoWindow.DataContext = MainViewmodel;
            infoWindow.Owner = this;
            infoWindow.ShowDialog();
        }

        void checkVacForeach(object oin)
        {
            ObservableCollection<Steamuser> SteamUsers = (ObservableCollection<Steamuser>)oin;
            int currentCount = 0;
            string totalCount = SteamUsers.Count().ToString();

            foreach (Steamuser su in SteamUsers)
            {
                currentCount++;
                this.Dispatcher.Invoke(() =>
                {
                    lblStatus.Content = $"Status: Checking VAC status: {currentCount.ToString()}/{totalCount}";
                });
                //su.vacStatus = GetVacStatus(su.SteamID).Result ? vacRedBrush : Brushes.Transparent; 
                bool VacOrLimited = false;
                XmlDocument profileXML = new XmlDocument();
                profileXML.Load($"https://steamcommunity.com/profiles/{su.SteamID}?xml=1");
                try
                {
                    bool isVAC = profileXML.DocumentElement.SelectNodes("/profile/vacBanned")[0].InnerText == "1";
                    bool isLimited = profileXML.DocumentElement.SelectNodes("/profile/isLimitedAccount")[0].InnerText == "1";
                    VacOrLimited =  isVAC || isLimited;
                }
                catch (NullReferenceException) // User has not set up their account
                {
                    VacOrLimited = false;
                }
                this.Dispatcher.Invoke(() =>
                {
                    su.vacStatus = VacOrLimited ? vacRedBrush : Brushes.Transparent;
                    UpdateListFromAsyncVacCheck(su);
                });
            }

            this.Dispatcher.Invoke(() =>
            {
                lblStatus.Content = "Status: Ready";
                saveVacInformation();
                VACCheckRunning = false;
            });
        }
        void UpdateListFromAsyncVacCheck(Steamuser UpdatedUser)
        {
            foreach (Steamuser su in MainViewmodel.SteamUsers)
            {
                if (su.SteamID == UpdatedUser.SteamID)
                {
                    su.vacStatus = UpdatedUser.vacStatus;
                }
            }
            listAccounts.Items.Refresh();
        }
        void saveVacInformation()
        {
            if (!Double.IsNaN(this.Height))
            {
                // Verifies that the program has started properly. Can be any property to do with the window. Just using Width.

                Dictionary<string, bool> VacInformation = new Dictionary<string, bool>{ };
                foreach (Steamuser su in MainViewmodel.SteamUsers)
                {
                    VacInformation.Add(su.SteamID, su.vacStatus == vacRedBrush ? true : false); // If red >> Vac or Limited
                }

                JsonSerializer serializer = new JsonSerializer() { NullValueHandling = NullValueHandling.Ignore };

                using (StreamWriter sw = new StreamWriter(@"Users.json"))
                using (JsonWriter writer = new JsonTextWriter(sw))
                {
                    serializer.Serialize(writer, VacInformation);
                }
            }
        }
        void loadVacInformation()
        {
            JsonSerializer serializer = new JsonSerializer() { NullValueHandling = NullValueHandling.Ignore };
            using (StreamReader sr = new StreamReader(@"Users.json"))
            {
                Dictionary<string, bool> VacInformation = JsonConvert.DeserializeObject<Dictionary<string, bool>>(sr.ReadToEnd());
                foreach (Steamuser su in MainViewmodel.SteamUsers)
                {
                    if (VacInformation.ContainsKey(su.SteamID))
                        su.vacStatus = VacInformation[su.SteamID] ? vacRedBrush : Brushes.Transparent;
                    else
                        su.vacStatus = Brushes.Transparent;
                }
            }
        }

        private void chkShowSettings_Click(object sender, RoutedEventArgs e)
        {
            Settings settingsDialog = new Settings();
            settingsDialog.ShareMainWindow(this);
            settingsDialog.DataContext = MainViewmodel;
            settingsDialog.Owner = this;
            settingsDialog.ShowDialog();
        }
        public void toggleVACStatus(bool VACEnabled)
        {
            if (!VACEnabled)
            {
                foreach (Steamuser su in MainViewmodel.SteamUsers)
                {
                    su.vacStatus = Brushes.Transparent;
                }
            }
            else if (File.Exists("Users.json"))
            {
                loadVacInformation();
            }
            listAccounts.Items.Refresh();
            persistentSettings.ShowVACStatus = VACEnabled;
            MainViewmodel.ShowVACStatus = VACEnabled;
        }
        //void DownloadImages(object oin)
        //{
        //    List<Steamuser> ImagesToDownload = (List<Steamuser>)oin;
        //private static async Task<bool> GetVacStatus(string steamID)
        //{
        //    XmlDocument profileXML = new XmlDocument();
        //    profileXML.Load($"https://steamcommunity.com/profiles/{steamID}?xml=1");
        //    try
        //    {
        //        bool isVAC = profileXML.DocumentElement.SelectNodes("/profile/vacBanned")[0].InnerText == "1" ? true : false;
        //        bool isLimited = profileXML.DocumentElement.SelectNodes("/profile/isLimitedAccount")[0].InnerText == "1" ? true : false;
        //        return isVAC || isLimited;
        //    }
        //    catch (NullReferenceException) // User has not set up their account
        //    {
        //        return false;
        //    }
        //}
    }
}
