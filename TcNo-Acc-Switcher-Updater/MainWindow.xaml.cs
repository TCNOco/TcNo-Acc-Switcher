using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Net;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
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
using System.Windows.Navigation;
using System.Windows.Threading;
using System.Xml;
using Microsoft.VisualBasic.CompilerServices;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using SevenZipExtractor;
using VCDiff.Encoders;
using VCDiff.Decoders;
using VCDiff.Includes;
using VCDiff.Shared;


namespace TcNo_Acc_Switcher_Updater
{

    /// <summary>
    /// Interaction logic for MainWindow.xaml
    /// </summary>
    public partial class MainWindow : Window
    {
        // Dictionaries of file paths, as well as MD5 Hashes
        private static Dictionary<string, string> oldDict = new();
        private static Dictionary<string, string> newDict = new();
        private static List<string> patchList = new();


        const string CurrentVersion = "2021-03-12";


        #region WINDOW_BUTTONS
        public MainWindow()
        {
            InitializeComponent();
        }
        private void StartUpdate_Click(object sender, RoutedEventArgs e)
        {
            new Thread(DoUpdate).Start();
        }

        private void LaunchAccSwitcher(object sender, RoutedEventArgs e)
        {
            Process.Start("TcNo-Acc-Switcher.exe");
            Environment.Exit(0);
        }
        // Window interaction
        private void BtnExit(object sender, RoutedEventArgs e)
        {
            WindowHandling.BtnExit(sender, e, this);
        }
        private void BtnMinimize(object sender, RoutedEventArgs e)
        {
            WindowHandling.BtnMinimize(sender, e, this);
        }
        private void DragWindow(object sender, MouseButtonEventArgs e)
        {
            WindowHandling.DragWindow(sender, e, this);
        }
        private void Window_Closed(object sender, EventArgs e)
        {
            Environment.Exit(1);
        }
        #endregion
        
        private void WriteLine(string line)
        {
            Dispatcher.BeginInvoke(new Action(() => {
                Debug.WriteLine(line);
                LogBox.Text += "\n" + line;
                LogBox.ScrollToEnd();
            }), DispatcherPriority.Normal);
        }
        private void SetStatus(string s)
        {
            Dispatcher.BeginInvoke(new Action(() => {
                Debug.WriteLine("Status: " + s);
                StatusLabel.Content = s;
            }), DispatcherPriority.Normal);
        }
        private void SetStatusAndLog(string s)
        {
            Dispatcher.BeginInvoke(new Action(() => {
                Debug.WriteLine("Status/Log: " + s);
                StatusLabel.Content = s;
                LogBox.Text += "\n" + s;
                LogBox.ScrollToEnd();
            }), DispatcherPriority.Normal);
        }

        private void ButtonHandler(bool enabled, string content)
        {
            Dispatcher.BeginInvoke(new Action(() => {
                StartButton.IsEnabled = enabled;
                StartButton.Content = content;
            }), DispatcherPriority.Normal);
        }

        private void UpdateProgress(int percent)
        {
            Dispatcher.BeginInvoke(new Action(() => {
                ProgressBar.Value = percent;
            }), DispatcherPriority.Normal);
        }

        private Dictionary<string, string> updatesAndChanges = new Dictionary<string, string>();
        private string currentDir = Directory.GetCurrentDirectory();
        private string updaterDirectory = Path.GetDirectoryName(System.Reflection.Assembly.GetEntryAssembly()?.Location); // Where this program is located

        private void MainWindow_OnContentRendered(object? sender, EventArgs e)
        {
            new Thread(Init).Start();
        }

        private void Init()
        {
            ButtonHandler(false, "...");
            SetStatus("Checking for updates");
            Directory.SetCurrentDirectory(Directory.GetParent(updaterDirectory!)!.ToString()); // Set working directory to same as .exe

            // Get info on updates, and get updates since last:
            GetUpdatesList(ref updatesAndChanges);
            if (updatesAndChanges.Count == 0)
            {
                Debug.WriteLine("No updates found!");
                Debug.WriteLine("Press any key to exit...");
                Console.ReadLine();
                Environment.Exit(0);
            }
            updatesAndChanges = updatesAndChanges.Reverse().ToDictionary(x => x.Key, x => x.Value);
        }

        public void DownloadFile(Uri uri, string desintaion)
        {
            using (var wc = new WebClient())
            {
                wc.DownloadProgressChanged += OnClientOnDownloadProgressChanged;
                wc.DownloadFileCompleted += HandleDownloadComplete;

                var syncObject = new Object();
                lock (syncObject)
                {
                    wc.DownloadFileAsync(uri, desintaion, syncObject);
                    //This would block the thread until download completes
                    Monitor.Wait(syncObject);
                }
            }

        }
        public void HandleDownloadComplete(object sender, AsyncCompletedEventArgs args)
        {
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
            // --> Update the updater through the main program with a simple hash check there.
            // ----> Just re-download the whole thing, or make a copy of it to update itself with,
            // ----> then get the main program to replace the updater (Most likely).

            ButtonHandler(false, "Started");
            SetStatusAndLog("Closing TcNo Account Switcher instances (if any).");
            // Check if files are in use
            CloseIfRunning(currentDir);

            // APPLY UPDATE
            // Cleanup previous updates
            if (Directory.Exists("temp_update")) Directory.Delete("temp_update", true);

            
            foreach (var kvp in updatesAndChanges)
            {
                // This update .exe exists inside an "update" folder, in the TcNo Account Switcher directory.
                // Set the working folder as the parent to this one, where the main program files are located.
                SetStatusAndLog("Downloading update: " + kvp.Key);
                var downloadUrl = "https://tcno.co/Projects/AccSwitcher/updates/" + kvp.Key + ".7z";
                var updateFilePath = Path.Combine(updaterDirectory, kvp.Key + ".7z");
                DownloadFile(new Uri(downloadUrl), updateFilePath);
                SetStatusAndLog("Download complete.");

                // Apply update
                ApplyUpdate(updateFilePath);
                SetStatusAndLog("Patch applied.");
                SetStatusAndLog("");

                // Cleanup
                Directory.Delete("temp_update", true);
                File.Delete(updateFilePath);
                oldDict = new Dictionary<string, string>();
                newDict = new Dictionary<string, string>();
                patchList = new List<string>();
            }


            // Compare hash list to files, and download any files that don't match
            var client = new WebClient();
            client.DownloadProgressChanged -= OnClientOnDownloadProgressChanged;
            Debug.WriteLine("--- VERIFYING ---");
            SetStatusAndLog("Verifying...");
            WriteLine("Downloading latest hash list... ");
            var hashFilePath = "hashes.json";
            client.DownloadFile(new Uri("https://tcno.co/Projects/AccSwitcher/latest/hashes.json"), hashFilePath);
            Debug.WriteLine("Done.");

            var verifyDictionary = JsonConvert.DeserializeObject<Dictionary<string, string>>(File.ReadAllText(hashFilePath));
            if (verifyDictionary != null)
            {
                var verifyDictTotal = verifyDictionary.Count;
                var cur = 0;
                UpdateProgress(0);
                foreach (var (key, value) in verifyDictionary)
                {
                    cur++;
                    UpdateProgress((cur * 100) / verifyDictTotal);
                    if (!File.Exists(key)) continue;
                    var md5 = GetFileMd5(key);
                    if (value == md5) continue;

                    WriteLine("File: " + key + " has MD5: " + md5 + " EXPECTED: " + value);
                    WriteLine("Deleting: " + key);
                    DeleteFile(key);
                    WriteLine("Downloading file from website... ");
                    var uri = new Uri("https://tcno.co/Projects/AccSwitcher/latest/" + key.Replace('\\', '/'));
                    client.DownloadFile(uri, key);
                    WriteLine("Done.");
                }
            }

            File.Delete(hashFilePath);
            WriteLine("");
            SetStatusAndLog("All updates complete!");
            ButtonHandler(true, "Start Acc Switcher");
            Dispatcher.BeginInvoke(new Action(() => {
                StartButton.Click -= StartUpdate_Click;
                StartButton.Click += LaunchAccSwitcher;
            }), DispatcherPriority.Normal);
        }

        private void OnClientOnDownloadProgressChanged(object o, DownloadProgressChangedEventArgs e)
        {
            UpdateProgress(e.ProgressPercentage);
        }


        /// <summary>
        /// Gets list of updates from https://tcno.co
        /// </summary>
        /// <param name="updatesAndChanges"></param>
        private void GetUpdatesList(ref Dictionary<string, string> updatesAndChanges)
        {
            double totalFilesize = 0;

            var client = new WebClient();
            client.Headers.Add("Cache-Control", "no-cache");
            var vUri = new Uri("https://tcno.co/Projects/AccSwitcher/api_v4.php");
            var versions = client.DownloadString(vUri);
            var jUpdates = JObject.Parse(versions)["updates"];
            foreach (var jToken in jUpdates)
            {
                var jUpdate = (JProperty) jToken;
                if (jUpdate.Name == CurrentVersion) break; // Get up to the current version
                var updateDetails = jUpdate.Value[0].ToString();
                var updateSize = FilesizeString((double) jUpdate.Value[1]);
                totalFilesize += (double) jUpdate.Value[1];

                updatesAndChanges.Add(jUpdate.Name, jUpdate.Value.ToString());
                WriteLine($"Update found: {jUpdate.Name}, {updateSize}");
                WriteLine(updateDetails);
                WriteLine("");
            }
            WriteLine("-------------------------------------------");
            WriteLine($"Total updates found: {updatesAndChanges.Count}");
            WriteLine($"Total size: {FilesizeString(totalFilesize)}");
            WriteLine("-------------------------------------------");
            WriteLine("Click the button below to start the update.");
            ButtonHandler(true, "Start update");
            SetStatus("Waiting for user input...");
        }

        private string FilesizeString(double len)
        {
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
            Debug.WriteLine("Checking for running instances of TcNo Account Switcher");
            foreach (var exe in Directory.GetFiles(currentDir, "*.exe", SearchOption.AllDirectories))
            {
                if (exe.Contains(Path.GetFileNameWithoutExtension(System.Reflection.Assembly.GetEntryAssembly()?.Location)!)) continue;
                if (Process.GetProcessesByName(Path.GetFileNameWithoutExtension(exe)).Length <= 0) continue;
                Debug.WriteLine("Killing: " + exe);
                KillProcess(exe);
            }
        }

        /// <summary>
        /// Used by TechNobo to create updates for the program.
        /// </summary>
        private void CreateUpdate()
        {
            // CREATE UPDATE
            const string oldFolder = "OldVersion";
            const string newFolder = "NewVersion";
            const string outputFolder = "UpdateOutput";
            var deleteFileList = Path.Join(outputFolder, "filesToDelete.txt");
            RecursiveDelete(new DirectoryInfo(outputFolder), false);
            if (File.Exists(deleteFileList)) File.Delete(deleteFileList);

            List<string> filesToDelete = new(); // Simply ignore these later
            CreateFolderPatches(oldFolder, newFolder, outputFolder, ref filesToDelete);
            File.WriteAllLines(deleteFileList, filesToDelete);
            Debug.WriteLine("Please 7z the update folder.");
            Debug.WriteLine("Please 7z the update folder.");
            Debug.WriteLine("Please 7z the update folder.");
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
            Directory.CreateDirectory("temp_update");
            SetStatusAndLog("Extracting patch files");
            using (var archiveFile = new ArchiveFile(updateFilePath))
            {
                archiveFile.Extract("temp_update", true); // extract all
            }
            SetStatusAndLog("Applying patch...");
            ApplyPatches(currentDir, "temp_update");

            // Remove files that need to be removed:
            List<string> filesToDelete = File.ReadAllLines(Path.Join("temp_update", "filesToDelete.txt")).ToList();

            SetStatusAndLog("Moving files...");
            MoveFilesRecursive(Path.Join("temp_update", "new"), currentDir);
            MoveFilesRecursive(Path.Join("temp_update", "patched"), currentDir);

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
        private void MoveFilesRecursive(string inputFolder, string outputFolder)
        {
            //Now Create all of the directories
            foreach (var dirPath in Directory.GetDirectories(inputFolder, "*", SearchOption.AllDirectories))
                Directory.CreateDirectory(dirPath.Replace(inputFolder, outputFolder));

            //Copy all the files & Replaces any files with the same name
            foreach (var newPath in Directory.GetFiles(inputFolder, "*.*", SearchOption.AllDirectories))
                File.Move(newPath, newPath.Replace(inputFolder, outputFolder), true);
        }

        /// <summary>
        /// Recursively copy files and directories
        /// </summary>
        /// <param name="inputFolder">Folder to copy files recursively from</param>
        /// <param name="outputFolder">Destination folder</param>
        private void CopyFilesRecursive(string inputFolder, string outputFolder)
        {
            //Now Create all of the directories
            foreach (var dirPath in Directory.GetDirectories(inputFolder, "*", SearchOption.AllDirectories))
                Directory.CreateDirectory(dirPath.Replace(inputFolder, outputFolder));

            //Copy all the files & Replaces any files with the same name
            foreach (var newPath in Directory.GetFiles(inputFolder, "*.*", SearchOption.AllDirectories))
                File.Copy(newPath, newPath.Replace(inputFolder, outputFolder), true);
        }

        /// <summary>
        /// Kills requested process. Will Write to Log and Console if unexpected output occurs (Anything more than "") 
        /// </summary>
        /// <param name="procName">Process name to kill (Will be used as {name}*)</param>
        private static void KillProcess(string procName)
        {

            var outputText = "";
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
            process.OutputDataReceived += (s, e) => outputText += e.Data + "\n";
            process.Start();
            process.BeginOutputReadLine();
            process.WaitForExit();

            Debug.WriteLine($"Tried to close {procName}. Unexpected output from cmd:\r\n{outputText}");
        }

        /// <summary>
        /// Recursively delete files in folders (Choose to keep or delete folders too)
        /// </summary>
        /// <param name="baseDir">Folder to start working inwards from (as DirectoryInfo)</param>
        /// <param name="keepFolders">Set to False to delete folders as well as files</param>
        /// <param name="jsDest">Place to send responses (if any)</param>
        private static void RecursiveDelete(DirectoryInfo baseDir, bool keepFolders, string jsDest = "")
        {
            if (!baseDir.Exists)
                return;

            foreach (var dir in baseDir.EnumerateDirectories())
            {
                RecursiveDelete(dir, keepFolders, jsDest);
            }
            var files = baseDir.GetFiles();
            foreach (var file in files)
            {
                DeleteFile(fileInfo: file, jsDest: jsDest);
            }

            if (keepFolders) return;
            baseDir.Delete();
        }

        /// <summary>
        /// Deletes a single file
        /// </summary>
        /// <param name="file">(Optional) File string to delete</param>
        /// <param name="fileInfo">(Optional) FileInfo of file to delete</param>
        /// <param name="jsDest">Place to send responses (if any)</param>
        private static void DeleteFile(string file = "", FileInfo fileInfo = null, string jsDest = "")
        {
            var f = fileInfo ?? new FileInfo(file);

            try
            {
                if (!f.Exists) Debug.WriteLine("err");
                else
                {
                    f.IsReadOnly = false;
                    f.Delete();
                }
            }
            catch (Exception e)
            {

            }
        }

        /// <summary>
        /// Apply patches from outputFolder\\patches into old folder
        /// </summary>
        /// <param name="oldFolder">Old program files path</param>
        /// <param name="outputFolder">Path to folder with patches folder in it</param>
        private void ApplyPatches(string oldFolder, string outputFolder)
        {
            DirSearch(Path.Join(outputFolder, "patches"), ref patchList);
            foreach (var p in patchList)
            {
                var relativePath = p.Remove(0, p.Split("\\")[0].Length + 1);
                var patchFile = Path.Join(outputFolder, "patches", relativePath);
                var patchedFile = Path.Join(outputFolder, "patched", relativePath);
                Directory.CreateDirectory(Path.GetDirectoryName(patchedFile)!);
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
        private void CreateFolderPatches(string oldFolder, string newFolder, string outputFolder, ref List<string> filesToDelete)
        {
            Directory.CreateDirectory(outputFolder);

            DirSearchWithHash(oldFolder, ref oldDict);
            DirSearchWithHash(newFolder, ref newDict);

            // -----------------------------------
            // SAVE DICT OF NEW FILES & HASHES
            // -----------------------------------
            File.WriteAllText(Path.Join(outputFolder, "hashes.json"), JsonConvert.SerializeObject(newDict, Newtonsoft.Json.Formatting.Indented));


            List<string> differentFiles = new();
            // Files left in newDict are completely new files.

            // COMPARE THE 2 FOLDERS
            foreach (var kvp in oldDict) // Foreach entry in old dictionary:
            {
                if (newDict.TryGetValue(kvp.Key, out string sVal)) // If new dictionary has it, get it's value
                {
                    if (kvp.Value != sVal) // Compare the values. If they don't match ==> FILES ARE DIFFERENT
                    {
                        differentFiles.Add(kvp.Key);
                    }
                    oldDict.Remove(kvp.Key); // Remove from both old
                    newDict.Remove(kvp.Key); // and new dictionaries
                }
                else // New dictionary does NOT have this file
                {
                    filesToDelete.Add(kvp.Key); // Add to list of files to delete
                    oldDict.Remove(kvp.Key); // Remove from old dictionary. They have been added to delete list.
                }
            }

            // -----------------------------------
            // HANDLE NEW FILES
            // -----------------------------------
            // Copy new files into output\\new
            string outputNewFolder = Path.Join(outputFolder, "new");
            Directory.CreateDirectory(outputNewFolder);
            foreach (var newFile in newDict.Keys)
            {
                var newFileInput = Path.Join(newFolder, newFile);
                var newFileOutput = Path.Join(outputNewFolder, newFile);

                Directory.CreateDirectory(Path.GetDirectoryName(newFileOutput)!);
                File.Copy(newFileInput, newFileOutput, true);
                Debug.WriteLine("Copied new file to output: " + newFileOutput);
            }


            // -----------------------------------
            // HANDLE DIFFERENT FILES
            // -----------------------------------
            foreach (var differentFile in differentFiles)
            {
                var oldFileInput = Path.Join(oldFolder, differentFile);
                var newFileInput = Path.Join(newFolder, differentFile);
                var patchFileOutput = Path.Join(outputFolder, "patches", differentFile);
                Directory.CreateDirectory(Path.GetDirectoryName(patchFileOutput)!);
                DoEncode(oldFileInput, newFileInput, patchFileOutput);
                Debug.WriteLine("Created patch: " + patchFileOutput);
            }
        }

        /// <summary>
        /// Search through a directory for all files, and add files to dictionary
        /// </summary>
        /// <param name="sDir">Directory to recursively search</param>
        /// <param name="dict">Dict of strings for files</param>
        private void DirSearch(string sDir, ref List<string> list)
        {
            try
            {
                // Foreach file in directory
                foreach (var f in Directory.GetFiles(sDir))
                {
                    Debug.WriteLine(f + "|" + GetFileMd5(f));
                    list.Add(f.Remove(0, f.Split("\\")[0].Length + 1));
                }

                // Foreach directory in file
                foreach (var d in Directory.GetDirectories(sDir))
                {
                    DirSearch(d, ref list);
                }
            }
            catch (System.Exception excpt)
            {
                Debug.WriteLine(excpt.Message);
            }
        }

        /// <summary>
        /// Gets a file's MD5 Hash
        /// </summary>
        /// <param name="filePath">Path to file to get hash of</param>
        /// <returns></returns>
        private string GetFileMd5(string filePath)
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
        private void DirSearchWithHash(string sDir, ref Dictionary<string, string> dict)
        {
            try
            {
                // Foreach file in directory
                foreach (var f in Directory.GetFiles(sDir))
                {
                    Debug.WriteLine(f + "|");
                    dict.Add(f.Remove(0, f.Split("\\")[0].Length + 1), GetFileMd5(f));
                }

                // Foreach directory in file
                foreach (var d in Directory.GetDirectories(sDir))
                {
                    DirSearchWithHash(d, ref dict);
                }
            }
            catch (System.Exception excpt)
            {
                Debug.WriteLine(excpt.Message);
            }
        }

        /// <summary>
        /// Encodes patch file from input old and input new files
        /// </summary>
        /// <param name="oldFile">Path to old file</param>
        /// <param name="newFile">Path to new file</param>
        /// <param name="patchFileOutput">Output of patch file (Differences encoded)</param>
        private void DoEncode(string oldFile, string newFile, string patchFileOutput)
        {
            using FileStream output = new FileStream(patchFileOutput, FileMode.Create, FileAccess.Write);
            using FileStream dict = new FileStream(oldFile, FileMode.Open, FileAccess.Read);
            using FileStream target = new FileStream(newFile, FileMode.Open, FileAccess.Read);

            VcEncoder coder = new VcEncoder(dict, target, output);
            VCDiffResult
                result = coder.Encode(interleaved: true,
                    checksumFormat: ChecksumFormat.SDCH); //encodes with no checksum and not interleaved
            if (result != VCDiffResult.SUCCESS)
            {
                //error was not able to encode properly
                Debug.WriteLine("oops :(");
            }
        }

        /// <summary>
        /// Decodes patch file from input patch and input old file into new file
        /// </summary>
        /// <param name="oldFile">Path to old file</param>
        /// <param name="patchFile">Path to patch file (Differences encoded)</param>
        /// <param name="outputNewFile">Output of new file (that's been updated)</param>
        private void DoDecode(string oldFile, string patchFile, string outputNewFile)
        {
            using FileStream dict = new FileStream(oldFile, FileMode.Open, FileAccess.Read);
            using FileStream target = new FileStream(patchFile, FileMode.Open, FileAccess.Read);
            using FileStream output = new FileStream(outputNewFile, FileMode.Create, FileAccess.Write);
            VcDecoder decoder = new VcDecoder(dict, target, output);

            // The header of the delta file must be available before the first call to decoder.Decode().
            long bytesWritten = 0;
            VCDiffResult result = decoder.Decode(out bytesWritten);

            if (result != VCDiffResult.SUCCESS)
            {
                //error decoding
                Debug.WriteLine(result + " - " + bytesWritten);
            }

            // if success bytesWritten will contain the number of bytes that were decoded
        }

    }



    /// <summary>
    /// Handles window buttons, resizing and dragging
    /// </summary>
    public class WindowHandling
    {
        public static void BtnMinimize(object sender, RoutedEventArgs e, Window window)
        {
            window.WindowState = WindowState.Minimized;
        }
        public static void BtnExit(object sender, RoutedEventArgs e, Window window)
        {
            window.Close();
        }
        public static void DragWindow(object sender, MouseButtonEventArgs e, Window window)
        {
            if (e.LeftButton == MouseButtonState.Pressed)
            {
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
            switch (window.WindowState)
            {
                case WindowState.Normal:
                    window.WindowState = WindowState.Maximized;
                    break;
                case WindowState.Maximized:
                    window.WindowState = WindowState.Normal;
                    break;
            }
        }
    }
}
