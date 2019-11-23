using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
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

namespace TCNO_Acc_Switcher_CSharp_WPF
{
    /// <summary>
    /// Interaction logic for MainWindow.xaml
    /// </summary>
    public partial class MainWindow : Window
    {
        List<Steamuser> userAccounts = new List<Steamuser>();

        string SteamFolder = "C:\\Program Files (x86)\\Steam\\",
            //LoginUsersVDF = @"C:\Users\TechNobo\Documents\loginusers.vdf",
            LoginUsersVDF = "C:\\Program Files (x86)\\Steam\\config\\loginusers.vdf",
            SteamEXE = "C:\\Program Files (x86)\\Steam\\Steam.exe";
        bool runasAdmin = false;

        List<string> fLoginUsersLines = new List<string>();
        MainWindowViewModel MainViewmodel = new MainWindowViewModel();

        public MainWindow()
        {
            /* TODO:
             * MainViewmodel.startAsAdmin
             * Save to and load from JSON file, for settings. Possibly an INI or something else?
             * Add button next to checkbox for InuptBox to get Steam location, if not detected autopopup dialog.
             * Add update check... Possibly autoupdater? Maybe not, user has decision power, usually not nessecary.
             * Display Date of last use under Steam User account, maybe even user account name AND 'ingame name'
             * Button to refresh all images? -- Counter in status saying image/images ie: "2/17 account images downloaded."
             */

            InitializeComponent();

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

        void getSteamAccounts()
        {
            string line, lineNoQuot;
            string username = "", steamID = "", rememberAccount = "", personaName = "", timestamp = "";

            System.IO.StreamReader file = new System.IO.StreamReader(LoginUsersVDF);
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
                    userAccounts.Add(new Steamuser() { Name = personaName, AccName = username, SteamID = steamID, ImgURL = Path.Combine("images", $"{steamID}.jpg"), lastLogin = timestamp });
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
            userAccounts.Add(new Steamuser() { Name = personaName, AccName = username, SteamID = steamID, ImgURL = Path.Combine("images", $"{steamID}.jpg"), lastLogin = timestamp });

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
            using (FileStream fs = File.Open(LoginUsersVDF, FileMode.Truncate, FileAccess.Write, FileShare.None))
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

        public class MainWindowViewModel
        {
            public MainWindowViewModel()
            {
                SteamUsers = new ObservableCollection<Steamuser>();
                startAsAdmin = new bool();
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


            private bool _startAsAdmin;
            public bool startAsAdmin
            {
                get { return _startAsAdmin; }
                set
                {
                    _startAsAdmin = value;
                }
            }
        }

        private void LoginMouseDown(object sender, MouseButtonEventArgs e)
        {
            LoginButtonAnimation("#0c0c0c", "#333333", 2000);

            lblStatus.Content = "Logging into: " + MainViewmodel.SelectedSteamUser.Name;
            btnLogin.IsEnabled = false;

            lblStatus.Content = "Status: Closing Steam";
            closeSteam();
            mostRecentUpdate();

            if (runasAdmin)
                Process.Start(SteamEXE);
            else
                Process.Start(new ProcessStartInfo("explorer.exe", SteamEXE));
            lblStatus.Content = "Status: Started Steam";
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
    }
}
