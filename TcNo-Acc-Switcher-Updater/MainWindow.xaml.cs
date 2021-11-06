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

using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Net;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Security.Principal;
using System.Threading;
using System.Windows;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Threading;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using SevenZipExtractor;
using VCDiff.Encoders;
using VCDiff.Decoders;
using VCDiff.Includes;
using VCDiff.Shared;
using YamlDotNet.Serialization;
using YamlDotNet.Serialization.NamingConventions;

namespace TcNo_Acc_Switcher_Updater
{

    /// <summary>
    /// A lot of these functions are copies from the Globals binary, but could not be referenced as this program updates that file.
    /// </summary>
    public class UGlobals
    {
        /// <summary>
        /// A replacement for File.ReadAllText() that doesn't crash if a file is in use.
        /// </summary>
        /// <param name="f">File to be read</param>
        /// <returns>string of content</returns>
        public static string ReadAllText(string f)
        {
            using var fileStream = new FileStream(f, FileMode.Open, FileAccess.Read, FileShare.ReadWrite);
            using var textReader = new StreamReader(fileStream);
            return textReader.ReadToEnd();
        }

        /// <summary>
        /// A replacement for File.ReadAllLines() that doesn't crash if a file is in use.
        /// </summary>
        /// <param name="f">File to be read</param>
        /// <returns>string[] of content</returns>
        public static string[] ReadAllLines(string f)
        {
            using var fs = new FileStream(f, FileMode.Open, FileAccess.Read, FileShare.ReadWrite);
            using var sr = new StreamReader(fs);
            var l = new List<string>();
            while (!sr.EndOfStream)
            {
                l.Add(sr.ReadLine());
            }

            return l.ToArray();
        }
    }
    /// <summary>
    /// Interaction logic for MainWindow.xaml
    /// </summary>
    public partial class MainWindow
    {
        // Dictionaries of file paths, as well as MD5 Hashes
        private static Dictionary<string, string> _oldDict = new();
        private static Dictionary<string, string> _newDict = new();
        private static Dictionary<string, string> _allNewDict = new();
        private static List<string> _patchList = new();

#pragma warning disable CA2211 // Non-constant fields should not be visible - Set as launch parameter
        public static bool VerifyAndClose;
        public static bool QueueHashList;
        public static bool QueueCreateUpdate;
        public static bool DownloadCef;
#pragma warning restore CA2211 // Non-constant fields should not be visible

        private string _currentVersion = "";
        private string _latestVersion = "";


        #region WINDOW_BUTTONS
        public MainWindow() => InitializeComponent();
        private void StartUpdate_Click(object sender, RoutedEventArgs e) => new Thread(DoUpdate).Start();

        private static void LaunchAccSwitcher(object sender, RoutedEventArgs e)
        {
            _ = Process.Start(new ProcessStartInfo(@"TcNo-Acc-Switcher.exe") { UseShellExecute = true });
            Process.GetCurrentProcess().Kill();
        }
        private static void ExitProgram(object sender, RoutedEventArgs e) => Environment.Exit(0);

        // Window interaction
        private void BtnExit(object sender, RoutedEventArgs e) => WindowHandling.BtnExit(this);
        private void BtnMinimize(object sender, RoutedEventArgs e) => WindowHandling.BtnMinimize(this);
        private void DragWindow(object sender, MouseButtonEventArgs e) => WindowHandling.DragWindow(sender, e, this);
        private void Window_Closed(object sender, EventArgs e) => Environment.Exit(0);
        #endregion

        private void CreateExitButton()
        {
            StartButton.Click -= StartUpdate_Click;
            StartButton.Click += ExitProgram;
            ButtonHandler(true, "Exit");
        }
        private void WriteLine(string line, string lineBreak = "\n")
        {
            _ = Dispatcher.BeginInvoke(new Action(() =>
              {
                  LogBox.Text += line + lineBreak;
                  LogBox.ScrollToEnd();
              }), DispatcherPriority.Normal);
        }
        private void SetStatus(string s)
        {
            _ = Dispatcher.BeginInvoke(new Action(() =>
              {
                  StatusLabel.Content = s;
              }), DispatcherPriority.Normal);
        }
        private void SetStatusAndLog(string s, string lineBreak = "\n")
        {
            _ = Dispatcher.BeginInvoke(new Action(() =>
              {
                  StatusLabel.Content = s;
                  LogBox.Text += lineBreak + s;
                  LogBox.ScrollToEnd();
              }), DispatcherPriority.Normal);
        }

        private void ButtonHandler(bool enabled, string content)
        {
            _ = Dispatcher.BeginInvoke(new Action(() =>
              {
                  StartButton.IsEnabled = enabled;
                  StartButton.Content = content;
              }), DispatcherPriority.Normal);
        }

        private void UpdateProgress(int percent)
        {
            _ = Dispatcher.BeginInvoke(new Action(() =>
              {
                  ProgressBar.Value = percent;
              }), DispatcherPriority.Normal);
        }

        private Dictionary<string, string> _updatesAndChanges = new();
        private readonly string _currentDir = Directory.GetCurrentDirectory();
        private readonly string _updaterDirectory = Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location); // Where this program is located

        private void MainWindow_OnContentRendered(object sender, EventArgs e) => new Thread(Init).Start();
        private readonly string _windowSettings = Path.Join(UserDataFolder, "WindowSettings.json");

        private static bool InstalledToProgramFiles()
        {
            var progFiles = Environment.GetFolderPath(Environment.SpecialFolder.ProgramFiles);
            var progFilesX86 = Environment.GetFolderPath(Environment.SpecialFolder.ProgramFilesX86);
            var appPath = Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location);

            return appPath != null && (appPath.Contains(progFiles) || appPath.Contains(progFilesX86));
        }

        private static bool HasFolderAccess(string folderPath)
        {
            try
            {
                using var fs = File.Create(Path.Combine(folderPath, Path.GetRandomFileName()), 1, FileOptions.DeleteOnClose);
                return true;
            }
            catch
            {
                return false;
            }
        }

        private static bool IsAdmin()
        {
            // Checks whether program is running as Admin or not
            var securityIdentifier = WindowsIdentity.GetCurrent().Owner;
            return securityIdentifier is not null && securityIdentifier.IsWellKnown(WellKnownSidType.BuiltinAdministratorsSid);
        }

        public static void RestartAsAdmin(string args)
        {
            var proc = new ProcessStartInfo
            {
                WorkingDirectory = Environment.CurrentDirectory,
                FileName = Assembly.GetEntryAssembly()?.Location.Replace(".dll", ".exe") ?? "TcNo-Acc-Switcher.exe",
                UseShellExecute = true,
                Arguments = args,
                Verb = "runas"
            };
            try
            {
                _ = Process.Start(proc);
                Environment.Exit(0);
            }
            catch (Exception ex)
            {
                _ = MessageBox.Show(@"This program must be run as an administrator!" + Environment.NewLine + ex);
                Environment.Exit(0);
            }
        }

        /// <summary>
        /// Move pre 2021-07-05 Documents userdata folder into AppData.
        /// </summary>
        private static void OldDocumentsToAppData()
        {
            // Check to see if folder still located in My Documents (from Pre 2021-07-05)
            var oldDocuments = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.MyDocuments), "TcNo Account Switcher\\");
            if (Directory.Exists(oldDocuments)) CopyFilesRecursive(oldDocuments, UserDataFolder); // Overwrites by default
            RecursiveDelete(new DirectoryInfo(oldDocuments), false);
        }

        private void Init()
        {
            // Check if installed to program files, and requires admin to run:
            if (InstalledToProgramFiles() && !IsAdmin() || !HasFolderAccess(Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location)))
                RestartAsAdmin("");

            Directory.SetCurrentDirectory(MainAppDataFolder);
            OldDocumentsToAppData();
            if (QueueHashList)
            {
                GenerateHashes();
                Environment.Exit(0);
            }

            if (QueueCreateUpdate)
            {
                CreateUpdate();
                Environment.Exit(0);
            }

            if (DownloadCef)
            {
                DownloadCefNow();
                Environment.Exit(0);
            }

            SetStatus("Checking version");
            // Try get version from Globals.dll
            _currentVersion = GetVersionFromGlobals();

            // If couldn't: Try get Version from WindowSettings.json
            if (string.IsNullOrEmpty(_currentVersion) && File.Exists(_windowSettings))
            {
                try
                {
                    _currentVersion = JObject.Parse(UGlobals.ReadAllText(_windowSettings))["Version"]?.ToString();
                }
                catch (Exception)
                {
                    //
                }
            }

            // If couldn't: Verify instead of updating (Done later).
            if (string.IsNullOrEmpty(_currentVersion)) WriteLine("Could not get version. Click \"Verify\" to update & verify files" + Environment.NewLine);

            // Styling
            Directory.SetCurrentDirectory(UserDataFolder);
            try
            {
                var o = JObject.Parse(UGlobals.ReadAllText(_windowSettings));
                var themeName = o["ActiveTheme"]?.ToString();
                var themeFile = Path.Join("themes", themeName, "info.yaml");

                // Separated so colors aren't only half changed, and a color from the file is missing
                var desc = new DeserializerBuilder().WithNamingConvention(HyphenatedNamingConvention.Instance).Build();
                var dict = JsonConvert.DeserializeObject<Dictionary<string, string>>(JsonConvert.SerializeObject(desc.Deserialize<object>(UGlobals.ReadAllText(themeFile))));
                if (File.Exists(themeFile))
                {
                    _ = Dispatcher.BeginInvoke(new Action(() =>
                    {
                        // Need to try catch every one of these, as they may be invalid.
                        var highlightColor = TryApplyTheme(dict, "#8BE9FD", "linkColor"); // Add a specific Highlight color later?
                        var headerColor = TryApplyTheme(dict, "#253340", "headerbarBackground");
                        var mainBackground = TryApplyTheme(dict, "#0E1419", "mainBackground");
                        var listBackground = TryApplyTheme(dict, "#070A0D", "modalInputBackground");
                        var borderedItemBorderColor = TryApplyTheme(dict, "#888888", "borderedItemBorderColor");
                        var buttonBackground = TryApplyTheme(dict, "#274560", "buttonBackground");
                        var buttonBackgroundHover = TryApplyTheme(dict, "#28374E", "buttonBackground-hover");
                        var buttonBackgroundActive = TryApplyTheme(dict, "#28374E", "buttonBackground-active");
                        var buttonColor = TryApplyTheme(dict, "#FFFFFF", "buttonColor");
                        var buttonBorder = TryApplyTheme(dict, "#274560", "buttonBorder");
                        var buttonBorderHover = TryApplyTheme(dict, "#274560", "buttonBorder-hover");
                        var buttonBorderActive = TryApplyTheme(dict, "#8BE9FD", "buttonBorder-active");
                        var windowControlsBackground = TryApplyTheme(dict, "#253340", "windowControlsBackground");
                        var windowControlsBackgroundActive = TryApplyTheme(dict, "#3B4853", "windowControlsBackground-active");
                        var windowControlsCloseBackground = TryApplyTheme(dict, "#D51426", "windowControlsCloseBackground");
                        var windowControlsCloseBackgroundActive = TryApplyTheme(dict, "#D51426", "windowControlsCloseBackground-active");

                        Resources["HighlightColor"] = highlightColor;
                        Resources["HeaderColor"] = headerColor;
                        Resources["MainBackground"] = mainBackground;
                        Resources["ListBackground"] = listBackground;
                        Resources["BorderedItemBorderColor"] = borderedItemBorderColor;
                        Resources["ButtonBackground"] = buttonBackground;
                        Resources["ButtonBackgroundHover"] = buttonBackgroundHover;
                        Resources["ButtonBackgroundActive"] = buttonBackgroundActive;
                        Resources["ButtonColor"] = buttonColor;
                        Resources["ButtonBorder"] = buttonBorder;
                        Resources["ButtonBorderHover"] = buttonBorderHover;
                        Resources["ButtonBorderActive"] = buttonBorderActive;
                        Resources["WindowControlsBackground"] = windowControlsBackground;
                        Resources["WindowControlsBackgroundActive"] = windowControlsBackgroundActive;
                        Resources["WindowControlsCloseBackground"] = windowControlsCloseBackground;
                        Resources["WindowControlsCloseBackgroundActive"] = windowControlsCloseBackgroundActive;
                    }), DispatcherPriority.Normal);
                }
            }
            catch (Exception e)
            {
                Console.WriteLine(e);
                throw;
            }

            Directory.SetCurrentDirectory(MainAppDataFolder);

            ButtonHandler(false, "...");
            if (string.IsNullOrEmpty(_currentVersion))
            {
                SetStatus("Press button below!");
                CreateVerifyAndExitButton();
                return;
            }
            SetStatus("Checking for updates");

            // Get info on updates, and get updates since last:
            GetUpdatesList(ref _updatesAndChanges);
            if (_updatesAndChanges.Count == 0)
            {
                if (VerifyAndClose) // Verify and close only works if up to date
                {
                    new Thread(DoVerifyAndExit).Start();
                    return;
                }
                CreateVerifyAndExitButton();
            }
            else if (VerifyAndClose)
                WriteLine("To verify files you need to update first.");

            _updatesAndChanges = _updatesAndChanges.Reverse().ToDictionary(x => x.Key, x => x.Value);
        }

        private static string GetVersionFromGlobals()
        {
            // Try read version directly from Globals file.
            var dllPath = Path.Join(Directory.GetCurrentDirectory(), "TcNo-Acc-Switcher-Globals.dll");
            if (!File.Exists(dllPath)) return "";
            try
            {
                // Copy file because you can't unload once loaded:
                var tempDll = Path.Join(Directory.GetCurrentDirectory(), "temp.dll");
                File.Copy(dllPath, tempDll, true);
                var version = Assembly.LoadFrom(tempDll).GetType("TcNo_Acc_Switcher_Globals.Globals")?.GetField("Version")?.GetValue("Version");
                if (version is string s) return s;
                return "";
            }
            catch (Exception)
            {
                return "";
            }
        }

        private static Brush TryApplyTheme(IReadOnlyDictionary<string, string> dict, string def, string e)
        {
            try
            {
                return (Brush)new BrushConverter().ConvertFromString(dict[e]);
            }
            catch (Exception)
            {
                return (Brush)new BrushConverter().ConvertFromString(def);
            }
        }

        private static void GenerateHashes()
        {
            const string newFolder = "TcNo-Acc-Switcher";
            DirSearchWithHash(newFolder, ref _newDict);
            File.WriteAllText(Path.Join(newFolder, "hashes.json"), JsonConvert.SerializeObject(_newDict, Formatting.Indented));
        }

        private void DownloadCefNow()
        {
            SetStatusAndLog("Preparing to install Chrome Embedded Framework");
            SetStatusAndLog("Downloading CEF (~60MB)");
            var downloadUrl = "https://tcno.co/Projects/AccSwitcher/updates/CEF.7z";
            var updateFilePath = Path.Join(_updaterDirectory, "CEF.7z");
            DownloadFile(new Uri(downloadUrl), updateFilePath);
            SetStatusAndLog("Download complete.");

            SetStatusAndLog("Extracting...");
            var tempCef = Path.Join(_updaterDirectory, "../CEF");
            using (var archiveFile = new ArchiveFile(updateFilePath))
            {
                archiveFile.Extract(tempCef, true); // extract all to parent folder (not updater)
            }

            SetStatusAndLog("Moving files...");
            Directory.CreateDirectory("runtimes//win-x64//native");
            MoveFilesRecursive(tempCef, "runtimes//win-x64//native");
            RecursiveDelete(new DirectoryInfo(tempCef), false);
            SetStatusAndLog("Done.");

            SetStatusAndLog("Starting account switcher in 3 seconds.");
            Thread.Sleep(3000);
            LaunchAccSwitcher(null, null);
        }

        /// <summary>
        /// Downloads requested file to supplied destination, with progress bar
        /// </summary>
        /// <param name="uri">Download from</param>
        /// <param name="destination">Download to</param>
        public void DownloadFile(Uri uri, string destination)
        {
            using var wc = new WebClient();
            wc.DownloadProgressChanged += OnClientOnDownloadProgressChanged;
            wc.DownloadFileCompleted += HandleDownloadComplete;

            var syncObject = new object();
            lock (syncObject)
            {
                wc.DownloadFileAsync(uri, destination, syncObject);
                //This would block the thread until download completes
                _ = Monitor.Wait(syncObject);
            }
        }

        public static string AppDataFolder =>
            Directory.GetParent(Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location) ?? string.Empty)?.FullName;

        private static string _userDataFolder = "";
        public static string UserDataFolder
        {
            get
            {
                if (!string.IsNullOrEmpty(_userDataFolder)) return _userDataFolder;
                // Has not yet been initialised
                // Check if set to something different
                if (File.Exists(Path.Join(AppDataFolder, "userdata_path.txt")))
                    _userDataFolder = UGlobals.ReadAllLines(Path.Join(AppDataFolder, "userdata_path.txt"))[0].Trim();
                else
                    _userDataFolder = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "TcNo Account Switcher\\");

                return _userDataFolder;
            }
        }

        public static string MainAppDataFolder => Directory.GetParent(Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location)!)?.FullName!;
        // FOR TESTING: public static string MainAppDataFolder => "C:\\Program Files\\TcNo Account Switcher";
        public static string OriginalWwwroot => Path.Join(MainAppDataFolder, "originalwwwroot");

        /// <summary>
        /// Recursively copy files and directories
        /// </summary>
        /// <param name="inputFolder">Folder to copy files recursively from</param>
        /// <param name="outputFolder">Destination folder</param>
        public static void CopyFilesRecursive(string inputFolder, string outputFolder)
        {
            _ = Directory.CreateDirectory(outputFolder);
            if (!outputFolder.EndsWith("\\")) outputFolder += "\\";
            //Now Create all of the directories
            foreach (var dirPath in Directory.GetDirectories(inputFolder, "*", SearchOption.AllDirectories))
                _ = Directory.CreateDirectory(dirPath.Replace(inputFolder, outputFolder));

            //Copy all the files & Replaces any files with the same name
            foreach (var newPath in Directory.GetFiles(inputFolder, "*.*", SearchOption.AllDirectories))
                File.Copy(newPath, newPath.Replace(inputFolder, outputFolder), true);
        }

        /// <summary>
        /// Recursively copies directories from install dir to documents dir.
        /// </summary>
        /// <param name="f">Folder to recursively copy</param>
        /// <param name="overwrite">Whether files should be overwritten anyways</param>
        private static void InitFolder(string f, bool overwrite)
        {
            if (overwrite || !Directory.Exists(Path.Join(UserDataFolder, f))) CopyFilesRecursive(Path.Join(MainAppDataFolder, f), Path.Join(UserDataFolder, f));
        }

        private static void InitWwwroot(bool overwrite)
        {
            if (!overwrite && Directory.Exists(Path.Join(UserDataFolder, "wwwroot"))) return;
            if (Directory.Exists(OriginalWwwroot))
                CopyFilesRecursive(OriginalWwwroot, Path.Join(UserDataFolder, "wwwroot"));
        }

        /// <summary>
        /// For progress bar
        /// </summary>
        public void HandleDownloadComplete(object sender, AsyncCompletedEventArgs args)
        {
            Debug.Assert(args.UserState != null, "args.UserState != null");
            lock (args.UserState)
            {
                Monitor.Pulse(args.UserState);
            }
        }

        /// <summary>
        /// Starts and does the update process
        /// </summary>
        private void DoUpdate()
        { //var updaterDirectory = Path.GetDirectoryName(System.Reflection.Assembly.GetEntryAssembly()?.Location); // Where this program is located
          //Directory.SetCurrentDirectory(Directory.GetParent(updaterDirectory!)!.ToString()); // Set working directory to same as .exe
          //CreateUpdate();

            // 1. Download update.7z
            // 2. Extract --> Done, mostly
            // 3. Delete files that need to be removed. --> Done, mostly
            // 4. Run patches from working folder and patches --> Done, mostly
            // 4. Run patches from working folder and patches --> Done, mostly
            // 5. Copy in new files --> Done
            // 6. Delete working temp folder, and update --> Done
            // 7. Download latest hash list and verify
            // 8. Copy and replace files in Documents folder (New 2021-06-15)
            // --> Update the updater through the main program with a simple hash check there.
            // ----> Just re-download the whole thing, or make a copy of it to update itself with,
            // ----> then get the main program to replace the updater (Most likely).

            ButtonHandler(false, "Started");
            SetStatusAndLog("Closing TcNo Account Switcher instances (if any).");
            // Check if files are in use
            CloseIfRunning(_currentDir);

            if (Directory.Exists("wwwroot"))
            {
                if (Directory.Exists("originalwwwroot")) RecursiveDelete(new DirectoryInfo("originalwwwroot"), false);
                Directory.Move("wwwroot", "originalwwwroot");
            }

            // APPLY UPDATE
            // Cleanup previous updates
            if (Directory.Exists("temp_update")) Directory.Delete("temp_update", true);

            foreach (var (key, _) in _updatesAndChanges)
            {
                // This update .exe exists inside an "update" folder, in the TcNo Account Switcher directory.
                // Set the working folder as the parent to this one, where the main program files are located.
                SetStatusAndLog("Downloading update: " + key);
                var downloadUrl = "https://tcno.co/Projects/AccSwitcher/updates/" + key + ".7z";
                var updateFilePath = Path.Join(_updaterDirectory, key + ".7z");
                DownloadFile(new Uri(downloadUrl), updateFilePath);
                SetStatusAndLog("Download complete.");

                // Apply update
                try
                {
                    ApplyUpdate(updateFilePath);
                }
                catch (SevenZipException)
                {
                    // Skip straight to verification
                    WriteLine("Failed to extract update.");
                    WriteLine("Verifying files to skip all small updates, and just jump to latest.");
                    break;
                }
                catch (UnauthorizedAccessException)
                {
                    // Stop process and add verify button
                    string[] lines = {
                        "ERROR: Some of the files are in use and can not be updated.",
                        "",
                        "1. Make sure all TcNo processes are completely closed",
                        "(or restart your PC)",
                        "",
                        "2. Then click Verify below, or if you restarted:",
                        "-   Open this updater and verify files",
                        "    (located in the 'updater' folder in the install directory)"
                    };
                    foreach (var l in lines)
                        WriteLine(l);
                    _ = MessageBox.Show(string.Join(Environment.NewLine, lines), "Unable to check for updates", MessageBoxButton.OK, MessageBoxImage.Error);
                    SetStatus("Press button below!");
                    CreateVerifyAndExitButton();
                    return;
                }
                SetStatusAndLog("Patch applied.");
                WriteLine("");

                // Cleanup
                Directory.Delete("temp_update", true);
                File.Delete(updateFilePath);
                _oldDict = new Dictionary<string, string>();
                _newDict = new Dictionary<string, string>();
                _patchList = new List<string>();
            }

            VerifyFiles();

            if (File.Exists(_windowSettings))
            {
                var o = JObject.Parse(UGlobals.ReadAllText(_windowSettings));
                o["Version"] = _latestVersion;
                // Save all settings back into file
                File.WriteAllText(_windowSettings, o.ToString());
            }

            SetStatusAndLog("Updating files in AppData!");
            InitWwwroot(true);
            InitFolder("themes", true);

            WriteLine("");
            SetStatusAndLog("All updates complete!");
            ButtonHandler(true, "Start Acc Switcher");
            _ = Dispatcher.BeginInvoke(new Action(() =>
              {
                  StartButton.Click -= StartUpdate_Click;
                  StartButton.Click += LaunchAccSwitcher;
              }), DispatcherPriority.Normal);
        }

        /// <summary>
        /// Creates button to verify files and once finished, presents exit button
        /// </summary>
        private void CreateVerifyAndExitButton()
        {
            StartButton.Click -= StartUpdate_Click;
            StartButton.Click += VerifyAndExitButton_Click;
            ButtonHandler(true, "Verify files");
        }

        /// <summary>
        /// Click handler for verify and exit button. After verification, changes to exit button
        /// </summary>
        private void VerifyAndExitButton_Click(object sender = null, RoutedEventArgs e = null) => new Thread(DoVerifyAndExit).Start();
        private void DoVerifyAndExit()
        {
            VerifyFiles();
            CreateExitButton();
        }

        /// <summary>
        /// Verifies existing files. Download different or missing files
        /// </summary>
        private void VerifyFiles()
        {
            // Compare hash list to files, and download any files that don't match
            var client = new WebClient();
            client.DownloadProgressChanged -= OnClientOnDownloadProgressChanged;
            SetStatusAndLog("Verifying...");
            WriteLine("Downloading latest hash list... ");
            const string hashFilePath = "hashes.json";
            try
            {
                client.DownloadFile(new Uri("https://tcno.co/Projects/AccSwitcher/latest/hashes.json"), hashFilePath);
            }
            catch (WebException e)
            {
                _ = MessageBox.Show("Could not connect to tcno.co to verify files. You can try manually downloading the installer from the GitHub page, and reinstalling to update." + Environment.NewLine + Environment.NewLine + "Error: " + e, "Could not reach update server", MessageBoxButton.OK, MessageBoxImage.Error, MessageBoxResult.OK, MessageBoxOptions.DefaultDesktopOnly);
                Environment.Exit(1);
            }

            var verifyDictionary = JsonConvert.DeserializeObject<Dictionary<string, string>>(UGlobals.ReadAllText(hashFilePath));
            if (verifyDictionary != null)
            {
                var verifyDictTotal = verifyDictionary.Count;
                var cur = 0;
                UpdateProgress(0);
                foreach (var (oKey, value) in verifyDictionary)
                {
                    var key = oKey;
                    if (key.StartsWith("updater")) continue; // Ignore own files >> Otherwise IOException
                    cur++;
                    UpdateProgress(cur * 100 / verifyDictTotal);
                    if (!File.Exists(key))
                        WriteLine("FILE MISSING: " + key);
                    else
                    {
                        var md5 = GetFileMd5(key);
                        if (value == md5) continue;

                        WriteLine("File: " + key + " has MD5: " + md5 + " EXPECTED: " + value);
                        WriteLine("Deleting: " + key);
                        DeleteFile(key);
                    }
                    WriteLine("Downloading file from website...");
                    var uri = new Uri("https://tcno.co/Projects/AccSwitcher/latest/" + oKey.Replace('\\', '/'));

                    var path = Path.GetDirectoryName(key);
                    if (!string.IsNullOrEmpty(path))
                        _ = Directory.CreateDirectory(path);

                    client.DownloadFile(uri, key);
                }

                WriteLine(Environment.NewLine + "Copying files to %AppData%/TcNo Account Switcher... (unless set otherwise)");
                InitWwwroot(true); // Overwrite files in Documents
                WriteLine("Done.");
            }

            File.Delete(hashFilePath);
            WriteLine("Files verified!");
            UpdateProgress(100);
        }

        /// <summary>
        /// Progress bar handler
        /// </summary>
        private void OnClientOnDownloadProgressChanged(object o, DownloadProgressChangedEventArgs e)
        {
            UpdateProgress(e.ProgressPercentage);
        }


        /// <summary>
        /// Checks whether the program version is equal to or newer than the servers
        /// </summary>
        /// <param name="latest">Latest version provided by server</param>
        /// <returns>True when the program is up-to-date or ahead</returns>
        private bool CheckLatest(string latest)
        {
            if (!DateTime.TryParseExact(latest, "yyyy-MM-dd_mm", null, DateTimeStyles.None, out var latestDate))
                return false;
            if (!DateTime.TryParseExact(_currentVersion, "yyyy-MM-dd_mm", null, DateTimeStyles.None,
                out var currentDate)) return false;
            return latestDate.Equals(currentDate) || currentDate.Subtract(latestDate) > TimeSpan.Zero;
        }

        /// <summary>
        /// Gets list of updates from https://tcno.co
        /// </summary>
        /// <param name="updatesAndChanges"></param>
        private void GetUpdatesList(ref Dictionary<string, string> updatesAndChanges)
        {
            double totalFileSize = 0;

            var client = new WebClient();
            client.Headers.Add("Cache-Control", "no-cache");
#if DEBUG
            var versions = client.DownloadString(new Uri("https://tcno.co/Projects/AccSwitcher/api/update?debug&v=" + _currentVersion));
#else
            var versions = client.DownloadString(new Uri("https://tcno.co/Projects/AccSwitcher/api/update?v=" + _currentVersion));
#endif
            try
            {
                var jUpdates = JObject.Parse(versions)["updates"];
                Debug.Assert(jUpdates != null, nameof(jUpdates) + " != null");
                var firstChecked = false;
                foreach (var jToken in jUpdates)
                {
                    var jUpdate = (JProperty)jToken;
                    if (CheckLatest(jUpdate.Name)) break; // Get up to the current version
                    if (!firstChecked)
                    {
                        firstChecked = true;
                        _latestVersion = jUpdate.Name;
                        if (CheckLatest(_latestVersion)) // If up to date or newer
                            break;
                    }

                    var updateDetails = jUpdate.Value[0]!.ToString();
                    var updateSize = FileSizeString((double)jUpdate.Value[1]);
                    totalFileSize += (double)jUpdate.Value[1];

                    updatesAndChanges.Add(jUpdate.Name, jUpdate.Value.ToString());
                    WriteLine($"Update found: {jUpdate.Name} [{updateSize}]");
                    WriteLine("- " + updateDetails);
                    WriteLine("");
                }

                WriteLine("-------------------------------------------");
                WriteLine($"Total updates found: {updatesAndChanges.Count}");
                if (updatesAndChanges.Count > 0)
                {
                    WriteLine($"Total size: {FileSizeString(totalFileSize)}");
                    WriteLine("-------------------------------------------");
                    WriteLine("Click the button below to start the update.");
                    ButtonHandler(true, "Start update");
                    SetStatus("Waiting for user input...");
                }
                else
                {
                    WriteLine("-------------------------------------------");
                    WriteLine("You're up to date.");
                    SetStatus(":)");
                }
            }
            catch (JsonReaderException)
            {
                MessageBox.Show("This is most likely due to me (TechNobo) pushing an update, and making a typo that broke the .json update file. Do let me know :)", "Failed to decode update list", MessageBoxButton.OK, MessageBoxImage.Error);
            }
        }

        /// <summary>
        /// Converts file length to easily read string.
        /// </summary>
        /// <param name="len"></param>
        /// <returns></returns>
        private static string FileSizeString(double len)
        {
            if (len < 0.001) return "0 bytes";
            string[] sizes = { "B", "KB", "MB", "GB" };
            var n2 = (int)Math.Log10(len) / 3;
            var n3 = len / Math.Pow(1e3, n2);
            return $"{n3:0.##} {sizes[n2]}";
        }

        /// <summary>
        /// Closes TcNo Account Switcher's processes if any are running. (Searches program's folder recursively)
        /// </summary>
        /// <param name="currentDir"></param>
        private void CloseIfRunning(string currentDir)
        {
            foreach (var exe in Directory.GetFiles(currentDir, "*.exe", SearchOption.AllDirectories))
            {
                if (exe.Contains(Path.GetFileNameWithoutExtension(Assembly.GetEntryAssembly()?.Location)!)) continue;
                var eName = Path.GetFileNameWithoutExtension(exe);
                if (Process.GetProcessesByName(eName).Length <= 0) continue;
                KillProcess(eName);
            }
        }

        /// <summary>
        /// Used by TechNobo to create updates for the program.
        /// </summary>
        private static void CreateUpdate()
        {
            // CREATE UPDATE
            const string oldFolder = "OldVersion";
            const string newFolder = "TcNo-Acc-Switcher";
            const string outputFolder = "UpdateOutput";
            var deleteFileList = Path.Join(outputFolder, "filesToDelete.txt");
            RecursiveDelete(new DirectoryInfo(outputFolder), false);
            if (File.Exists(deleteFileList)) File.Delete(deleteFileList);

            List<string> filesToDelete = new(); // Simply ignore these later
            CreateFolderPatches(oldFolder, newFolder, outputFolder, ref filesToDelete);
            File.WriteAllLines(deleteFileList, filesToDelete);
            Environment.Exit(0);
        }

        /// <summary>
        /// Extract files from 7z, Apply patches, move files back into main folder & delete files that need to be removed.
        /// </summary>
        /// <param name="updateFilePath">Path to 7z file for extraction</param>
        private void ApplyUpdate(string updateFilePath)
        {
            var currentDir = Directory.GetCurrentDirectory();
            // Patches, Compression and the rest to be done manually by me.
            // update.7z or version.7z, something along those lines is downloaded
            // Decompressed to a test area
            _ = Directory.CreateDirectory("temp_update");
            SetStatusAndLog("Extracting patch files");
            using (var archiveFile = new ArchiveFile(updateFilePath))
            {
                archiveFile.Extract("temp_update", true); // extract all
            }
            SetStatusAndLog("Applying patch...");
            ApplyPatches(currentDir, "temp_update");

            // Remove files that need to be removed:
            var filesToDelete = UGlobals.ReadAllLines(Path.Join("temp_update", "filesToDelete.txt")).ToList();

            SetStatusAndLog("Moving files...");

            // Move updater files, if any. Will be moved to replace currently running process when main process launched.
            var newFolder = Path.Join("temp_update", "new");
            var patchedFolder = Path.Join("temp_update", "patched");

            // Move main files
            MoveFilesRecursive(newFolder, currentDir);
            MoveFilesRecursive(patchedFolder, currentDir);

            foreach (var f in filesToDelete)
            {
                File.Delete(f);
            }
        }

        /// <summary>
        /// Recursively move files and directories
        /// </summary>
        /// <param name="inputFolder">Folder to move files recursively from</param>
        /// <param name="outputFolder">Destination folder</param>
        private static void MoveFilesRecursive(string inputFolder, string outputFolder)
        {
            //Now Create all of the directories
            foreach (var dirPath in Directory.GetDirectories(inputFolder, "*", SearchOption.AllDirectories))
                _ = Directory.CreateDirectory(dirPath.Replace(inputFolder, outputFolder));

            //Move all the files & Replaces any files with the same name
            foreach (var newPath in Directory.GetFiles(inputFolder, "*.*", SearchOption.AllDirectories))
                File.Move(newPath, newPath.Replace(inputFolder, outputFolder), true);
        }

        /// <summary>
        /// Kills requested process. Will Write to Log and Console if unexpected output occurs (Doesn't start with "SUCCESS")
        /// </summary>
        /// <param name="procName">Process name to kill (Will be used as {name}*)</param>
        private void KillProcess(string procName)
        {
            var startInfo = new ProcessStartInfo
            {
                UseShellExecute = false,
                WindowStyle = ProcessWindowStyle.Hidden,
                FileName = "cmd.exe",
                Arguments = $"/C TASKKILL /F /T /IM {procName}*",
                CreateNoWindow = true,
                RedirectStandardInput = true,
                RedirectStandardOutput = true
            };
            var process = new Process { StartInfo = startInfo };
            _ = process.Start();
            process.BeginOutputReadLine();
            process.WaitForExit();

            WriteLine($"/C TASKKILL /F /T /IM {procName}*");
        }

        /// <summary>
        /// Recursively delete files in folders (Choose to keep or delete folders too)
        /// </summary>
        /// <param name="baseDir">Folder to start working inwards from (as DirectoryInfo)</param>
        /// <param name="keepFolders">Set to False to delete folders as well as files</param>
        private static void RecursiveDelete(DirectoryInfo baseDir, bool keepFolders)
        {
            if (!baseDir.Exists)
                return;

            foreach (var dir in baseDir.EnumerateDirectories())
            {
                RecursiveDelete(dir, keepFolders);
            }
            var files = baseDir.GetFiles();
            foreach (var file in files)
            {
                DeleteFile(fileInfo: file);
            }

            if (keepFolders) return;
            baseDir.Delete();
        }

        /// <summary>
        /// Deletes a single file
        /// </summary>
        /// <param name="file">(Optional) File string to delete</param>
        /// <param name="fileInfo">(Optional) FileInfo of file to delete</param>
        private static void DeleteFile(string file = "", FileInfo fileInfo = null)
        {
            var f = fileInfo ?? new FileInfo(file);

            try
            {
                if (!f.Exists) return;
                f.IsReadOnly = false;
                f.Delete();
            }
            catch (Exception)
            {
                // ignored
            }
        }

        /// <summary>
        /// Apply patches from outputFolder\\patches into old folder
        /// </summary>
        /// <param name="oldFolder">Old program files path</param>
        /// <param name="outputFolder">Path to folder with patches folder in it</param>
        private static void ApplyPatches(string oldFolder, string outputFolder)
        {
            DirSearch(Path.Join(outputFolder, "patches"), ref _patchList);
            foreach (var p in _patchList)
            {
                var relativePath = p.Remove(0, p.Split("\\")[0].Length + 1);
                var patchFile = Path.Join(outputFolder, "patches", relativePath);                // Necessary fix for pre 2021-06-28_01 versions to update using this new updater.
                if (relativePath.StartsWith("wwwroot"))
                    relativePath = relativePath.Replace("wwwroot", "originalwwwroot");
                var patchedFile = Path.Join(outputFolder, "patched", relativePath);
                _ = Directory.CreateDirectory(Path.GetDirectoryName(patchedFile)!);
                DoDecode(Path.Join(oldFolder, relativePath), patchFile, patchedFile);
            }
        }

        /// <summary>
        /// Create patches for input old and new folder, as well as list of deleted files
        /// </summary>
        /// <param name="oldFolder">Path of reference old version files</param>
        /// <param name="newFolder">Path of reference new version files</param>
        /// <param name="outputFolder">Path to output patch files, new files and list of files to delete</param>
        /// <param name="filesToDelete"></param>
        private static void CreateFolderPatches(string oldFolder, string newFolder, string outputFolder, ref List<string> filesToDelete)
        {
            _ = Directory.CreateDirectory(outputFolder);

            DirSearchWithHash(oldFolder, ref _oldDict);
            DirSearchWithHash(newFolder, ref _newDict);
            DirSearchWithHash(newFolder, ref _allNewDict, true);

            // -----------------------------------
            // SAVE DICT OF NEW FILES & HASHES
            // -----------------------------------
            File.WriteAllText(Path.Join(outputFolder, "hashes.json"), JsonConvert.SerializeObject(_allNewDict, Formatting.Indented));


            List<string> differentFiles = new();
            // Files left in newDict are completely new files.

            // COMPARE THE 2 FOLDERS
            foreach (var (key, value) in _oldDict) // Foreach entry in old dictionary:
            {
                if (_newDict.TryGetValue(key, out var sVal)) // If new dictionary has it, get it's value
                {
                    if (value != sVal) // Compare the values. If they don't match ==> FILES ARE DIFFERENT
                    {
                        differentFiles.Add(key);
                    }
                    _ = _oldDict.Remove(key); // Remove from both old
                    _ = _newDict.Remove(key); // and new dictionaries
                }
                else // New dictionary does NOT have this file
                {
                    filesToDelete.Add(key); // Add to list of files to delete
                    _ = _oldDict.Remove(key); // Remove from old dictionary. They have been added to delete list.
                }
            }

            // -----------------------------------
            // HANDLE NEW FILES
            // -----------------------------------
            // Copy new files into output\\new
            var outputNewFolder = Path.Join(outputFolder, "new");
            _ = Directory.CreateDirectory(outputNewFolder);
            foreach (var newFile in _newDict.Keys)
            {
                var newFileInput = Path.Join(newFolder, newFile);
                var newFileOutput = Path.Join(outputNewFolder, newFile);

                _ = Directory.CreateDirectory(Path.GetDirectoryName(newFileOutput)!);
                File.Copy(newFileInput, newFileOutput, true);
            }


            // -----------------------------------
            // HANDLE DIFFERENT FILES
            // -----------------------------------
            foreach (var differentFile in differentFiles)
            {
                var oldFileInput = Path.Join(oldFolder, differentFile);
                var newFileInput = Path.Join(newFolder, differentFile);
                var patchFileOutput = Path.Join(outputFolder, "patches", differentFile);
                _ = Directory.CreateDirectory(Path.GetDirectoryName(patchFileOutput)!);
                DoEncode(oldFileInput, newFileInput, patchFileOutput);
            }
        }

        /// <summary>
        /// Search through a directory for all files, and add files to dictionary
        /// </summary>
        /// <param name="sDir">Directory to recursively search</param>
        /// <param name="list">Dict of strings for files</param>
        private static void DirSearch(string sDir, ref List<string> list)
        {
            try
            {
                // Foreach file in directory
                list.AddRange(Directory.GetFiles(sDir).Select(f => f.Remove(0, f.Split("\\")[0].Length + 1)));

                // Foreach directory in file
                foreach (var d in Directory.GetDirectories(sDir))
                {
                    DirSearch(d, ref list);
                }
            }
            catch (Exception)
            {
                //
            }
        }

        /// <summary>
        /// Gets a file's MD5 Hash
        /// </summary>
        /// <param name="filePath">Path to file to get hash of</param>
        /// <returns></returns>
        private static string GetFileMd5(string filePath)
        {
            using var md5 = MD5.Create();
            using var stream = File.OpenRead(filePath);
            return stream.Length != 0 ? BitConverter.ToString(md5.ComputeHash(stream)).Replace("-", "").ToLowerInvariant() : "0";
        }

        /// <summary>
        /// Search through a directory for all files, and add files as well as hashes to dictionary
        /// </summary>
        /// <param name="sDir">Directory to recursively search</param>
        /// <param name="dict">Dict of strings for files and MD5 hashes</param>
        /// <param name="includeUpdater">Whether to include updater folder or not</param>
        private static void DirSearchWithHash(string sDir, ref Dictionary<string, string> dict, bool includeUpdater = false)
        {
            try
            {
                // Foreach file in directory
                foreach (var f in Directory.GetFiles(sDir))
                {
                    dict.Add(f.Remove(0, f.Split("\\")[0].Length + 1), GetFileMd5(f));
                }

                // Foreach directory in file
                foreach (var d in Directory.GetDirectories(sDir))
                {
                    if (!includeUpdater && d.EndsWith("updater")) continue; // Ignore updater folder, as this is verified in the main program now.
                    DirSearchWithHash(d, ref dict);
                }
            }
            catch (Exception)
            {
                //
            }
        }

        /// <summary>
        /// Encodes patch file from input old and input new files
        /// </summary>
        /// <param name="oldFile">Path to old file</param>
        /// <param name="newFile">Path to new file</param>
        /// <param name="patchFileOutput">Output of patch file (Differences encoded)</param>
        private static void DoEncode(string oldFile, string newFile, string patchFileOutput)
        {
            using FileStream output = new(patchFileOutput, FileMode.Create, FileAccess.Write);
            using FileStream dict = new(oldFile, FileMode.Open, FileAccess.Read);
            using FileStream target = new(newFile, FileMode.Open, FileAccess.Read);

            VcEncoder coder = new(dict, target, output);
            var result = coder.Encode(true, ChecksumFormat.SDCH); //encodes with no checksum and not interleaved
            if (result != VCDiffResult.SUCCESS)
            {
                //error was not able to encode properly
            }
        }

        /// <summary>
        /// Decodes patch file from input patch and input old file into new file
        /// </summary>
        /// <param name="oldFile">Path to old file</param>
        /// <param name="patchFile">Path to patch file (Differences encoded)</param>
        /// <param name="outputNewFile">Output of new file (that's been updated)</param>
        private static void DoDecode(string oldFile, string patchFile, string outputNewFile)
        {
            try
            {
                using FileStream dict = new(oldFile, FileMode.Open, FileAccess.Read);
                using FileStream target = new(patchFile, FileMode.Open, FileAccess.Read);
                using FileStream output = new(outputNewFile, FileMode.Create, FileAccess.Write);
                VcDecoder decoder = new(dict, target, output);
                // The header of the delta file must be available before the first call to decoder.Decode().
                var result = decoder.Decode(out _);
                if (result != VCDiffResult.SUCCESS)
                {
                    //error decoding
                }
                // if success bytesWritten will contain the number of bytes that were decoded
            }
            catch (FileNotFoundException)
            {
                //
            }
        }
    }



    /// <summary>
    /// Handles window buttons, resizing and dragging
    /// </summary>
    public class WindowHandling
    {
        public static void BtnMinimize(Window window)
        {
            window.WindowState = WindowState.Minimized;
        }
        public static void BtnExit(Window window)
        {
            window.Close();
        }
        public static void DragWindow(object _, MouseButtonEventArgs e, Window window)
        {
            if (e.LeftButton != MouseButtonState.Pressed) return;
            if (e.ClickCount == 2)
            {
                SwitchState(window);
            }
            else
            {
                if (window.WindowState == WindowState.Maximized)
                {
                    var percentHorizontal = e.GetPosition(window).X / window.ActualWidth;
                    var targetHorizontal = window.RestoreBounds.Width * percentHorizontal;

                    var percentVertical = e.GetPosition(window).Y / window.ActualHeight;
                    var targetVertical = window.RestoreBounds.Height * percentVertical;

                    window.WindowState = WindowState.Normal;

                    _ = GetCursorPos(out var lMousePosition);

                    window.Left = lMousePosition.X - targetHorizontal;
                    window.Top = lMousePosition.Y - targetVertical;
                }


                window.DragMove();
            }
        }
        [DllImport("user32.dll")]
        [return: MarshalAs(UnmanagedType.Bool)]
        private static extern bool GetCursorPos(out Point lpPoint);


        [StructLayout(LayoutKind.Sequential)]
        public struct Point
        {
            public int X;
            public int Y;
        }
        public static void SwitchState(Window window)
        {
            window.WindowState = window.WindowState switch
            {
                WindowState.Normal => WindowState.Maximized,
                WindowState.Maximized => WindowState.Normal,
                _ => window.WindowState
            };
        }
    }
}
