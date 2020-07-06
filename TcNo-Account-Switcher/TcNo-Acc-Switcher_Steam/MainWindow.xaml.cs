// For registry keys
using Microsoft.Win32;
using Microsoft.Win32.TaskScheduler;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.ComponentModel;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Net;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Animation;
//using System.Windows.Shapes; -- Commented because of clash with System.IO.Path. If causes issues, uncomment.
using System.Xml;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Steam
{
    /// <summary>
    /// Interaction logic for MainWindow.xaml
    /// </summary>
    public partial class MainWindow
    {
        List<Steamuser> _userAccounts = new List<Steamuser>();
        private List<string> _fLoginUsersLines = new List<string>();
        MainWindowViewModel MainViewmodel = new MainWindowViewModel();
        private TrayUsers trayUsers = new TrayUsers();

        private readonly Color _darkGreen = Color.FromRgb(5, 51, 5);
        private readonly Color _defaultGray = Color.FromRgb(51, 51, 51);


        // Settings will load later. Just defined here.
        private const int SteamVersion = 3002;
        private UserSettings _persistentSettings = new UserSettings();
        private readonly SolidColorBrush _vacRedBrush = new SolidColorBrush(Color.FromRgb(255,41,58));

        public MainWindow(bool argumentLaunch = false)
        {
            /* TODO:
             - Test extract and replace of 7-zip while updating. Don't think it's working, but haven't been able to test just yet.
             - Reset images. Start a program that restarts the main program, or restart the program in another way.
             */
            // Single instance check
            if (!argumentLaunch && SelfAlreadyRunning())
            {
                Console.WriteLine(Strings.SwitcherAlreadyRunning);
                MessageBox.Show(Strings.SwitcherAlreadyRunning, Strings.SwitcherAlreadyRunningHeading, MessageBoxButton.OK, MessageBoxImage.Error);
                Environment.Exit(99);
            }
            Directory.SetCurrentDirectory(Path.GetDirectoryName(System.Reflection.Assembly.GetEntryAssembly().Location)); // Set working directory to the same as the actual .exe

            // Crash handler
            AppDomain.CurrentDomain.UnhandledException += new UnhandledExceptionEventHandler(Globals.CurrentDomain_UnhandledException);

            // Version/update handling
            MainViewmodel.ProgramVersion = Strings.Version + ": " + SteamVersion;
            var globals = Globals.LoadExisting(Path.GetDirectoryName(Directory.GetCurrentDirectory()));
            if (globals.NeedsUpdateCheck()) globals.RunUpdateCheck();

            if (File.Exists("DeleteImagesOnStart"))
            {
                File.Delete("DeleteImagesOnStart");
                new DirectoryInfo("Images").Delete(true);
            }

            this.DataContext = MainViewmodel;
            InitializeSettings(); // Load user settings
            InitializeComponent();
            UpdateFromSettings(); // Update components
            CheckShortcuts();

            // Create image folder
            Directory.CreateDirectory("images");
            RefreshSteamAccounts();
        }
        private static bool SelfAlreadyRunning()
        {
                var processes = Process.GetProcesses();
                var currentProc = Process.GetCurrentProcess();
                return processes.Any(process => currentProc.ProcessName == process.ProcessName && currentProc.Id != process.Id);
        }


        public void RefreshSteamAccounts()
        {
            // Collect Steam Account basic info from Steam file
            lblStatus.Content = Strings.StatusCollectingAccounts;
            CheckBrokenImages();
            GetSteamAccounts();

            // Clear in-case it's a refresh
            MainViewmodel.SteamUsers.Clear();
            listAccounts.Items.Refresh();

            // Check if profile images exist, otherwise queue
            var imagesToDownload = new List<Steamuser>();
            foreach (var su in _userAccounts)
            {
                var fi = new FileInfo(su.ImgURL);
                if (fi.LastWriteTime < DateTime.Now.AddDays(-1 * _persistentSettings.ImageLifetime)){
                    imagesToDownload.Add(su);
                    fi.Delete();
                } else if (!File.Exists(su.ImgURL))
                    imagesToDownload.Add(su);
                else
                {
                    su.ImgURL = Path.GetFullPath(su.ImgURL);
                    MainViewmodel.SteamUsers.Add(su);
                }
            }

            if (File.Exists("SteamVACCache.json") && _persistentSettings.ShowVACStatus)
                LoadVacInformation();

            if (imagesToDownload.Count > 0)
            {
                var t = new Thread(DownloadImages);
                t.Start(imagesToDownload);
                lblStatus.Content = Strings.StatusImageDownloadStart;
            }
            else
                lblStatus.Content = Strings.StatusReady;
        }

        private bool _vacBanned;

        private void DownloadImages(object oin)
        {
            var imagesToDownload = (List<Steamuser>)oin;

            var totalUsers = imagesToDownload.Count();
            var currentUser = 0;
            var downloadError = false;
            // DISABLE LISTBOX
            foreach (var su in imagesToDownload)
            {
                currentUser++;
                var user = currentUser;
                this.Dispatcher.Invoke(() =>
                {
                    lblStatus.Content = $"{Strings.StatusDownloadingProfile} {user.ToString()}/{totalUsers}";
                });
                var imageUrl = GetUserImageUrl(su.SteamID);

                if (!string.IsNullOrEmpty(imageUrl))
                {
                    try
                    {
                        using (var client = new WebClient())
                        {
                            client.DownloadFile(new Uri(imageUrl), su.ImgURL);
                        }
                    }
                    catch (WebException ex)
                    {
                        if (!downloadError && ex.HResult != -2146233079) // Ignore currently in use error, for when program is still writing to file.
                        {
                            downloadError = true; // Show error only once
                            // .net Core way: File.WriteAllBytes(su.ImgURL, Properties.Resources.QuestionMark); // Give the user's profile picture a question mark.
                            Properties.Resources.QuestionMark.Save(su.ImgURL);
                            MessageBox.Show($"{Strings.ErrImageDownloadFail} {ex}", Strings.ErrProfileImageDlFail, MessageBoxButton.OK, MessageBoxImage.Error);
                        }
                    }
                }
                else
                {
                    // .net Core way: File.WriteAllBytes(su.ImgURL, Properties.Resources.QuestionMark);
                    var written = 0;
                    while (written < 3 && written != -1)
                    {
                        try
                        {
                            written += 1;
                            Properties.Resources.QuestionMark.Save(su.ImgURL);
                            written = -1;
                        }
                        catch (ExternalException)
                        {
                            // Error saving properly. Often happens on first run.
                            // Ignoring this error seems to work well, without causing other issues.
                            Thread.Sleep(500);
                        }
                    }
                }
                su.ImgURL = Path.GetFullPath(su.ImgURL);
                if (_persistentSettings.ShowVACStatus)
                    su.vacStatus = _vacBanned ? _vacRedBrush : Brushes.Transparent;

                this.Dispatcher.Invoke(() =>
                {
                    MainViewmodel.SteamUsers.Add(su);
                });
            }
            this.Dispatcher.Invoke(() =>
            {
                SaveVacInformation();
                lblStatus.Content = Strings.StatusReady;
            });
            // ENABLE LISTBOX
        }
        public void Window_SizeUpdated(object sender, RoutedEventArgs e)
        {
            //if (resultsTab.IsSelected)
            //{
            //    Grid.SetRoLoadTrayUsersSpan(dataGrid1, 2);
            //    Grid.SetRowSpan(dataGrid2, 2);
            //}
        }

        private void InitializeSettings()
        {
            if (!File.Exists("SteamSettings.json"))
                SaveSettings();
            else
                LoadSettings();
            var validSteamFound = (File.Exists(_persistentSettings.SteamExe()));
            //bool validSteamFound = false; // Testing
            if (!validSteamFound)
            {
                validSteamFound = SetAndCheckSteamFolder(false);
                if (!validSteamFound)
                {
                    MessageBox.Show(Strings.RequiredPickSteamDir);
                    Environment.Exit(1);
                    // this.Close() won't work, because the main window hasn't appeared just yet. Still needs to be populated with Steam Accounts.
                }
            }
            if (File.Exists("Tray_Users.json"))
                trayUsers.LoadTrayUsers();
        }

        private bool SetAndCheckSteamFolder(bool manual)
        {
            if (!manual)
            {
                MainViewmodel.SteamNotFound = true;
                const string programFiles = "C:\\Program Files\\Steam\\Steam.exe";
                const string programFiles86 = "C:\\Program Files (x86)\\Steam\\Steam.exe";
                bool exists = File.Exists(programFiles),
                     exists86 = File.Exists(programFiles86);

                if (exists86)
                    _persistentSettings.SteamFolder = Directory.GetParent(programFiles86).ToString();
                else if (exists)
                    _persistentSettings.SteamFolder = Directory.GetParent(programFiles).ToString();

                if (exists86 || exists)
                {
                    SaveSettings();
                    return (File.Exists(_persistentSettings.SteamExe()));
                }
            }
            else
                MainViewmodel.SteamNotFound = false;

            var getInputFolderDialog = new SteamFolderInput {DataContext = MainViewmodel};
            getInputFolderDialog.ShowDialog();
            if (!string.IsNullOrEmpty(MainViewmodel.InputFolderDialogResponse))
            {
                _persistentSettings.SteamFolder = MainViewmodel.InputFolderDialogResponse;
                SaveSettings();
                return (File.Exists(_persistentSettings.SteamExe()));
            }
            else
                return false;
        }

        private void LoadSettings()
        {
            using (var sr = new StreamReader(@"SteamSettings.json"))
            {
                // persistentSettings = JsonConvert.DeserializeObject<UserSettings>(sr.ReadToEnd()); -- Entirely replaces, instead of merging. New variables won't have values.
                // Using a JSON Union Merge means that settings that are missing will have default values, set at the top of this file.
                var jCurrent = JObject.Parse(JsonConvert.SerializeObject(_persistentSettings));
                try
                {
                    jCurrent.Merge(JObject.Parse(sr.ReadToEnd()), new JsonMergeSettings
                    {
                        MergeArrayHandling = MergeArrayHandling.Union
                    });
                    _persistentSettings = jCurrent.ToObject<UserSettings>();
                }
                catch (Exception)
                {
                    if (File.Exists("SteamSettings.json"))
                    {
                        if (File.Exists("SteamSettings.old.json"))
                            File.Delete("SteamSettings.old.json");
                        File.Copy("SteamSettings.json", "SteamSettings.old.json");
                    }

                    SaveSettings();
                    MessageBox.Show(Strings.ErrSteamSettingsLoadFail, Strings.ErrSteamSettingsLoadFailHeader, MessageBoxButton.OK, MessageBoxImage.Error);
                }
            }
        }

        private void UpdateFromSettings()
        {
            MainViewmodel.StartAsAdmin = _persistentSettings.StartAsAdmin;
            MainViewmodel.ShowSteamID = _persistentSettings.ShowSteamID;
            MainViewmodel.ShowVACStatus = _persistentSettings.ShowVACStatus;
            MainViewmodel.LimitedAsVAC = _persistentSettings.LimitedAsVAC;
            MainViewmodel.InputFolderDialogResponse = _persistentSettings.SteamFolder;
            MainViewmodel.ForgetAccountEnabled = _persistentSettings.ForgetAccountEnabled;
            MainViewmodel.TrayAccounts = _persistentSettings.TrayAccounts;
            MainViewmodel.TrayAccountAccNames = _persistentSettings.TrayAccountAccNames;
            MainViewmodel.ImageLifetime = _persistentSettings.ImageLifetime;
            this.Width = _persistentSettings.WindowSize.Width;
            this.Height = _persistentSettings.WindowSize.Height;
            ShowSteamIDHidden.IsChecked = _persistentSettings.ShowSteamID;
            ToggleVacStatus(_persistentSettings.ShowVACStatus);
        }

        private void SaveOtherVarsToSettings()
        {
            _persistentSettings.WindowSize = new Size(this.Width, this.Height);
            _persistentSettings.StartAsAdmin = MainViewmodel.StartAsAdmin;
            _persistentSettings.ShowVACStatus = MainViewmodel.ShowVACStatus;
            _persistentSettings.LimitedAsVAC = MainViewmodel.LimitedAsVAC;
            _persistentSettings.SteamFolder = MainViewmodel.InputFolderDialogResponse;
            _persistentSettings.ForgetAccountEnabled = MainViewmodel.ForgetAccountEnabled;
            _persistentSettings.TrayAccounts = MainViewmodel.TrayAccounts;
            _persistentSettings.TrayAccountAccNames = MainViewmodel.TrayAccountAccNames;
            _persistentSettings.ImageLifetime = MainViewmodel.ImageLifetime;
        }

        private void SaveSettings()
        {
            if (double.IsNaN(this.Height)) return;
            // Verifies that the program has started properly. Can be any property to do with the window. Just using Width.
            SaveOtherVarsToSettings();
            var serializer = new JsonSerializer() { NullValueHandling = NullValueHandling.Ignore };

            using (var sw = new StreamWriter(@"SteamSettings.json"))
            using (JsonWriter writer = new JsonTextWriter(sw))
            {
                serializer.Serialize(writer, _persistentSettings);
            }
        }
        private static string UnixTimeStampToDateTime(string unixTimeStampString)
        {
            // Unix timestamp is seconds past epoch
            var localDateTimeOffset = DateTimeOffset.FromUnixTimeSeconds(long.Parse(unixTimeStampString)).DateTime.ToLocalTime();
            return localDateTimeOffset.ToString("dd/MM/yyyy hh:mm:ss");
        }
        private static bool IsValidGdiPlusImage(string filename)
        {
            //From https://stackoverflow.com/questions/8846654/read-image-and-determine-if-its-corrupt-c-sharp
            try
            {
                using (var bmp = new System.Drawing.Bitmap(filename))
                    return true;
            }
            catch
            {
                return false;
            }
        }
        private void CheckBrokenImages()
        {
            if (!Directory.Exists("images")) return;
            var d = new DirectoryInfo("images");
            foreach (var file in d.GetFiles("*.jpg"))
            {
                try
                {
                    if (!IsValidGdiPlusImage(file.FullName)) // Delete image if is not as valid, working image.
                    {
                        File.Delete(file.FullName);
                    }
                }
                catch (Exception ex)
                {
                    try
                    {
                        File.Delete(file.FullName);
                    }
                    catch (Exception)
                    {
                        MessageBox.Show($"{Strings.ErrEmptyImage} {ex}", Strings.ErrEmptyImageHeader, MessageBoxButton.OK, MessageBoxImage.Error);
                        throw;
                    }
                }
            }
        }

        private void GetSteamAccounts()
        {
            string username = "", steamId = "", personaName = "", timestamp = "";

            // Clear in-case it's a refresh
            _fLoginUsersLines.Clear();
            _userAccounts.Clear();

            try
            {
                var file = new StreamReader(_persistentSettings.LoginusersVdf());

                string line;
                while ((line = file.ReadLine()) != null)
                {
                    _fLoginUsersLines.Add(line);
                    line = line.Replace("\t", "");
                    var lineNoQuot = line.Replace("\"", "");

                    if (lineNoQuot.All(char.IsDigit) && string.IsNullOrEmpty(steamId)) // Line is SteamID and steamID is empty >> New user.
                    {
                        steamId = lineNoQuot;
                    }
                    else if (lineNoQuot.All(char.IsDigit) && !string.IsNullOrEmpty(steamId)) // If steamID isn't empty, save account details, empty temp vars for collection.
                    {
                        //lastLogin = UnixTimeStampToDateTime(timestamp);
                        _userAccounts.Add(new Steamuser() { Name = personaName, AccName = username, SteamID = steamId, ImgURL = Path.Combine("images", $"{steamId}.jpg"), lastLogin = UnixTimeStampToDateTime(timestamp) });
                        username = "";
                        personaName = "";
                        timestamp = "";
                        steamId = lineNoQuot;
                    }
                    else if (line.Contains("AccountName"))
                    {
                        username = lineNoQuot.Substring(11, lineNoQuot.Length - 11);
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
                if (!String.IsNullOrEmpty(steamId))
                    _userAccounts.Add(new Steamuser() { Name = personaName, AccName = username, SteamID = steamId, ImgURL = Path.Combine("images", $"{steamId}.jpg"), lastLogin = UnixTimeStampToDateTime(timestamp) });

                file.Close();
            }
            catch (FileNotFoundException ex)
            {
                MessageBox.Show(Strings.ErrLoginusersNonExist, Strings.ErrLoginusersNonExistHeader, MessageBoxButton.OK, MessageBoxImage.Error);
                MessageBox.Show($"{Strings.ErrInformation} {ex}", Strings.ErrLoginusersNonExistHeader, MessageBoxButton.OK, MessageBoxImage.Error);
                Environment.Exit(2);
            }
        }

        private string GetUserImageUrl(string steamId)
        {
            var imageUrl = "";
            var profileXml = new XmlDocument();
            try
            {
                profileXml.Load($"https://steamcommunity.com/profiles/{steamId}?xml=1");
                if (profileXml.DocumentElement != null && profileXml.DocumentElement.SelectNodes("/profile/privacyMessage").Count == 0) // Fix for accounts that haven't set up their Community Profile
                {
                    try
                    {
                        imageUrl = profileXml.DocumentElement.SelectNodes("/profile/avatarFull")[0].InnerText;
                        var isVac = false;
                        var isLimited = true;
                        if (profileXml.DocumentElement != null)
                        {
                            if (profileXml.DocumentElement.SelectNodes("/profile/vacBanned")?[0] != null)
                                isVac = profileXml.DocumentElement.SelectNodes("/profile/vacBanned")?[0].InnerText == "1";
                            if (profileXml.DocumentElement.SelectNodes("/profile/isLimitedAccount")?[0] != null)
                                isLimited = profileXml.DocumentElement.SelectNodes("/profile/isLimitedAccount")?[0].InnerText == "1";
                        }

                        if (!_persistentSettings.LimitedAsVAC) // Ignores limited accounts
                            isLimited = false;
                        _vacBanned = isVac || isLimited;
                    }
                    catch (NullReferenceException) // User has not set up their account, or does not have an image.
                    {
                        imageUrl = "";
                    }
                }
            }
            catch (Exception e)
            {
                _vacBanned = false;
                imageUrl = "";
                Directory.CreateDirectory("Errors");
                using (var sw = File.AppendText($"Errors\\AccSwitcher-Error-{DateTime.Now:dd-MM-yy_hh-mm-ss.fff}.txt"))
                {
                    sw.WriteLine(DateTime.Now.ToString(CultureInfo.InvariantCulture) + "\t" + Strings.ErrUnhandledCrash + ": " + e + Environment.NewLine + Environment.NewLine);
                }
                using (var sw = File.AppendText($"Errors\\AccSwitcher-Error-{DateTime.Now:dd-MM-yy_hh-mm-ss.fff}.txt"))
                {
                    sw.WriteLine(JsonConvert.SerializeObject(profileXml));
                }
            }
            return imageUrl;
        }
        private void SteamUserSelect(object sender, SelectionChangedEventArgs e)
        {
            if (!listAccounts.IsLoaded) return;
            var item = (ListBox)sender;
            var su = (Steamuser)item.SelectedItem;
            try
            {
                lblStatus.Content = $"{Strings.StatusAccountSelected} {su.Name}";
                HeaderInstruction.Content = Strings.StatusPressLogin;
                btnLogin.IsEnabled = true;
                btnLogin.Background = new SolidColorBrush(MainViewmodel.SelectedSteamUser != null ? _darkGreen : _defaultGray);
            }
            catch
            {
                // Non-existent user account is selected, or none are available.
            }
        }
        //private void SteamUserUnselect(object sender, RoutedEventArgs e)
        //{
        //    if (!listAccounts.IsLoaded) return;
        //    //MessageBox.Show("You have selected a ListBoxItem!");
        //    MessageBox.Show(MainViewmodel.SelectedSteamUser.Name);
        //}
        private void UpdateLoginUsers(bool loginNone, string selectedSteamId, string accName)
        {
            // -----------------------------------
            // ----- Manage "loginusers.vdf" -----
            // -----------------------------------
            var targetUsername = accName;
            var tempFile = _persistentSettings.LoginusersVdf() + "_temp";
            File.Delete(tempFile);
            using (var fs = File.Create(tempFile))
            {
                lblStatus.Content = Strings.StatusEditingLoginusers;
                var userIdMatch = false;
                foreach (var line in _fLoginUsersLines)
                {
                    var outline = line;
                    var lineNoQuot = line;
                    lineNoQuot = lineNoQuot.Replace("\t", "").Replace("\"", "");

                    if (lineNoQuot.All(char.IsDigit)) // Check if line is JUST digits -> SteamID
                    {
                        // Most recent ID matches! Set this account to active.
                        userIdMatch = lineNoQuot == selectedSteamId;
                    }
                    else if (line.Contains("AccountName") && userIdMatch) // Username not supplied
                            targetUsername = lineNoQuot.Substring(11);
                    else if (line.Contains("mostrecent"))
                    {
                        // Set every mostrecent to 0, unless it's the one you want to switch to.
                        if (!loginNone && userIdMatch)
                        {
                            outline = "\t\t\"mostrecent\"\t\t\"1\"";
                        }
                        else
                        {
                            outline = "\t\t\"mostrecent\"\t\t\"0\"";
                        }
                    }
                    else if (line.Contains("}"))
                    {
                        // Reset variables for next user.
                        if (userIdMatch && string.IsNullOrEmpty(targetUsername))
                            MessageBox.Show(Strings.ErrSwitchingAcc, Strings.ErrSwitchingAcc_MissingUsername, MessageBoxButton.OK, MessageBoxImage.Error);
                    }
                    var info = new UTF8Encoding(true).GetBytes(outline + "\n");
                    fs.Write(info, 0, info.Length);
                }
            }
            // Replace original file with temp
            File.Replace(tempFile, _persistentSettings.LoginusersVdf(), _persistentSettings.LoginusersVdf() + "_last");

            // -----------------------------------
            // --------- Manage registry ---------
            // -----------------------------------
            /*
            ------------ Structure ------------
            HKEY_CURRENT_USER\Software\Valve\Steam\
                --> AutoLoginUser = username
                --> RememberPassword = 1
            */
            lblStatus.Content = Strings.StatusEditingRegistry;
            using (var key = Registry.CurrentUser.CreateSubKey(@"Software\Valve\Steam"))
            {
                if (loginNone)
                {
                    key.SetValue("AutoLoginUser", "");
                    key.SetValue("RememberPassword", 1);
                }
                else
                {
                    key.SetValue("AutoLoginUser", targetUsername); // Account name is not set when changing user accounts from launch arguments (part of the viewmodel).
                    key.SetValue("RememberPassword", 1);
                }
            }
        }
        public void CloseSteam()
        {
            // This is what Administrator permissions are required for.
            var startInfo = new ProcessStartInfo
            {
                WindowStyle = System.Diagnostics.ProcessWindowStyle.Hidden,
                FileName = "cmd.exe",
                Arguments = "/C TASKKILL /F /T /IM steam*"
            };
            var process = new Process {StartInfo = startInfo};
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
            public bool LimitedAsVAC { get; set; } = true;
            public bool ForgetAccountEnabled { get; set; } = false;
            public int ImageLifetime { get; set; } = 7;
            public string SteamFolder { get; set; } = "C:\\Program Files (x86)\\Steam\\";
            public Size WindowSize { get; set; } = new Size(680, 360);
            public string LoginusersVdf()
            {
                return Path.Combine(SteamFolder, "config\\loginusers.vdf");
            }
            public string SteamExe()
            {
                return Path.Combine(SteamFolder, "Steam.exe");
            }

            public int TrayAccounts { get; set; } = 3; // Default: Up to 3 accounts.
            public bool TrayAccountAccNames { get; set; } = false; // Use account names instead of friendly names.
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
                LimitedAsVAC = new bool();
                StartMenuIcon = new bool();
                StartWithWindows = new bool();
                DesktopShortcut = new bool();
                vacStatus = Brushes.Black;
                ProgramVersion = "";
                ForgetAccountEnabled = new bool();
                TrayAccounts = 3;
                TrayAccountAccNames = new bool();
                ImageLifetime = 7;
            }
            public ObservableCollection<Steamuser> SteamUsers { get; private set; }
            public Steamuser SelectedSteamUser { get; set; }
            public string InputFolderDialogResponse { get; set; }
            public bool ShowSteamID { get; set; }
            public bool ShowVACStatus { get; set; }
            public bool LimitedAsVAC { get; set; }
            public bool StartMenuIcon { get; set; }
            public bool DesktopShortcut { get; set; }
            public bool StartWithWindows { get; set; }
            public bool StartAsAdmin { get; set; }
            public bool SteamNotFound { get; set; }
            public bool ForgetAccountEnabled { get; set; }
            public string ProgramVersion { get; set; }
            public int TrayAccounts { get; set; }
            public int ImageLifetime { get; set; }
            public bool TrayAccountAccNames { get; set; }
            public System.Windows.Media.Brush vacStatus { get; set; }
            public event PropertyChangedEventHandler PropertyChanged;
            protected void NotifyPropertyChanged(string info)
            {
                PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(info));
            }
        }

        private void LoginMouseDown(object sender, MouseButtonEventArgs e) => LoginSelected();
        private void ListBoxItem_MouseDoubleClick(object sender, MouseButtonEventArgs e) => LoginSelected();
        private static bool IsDigitsOnly(string str) => str.All(c => c >= '0' && c <= '9');
        private void LoginSelected()
        {
            SaveSettings();
            LoginButtonAnimation(Color.FromRgb(12,12,12), _defaultGray, 2000);

            lblStatus.Content = "Logging into: " + MainViewmodel.SelectedSteamUser.Name;
            btnLogin.IsEnabled = false;

            MainViewmodel.SelectedSteamUser.lastLogin = DateTime.Now.ToString("dd/MM/yyyy hh:mm:ss");
            listAccounts.Items.Refresh();

            // Add to tray access accounts for tray icon.
            // Can switch to this account quickly later.
            if (_persistentSettings.TrayAccounts > 0)
            {
                // Add to list, or move to most recent spot.
                if (trayUsers.AlreadyInList(MainViewmodel.SelectedSteamUser.SteamID))
                    trayUsers.MoveItemToLast(MainViewmodel.SelectedSteamUser.SteamID);
                else{
                    while (trayUsers.ListTrayUsers.Count >= _persistentSettings.TrayAccounts) // Add to list, drop first item if full.
                        trayUsers.ListTrayUsers.RemoveAt(0);

                    trayUsers.ListTrayUsers.Add(new TrayUsers.TrayUser()
                        {
                            AccName = MainViewmodel.SelectedSteamUser.AccName,
                            Name = MainViewmodel.SelectedSteamUser.Name,
                            DisplayAs = (_persistentSettings.TrayAccountAccNames ? MainViewmodel.SelectedSteamUser.AccName : MainViewmodel.SelectedSteamUser.Name),
                            SteamID = MainViewmodel.SelectedSteamUser.SteamID
                        }
                    );
                }

                trayUsers.SaveTrayUsers();
            }

            lblStatus.Content = Strings.StatusClosingSteam;
            SwapSteamAccounts(false, MainViewmodel.SelectedSteamUser.SteamID, MainViewmodel.SelectedSteamUser.AccName);
            lblStatus.Content = Strings.StatusStartedSteam;
            btnLogin.IsEnabled = true;
        }


        private static bool VerifySteamId(string steamid)
        {
            if (!IsDigitsOnly(steamid) || steamid.Length != 17) return false;
            // Size check: https://stackoverflow.com/questions/33933705/steamid64-minimum-and-maximum-length#40810076
            var steamidVal = double.Parse(steamid);
            return steamidVal > 0x0110000100000001 && steamidVal < 0x01100001FFFFFFFF;
        }
        public void SwapSteamAccounts(bool loginNone, string steamid, string accName)
        {
            if (!VerifySteamId(steamid)) {
                MessageBox.Show("Invalid SteamID: " + steamid);
                return;
            }

            CloseSteam();
            UpdateLoginUsers(loginNone, steamid, accName);

            if (_persistentSettings.StartAsAdmin)
                Process.Start(_persistentSettings
                    .SteamExe()); // Maybe get steamID from elsewhere? Or load persistent settings first...
            else
                Process.Start(new ProcessStartInfo("explorer.exe", _persistentSettings.SteamExe()));
        }
        private void ShowForgetRememberDialog()
        {
            var forgetAccountCheckDialog = new ForgetAccountCheck(){ DataContext = MainViewmodel, Owner = this };
            forgetAccountCheckDialog.ShareMainWindow(this);
            forgetAccountCheckDialog.ShowDialog();
        }
        public string GetForgottenBackupPath() { return Path.Combine(_persistentSettings.SteamFolder, $"config\\TcNo-Acc-Switcher-Backups\\"); }
        public string GetPersistentFolder() { return Path.Combine(_persistentSettings.SteamFolder, "config\\"); }
        public string GetSteamDirectory() { return _persistentSettings.SteamFolder; }

        public void ClearForgottenBackups()
        {
            if (MessageBox.Show(Strings.ClearBackups, Strings.AreYouSure, MessageBoxButton.YesNo,
                MessageBoxImage.Warning) != MessageBoxResult.Yes) return;
            var backupPath = GetForgottenBackupPath();
            try
            {
                if (Directory.Exists(backupPath))
                    Directory.Delete(backupPath, true);
            }
            catch (Exception ex)
            {
                MessageBox.Show($"{Strings.ErrRecursivelyDelete} {ex}", Strings.ErrDeleteFilesHeader);
            }
        }
        public void OpenSteamFolder()
        {
            Process.Start(_persistentSettings.SteamFolder);
        }
        private void DeleteSelected()
        {
            btnLogin.IsEnabled = false;

            // Check if user understands what "forget" does.
            if (!MainViewmodel.ForgetAccountEnabled)
            {
                ShowForgetRememberDialog();
                return;
            }

            // Backup loginusers.vdf
            string backupFileName = $"loginusers-{ DateTime.Now:dd-MM-yyyy_HH-mm-ss.fff}.vdf",
            backup = Path.Combine(_persistentSettings.SteamFolder, $"config\\TcNo-Acc-Switcher-Backups\\");
            Directory.CreateDirectory(backup);
            try
            {
                File.Copy(_persistentSettings.LoginusersVdf(), Path.Combine(_persistentSettings.SteamFolder, $"config\\TcNo-Acc-Switcher-Backups\\{backupFileName}"));
            }
            catch (IOException e) when (e.HResult == -2147024816) // File already exists -- User deleting > 1 account per second
            {
                File.Copy(_persistentSettings.LoginusersVdf(), Path.Combine(_persistentSettings.SteamFolder, $"config\\TcNo-Acc-Switcher-Backups\\{backupFileName}"));
            }

            // ---------------------------------------------
            // ----- Remove user from "loginusers.vdf" -----
            // ---------------------------------------------
            using (var fs = File.Open(_persistentSettings.LoginusersVdf(), FileMode.Truncate, FileAccess.Write, FileShare.None))
            {
                lblStatus.Content = $"{Strings.StatusRemoving} {MainViewmodel.SelectedSteamUser.Name} {Strings.StatusFromLoginusers}";
                bool userIdMatch = false,
                     completedRemove = false;
                var selectedSteamId = MainViewmodel.SelectedSteamUser.SteamID;
                var newLoginUsersLines = new List<string>();
                foreach (var line in _fLoginUsersLines)
                {
                    var outline = line;

                    var lineNoQuot = line;
                    lineNoQuot = lineNoQuot.Replace("\t", "").Replace("\"", "");
                    if (!completedRemove)
                    {
                        if (!userIdMatch)
                        {
                            if (lineNoQuot.All(char.IsDigit)) // Check if line is JUST digits -> SteamID
                            {
                                if (lineNoQuot == selectedSteamId)
                                {
                                    // Most recent ID matches! Start ignoring lines until "}" found.
                                    userIdMatch = true;
                                    continue; // Skip line output
                                }
                            }
                        }
                        else // Currently going through a user
                        {
                            if (lineNoQuot.Contains("}"))
                                completedRemove = true; // Found the end of the user to remove
                            continue; // Skip line output
                        }
                    }
                    newLoginUsersLines.Add(line);
                    var info = new UTF8Encoding(true).GetBytes(outline + "\n");
                    fs.Write(info, 0, info.Length);
                }
                _fLoginUsersLines = newLoginUsersLines;
            }

            // Remove from list in memory
            MainViewmodel.SteamUsers.Remove(MainViewmodel.SelectedSteamUser);
            listAccounts.Items.Refresh();
        }
        private void AccountItem_Forget(object sender, RoutedEventArgs e) => DeleteSelected();

        private void listAccounts_PreviewKeyDown(object sender, KeyEventArgs e)
        {
            if (listAccounts.SelectedIndex != -1 && e.Key == Key.Delete)
                DeleteSelected();
        }

        // Context Menu (Right-Click on Steam Account) handling
        private void AccountItem_CopyProfile(object sender, RoutedEventArgs e)
        {
            var selected = MainViewmodel.SelectedSteamUser;
            var menuItem = (MenuItem)e.OriginalSource;
            switch (menuItem.Header.ToString())
            {
                case "Community URL":
                    Clipboard.SetText("https://steamcommunity.com/profiles/" + selected.SteamID);
                    break;
                case "Community Username":
                    Clipboard.SetText(selected.Name);
                    break;
                case "Login Username":
                    Clipboard.SetText(selected.AccName);
                    break;
            }
        }
        private void AccountItem_CopySteamIds(object sender, RoutedEventArgs e)
        {
            var steamId = MainViewmodel.SelectedSteamUser.SteamID;
            var menuItem = (MenuItem)e.OriginalSource;
            var sel = menuItem.Header.ToString().ToLower();
            sel = (sel.Contains(' ') ? sel.Split(' ')[0] : sel); // Remove example from string
            switch (sel)
            {
                case "steamid":
                    Clipboard.SetText(new SteamIdConvert(steamId).Id);
                    break;
                case "steamid3":
                    Clipboard.SetText(new SteamIdConvert(steamId).Id3);
                    break;
                case "steamid32":
                    Clipboard.SetText(new SteamIdConvert(steamId).Id32);
                    break;
                case "steamid64":
                    Clipboard.SetText(new SteamIdConvert(steamId).Id64);
                    break;
            }
        }
        private void AccountItem_OtherSites(object sender, RoutedEventArgs e)
        {
            var steamId = MainViewmodel.SelectedSteamUser.SteamID;
            var menuItem = (MenuItem)e.OriginalSource;
            switch (menuItem.Header.ToString().ToLower())
            {
                case "steamrep":
                    Clipboard.SetText($"https://steamrep.com/search?q={steamId}");
                    break;
                case "steamid.uk":
                    Clipboard.SetText($"https://steamid.uk/profile/{steamId}");
                    break;
                case "steamid.io":
                    Clipboard.SetText($"https://steamid.io/lookup/{steamId}");
                    break;
                case "steamidfinder.com":
                    Clipboard.SetText($"https://steamidfinder.com/lookup/{steamId}/");
                    break;
            }
        }
        private void AccountItem_SwitchShortcut(object sender, RoutedEventArgs e)
        {
            var steamId = MainViewmodel.SelectedSteamUser.SteamID;
            var username = MainViewmodel.SelectedSteamUser.AccName;
            CreateSwitchShortcut(steamId, username);
        }

        private void LoginButtonAnimation(Color colFrom, Color colTo, int len)
        {
            var animation = new ColorAnimation
            {
                From = colFrom, To = colTo, Duration = new Duration(TimeSpan.FromMilliseconds(len))
            };

            btnLogin.Background = new SolidColorBrush(Colors.Orange);
            btnLogin.Background.BeginAnimation(SolidColorBrush.ColorProperty, animation);
        }

        private void btnLogin_MouseEnter(object sender, MouseEventArgs e) =>
            btnLogin.Background = new SolidColorBrush(MainViewmodel.SelectedSteamUser != null ? Colors.Green : _defaultGray);

        private void btnLogin_MouseLeave(object sender, MouseEventArgs e) =>
            btnLogin.Background = new SolidColorBrush(MainViewmodel.SelectedSteamUser != null ? _darkGreen : _defaultGray);
        private void BtnExit(object sender, RoutedEventArgs e) =>
            Globals.WindowHandling.BtnExit(sender, e, this);
        private void BtnMinimize(object sender, RoutedEventArgs e) =>
            Globals.WindowHandling.BtnMinimize(sender, e, this);
        private void DragWindow(object sender, MouseButtonEventArgs e) =>
            Globals.WindowHandling.DragWindow(sender, e, this);
        private void Window_Closing(object sender, CancelEventArgs e)
        {
            if (!double.IsNaN(this.Height)) // Verifies that the program has started properly. Can be any property to do with the window. Just using Width.
                SaveSettings();
        }

        public void ResetSettings()
        {
            _persistentSettings = new UserSettings();
            UpdateFromSettings();
            SetAndCheckSteamFolder(false);
            listAccounts.Items.Refresh();

        }
        public void PickSteamFolder()
        {
            var oldLocation = _persistentSettings.SteamFolder;

            var validSteamFound = SetAndCheckSteamFolder(true);
            if (validSteamFound) return;
            _persistentSettings.SteamFolder = oldLocation;
            MainViewmodel.InputFolderDialogResponse = oldLocation;
            MessageBox.Show($"{Strings.ErrSteamLocation} {oldLocation}");
        }
        public void ResetImages()
        {
            File.Create("DeleteImagesOnStart");
            MessageBox.Show(Strings.InfoReopenImageDl);
            this.Close();
        }
        public bool VacCheckRunning;
        public void CheckVac()
        {
            if (VacCheckRunning) return;
            VacCheckRunning = true;
            lblStatus.Content = Strings.StatusCheckingVac;

            foreach (var su in MainViewmodel.SteamUsers)
            {
                su.vacStatus = Brushes.Transparent;
            }
            listAccounts.Items.Refresh();


            var t = new Thread(CheckVacForeach);
            t.Start(MainViewmodel.SteamUsers);
        }

        private void Window_Closed(object sender, EventArgs e)
        {
            Environment.Exit(1);
        }

        private void btnShowInfo_Click(object sender, RoutedEventArgs e)
        {
            var infoWindow = new InfoWindow {DataContext = MainViewmodel, Owner = this};
            infoWindow.ShowDialog();
        }

        private void CheckVacForeach(object oin)
        {
            var steamUsers = (ObservableCollection<Steamuser>)oin;
            var currentCount = 0;
            var totalCount = steamUsers.Count().ToString();

            foreach (var su in steamUsers)
            {
                currentCount++;
                var count = currentCount;
                this.Dispatcher.Invoke(() =>
                {
                    lblStatus.Content = $"{Strings.StatusCheckingVacActive} {count.ToString()}/{totalCount}";
                });
                //su.vacStatus = GetVacStatus(su.SteamID).Result ? _vacRedBrush : Brushes.Transparent; 
                bool vacOrLimited;
                var profileXml = new XmlDocument();
                profileXml.Load($"https://steamcommunity.com/profiles/{su.SteamID}?xml=1");
                try
                {
                    var isVac = false;
                    var isLimited = true;
                    if (profileXml.DocumentElement != null)
                    {
                        if (profileXml.DocumentElement.SelectNodes("/profile/vacBanned")?[0] != null)
                            isVac = profileXml.DocumentElement.SelectNodes("/profile/vacBanned")?[0].InnerText == "1";
                        if (profileXml.DocumentElement.SelectNodes("/profile/isLimitedAccount")?[0] != null)
                            isLimited = profileXml.DocumentElement.SelectNodes("/profile/isLimitedAccount")?[0].InnerText == "1";
                    }

                    if (!_persistentSettings.LimitedAsVAC) // Ignores limited accounts
                        isLimited = false;
                    vacOrLimited = isVac || isLimited;
                }
                catch (NullReferenceException) // User has not set up their account
                {
                    vacOrLimited = false;
                }
                this.Dispatcher.Invoke(() =>
                {
                    su.vacStatus = vacOrLimited ? _vacRedBrush : Brushes.Transparent;
                    UpdateListFromAsyncVacCheck(su);
                });
            }

            this.Dispatcher.Invoke(() =>
            {
                lblStatus.Content = Strings.StatusReady;
                SaveVacInformation();
                VacCheckRunning = false;
            });
        }

        private void UpdateListFromAsyncVacCheck(Steamuser updatedUser)
        {
            foreach (var su in MainViewmodel.SteamUsers)
            {
                if (su.SteamID == updatedUser.SteamID)
                {
                    su.vacStatus = updatedUser.vacStatus;
                }
            }
            listAccounts.Items.Refresh();
        }

        private void SaveVacInformation()
        {
            if (double.IsNaN(this.Height)) return;
            // Verifies that the program has started properly. Can be any property to do with the window. Just using Width.

            var vacInformation = new Dictionary<string, bool> { };
            foreach (var su in MainViewmodel.SteamUsers)
                vacInformation.Add(su.SteamID, su.vacStatus == _vacRedBrush); // If red >> Vac or Limited

            var serializer = new JsonSerializer() { NullValueHandling = NullValueHandling.Ignore };

            using (var sw = new StreamWriter(@"SteamVACCache.json"))
            using (JsonWriter writer = new JsonTextWriter(sw))
            {
                serializer.Serialize(writer, vacInformation);
            }
        }

        private void LoadVacInformation()
        {
            using (var sr = new StreamReader(@"SteamVACCache.json"))
            {
                var vacInformation = JsonConvert.DeserializeObject<Dictionary<string, bool>>(sr.ReadToEnd());
                foreach (var su in MainViewmodel.SteamUsers)
                {
                    if (vacInformation.ContainsKey(su.SteamID))
                        su.vacStatus = vacInformation[su.SteamID] ? _vacRedBrush : Brushes.Transparent;
                    else
                        su.vacStatus = Brushes.Transparent;
                }
            }
        }
        private void btnNewLogin_Click(object sender, RoutedEventArgs e)
        {
            // Kill Steam
            CloseSteam();
            // Set all accounts to 'not used last' status
            UpdateLoginUsers(true, "", "");
            // Start Steam
            if (_persistentSettings.StartAsAdmin)
                Process.Start(_persistentSettings.SteamExe());
            else
                Process.Start(new ProcessStartInfo("explorer.exe", _persistentSettings.SteamExe()));
            lblStatus.Content = Strings.StatusStartedSteam;
        }
        private void chkShowSettings_Click(object sender, RoutedEventArgs e)
        {
            var settingsDialog = new Settings(){ DataContext = MainViewmodel, Owner = this };
            settingsDialog.ShareMainWindow(this);
            settingsDialog.ShowDialog();
        }
        public void ToggleVacStatus(bool vacEnabled)
        {
            if (!vacEnabled)
            {
                foreach (var su in MainViewmodel.SteamUsers)
                {
                    su.vacStatus = Brushes.Transparent;
                }
            }
            else if (File.Exists("SteamVACCache.json"))
            {
                LoadVacInformation();
            }
            listAccounts.Items.Refresh();
            _persistentSettings.ShowVACStatus = vacEnabled;
            MainViewmodel.ShowVACStatus = vacEnabled;
        }

        public void ToggleAccNames(bool val)
        {
            _persistentSettings.TrayAccountAccNames = val;
            MainViewmodel.TrayAccountAccNames = val;

            foreach (var t in trayUsers.ListTrayUsers)
                t.DisplayAs = (val ? t.AccName : t.Name);
            trayUsers.SaveTrayUsers();
        }

        public void CapTotalTrayUsers()
        {
            if (_persistentSettings.TrayAccounts > 0)
                while (trayUsers.ListTrayUsers.Count >= _persistentSettings.TrayAccounts) // Add to list, drop first item if full.
                    trayUsers.ListTrayUsers.RemoveAt(0);
            else
                trayUsers.ListTrayUsers.Clear();
            trayUsers.SaveTrayUsers();
        }
        public void SetTotalRecentAccount(string val)
        {
            _persistentSettings.TrayAccounts = int.Parse(val);
            MainViewmodel.TrayAccounts = int.Parse(val);
        }
        public void SetImageExpiry(string val)
        {
            _persistentSettings.ImageLifetime = int.Parse(val);
            MainViewmodel.ImageLifetime = int.Parse(val);
        }

        public void ToggleLimitedAsVac(bool lav)
        {
            _persistentSettings.LimitedAsVAC = lav;
            MainViewmodel.LimitedAsVAC = lav;
            MessageBox.Show(Strings.InfoRefreshLimitedAsVac);
        }

        private void CheckShortcuts()
        {
            MainViewmodel.DesktopShortcut = ShortcutExist(Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory));
            MainViewmodel.StartWithWindows = CheckStartWithWindows();
            MainViewmodel.StartMenuIcon = ShortcutExist(Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.Programs), @"TcNo Account Switcher\"));
        }
        private static bool CheckStartWithWindows()
        {
            using (var ts = new TaskService())
            {
                var tasks = ts.RootFolder.Tasks;
                return tasks.Exists("TcNo Account Switcher - Tray start with logon");
            }
        }
        public void DesktopShortcut(bool bEnabled)
        {
            MainViewmodel.DesktopShortcut = bEnabled;
            var desktopPath = Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory);
            if (bEnabled)
                CreateShortcut(desktopPath);
            else
                DeleteShortcut(desktopPath, "TcNo Account Switcher - Steam.lnk", false);
        }
        public void StartWithWindows(bool bEnabled)
        {
            MainViewmodel.StartWithWindows = bEnabled;

            if (bEnabled)
            {
                if (CheckStartWithWindows()) return;
                var ts = new TaskService();
                var td = ts.NewTask();
                td.Principal.RunLevel = TaskRunLevel.Highest;
                td.Triggers.AddNew(TaskTriggerType.Logon);
                var programPath = Path.GetFullPath("TcNo Acc Switcher SteamTray.exe");
                td.Actions.Add(new ExecAction(programPath));
                ts.RootFolder.RegisterTaskDefinition("TcNo Account Switcher - Steam Tray start with logon", td);
                MessageBox.Show(Strings.InfoTrayWindowsStart);
            }
            else
            {
                var ts = new TaskService();
                ts.RootFolder.DeleteTask("TcNo Account Switcher - Steam Tray start with logon");
                MessageBox.Show(Strings.InfoTrayWindowsStartOff);
            }
        }
        public void StartMenuShortcut(bool bEnabled)
        {
            MainViewmodel.StartMenuIcon = bEnabled;
            string programsPath = Environment.GetFolderPath(Environment.SpecialFolder.Programs),
                   shortcutFolder = Path.Combine(programsPath, @"TcNo Account Switcher\");
            if (bEnabled)
            {
                CreateShortcut(shortcutFolder);
                CreateTrayShortcut(shortcutFolder);
            }
            else
            {
                DeleteShortcut(shortcutFolder, "TcNo Account Switcher - Steam.lnk", false);
                DeleteShortcut(shortcutFolder, "TcNo Account Switcher - Steam tray.lnk", true);
            }
        }
        private static bool ShortcutExist(string location) =>
            File.Exists(Path.Combine(location, "TcNo Account Switcher - Steam.lnk"));

        private string ParentDirectory(string dir) =>
             dir.Substring(0, dir.LastIndexOf(Path.DirectorySeparatorChar));
        private void CreateShortcut(string location)
        {
            Directory.CreateDirectory(location);
            // Starts the main picker, with the Steam argument.
            string selfExe = Path.Combine(ParentDirectory(Directory.GetCurrentDirectory()), "TcNo Account Switcher.exe"),
                selfLocation = ParentDirectory(Directory.GetCurrentDirectory()),
                iconDirectory = Path.Combine(selfLocation, "icon.ico"),
                settingsLink = Path.Combine(location, "TcNo Account Switcher - Steam.lnk");
            const string description = "TcNo Account Switcher - Steam";
            const string arguments = "+steam";

            WriteShortcut(selfExe, selfLocation, iconDirectory, description, settingsLink, arguments);
        }
        private void CreateSwitchShortcut(string steamId, string username)
        {
            // Starts the main picker, with the Steam argument.
            string selfExe = Path.Combine(Directory.GetCurrentDirectory(), "TcNo Account Switcher Steam.exe"),
                selfLocation = ParentDirectory(Directory.GetCurrentDirectory()),
                iconDirectory = Path.Combine(selfLocation, "icon.ico"),
                settingsLink = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory), $"Switch to {username}.lnk"),
                description = $"TcNo Account Switcher - Steam: Switches to {username}",
                arguments = $"+{steamId}";

            WriteShortcut(selfExe, selfLocation, iconDirectory, description, settingsLink, arguments);
        }
        private static void CreateTrayShortcut(string location)
        {
            string selfExe = Path.Combine(Directory.GetCurrentDirectory(), "TcNo Acc Switcher SteamTray.exe"),
                selfLocation = Directory.GetCurrentDirectory(),
                iconDirectory = Path.Combine(selfLocation, "icon.ico"),
                settingsLink = Path.Combine(location, "TcNo Account Switcher - Steam tray.lnk");
            const string description = "TcNo Account Switcher - Steam tray";
            const string arguments = "";

            WriteShortcut(selfExe, selfLocation, iconDirectory, description, settingsLink, arguments);
        }
        private static void WriteShortcut(string exe, string selfLocation, string iconDirectory, string description, string settingsLink, string arguments)
        {
            if (File.Exists(settingsLink)) return;
            if (File.Exists("CreateShortcut.vbs"))
                File.Delete("CreateShortcut.vbs");

            using (var fs = new FileStream(iconDirectory, FileMode.Create))
                Properties.Resources.icon.Save(fs);


            string[] lines = {"set WshShell = WScript.CreateObject(\"WScript.Shell\")",
                "set oShellLink = WshShell.CreateShortcut(\"" + settingsLink  + "\")",
                "oShellLink.TargetPath = \"" + exe + "\"",
                "oShellLink.WindowStyle = 1",
                "oShellLink.IconLocation = \"" + iconDirectory + "\"",
                "oShellLink.Description = \"" + description + "\"",
                "oShellLink.WorkingDirectory = \"" + selfLocation + "\"",
                "oShellLink.Arguments = \"" + arguments + "\"",
                "oShellLink.Save()"
            };
            File.WriteAllLines("CreateShortcut.vbs", lines);

            var vbsProcess = new Process
            {
                StartInfo =
                {
                    FileName = "cscript",
                    Arguments = "//nologo \"" + Path.GetFullPath("CreateShortcut.vbs") + "\"",
                    UseShellExecute = false,
                    RedirectStandardOutput = true,
                    CreateNoWindow = true
                }
            };


            vbsProcess.Start();
            vbsProcess.StandardOutput.ReadToEnd();
            vbsProcess.Close();

            File.Delete("CreateShortcut.vbs");
            MessageBox.Show("Shortcut created!\n\nLocation: " + settingsLink);
        }
        private static void DeleteShortcut(string location, string name, bool delFolder)
        {
            var settingsLink = Path.Combine(location, name);
            if (File.Exists(settingsLink))
                File.Delete(settingsLink);
            if (delFolder)
            {
                if (Directory.GetFiles(location).Length == 0)
                    Directory.Delete(location);
                else
                    MessageBox.Show($"{Strings.ErrDeleteFolderNonempty} {location}");
            }
            MessageBox.Show(Strings.InfoShortcutDeleted.Replace("{}", name));
        }

    }
}
