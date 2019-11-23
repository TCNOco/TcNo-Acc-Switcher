using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Net;
using System.Text;
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

namespace TCNO_Acc_Switcher_CSharp_WPF
{
    /// <summary>
    /// Interaction logic for MainWindow.xaml
    /// </summary>
    public partial class MainWindow : Window
    {
        List<Steamuser> userAccounts = new List<Steamuser>();
        List<string> fLoginUsersLines = new List<string>();
        MainWindowViewModel MainViewmodel = new MainWindowViewModel();


        // Settings will load later. Just defined here.
        UserSettings persistentSettings = new UserSettings { StartAsAdmin = false, SteamFolder = "C:\\Program Files (x86)\\Steam\\", ShowSteamID = false };

        public MainWindow()
        {
            /* TODO:
             * Add update check... Possibly autoupdater? Maybe not, user has decision power, usually not nessecary.
             * Display Date of last use under Steam User account, maybe even user account name AND 'ingame name'
             * Button to refresh all images? -- Counter in status saying image/images ie: "2/17 account images downloaded."
             * Get quit/minimize buttons working.
             */
            InitializeSettings(); // Load user settings
            InitializeComponent();
            updateFromSettings(); // Update components

            // Create image folder
            Directory.CreateDirectory("images");

            // Collect Steam Account basic info from Steam file
            lblStatus.Content = "Status: Collecting Steam Accounts";
            getSteamAccounts();

            // Check if profile images exist, otherwise queue in other threads
            lblStatus.Content = "Status: Downloading missing profile images";
            foreach (Steamuser su in userAccounts)
            {
                if (!File.Exists(su.ImgURL))
                {
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
                }
                su.ImgURL = Path.GetFullPath(su.ImgURL);

                MainViewmodel.SteamUsers.Add(su);
            }

            this.DataContext = MainViewmodel;
            lblStatus.Content = "";
        }
        void InitializeSettings()
        {
            if (!File.Exists("settings.json"))
                saveSettings();
            else
                loadSettings();
            bool validSteamFound = (File.Exists(persistentSettings.SteamEXE()));
            //bool validSteamFound = false; // Testing
            while (!validSteamFound)
            {
                validSteamFound = setAndCheckSteamFolder();
            }
        }
        bool setAndCheckSteamFolder()
        {
            SteamFolderInput getInputFolderDialog = new SteamFolderInput();
            getInputFolderDialog.DataContext = MainViewmodel;
            getInputFolderDialog.ShowDialog();
            persistentSettings.SteamFolder = MainViewmodel.InputFolderDialogResponse;
            saveSettings();
            return (File.Exists(persistentSettings.SteamEXE()));
        }
        void loadSettings()
        {
            using (StreamReader sr = new StreamReader(@"settings.json"))
                persistentSettings = JsonConvert.DeserializeObject<UserSettings>(sr.ReadToEnd());
        }
        void updateFromSettings()
        {
            MainViewmodel.StartAsAdmin = persistentSettings.StartAsAdmin;
            MainViewmodel.ShowSteamID = persistentSettings.ShowSteamID;
        }
        void saveSettings()
        {
            JsonSerializer serializer = new JsonSerializer();
            serializer.NullValueHandling = NullValueHandling.Ignore;

            using (StreamWriter sw = new StreamWriter(@"settings.json"))
            using (JsonWriter writer = new JsonTextWriter(sw))
            {
                serializer.Serialize(writer, persistentSettings);
            }
        }
        public static string UnixTimeStampToDateTime(string unixTimeStampString)
        {
            double unixTimeStamp = Convert.ToDouble(unixTimeStampString);
            // Unix timestamp is seconds past epoch
            System.DateTime dtDateTime = new DateTime(1970, 1, 1, 0, 0, 0, 0, System.DateTimeKind.Utc);
            dtDateTime = dtDateTime.AddSeconds(unixTimeStamp).ToLocalTime();
            return dtDateTime.ToString("dd/mm/yyyy hh:mm:ss");
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
        }
        public class UserSettings
        {
            public bool StartAsAdmin;
            public bool ShowSteamID;
            public string SteamFolder;
            public string LoginusersVDF()
            {
                return Path.Combine(SteamFolder, "config\\loginusers.vdf");
            }
            public string SteamEXE()
            {
                return Path.Combine(SteamFolder, "Steam.exe");
            }
        }

        public class MainWindowViewModel
        {
            public MainWindowViewModel()
            {
                SteamUsers = new ObservableCollection<Steamuser>();
                InputFolderDialogResponse = "";
                StartAsAdmin = new bool();
                ShowSteamID = new bool();
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
            public event PropertyChangedEventHandler PropertyChanged;
            protected void OnPropertyChanged(string propertyName)
            {
                if (PropertyChanged != null)
                    PropertyChanged(this, new PropertyChangedEventArgs(propertyName));
            }
        }

        private void LoginMouseDown(object sender, MouseButtonEventArgs e)
        {
            saveSettings();
            LoginButtonAnimation("#0c0c0c", "#333333", 2000);

            lblStatus.Content = "Logging into: " + MainViewmodel.SelectedSteamUser.Name;
            btnLogin.IsEnabled = false;

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

        private void AdminCheckbox_CheckedChanged(object sender, RoutedEventArgs e)
        {
            persistentSettings.StartAsAdmin = (bool)RunasAdmin.IsChecked;
        }

        private void btnPickSteamFolder_Click(object sender, RoutedEventArgs e)
        {
            bool validSteamFound = false;
            while (!validSteamFound)
            {
                validSteamFound = setAndCheckSteamFolder();
            }
        }

        private void ShowSteamID_CheckedChanged(object sender, RoutedEventArgs e)
        {
            persistentSettings.ShowSteamID = (bool)ShowSteamID.IsChecked;
        }

        private void Window_Closed(object sender, EventArgs e)
        {
            saveSettings();
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

    }
}
