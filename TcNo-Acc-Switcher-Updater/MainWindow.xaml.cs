// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2024 TroubleChute (Wesley Pyburn)
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
using System.Runtime.ConstrainedExecution;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Security.Principal;
using System.Threading;
using System.Windows;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Threading;
using Microsoft.Win32;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using SevenZipExtractor;
using VCDiff.Decoders;
using VCDiff.Encoders;
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
        private const string MinimumVc = "14.30.30704";
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
        private string _mainBrowser = "WebView";
        private string _latestVersion = "";


        #region WINDOW_BUTTONS

        public MainWindow() => InitializeComponent();
        private void StartUpdate_Click(object sender, RoutedEventArgs e) => new Thread(DoUpdate).Start();

        private void LaunchAccSwitcher(object sender, RoutedEventArgs e)
        {
            try
            {
                _ = Process.Start(new ProcessStartInfo(@"TcNo-Acc-Switcher.exe") { UseShellExecute = true });
                Process.GetCurrentProcess().Kill();
            }
            catch (Win32Exception we)
            {
                MessageBox.Show(
                    $"An error occurred while trying to start. Patching may have failed. Try verify using this updater, or re-download a fresh copy from the TcNo Account Switcher GitHub.{Environment.NewLine}{we}",
                    "Could not start TcNo Account Switcher", MessageBoxButton.OK, MessageBoxImage.Error);


                SetStatus("An error occurred while trying to start. Patching may have failed. Try verify using this updater, or re-download a fresh copy from the TcNo Account Switcher GitHub.");
                CreateVerifyAndExitButton();
            }
        }

        private static void ExitProgram(object sender, RoutedEventArgs e) {
            Logger.WriteLine("ExitProgram");
            Environment.Exit(0);
        }

        // Window interaction
        private void BtnExit(object sender, RoutedEventArgs e) => WindowHandling.BtnExit(this);
        private void BtnMinimize(object sender, RoutedEventArgs e) => WindowHandling.BtnMinimize(this);
        private void DragWindow(object sender, MouseButtonEventArgs e) => WindowHandling.DragWindow(sender, e, this);
        private void Window_Closed(object sender, EventArgs e)
        {
            Logger.WriteLine("Window_Closed: Exiting");
            Environment.Exit(0);
        }

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

            Console.WriteLine(line);
            Logger.WriteLine(line);
        }

        private void SetStatus(string s)
        {
            _ = Dispatcher.BeginInvoke(new Action(() =>
            {
                StatusLabel.Content = s;
            }), DispatcherPriority.Normal);

            Logger.WriteLine($"Status: {s}");
        }

        private void SetStatusAndLog(string s, string lineBreak = "\n")
        {
            _ = Dispatcher.BeginInvoke(new Action(() =>
            {
                StatusLabel.Content = s;
                LogBox.Text += lineBreak + s;
            }), DispatcherPriority.Normal);

            _ = Dispatcher.BeginInvoke(new Action(() =>
            {
                LogBox.ScrollToEnd();
            }), DispatcherPriority.Normal);

            Logger.WriteLine($"Status & Log: {s}");
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
        private string latestAvailable = "";

        private readonly string
            _updaterDirectory =
                Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location); // Where this program is located

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
                using var fs = File.Create(Path.Combine(folderPath, Path.GetRandomFileName()), 1,
                    FileOptions.DeleteOnClose);
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
            return securityIdentifier is not null &&
                   securityIdentifier.IsWellKnown(WellKnownSidType.BuiltinAdministratorsSid);
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
                Logger.WriteLine("RestartAsAdmin: Exiting");
                Environment.Exit(0);
            }
            catch (Exception ex)
            {
                _ = MessageBox.Show($"This program must be run as an administrator!{Environment.NewLine}{ex}");
                Logger.WriteLine("RestartAsAdmin (Failed): Exiting");
                Environment.Exit(0);
            }
        }

        /// <summary>
        /// Move pre 2021-07-05 Documents userdata folder into AppData.
        /// </summary>
        private void OldDocumentsToAppData()
        {
            // Check to see if folder still located in My Documents (from Pre 2021-07-05)
            var oldDocuments = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.MyDocuments),
                "TcNo Account Switcher\\");
            if (Directory.Exists(oldDocuments))
                CopyFilesRecursive(oldDocuments, UserDataFolder); // Overwrites by default
            RecursiveDelete(oldDocuments, false);
        }

        private void Init()
        {
            if (QueueCreateUpdate)
            {
                CreateUpdate();
                Logger.WriteLine("CreateUpdate Done: Exiting");
                Environment.Exit(0);
            }

            if (!Directory.Exists(UserDataFolder))
            {
                MessageBox.Show("Please run the main program at least once, with 'TcNo_Acc_Switcher.exe'", "Error", MessageBoxButton.OK, MessageBoxImage.Warning);
                Logger.WriteLine("UserDataFolder doesn't exist: Exiting");
                Environment.Exit(1);
            }

            // Check if installed to program files, and requires admin to run:
            if (InstalledToProgramFiles() && !IsAdmin() ||
                !HasFolderAccess(Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location)))
                RestartAsAdmin("");

            Directory.SetCurrentDirectory(MainAppDataFolder);
            OldDocumentsToAppData();
            if (QueueHashList)
            {
                GenerateHashes();
                Logger.WriteLine("GenerateHashes (Done): Exiting");
                Environment.Exit(0);
            }

            if (DownloadCef)
            {
                DownloadCefNow();
                Logger.WriteLine("DownloadCefNow (Done): Exiting");
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

            try
            {
                _mainBrowser = JObject.Parse(UGlobals.ReadAllText(_windowSettings))["ActiveBrowser"]?.ToString();
            }
            catch (Exception)
            {
                //
            }

            // If couldn't: Verify instead of updating (Done later).
            if (string.IsNullOrEmpty(_currentVersion))
                WriteLine("Could not get version. Click \"Verify\" to update & verify files" + Environment.NewLine);

            // Styling
            Directory.SetCurrentDirectory(UserDataFolder);
            try
            {
                var o = JObject.Parse(UGlobals.ReadAllText(_windowSettings));
                var themeName = o["ActiveTheme"]?.ToString();
                var themeFile = Path.Join("themes", themeName, "info.yaml");

                // Separated so colors aren't only half changed, and a color from the file is missing
                var desc = new DeserializerBuilder().WithNamingConvention(HyphenatedNamingConvention.Instance).Build();
                if (File.Exists(themeFile))
                {
                    var dict = JsonConvert.DeserializeObject<Dictionary<string, string>>(
                        JsonConvert.SerializeObject(desc.Deserialize<object>(UGlobals.ReadAllText(themeFile))));
                    _ = Dispatcher.BeginInvoke(new Action(() =>
                    {
                        // Need to try catch every one of these, as they may be invalid.
                        var highlightColor =
                            TryApplyTheme(dict, "#8BE9FD", "linkColor"); // Add a specific Highlight color later?
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
                        var windowControlsBackgroundActive =
                            TryApplyTheme(dict, "#3B4853", "windowControlsBackground-active");
                        var windowControlsCloseBackground =
                            TryApplyTheme(dict, "#D51426", "windowControlsCloseBackground");
                        var windowControlsCloseBackgroundActive =
                            TryApplyTheme(dict, "#D51426", "windowControlsCloseBackground-active");

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

            if (VerifyAndClose) // Verify and close only works if up to date
            {
                new Thread(DoVerifyAndExit).Start();
                return;
            }

            // Get info on updates, and get updates since last:
            GetUpdatesList(ref _updatesAndChanges);

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
                var version = Assembly.LoadFrom(tempDll).GetType("TcNo_Acc_Switcher_Globals.Globals")
                    ?.GetField("Version")?.GetValue("Version");
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
                return (Brush) new BrushConverter().ConvertFromString(dict[e]);
            }
            catch (Exception)
            {
                return (Brush) new BrushConverter().ConvertFromString(def);
            }
        }

        private static void GenerateHashes()
        {
            const string newFolder = "TcNo-Acc-Switcher";
            DirSearchWithHash(newFolder, ref _newDict);
            File.WriteAllText(Path.Join(newFolder, "hashes.json"),
                JsonConvert.SerializeObject(_newDict, Formatting.Indented));
        }

        /// <summary>
        /// Returns whether v2 is newer than, or equal to v1.
        /// Works with unequal string sizes.
        /// </summary>
        static bool compare_versions(string v1, string v2, string delimiter)
        {
            if (v1 == v2) return true;

            var v1Arr = Array.ConvertAll(v1.Split(delimiter), int.Parse);
            var v2Arr = Array.ConvertAll(v2.Split(delimiter), int.Parse);

            for (var i = 0; i < Math.Min(v1Arr.Length, v2Arr.Length); ++i)
            {
                if (v2Arr[i] < v1Arr[i]) return false;
                if (v2Arr[i] > v1Arr[i]) return true;
            }

            return true;
        }

        public static bool IsVcRuntimeInstalled()
        {
            try
            {
                using var vcKey = Registry.LocalMachine.OpenSubKey("SOFTWARE\\Microsoft\\DevDiv\\VC\\Servicing\\14.0\\RuntimeMinimum");
                if (vcKey == null) return false;
                var version = vcKey.GetValue("Version")?.ToString();
                return !string.IsNullOrEmpty(version) && compare_versions(MinimumVc, version, ".");
            }
            catch (Exception)
            {
                return false;
            }
        }

        private void DownloadCefNow()
        {
            // Check if VCRuntime 2015-2022 installed or not. If not, download and install.
            if (!IsVcRuntimeInstalled())
            {
                SetStatusAndLog("Downloading and installing Visual C++ Runtime 2015-2022");
                var proc = new Process
                {
                    StartInfo = new ProcessStartInfo
                    {
                        FileName = @"_First_Run_Installer.exe",
                        UseShellExecute = true,
                        Arguments = "vc"
                    }
                };
                proc.Start();
                proc.WaitForExit();
            }

            // Download CEF files from tcno.co
            SetStatusAndLog("Preparing to install Chrome Embedded Framework");
            SetStatus("Checking latest version number");
            var client = new WebClient();
#if DEBUG
            latestAvailable = client.DownloadString(new Uri("https://tcno.co/Projects/AccSwitcher/api?debug&v=" +
                                              _currentVersion));
#else
             latestAvailable = client.DownloadString(new Uri("https://tcno.co/Projects/AccSwitcher/api?v=" +
                                                          _currentVersion));
#endif
            DoUpdate(cef: true);
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
                    _userDataFolder = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
                        "TcNo Account Switcher\\");

                return _userDataFolder;
            }
        }

        public static string MainAppDataFolder =>
            Directory.GetParent(Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location)!)?.FullName!;

        // FOR TESTING: public static string MainAppDataFolder => "C:\\Program Files\\TcNo Account Switcher";
        public static string OriginalWwwroot => Path.Join(MainAppDataFolder, "originalwwwroot");

        /// <summary>
        /// Recursively copy files and directories
        /// </summary>
        /// <param name="inputFolder">Folder to copy files recursively from</param>
        /// <param name="outputFolder">Destination folder</param>
        public bool CopyFilesRecursive(string inputFolder, string outputFolder)
        {
            if (Directory.Exists(inputFolder)) return false;
            try
            {
                _ = Directory.CreateDirectory(outputFolder);
                if (!outputFolder.EndsWith("\\")) outputFolder += "\\";
                //Now Create all of the directories
                if (inputFolder == null)
                {
                    WriteLine("Failed to copy files. Input folder was null.");
                    return true;
                }
                foreach (var dirPath in Directory.GetDirectories(inputFolder, "*", SearchOption.AllDirectories))
                    _ = Directory.CreateDirectory(dirPath.Replace(inputFolder, outputFolder));

                //Copy all the files & Replaces any files with the same name
                foreach (var newPath in Directory.GetFiles(inputFolder, "*.*", SearchOption.AllDirectories))
                    File.Copy(newPath, newPath.Replace(inputFolder, outputFolder), true);

                return true;
            }
            catch (Exception ex)
            {
                WriteLine("Failed to copy files. Make sure you have permission to access the files, and they are not in use.");
                WriteLine("----------");
                WriteLine(ex.ToString());
                return false;
            }
        }

        /// <summary>
        /// Recursively copies directories from install dir to documents dir.
        /// </summary>
        /// <param name="f">Folder to recursively copy</param>
        /// <param name="overwrite">Whether files should be overwritten anyways</param>
        private void InitFolder(string f, bool overwrite)
        {
            if (overwrite || !Directory.Exists(Path.Join(UserDataFolder, f)))
                CopyFilesRecursive(Path.Join(MainAppDataFolder, f), Path.Join(UserDataFolder, f));
        }

        private void InitWwwroot(bool overwrite)
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

        private void DoUpdate() => DoUpdate(false);

        /// <summary>
        /// Starts and does the update process
        /// </summary>
        private void DoUpdate(bool cef = false)
        {
            ButtonHandler(false, "Started");
            SetStatusAndLog("Closing TcNo Account Switcher instances (if any).");

            // Check if files are in use
            var CheckFolder = Directory.GetParent(Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location) ?? string.Empty)?.FullName;
            WriteLine($"Exiting all .exe's in {CheckFolder}");
            CloseIfRunning(CheckFolder);

            if (Directory.Exists("wwwroot"))
            {
                if (Directory.Exists("originalwwwroot")) RecursiveDelete("originalwwwroot", false);
                Directory.Move("wwwroot", "originalwwwroot");
            }

            // Cleanup previous updates
            RecursiveDelete("temp_update", false);

            // This updater doesn't download and run the installer, this just updates in-place. This method works properly for both the portable and installed version.
            // Get latest version from _updatesAndChanges[0]'s key
            _latestVersion = latestAvailable;
            if (cef) SetStatusAndLog($"Downloading update: {_latestVersion} + CEF (~110MB)");
            else SetStatusAndLog($"Downloading update: {_latestVersion} (~40MB)");

            var updateFilePath = "";
            var currentDir = "";
            try
            {
                var downloadUrl = "";
                if (cef) downloadUrl = $"https://github.com/TCNOco/TcNo-Acc-Switcher/releases/download/{_latestVersion}/TcNo-Acc-Switcher_and_CEF_{_latestVersion}.7z";
                else downloadUrl = $"https://github.com/TCNOco/TcNo-Acc-Switcher/releases/download/{_latestVersion}/TcNo-Acc-Switcher_{_latestVersion}.7z";
                WriteLine("Downloading from: " + downloadUrl);
                updateFilePath = Path.Join(_updaterDirectory, _latestVersion + ".7z");
                if (File.Exists(updateFilePath)) File.Delete(updateFilePath);

                DownloadFile(new Uri(downloadUrl), updateFilePath);
                SetStatusAndLog("Download complete.");

                // Apply the update.
                currentDir = Directory.GetCurrentDirectory();
                if (Directory.Exists("temp_update")) Directory.Delete("temp_update", true);

                _ = Directory.CreateDirectory("temp_update");
                SetStatusAndLog("Extracting patch files");
                using (var archiveFile = new ArchiveFile(updateFilePath))
                {
                    archiveFile.Extract("temp_update", true); // extract all
                }
                if (File.Exists("UpdateFinalizeLog.txt")) File.Delete("UpdateFinalizeLog.txt");
            } catch (Exception e)
            {
                SetStatusAndLog("Error!");
                WriteLine("There was an error with either downloading the latest version, or extracting.");
                WriteLine("If the update JUST came out then please wait ~an hour before trying again.");
                WriteLine("Should issues persist please contact TroubleChute with this info.");
                return;
            }

            SetStatusAndLog("Applying update. Window will close...");

            File.Delete(updateFilePath);

            // Copy _First_Run_Installer.exe to a temporary folder in %temp%.
            var tempFolder = Path.Combine(Path.GetTempPath(), "TcNo-Acc-Switcher");
            var tempFolderInstaller = Path.Combine(tempFolder, "_First_Run_Installer.exe");
            Directory.CreateDirectory(tempFolder);
            File.Copy(Path.Combine(currentDir, "updater", "_First_Run_Installer.exe"), tempFolderInstaller, true);

            // Run this temp folder with the temp_update full path as arg 1 and the current directory as arg 2
            Process.Start(new ProcessStartInfo
            {
                FileName = tempFolderInstaller,
                Arguments = $"finalizeupdate \"{Path.Combine(currentDir, "temp_update")}\" \"{currentDir}\"",
                UseShellExecute = true
            });

            // Exit
            Environment.Exit(1);
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
        private void VerifyAndExitButton_Click(object sender = null, RoutedEventArgs e = null) =>
            new Thread(DoVerifyAndExit).Start();

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
            var client = new WebClient();
#if DEBUG
            latestAvailable = client.DownloadString(new Uri("https://tcno.co/Projects/AccSwitcher/api?debug&v=" +
                                              _currentVersion));
#else
             latestAvailable = client.DownloadString(new Uri("https://tcno.co/Projects/AccSwitcher/api?v=" +
                                                          _currentVersion));
#endif
            // Find if using CEF from settings file

            var usingCef = usingCEF();
            DoUpdate(cef: usingCef);
        }
        private static bool usingCEF()
        {
            var path = File.Exists(Path.Join(AppDataFolder, "userdata_path.txt"))
                ? UGlobals.ReadAllLines(Path.Join(AppDataFolder, "userdata_path.txt"))[0].Trim()
                : Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "TcNo Account Switcher\\");

            var windowSettingsPath = Path.Join(path, "WindowSettings.json");
            try
            {
                if (File.Exists(windowSettingsPath))
                {
                    var o = JObject.Parse(UGlobals.ReadAllText(windowSettingsPath));
                    return o["ActiveBrowser"]?.ToString().ToLowerInvariant() != "webview";
                }
            }
            catch (Exception)
            {
                return true;
            }

            return true;
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
            var client = new WebClient();
            client.Headers.Add("Cache-Control", "no-cache");
#if DEBUG
            var versions =
                client.DownloadString(new Uri("https://tcno.co/Projects/AccSwitcher/api/update?debug&v=" +
                                              _currentVersion));
            latestAvailable = client.DownloadString(new Uri("https://tcno.co/Projects/AccSwitcher/api?debug&v=" +
                                              _currentVersion));
#else
            var versions = client.DownloadString(new Uri("https://tcno.co/Projects/AccSwitcher/api/update?v=" + _currentVersion));
             latestAvailable = client.DownloadString(new Uri("https://tcno.co/Projects/AccSwitcher/api?v=" +
                                                          _currentVersion));
#endif
            try
            {
                var jUpdates = JObject.Parse(versions)["updates"];
                Debug.Assert(jUpdates != null, nameof(jUpdates) + " != null");
                var firstChecked = false;
                foreach (var jToken in jUpdates)
                {
                    var jUpdate = (JProperty) jToken;
                    if (CheckLatest(jUpdate.Name)) break; // Get up to the current version
                    if (!firstChecked)
                    {
                        firstChecked = true;
                        _latestVersion = jUpdate.Name;
                        if (CheckLatest(_latestVersion)) // If up to date or newer
                            break;
                    }

                    var updateDetails = jUpdate.Value[0]!.ToString();

                    updatesAndChanges.Add(jUpdate.Name, jUpdate.Value.ToString());
                    WriteLine($"Update found: {jUpdate.Name}");
                    WriteLine("- " + updateDetails);
                    WriteLine("");
                }

                WriteLine("-------------------------------------------");
                WriteLine($"Total updates found: {updatesAndChanges.Count}");
                if (updatesAndChanges.Count > 0)
                {
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
                MessageBox.Show(
                    "This is most likely due to me (TroubleChute) pushing an update, and making a typo that broke the .json update file. Do let me know :)",
                    "Failed to decode update list", MessageBoxButton.OK, MessageBoxImage.Error);
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
            string[] sizes = {"B", "KB", "MB", "GB"};
            var n2 = (int) Math.Log10(len) / 3;
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
                try
                {
                    if (Process.GetProcessesByName(exe).Length <= 0 || exe == "TcNo-Acc-Switcher-Updater.exe" || exe == "TcNo-Acc-Switcher-Updater_main.exe") continue;
                    Logger.WriteLine($"Exiting: {exe}");
                    KillProcess(exe);
                } catch(Exception e)
                {
                    Logger.WriteLine($"Failed to exit: ${exe}\n{e}");
                }
            }
        }

        /// <summary>
        /// Used by TroubleChute to create updates for the program.
        /// </summary>
        private void CreateUpdate()
        {
            // Set working directory as exe directory
            Directory.SetCurrentDirectory(Directory.GetParent(Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location) ?? string.Empty)!.ToString());

            WriteLine("Creating update..." + Environment.NewLine);
            // CREATE UPDATE
            const string oldFolder = "OldVersion";
            const string newFolder = "TcNo-Acc-Switcher";
            const string outputFolder = "UpdateOutput";
            // Create if doesn't exist
            _ = Directory.CreateDirectory(outputFolder);

            var deleteFileList = Path.Join(outputFolder, "filesToDelete.txt");
            RecursiveDelete(outputFolder, false);
            WriteLine("RecursiveDelete..." + Environment.NewLine);
            DeleteFile(deleteFileList);
            WriteLine("deleteFileList..." + Environment.NewLine);

            List<string> filesToDelete = new(); // Simply ignore these later
            WriteLine("CreateFolderPatches..." + Environment.NewLine);
            CreateFolderPatches(oldFolder, newFolder, outputFolder, ref filesToDelete);
            File.WriteAllLines(deleteFileList, filesToDelete);
            WriteLine("Done!" + Environment.NewLine);
            Logger.WriteLine($"CreateUpdate complete: Exiting");
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
                DeleteFile(f);
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
            var process = new Process {StartInfo = startInfo};
            _ = process.Start();
            process.BeginOutputReadLine();
            process.WaitForExit();

            WriteLine($"/C TASKKILL /F /T /IM {procName}*");
        }


        private static void RecursiveDelete(string baseDir, bool keepFolders) =>
            RecursiveDelete(new DirectoryInfo(baseDir), keepFolders);

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
        private void ApplyPatches(string oldFolder, string outputFolder)
        {
            DirSearch(Path.Join(outputFolder, "patches"), ref _patchList);
            foreach (var p in _patchList)
            {
                var relativePath = p.Remove(0, p.Split("\\")[0].Length + 1);
                var patchFile =
                    Path.Join(outputFolder, "patches",
                        relativePath); // Necessary fix for pre 2021-06-28_01 versions to update using this new updater.
                if (relativePath.StartsWith("wwwroot"))
                    relativePath = relativePath.Replace("wwwroot", "originalwwwroot");

                // Check if part of CEF, and skip if CEF not selected:
                if (relativePath.StartsWith("runtimes") && IsCefFile(p))
                {
                    if (!File.Exists(p)) {
                        File.Create(p.Split("patches\\")[1]).Dispose(); // Create empty file
                        continue;
                    }
                    if (_mainBrowser != "CEF")
                        continue;
                }

                var patchedFile = Path.Join(outputFolder, "patched", relativePath);
                _ = Directory.CreateDirectory(Path.GetDirectoryName(patchedFile)!);
                if (!File.Exists(Path.Join(oldFolder, relativePath))) continue;
                DoDecode(Path.Join(oldFolder, relativePath), patchFile, patchedFile);
            }
        }

        private static readonly string[] CefFiles = {
            "libcef.dll", "icudtl.dat", "resources.pak", "libGLESv2.dll", "d3dcompiler_47.dll", "vk_swiftshader.dll", "chrome_elf.dll", "CefSharp.BrowserSubprocess.Core.dll"
        };
        private static bool IsCefFile(string file) => CefFiles.Any(file.Contains);

        /// <summary>
        /// Create patches for input old and new folder, as well as list of deleted files
        /// </summary>
        /// <param name="oldFolder">Path of reference old version files</param>
        /// <param name="newFolder">Path of reference new version files</param>
        /// <param name="outputFolder">Path to output patch files, new files and list of files to delete</param>
        /// <param name="filesToDelete"></param>
        private void CreateFolderPatches(string oldFolder, string newFolder, string outputFolder, ref List<string> filesToDelete)
        {
            _ = Directory.CreateDirectory(outputFolder);

            DirSearchWithHash(oldFolder, ref _oldDict);
            DirSearchWithHash(newFolder, ref _newDict);
            DirSearchWithHash(newFolder, ref _allNewDict, true);

            // -----------------------------------
            // SAVE DICT OF NEW FILES & HASHES
            // -----------------------------------
            WriteLine("SAVE DICT OF NEW FILES & HASHES...");
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
            WriteLine("HANDLE NEW FILES...");
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
            WriteLine("HANDLE DIFFERENT FILES..." + Environment.NewLine);
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
        public static void DragWindow(object o, MouseButtonEventArgs e, Window window)
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

                    GetCursorPos(out var lMousePosition);

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
