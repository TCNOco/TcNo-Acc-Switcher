// TcNo Account Switcher - A Super fast account switcher
// Copyright (C) 2019-2022 TechNobo (Wesley Pyburn)
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
using System.Collections;
using System.Collections.Generic;
using System.Diagnostics;
using System.Drawing;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Net.Http;
using System.Reflection;
using System.Runtime.Versioning;
using System.Security.Cryptography;
using System.Text.RegularExpressions;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.AspNetCore.WebUtilities;
using Microsoft.JSInterop;
using Microsoft.Win32;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data.Classes;
using TcNo_Acc_Switcher_Server.Data.Settings;

namespace TcNo_Acc_Switcher_Server.Data
{
    public class GeneralFuncs : IGeneralFuncs
    {
        private readonly ILang _lang;
        private readonly IAppData _appData;
        private readonly IAppStats _appStats;
        private readonly IAppFuncs _appFuncs;
        private readonly IBasicStats _basicStats;
        private readonly IAppSettings _appSettings;
        private readonly IModalData _modalData;
        private readonly ICurrentPlatform _currentPlatform;
        private readonly NavigationManager _navManager;
        private readonly IGeneralFuncs _generalFuncs;
        private readonly ISteam _Steam;

        private IBasic Basic => _lBasic.Value;
        private readonly Lazy<IBasic> _lBasic;

        public GeneralFuncs(ILang lang, ISteam steam, Lazy<IBasic> basic, IAppData appData, IAppStats appStats,
            IAppFuncs appFuncs, IBasicStats basicStats, IAppSettings appSettings, IModalData modalData,
            ICurrentPlatform currentPlatform, NavigationManager navManager, IGeneralFuncs generalFuncs)
        {
            _lang = lang;
            _Steam = steam;
            _lBasic = basic;
            _appData = appData;
            _appStats = appStats;
            _appFuncs = appFuncs;
            _basicStats = basicStats;
            _appSettings = appSettings;
            _modalData = modalData;
            _currentPlatform = currentPlatform;
            _navManager = navManager;
            _generalFuncs = generalFuncs;
        }

        #region PROCESS_OPERATIONS

        //public bool CanKillProcess(List<string> procNames) => procNames.Aggregate(true, (current, s) => current & CanKillProcess(s));
        public async Task<bool> CanKillProcess(List<string> procNames, string closingMethod = "Combined", bool showModal = true)
        {
            var canKillAll = true;
            foreach (var procName in procNames)
            {
                if (procName.StartsWith("SERVICE:") && closingMethod == "TaskKill") continue; // Ignore explicit services when using TaskKill - Admin isn't ALWAYS needed. Eg: Steam.
                canKillAll = canKillAll && await CanKillProcess(procName, closingMethod);
            }

            return canKillAll;
        }

        public async Task<bool> CanKillProcess(string processName, string closingMethod = "Combined", bool showModal = true)
        {
            if (processName.StartsWith("SERVICE:") && closingMethod == "TaskKill") return true; // Ignore explicit services when using TaskKill - Admin isn't ALWAYS needed. Eg: Steam.
            if (processName.StartsWith("SERVICE:")) // Services need admin to close (as far as I understand)
                processName = processName[8..].Split(".exe")[0];


            // Restart self as if can't close admin.
            if (Globals.CanKillProcess(processName)) return true;
            if (showModal)
                _modalData.ShowModal("confirm", ExtraArg.RestartAsAdmin);
            return false;
        }

        public async Task<bool> CloseProcesses(string procName, string closingMethod)
        {
            if (!OperatingSystem.IsWindows()) return false;
            Globals.DebugWriteLine(@"Closing: " + procName);
            if (!await CanKillProcess(procName, closingMethod)) return false;
            Globals.KillProcess(procName, closingMethod);

            return await WaitForClose(procName);
        }
        public async Task<bool> CloseProcesses(List<string> procNames, string closingMethod)
        {
            if (!OperatingSystem.IsWindows()) return false;
            Globals.DebugWriteLine(@"Closing: " + string.Join(", ", procNames));
            if (!await CanKillProcess(procNames, closingMethod)) return false;
            Globals.KillProcess(procNames, closingMethod);

            return await WaitForClose(procNames, closingMethod);
        }

        /// <summary>
        /// Waits for a program to close, and returns true if not running anymore.
        /// </summary>
        /// <param name="procName">Name of process to lookup</param>
        /// <returns>Whether it was closed before this function returns or not.</returns>
        public async Task<bool> WaitForClose(string procName)
        {
            if (!OperatingSystem.IsWindows()) return false;
            var timeout = 0;
            while (Globals.ProcessHelper.IsProcessRunning(procName) && timeout < 10)
            {
                timeout++;
                await _appData.InvokeVoidAsync("updateStatus", _lang["Status_WaitingForClose", new { processName = procName, timeout, timeLimit = "10" }]);
                Thread.Sleep(1000);
            }

            if (timeout == 10)
                await ShowToast("error", _lang["CouldNotCloseX", new { x = procName }], _lang["Error"], "toastarea");

            return timeout != 10; // Returns true if timeout wasn't reached.
        }
        public async Task<bool> WaitForClose(List<string> procNames, string closingMethod)
        {
            if (!OperatingSystem.IsWindows()) return false;
            var procToClose = new List<string>(); // Make a copy to edit
            foreach (var p in procNames)
            {
                var cur = p;
                if (cur.StartsWith("SERVICE:"))
                {
                    if (closingMethod == "TaskKill")
                        continue; // Ignore explicit services when using TaskKill - Admin isn't ALWAYS needed. Eg: Steam.
                    cur = cur[8..].Split(".exe")[0]; // Remove "SERVICE:" and ".exe"
                }
                procToClose.Add(cur);
            }

            var timeout = 0;
            var areAnyRunning = false;
            while (timeout < 10) // Gives 10 seconds to verify app is closed.
            {
                var alreadyClosed = new List<string>();
                var appCount = 0;
                timeout++;
                foreach (var p in procToClose)
                {
                    var isProcRunning = Globals.ProcessHelper.IsProcessRunning(p);
                    if (!isProcRunning)
                        alreadyClosed.Add(p); // Already closed, so remove from list after loop
                    areAnyRunning = areAnyRunning || Globals.ProcessHelper.IsProcessRunning(p);
                    if (areAnyRunning) appCount++;
                }

                foreach (var p in alreadyClosed)
                    procToClose.Remove(p);

                if (procToClose.Count > 0)
                    await _appData.InvokeVoidAsync("updateStatus", _lang["Status_WaitingForMultipleClose", new { processName = procToClose[0], count = appCount, timeout, timeLimit = "10" }]);
                if (areAnyRunning)
                    Thread.Sleep(1000);
                else
                    break;
                areAnyRunning = false;
            }

            if (timeout != 10) return true; // Returns true if timeout wasn't reached.
#pragma warning disable CA1416 // Validate platform compatibility
            var leftOvers = procNames.Where(x => !Globals.ProcessHelper.IsProcessRunning(x));
#pragma warning restore CA1416 // Validate platform compatibility
            await ShowToast("error", _lang["CouldNotCloseX", new { x = string.Join(", ", leftOvers.ToArray()) }], _lang["Error"], "toastarea");
            return false; // Returns true if timeout wasn't reached.
        }

        /// <summary>
        /// Restart the TcNo Account Switcher as Admin
        /// Launches either the Server or main exe, depending on what's currently running.
        /// </summary>
        public void RestartAsAdmin(string args = "")
        {
            var fileName = "TcNo-Acc-Switcher_main.exe";
            if (!_appData.TcNoClientApp) fileName = Assembly.GetEntryAssembly()?.Location.Replace(".dll", ".exe") ?? "TcNo-Acc-Switcher-Server_main.exe";
            else
            {
                // Is client app, but could be developing >> No _main just yet.
                if (!File.Exists(Path.Join(Globals.AppDataFolder, fileName)) && File.Exists(Path.Join(Globals.AppDataFolder, "TcNo-Acc-Switcher.exe")))
                    fileName = Path.Combine(Globals.AppDataFolder, "TcNo-Acc-Switcher.exe");
            }

            var proc = new ProcessStartInfo
            {
                WorkingDirectory = Globals.AppDataFolder,
                FileName = fileName,
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
                Globals.WriteToLog(@"This program must be run as an administrator!" + Environment.NewLine + ex);
                Environment.Exit(0);
            }
        }
        #endregion

        #region FILE_OPERATIONS

        /// <summary>
        /// Read all ids from requested platform file
        /// </summary>
        /// <param name="dictPath">Full *.json file path (file safe)</param>
        /// <param name="isBasic"></param>
        public Dictionary<string, string> ReadDict(string dictPath, bool isBasic = false)
        {
            Globals.DebugWriteLine(@"[Func:General\GeneralSwitcherFuncs.ReadDict]");
            var s = JsonConvert.SerializeObject(new Dictionary<string, string>());
            if (!File.Exists(dictPath))
            {
                if (isBasic && !Globals.IsDirectoryEmpty(Path.GetDirectoryName(dictPath)))
                {
                    _ = ShowToast("error", _lang["Toast_RegSaveMissing"], _lang["Error"], "toastarea");
                }
                return JsonConvert.DeserializeObject<Dictionary<string, string>>(s);
            }
            try
            {
                s = Globals.ReadAllText(dictPath);
            }
            catch (Exception)
            {
                //
            }

            return JsonConvert.DeserializeObject<Dictionary<string, string>>(s);
        }

        public void SaveDict(Dictionary<string, string> dict, string path, bool deleteIfEmpty = false)
        {
            Globals.DebugWriteLine(@"[Func:General\GeneralSwitcherFuncs.SaveDict]");
            if (path == null) return;
            var outText = JsonConvert.SerializeObject(dict);
            if (outText.Length < 4 && File.Exists(path))
                Globals.DeleteFile(path);
            else
                File.WriteAllText(path, outText);
        }


        public string WwwRoot()
        {
            return Path.Join(Globals.UserDataFolder, "\\wwwroot");
        }

        // Overload for below
        public bool DeletedOutdatedFile(string filename) => DeletedOutdatedFile(filename, 0);

        /// <summary>
        /// Checks if input file is older than 7 days, then deletes if it is
        /// </summary>
        /// <param name="filename">File path to be checked, and possibly deleted</param>
        /// <param name="daysOld">How many days old the file needs to be to be deleted</param>
        /// <returns>Whether file was deleted or not (Outdated or not)</returns>
        public bool DeletedOutdatedFile(string filename, int daysOld)
        {
            Globals.DebugWriteLine($@"[Func:General\DeletedOutdatedFile] filename={filename.Substring(filename.Length - 8, 8)}, daysOld={daysOld}");
            if (!File.Exists(filename)) return true;
            if (DateTime.Now.Subtract(File.GetLastWriteTime(filename)).Days <= daysOld) return false;
            Globals.DeleteFile(filename);
            return true;
        }

        /// <summary>
        /// Checks if images is a valid GDI+ image, deleted if not.
        /// </summary>
        /// <param name="filename">File path of image to be checked</param>
        /// <returns>Whether file was deleted, or file was not deleted and was valid</returns>
        public bool DeletedInvalidImage(string filename)
        {
            Globals.DebugWriteLine($@"[Func:General\DeletedInvalidImage] filename={filename.Substring(filename.Length - 8, 8)}");
            try
            {
                if (File.Exists(filename) && OperatingSystem.IsWindows() && !IsValidGdiPlusImage(filename)) // Delete image if is not as valid, working image.
                {
                    Globals.DeleteFile(filename);
                    return true;
                }
            }
            catch (Exception ex)
            {
                try
                {
                    Globals.DeleteFile(filename);
                    return true;
                }
                catch (Exception)
                {
                    Globals.WriteToLog("Empty profile image detected (0 bytes). Can't delete to re-download.\nInfo: \n" + ex);
                    throw;
                }
            }

            return false;
        }

        /// <summary>
        /// Checks if image is a valid GDI+ image
        /// </summary>
        /// <param name="filename">File path of image to be checked</param>
        /// <returns>Whether image is a valid file or not</returns>
        [SupportedOSPlatform("windows")]
        private bool IsValidGdiPlusImage(string filename)
        {
            Globals.DebugWriteLine($@"[Func:General\IsValidGdiPlusImage] filename={filename.Substring(filename.Length - 8, 8)}");
            //From https://stackoverflow.com/questions/8846654/read-image-and-determine-if-its-corrupt-c-sharp
            try
            {
                using var bmp = new Bitmap(filename);
                return true;
            }
            catch
            {
                return false;
            }
        }

        public async Task JsDestNewline(string jsDest)
        {
            if (string.IsNullOrEmpty(jsDest)) return;
            Globals.DebugWriteLine($@"[Func:General\JsDestNewline] jsDest={jsDest}");
            await _appData.InvokeVoidAsync(jsDest, "<br />"); //Newline
        }

        // Overload for below
        public async Task DeleteFile(string file, string jsDest) => await DeleteFile(new FileInfo(file), jsDest);

        /// <summary>
        /// Deletes a single file
        /// </summary>
        /// <param name="f">(Optional) FileInfo of file to delete</param>
        /// <param name="jsDest">Place to send responses (if any)</param>
        public async Task DeleteFile(FileInfo f, string jsDest)
        {
            Globals.DebugWriteLine($@"[Func:General\DeleteFile] file={f?.FullName ?? ""}{(jsDest != "" ? ", jsDest=" + jsDest : "")}");
            try
            {
                if (f is { Exists: false } && !string.IsNullOrEmpty(jsDest)) await _appData.InvokeVoidAsync(jsDest, "File not found: " + f.FullName);
                else
                {
                    if (f == null) return;
                    f.IsReadOnly = false;
                    f.Delete();
                    if (!string.IsNullOrEmpty(jsDest))
                        await _appData.InvokeVoidAsync(jsDest, "Deleted: " + f.FullName);
                }
            }
            catch (Exception e)
            {
                if (string.IsNullOrEmpty(jsDest)) return;
                await _appData.InvokeVoidAsync(jsDest,
                    f != null ? _lang["CouldntDeleteX", new {x = f.FullName}] : _lang["CouldntDeleteUndefined"]);
                await _appData.InvokeVoidAsync(jsDest, e.ToString());
                await JsDestNewline(jsDest);
            }
        }

        // Overload for below
        public async Task ClearFolder(string folder) => await ClearFolder(folder, "");

        /// <summary>
        /// Shorter RecursiveDelete (Sets keep folders to true)
        /// </summary>
        public async Task ClearFolder(string folder, string jsDest)
        {
            Globals.DebugWriteLine($@"[Func:General\ClearFolder] folder={folder}, jsDest={jsDest}");
            await RecursiveDelete(new DirectoryInfo(folder), true, jsDest);
        }

        /// <summary>
        /// Recursively delete files in folders (Choose to keep or delete folders too)
        /// </summary>
        /// <param name="baseDir">Folder to start working inwards from (as DirectoryInfo)</param>
        /// <param name="keepFolders">Set to False to delete folders as well as files</param>
        /// <param name="jsDest">Place to send responses (if any)</param>
        public async Task RecursiveDelete(DirectoryInfo baseDir, bool keepFolders, string jsDest)
        {
            Globals.DebugWriteLine($@"[Func:General\RecursiveDelete] baseDir={baseDir.Name}, jsDest={jsDest}");
            if (!baseDir.Exists)
                return;

            foreach (var dir in baseDir.EnumerateDirectories())
            {
                await RecursiveDelete(dir, keepFolders, jsDest);
            }
            var files = baseDir.GetFiles();
            foreach (var file in files)
            {
                await DeleteFile(file, jsDest);
            }

            if (keepFolders) return;
            baseDir.Delete();
            if (!string.IsNullOrEmpty(jsDest)) await  _appData.InvokeVoidAsync(jsDest, _lang["DeletingFolder"] + baseDir.FullName);
            await JsDestNewline(jsDest);
        }

        /// <summary>
        /// Deletes registry keys
        /// </summary>
        /// <param name="subKey">Subkey to delete</param>
        /// <param name="val">Value to delete</param>
        /// <param name="jsDest">Place to send responses (if any)</param>
        [SupportedOSPlatform("windows")]
        public async Task DeleteRegKey(string subKey, string val, string jsDest)
        {
            Globals.DebugWriteLine($@"[Func:General\DeleteRegKey] subKey={subKey}, val={val}, jsDest={jsDest}");
            using var key = Registry.CurrentUser.OpenSubKey(subKey, true);
            if (key == null)
                await _appData.InvokeVoidAsync(jsDest, _lang["Reg_DoesntExist", new { subKey }]);
            else if (key.GetValue(val) == null)
                await _appData.InvokeVoidAsync(jsDest, _lang["Reg_DoesntContain", new { subKey, val }]);
            else
            {
                await _appData.InvokeVoidAsync(jsDest, _lang["Reg_Removing", new { subKey, val }]);
                key.DeleteValue(val);
            }
            await JsDestNewline(jsDest);
        }

        /// <summary>
        /// Returns a string array of files in a folder, based on a SearchOption.
        /// </summary>
        /// <param name="sourceFolder">Folder to search for files in</param>
        /// <param name="filter">Filter for files in folder</param>
        /// <param name="searchOption">Option: ie: Sub-folders, TopLevel only etc.</param>
        private IEnumerable<string> GetFiles(string sourceFolder, string filter, SearchOption searchOption)
        {
            Globals.DebugWriteLine($@"[Func:General\GetFiles] sourceFolder={sourceFolder}, filter={filter}");
            var alFiles = new ArrayList();
            var multipleFilters = filter.Split('|');
            foreach (var fileFilter in multipleFilters)
                alFiles.AddRange(Directory.GetFiles(sourceFolder, fileFilter, searchOption));

            return (string[])alFiles.ToArray(typeof(string));
        }

        /// <summary>
        /// Deletes all files of a specific type in a directory.
        /// </summary>
        /// <param name="folder">Folder to search for files in</param>
        /// <param name="extensions">Extensions of files to delete</param>
        /// <param name="so">SearchOption of where to look for files</param>
        /// <param name="jsDest">Place to send responses (if any)</param>
        public async Task ClearFilesOfType(string folder, string extensions, SearchOption so, string jsDest)
        {
            Globals.DebugWriteLine($@"[Func:General\ClearFilesOfType] folder={folder}, extensions={extensions}, jsDest={jsDest}");
            if (!Directory.Exists(folder))
            {
                await _appData.InvokeVoidAsync(jsDest, _lang["DirectoryNotFound", new { folder }]);
                await JsDestNewline(jsDest);
                return;
            }
            foreach (var file in GetFiles(folder, extensions, so))
            {
                await _appData.InvokeVoidAsync(jsDest, _lang["DeletingFile", new { file }]);
                try
                {
                    Globals.DeleteFile(file);
                }
                catch (Exception ex)
                {
                    await _appData.InvokeVoidAsync(jsDest, _lang["ErrorDetails", new { ex = Globals.MessageFromHResult(ex.HResult) }]);
                }
            }
            await JsDestNewline(jsDest);
        }

        /// <summary>
        /// Gets a file's MD5 Hash
        /// </summary>
        /// <param name="filePath">Path to file to get hash of</param>
        /// <returns></returns>
        public string GetFileMd5(string filePath)
        {
            using var md5 = MD5.Create();
            using var stream = File.OpenRead(filePath);
            return stream.Length != 0 ? BitConverter.ToString(md5.ComputeHash(stream)).Replace("-", "").ToLowerInvariant() : "0";
        }

        public string ReadOnlyReadAllText(string f)
        {
            var text = "";
            using var stream = File.Open(f, FileMode.Open, FileAccess.Read, FileShare.ReadWrite);
            using var reader = new StreamReader(stream);
            while (!reader.EndOfStream)
            {
                text += reader.ReadLine() + Environment.NewLine;
            }

            return text;
        }


        /// <summary>
        /// Converts file length to easily read string.
        /// </summary>
        /// <param name="len"></param>
        /// <returns></returns>
        public string FileSizeString(double len)
        {
            if (len < 0.001) return "0 bytes";
            string[] sizes = { "B", "KB", "MB", "GB" };
            var n2 = (int)Math.Log10(len) / 3;
            var n3 = len / Math.Pow(1e3, n2);
            return $"{n3:0.##} {sizes[n2]}";
        }
        #endregion

        #region SETTINGS
        // Overload for below
        public static void SaveSettings(string file, JObject joNewSettings) => SaveSettings(file, joNewSettings, false);

        public static void SaveSettings<T>(string file, T jSettings)
        {
            if (file is null) return;

            try
            {
                file = file.EndsWith(".json") ? file : file + ".json";
                // Create folder if it doesn't exist:
                var folder = Path.GetDirectoryName(file);
                if (folder != "") _ = Directory.CreateDirectory(folder ?? string.Empty);

                File.WriteAllText(file,
                    JsonConvert.SerializeObject(JObject.FromObject(jSettings), Formatting.Indented));
            }
            catch (Exception ex)
            {
                Globals.WriteToLog(ex.ToString());
            }
        }

        /// <summary>
        /// Saves input JObject of settings to input file path
        /// </summary>
        /// <param name="file">File path to save JSON string to</param>
        /// <param name="joNewSettings">JObject of settings to be saved</param>
        /// <param name="mergeNewIntoOld">True merges old with new settings, false merges new with old</param>
        /// <param name="replaceAll"></param>
        public static void SaveSettings(string file, JObject joNewSettings, bool mergeNewIntoOld, bool replaceAll = false)
        {
            Globals.DebugWriteLine($@"[Func:General\SaveSettings] file={file}, joNewSettings=hidden, mergeNewIntoOld={mergeNewIntoOld}");
            var sFilename = file.EndsWith(".json") ? file : file + ".json";

            // Create folder if it doesn't exist:
            var folder = Path.GetDirectoryName(file);
            if (folder != "") _ = Directory.CreateDirectory(folder ?? string.Empty);

            // Get existing settings
            var joSettings = new JObject();
            if (File.Exists(sFilename))
                try
                {
                    joSettings = JObject.Parse(Globals.ReadAllText(sFilename));
                }
                catch (Exception ex)
                {
                    Globals.WriteToLog(ex.ToString());
                }

            if (mergeNewIntoOld)
            {
                // Merge new settings with existing settings --> Adds missing variables etc
                joNewSettings.Merge(joSettings, new JsonMergeSettings
                {
                    MergeArrayHandling = MergeArrayHandling.Union
                });
                // Save all settings back into file
                File.WriteAllText(sFilename, joNewSettings.ToString());
            }
            else
            {
                // Merge existing settings with settings from site
                joSettings.Merge(joNewSettings, new JsonMergeSettings
                {
                    MergeArrayHandling = MergeArrayHandling.Replace
                });
                // Save all settings back into file
                File.WriteAllText(sFilename, joSettings.ToString());
            }
        }

        /// <summary>
        /// Saves input JArray of items to input file path
        /// </summary>
        /// <param name="file">File path to save JSON string to</param>
        /// <param name="joOrder">JArray order of items on page</param>
        public void SaveOrder(string file, JArray joOrder)
        {
            Globals.DebugWriteLine($@"[Func:General\SaveOrder] file={file}, joOrder=hidden");
            var sFilename = file.EndsWith(".json") ? file : file + ".json";

            // Create folder if it doesn't exist:
            var folder = Path.GetDirectoryName(file);
            if (folder != "") _ = Directory.CreateDirectory(folder ?? string.Empty);

            File.WriteAllText(sFilename, joOrder.ToString());
        }

        // Overload for below
        public JObject LoadSettings(string file) => LoadSettings(file, null);

        /// <summary>
        /// Loads settings from input file (JSON string to JObject)
        /// </summary>
        /// <param name="file">JSON file to be read</param>
        /// <param name="defaultSettings">(Optional) Default JObject, for merging in missing parameters</param>
        /// <returns>JObject created from file</returns>
        public JObject LoadSettings(string file, JObject defaultSettings)
        {
            Globals.DebugWriteLine($@"[Func:General\LoadSettings] file={file}, defaultSettings=hidden");
            var sFilename = file.EndsWith(".json") ? file : file + ".json";
            if (!File.Exists(sFilename)) return defaultSettings ?? new JObject();

            var fileSettingsText = Globals.ReadAllLines(sFilename);
            if (fileSettingsText.Length == 0 && defaultSettings != null)
            {
                File.WriteAllText(sFilename, defaultSettings.ToString());
                return defaultSettings;
            }

            var fileSettings = new JObject();
            var tryAgain = true;
            var handledError = false;
            while (tryAgain)
                try
                {
                    fileSettings = JObject.Parse(string.Join(Environment.NewLine, fileSettingsText));
                    tryAgain = false;
                    if (handledError)
                        File.WriteAllLines(sFilename, fileSettingsText);
                }
                catch (JsonReaderException e)
                {
                    if (handledError) // Only try once
                    {
                        Globals.WriteToLog(e.ToString());

                        // Reset file:
                        var errFile = sFilename.Replace(".json", "_err.json");
                        Globals.DeleteFile(errFile);
                        File.Move(sFilename, errFile);

                        File.WriteAllText("LastError.txt", "LAST CRASH DETAILS:\nThe following file appears to be corrupt:" + sFilename + "\nThe file was reset. Check the CrashLogs folder for more details.");
                        throw;
                    }

                    // Possible error: Fixes single slashes in string, where there should be double.
                    for (var i = 0; i < fileSettingsText.Length; i++)
                        if (fileSettingsText[i].Contains("FolderPath"))
                            fileSettingsText[i] = Regex.Replace(fileSettingsText[i], @"(?<=[^\\])(\\)(?=[^\\])", @"\\");
                    // Other fixes go here
                    handledError = true;
                }
                catch (Exception e)
                {
                    Globals.WriteToLog(e.ToString());
                    throw;
                }

            if (defaultSettings == null) return fileSettings;

            var addedKey = false;
            // Add missing keys from default
            foreach (var (key, value) in defaultSettings)
            {
                if (fileSettings.ContainsKey(key)) continue;
                fileSettings[key] = value;
                addedKey = true;
            }
            // Save all settings back into file
            if (addedKey) File.WriteAllText(sFilename, fileSettings.ToString());
            return fileSettings;
        }

        /// <summary>
        /// Read a JSON file from provided path. Returns JObject
        /// </summary>
        public async Task<JToken> ReadJsonFile(string path)
        {
            Globals.DebugWriteLine($@"[Func:General\ReadJsonFile] path={path}");
            JToken jToken             = null;

            if (Globals.TryReadJsonFile(path, ref jToken)) return jToken;

            await ShowToast("error", _lang["CouldNotReadFile", new { file = path }], renderTo: "toastarea");
            return new JObject();

        }

        //public JObject SortJObject(JObject joIn)
        //{
        //    return new JObject( joIn.Properties().OrderByDescending(p => p.Name) );
        //}

        #endregion

        #region OTHER
        /// <summary>
        /// Replaces last occurrence of string in string
        /// </summary>
        /// <param name="input">String to modify</param>
        /// <param name="sOld">String to find (and replace)</param>
        /// <param name="sNew">New string to input</param>
        /// <returns></returns>
        public string ReplaceLast(string input, string sOld, string sNew)
        {
            var lastIndex = input.LastIndexOf(sOld, StringComparison.Ordinal);
            var lastIndexEnd = lastIndex + sOld.Length;
            return input[..lastIndex] + sNew + input[lastIndexEnd..];
        }

        /// <summary>
        /// Escape text to be used as text inside HTML elements, using innerHTML
        /// </summary>
        /// <param name="text">String to escape</param>
        /// <returns>HTML escaped string</returns>
        public string EscapeText(string text)
        {
            return text.Replace("&", "&amp;")
                .Replace("<", "&lt;")
                .Replace(">", "&gt;")
                .Replace("\"", "&#34;")
                .Replace("'", "&#39;")
                .Replace("/", "&#47;");
        }

        public class CrowdinResponse
        {
            [JsonProperty("ProofReaders")]
            public SortedDictionary<string, string> ProofReaders { get; set; }

            [JsonProperty("Translators")]
            public List<string> Translators { get; set; }
        }


        /// <summary>
        /// Returns an object with a list of all translators, and proofreaders with their languages.
        /// </summary>
        public CrowdinResponse CrowdinList()
        {
            try
            {
                var html = new HttpClient().GetStringAsync(
                    "https://tcno.co/Projects/AccSwitcher/api/crowdinNew/").Result;
                var resp = JsonConvert.DeserializeObject<CrowdinResponse>(html);
                if (resp is null)
                    return new CrowdinResponse();

                resp.Translators.Sort();

                var expandedProofreaders = new SortedDictionary<string, string>();
                foreach (var proofReader in resp?.ProofReaders)
                {
                    expandedProofreaders.Add(proofReader.Key,
                        string.Join(", ",
                            proofReader.Value.Split(',').Select(lang => new CultureInfo(lang).DisplayName)
                                .ToList()));
                }

                resp.ProofReaders = expandedProofreaders;
                return resp;
            }
            catch (Exception e)
            {
                // Handle website not loading or JObject not loading properly
                Globals.WriteToLog("Failed to load Crowdin users", e);
                _ = ShowToast("error", _lang["Crowdin_Fail"], renderTo: "toastarea");
                return new CrowdinResponse();
            }
        }
        #endregion

        #region SWITCHER_FUNCTIONS

        public async Task HandleFirstRender(bool firstRender, string platform)
        {
            if (firstRender)
            {
                _appData.WindowTitle = _lang["Title_AccountsList", new { platform }];
                // Handle Streamer Mode notification
                if (_appSettings.StreamerModeEnabled && _appSettings.StreamerModeTriggered)
                    await ShowToast("info", _lang["Toast_StreamerModeHint"], _lang["Toast_StreamerModeTitle"], "toastarea");

                // Handle loading accounts for specific platforms
                // - Init file if it doesn't exist, or isn't fully initialised (adds missing settings when true)
                switch (platform)
                {
                    case null:
                        return;

                    case "Steam":
                        await _Steam.LoadProfiles();
                        _Steam.SaveSettings();
                        break;

                    default:
                        await Basic.LoadProfiles();
                        Basic.SaveSettings();
                        break;
                }

                // Handle queries and invoke status "Ready"
                await HandleQueries();
                await _appData.InvokeVoidAsync("updateStatus", _lang["Done"]);
            }
        }

        /// <summary>
        /// For handling queries in URI
        /// </summary>
        public async Task<bool> HandleQueries()
        {
            Globals.DebugWriteLine(@"[JSInvoke:General\HandleQueries]");
            var uri = _navManager.ToAbsoluteUri(_navManager.Uri);
            // Clear cache reload
            var queries = QueryHelpers.ParseQuery(uri.Query);
            // cacheReload handled in JS

            //Modal
            if (queries.TryGetValue("modal", out var modalValue))
                foreach (var stringValue in modalValue) await ShowModal(Uri.UnescapeDataString(stringValue));

            // Toast
            if (!queries.TryGetValue("toast_type", out var toastType) ||
                !queries.TryGetValue("toast_title", out var toastTitle) ||
                !queries.TryGetValue("toast_message", out var toastMessage)) return true;
            for (var i = 0; i < toastType.Count; i++)
            {
                try
                {
                    await ShowToast(toastType[i], toastMessage[i], toastTitle[i], "toastarea");
                    await _appData.InvokeVoidAsync("removeUrlArgs", "toast_type,toast_title,toast_message");
                }
                catch (TaskCanceledException e)
                {
                    Globals.WriteToLog(e.ToString());
                }
            }

            return true;
        }


        public async Task<string> ExportAccountList()
        {
            var platform = _appSettings.GetPlatform(_appData.SelectedPlatform);
            Globals.DebugWriteLine(@$"[Func:Pages\General\GiExportAccountList] platform={platform}");
            if (!Directory.Exists(Path.Join("LoginCache", platform.SafeName)))
            {
                await ShowToast("error", _lang["Toast_AddAccountsFirst"], _lang["Toast_AddAccountsFirstTitle"], "toastarea");
                return "";
            }

            var s = CultureInfo.CurrentCulture.TextInfo.ListSeparator; // Different regions use different separators in csv files.

            await _basicStats.SetCurrentPlatform(platform.Name);

            List<string> allAccountsTable = new();
            if (platform.Name == "Steam")
            {
                // Add headings and separator for programs like Excel
                allAccountsTable.Add($"SEP={s}");
                allAccountsTable.Add($"Account name:{s}Community name:{s}SteamID:{s}VAC status:{s}Last login:{s}Saved profile image:{s}Stats game:{s}Stat name:{s}Stat value:");

                _appData.SteamUsers = await _Steam.GetSteamUsers(_Steam.LoginUsersVdf());
                // Load cached ban info
                _Steam.LoadCachedBanInfo();

                foreach (var su in _appData.SteamUsers)
                {
                    var banInfo = "";
                    if (su.Vac && su.Limited) banInfo += "VAC + Limited";
                    else banInfo += (su.Vac ? "VAC" : "") + (su.Limited ? "Limited" : "");

                    var imagePath = Path.GetFullPath($"{_Steam.SteamImagePath + su.SteamId}.jpg");

                    allAccountsTable.Add(su.AccName + s +
                                         su.Name + s +
                                         su.SteamId + s +
                                         banInfo + s +
                                         _Steam.UnixTimeStampToDateTime(su.LastLogin) + s +
                                         (File.Exists(imagePath) ? imagePath : "Missing from disk") + s +
                                         _basicStats.GetGameStatsString(su.SteamId, s));
                }
            }
            else
            {
                // Add headings and separator for programs like Excel
                allAccountsTable.Add($"SEP={s}");
                // Platform does not have specific details other than usernames saved.
                allAccountsTable.Add($"Account name:{s}Stats game:{s}Stat name:{s}Stat value:");
                foreach (var accDirectory in Directory.GetDirectories(Path.Join("LoginCache", platform.SafeName)))
                {
                    allAccountsTable.Add(Path.GetFileName(accDirectory) + s +
                                         _basicStats.GetGameStatsString(accDirectory, s, true));
                }
            }

            var outputFolder = Path.Join("wwwroot", "Exported");
            _ = Directory.CreateDirectory(outputFolder);

            var outputFile = Path.Join(outputFolder, platform + ".csv");
            await File.WriteAllLinesAsync(outputFile, allAccountsTable).ConfigureAwait(false);
            return Path.Join("Exported", platform + ".csv");
        }

        #endregion



        /// <summary>
        /// JS function handler for saving settings from Settings GUI page into [Platform]Settings.json file
        /// </summary>
        /// <param name="file">Platform specific filename (has .json appended later)</param>
        /// <param name="jsonString">JSON String to be saved to file, from GUI</param>
        [JSInvokable]
        public void GiSaveSettings(string file, string jsonString)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GiSaveSettings] file={file}, jsonString.length={jsonString.Length}");
            SaveSettings(file, JObject.Parse(jsonString));
        }

        [JSInvokable]
        public void GiSaveOrder(string file, string jsonString)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GiSaveOrder] file={file}, jsonString.length={jsonString.Length}");
            SaveOrder(file, JArray.Parse(jsonString));
        }

        /// <summary>
        /// JS function handler for returning JObject of settings from [Platform]Settings.json file
        /// </summary>
        /// <param name="file">Platform specific filename (has .json appended later)</param>
        /// <returns>JObject of settings, to be loaded into GUI</returns>
        [JSInvokable]
        public Task GiLoadSettings(string file)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GiLoadSettings] file={file}");
            return Task.FromResult(LoadSettings(file).ToString());
        }

        /// <summary>
        /// JS function handler for returning string contents of a *.* file
        /// </summary>
        /// <param name="file">Name of file to be read and contents returned in string format</param>
        /// <returns>string of file contents</returns>
        [JSInvokable]
        public Task GiFileReadAllText(string file)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GiFileReadAllText] file={file}");
            return Task.FromResult(File.Exists(file) ? Globals.ReadAllText(file) : "");
        }

        /// <summary>
        /// Opens a link in user's browser through Shell
        /// </summary>
        /// <param name="link">URL string</param>
        [JSInvokable]
        public static void OpenLinkInBrowser(string link)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\OpenLinkInBrowser] link={link}");
            var ps = new ProcessStartInfo(link)
            {
                UseShellExecute = true,
                Verb = "open"
            };
            _ = Process.Start(ps);
        }

        /// <summary>
        /// JS function handler for running showModal JS function, with input arguments.
        /// </summary>
        /// <param name="args">Argument string, containing a command to be handled later by modal</param>
        /// <returns></returns>
        public async Task<bool> ShowModal(string args)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\ShowModal] args={args}");
            return await _appData.InvokeVoidAsync("showModal", args);
        }

        /// <summary>
        /// JS function handler for showing Toast message.
        /// </summary>
        /// <param name="toastType">success, info, warning, error</param>
        /// <param name="toastMessage">Message to be shown in toast</param>
        /// <param name="toastTitle">(Optional) Title to be shown in toast (Empty doesn't show any title)</param>
        /// <param name="renderTo">(Optional) Part of the document to append the toast to (Empty = Default, document.body)</param>
        /// <param name="duration">(Optional) Duration to show the toast before fading</param>
        /// <returns></returns>
        public async Task<bool> ShowToast(string toastType, string toastMessage, string toastTitle = "", string renderTo = "body", int duration = 5000)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\ShowToast] type={toastType}, message={toastMessage}, title={toastTitle}, renderTo={renderTo}, duration={duration}");
            return await _appData.InvokeVoidAsync("window.notification.new", new { type = toastType, title = toastTitle, message = toastMessage, renderTo, duration });
        }

        /// <summary>
        /// JS function handler for showing Toast message.
        /// Instead of putting in messages (which you still can), use lang vars and they will expand.
        /// </summary>
        public async Task<bool> ShowToastLangVars(string toastType, string langToastMessage, string langToastTitle = "", string renderTo = "body", int duration = 5000)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\ShowToast] type={toastType}, message={langToastMessage}, title={langToastTitle}, renderTo={renderTo}, duration={duration}");
            return await _appData.InvokeVoidAsync("window.notification.new", new { type = toastType, title = _lang[langToastTitle], message = _lang[langToastMessage], renderTo, duration });
        }

        /// <summary>
        /// JS function handler for showing Toast message.
        /// Instead of putting in messages (which you still can), use lang vars and they will expand.
        /// </summary>
        public async Task<bool> ShowToastLangVars(string toastType, LangItem langItem, string langToastTitle = "", string renderTo = "body", int duration = 5000)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\ShowToast] type={toastType}, message={langItem.LangTitle}, title={langToastTitle}, renderTo={renderTo}, duration={duration}");
            return await _appData.InvokeVoidAsync("window.notification.new", new { type = toastType, title = _lang[langToastTitle], message = _lang[langItem.LangTitle, langItem.LangObject], renderTo, duration });
        }

        /// <summary>
        /// Creates a shortcut to start the Account Switcher, and swap to the account related.
        /// </summary>
        /// <param name="args">(Optional) arguments for shortcut</param>
        [JSInvokable]
        public async Task CreateShortcut(string args = "")
        {
            Globals.DebugWriteLine(@"[JSInvoke:General\CreateShortcut]");
            if (args.Length > 0 && args[0] != ':') args = $" {args}"; // Add a space before arguments if doesn't start with ':'
            string platformName;
            var primaryPlatformId = "" + _appData.CurrentSwitcher[0];
            var bgImg = Path.Join(WwwRoot(), $"\\img\\platform\\{_appData.CurrentSwitcherSafe}.svg");
            string currentPlatformImgPath, currentPlatformImgPathOverride;
            switch (_appData.CurrentSwitcher)
            {
                case "Steam":
                    currentPlatformImgPath = Path.Join(WwwRoot(), "\\img\\platform\\Steam.svg");
                    currentPlatformImgPathOverride = Path.Join(WwwRoot(), "\\img\\platform\\Steam.png");
                    var ePersonaState = -1;
                    if (args.Length == 2) _ = int.TryParse(args[1].ToString(), out ePersonaState);
                    platformName = $"Switch to {_appData.SelectedAccount.DisplayName} {(args.Length > 0 ? $"({_Steam.PersonaStateToString(ePersonaState)})" : "")} [{_appData.CurrentSwitcher}]";
                    break;
                default:
                    currentPlatformImgPath = Path.Join(WwwRoot(), $"\\img\\platform\\{_currentPlatform.SafeName}.svg");
                    currentPlatformImgPathOverride = Path.Join(WwwRoot(), $"\\img\\platform\\{_currentPlatform.SafeName}.png");
                    primaryPlatformId = _currentPlatform.PrimaryId;
                    platformName = $"Switch to {_appData.SelectedAccount.DisplayName} [{_appData.CurrentSwitcher}]";
                    break;
            }

            if (File.Exists(currentPlatformImgPathOverride))
                bgImg = currentPlatformImgPathOverride;
            else if (File.Exists(currentPlatformImgPath))
                bgImg = currentPlatformImgPath;
            else if (File.Exists(Path.Join(WwwRoot(), "\\img\\BasicDefault.png")))
                bgImg = Path.Join(WwwRoot(), "\\img\\BasicDefault.png");


            var fgImg = Path.Join(WwwRoot(), $"\\img\\profiles\\{_appData.CurrentSwitcherSafe}\\{_appData.SelectedAccountId}.jpg");
            if (!File.Exists(fgImg)) fgImg = Path.Join(WwwRoot(), $"\\img\\profiles\\{_appData.CurrentSwitcherSafe}\\{_appData.SelectedAccountId}.png");
            if (!File.Exists(fgImg))
            {
                await ShowToast("error", _lang["Toast_CantFindImage"], _lang["Toast_CantCreateShortcut"], "toastarea");
                return;
            }

            var s = new Shortcut();
            _ = s.Shortcut_Platform(
                ShortcutFuncs.Desktop,
                platformName,
                $"+{primaryPlatformId}:{_appData.SelectedAccountId}{args}",
                $"Switch to {_appData.SelectedAccount.DisplayName} [{_appData.CurrentSwitcher}] in TcNo Account Switcher",
                true);
            await s.CreateCombinedIcon(_generalFuncs, _appSettings, bgImg, fgImg, $"{_appData.SelectedAccountId}.ico");
            s.TryWrite();

            if (_appSettings.StreamerModeTriggered)
                await ShowToast("success", _lang["Toast_ShortcutCreated"], _lang["Success"], "toastarea");
            else
                await ShowToast("success", _lang["ForName", new { name = _appData.SelectedAccount.DisplayName }], _lang["Toast_ShortcutCreated"], "toastarea");
        }

        [JSInvokable]
        public string PlatformUserModalCopyText() => _currentPlatform.GetUserModalCopyText;
        [JSInvokable]
        public string PlatformHintText() => _currentPlatform.GetUserModalHintText();

        [JSInvokable]
        public string GiLocale(string k) => _lang[k];

        [JSInvokable]
        public string GiLocaleObj(string k, object obj) => _lang[k, obj];


        [JSInvokable]
        public string GiCurrentBasicPlatform(string platform) => platform == "Basic" ? _currentPlatform.FullName : _appSettings.GetPlatform(platform).Name;

        [JSInvokable]
        public string GiCurrentBasicPlatformExe(string platform)
        {
            // EXE name from current platform by name:
            return platform == "Basic" ? _currentPlatform.ExeName : _appSettings.GetPlatform(platform).Name;
        }

        [JSInvokable]
        public string GiGetCleanFilePath(string f) => Globals.GetCleanFilePath(f);


        /// <summary>
        /// Save settings with Ctrl+S Hot key
        /// </summary>
        [JSInvokable]
        public async Task GiCtrlS(string platform)
        {
            _appSettings.SaveSettings();
            switch (platform)
            {
                case "Steam":
                    _Steam.SaveSettings();
                    break;
                case "Basic":
                    Basic.SaveSettings();
                    break;
            }
            await ShowToast("success", _lang["Saved"], renderTo: "toastarea");
        }

        #region ACCOUNT SWITCHER SHARED FUNCTIONS
        public async Task<bool> GenericLoadAccounts(string name, bool isBasic = false)
        {
            var localCachePath = Path.Join(Globals.UserDataFolder, $"LoginCache\\{name}\\");
            if (!Directory.Exists(localCachePath)) return false;
            if (!ListAccountsFromFolder(localCachePath, out var accList)) return false;

            // Order
            accList = OrderAccounts(accList, $"{localCachePath}\\order.json");

            await InsertAccounts(accList, name, isBasic);
            _appStats.SetAccountCount(_currentPlatform.SafeName, accList.Count);

            // Load notes
            _appFuncs.LoadNotes();
            return true;
        }

        /// <summary>
        /// Gets a list of 'account names' from cache folder provided
        /// </summary>
        /// <param name="folder">Cache folder containing accounts</param>
        /// <param name="accList">List of account strings</param>
        /// <returns>Whether the directory exists and successfully added listed names</returns>
        public bool ListAccountsFromFolder(string folder, out List<string> accList)
        {
            accList = new List<string>();

            if (!Directory.Exists(folder)) return false;
            var idsFile = Path.Join(folder, "ids.json");
            accList = File.Exists(idsFile)
                ? ReadDict(idsFile).Keys.ToList()
                : (from f in Directory.GetDirectories(folder)
                   where !f.EndsWith("Shortcuts")
                   let lastSlash = f.LastIndexOf("\\", StringComparison.Ordinal) + 1
                   select f[lastSlash..]).ToList();

            return true;
        }

        /// <summary>
        /// Orders a list of strings by order specific in jsonOrderFile
        /// </summary>
        /// <param name="accList">List of strings to sort</param>
        /// <param name="jsonOrderFile">JSON file containing list order</param>
        /// <returns></returns>
        public List<string> OrderAccounts(List<string> accList, string jsonOrderFile)
        {
            // Order
            if (!File.Exists(jsonOrderFile)) return accList;
            var savedOrder = JsonConvert.DeserializeObject<List<string>>(Globals.ReadAllText(jsonOrderFile));
            if (savedOrder == null) return accList;
            var index = 0;
            if (savedOrder is not { Count: > 0 }) return accList;
            foreach (var acc in from i in savedOrder where accList.Any(x => x == i) select accList.Single(x => x == i))
            {
                _ = accList.Remove(acc);
                accList.Insert(Math.Min(index, accList.Count), acc);
                index++;
            }
            return accList;
        }

        /// <summary>
        /// Runs jQueryProcessAccListSize, initContextMenu and initAccListSortable - Final init needed for account switcher to work.
        /// </summary>
        public async Task FinaliseAccountList()
        {
            await _appData.InvokeVoidAsync("jQueryProcessAccListSize");
            await _appData.InvokeVoidAsync("initContextMenu");
            await _appData.InvokeVoidAsync("initAccListSortable");
        }

        /// <summary>
        /// Iterate through account list and insert into platforms account screen
        /// </summary>
        /// <param name="accList">Account list</param>
        /// <param name="platform">Platform name</param>
        /// <param name="isBasic">(Unused for now) To use Basic platform account's ID as accId)</param>
        public async Task InsertAccounts(IEnumerable accList, string platform, bool isBasic = false)
        {
            if (isBasic)
                Basic.LoadAccountIds();

            _appData.BasicAccounts.Clear();

            foreach (var element in accList)
            {
                var account = new Shared.Accounts.Account
                {
                    Platform = _currentPlatform.SafeName
                };

                if (element is string str)
                {
                    account.AccountId = str;
                    account.DisplayName = isBasic ? Basic.GetNameFromId(str) : str;

                    // Handle account image
                    account.ImagePath = GetImgPath(platform, str).Replace("%", "%25");
                    var actualImagePath = Path.Join("wwwroot\\", GetImgPath(platform, str));
                    if (!File.Exists(actualImagePath))
                    {
                        // Make sure the directory exists
                        Directory.CreateDirectory(Path.GetDirectoryName(actualImagePath)!);
                        var defaultPng = $"wwwroot\\img\\platform\\{platform}Default.png";
                        const string defaultFallback = "wwwroot\\img\\BasicDefault.png";
                        if (File.Exists(defaultPng))
                            File.Copy(defaultPng, actualImagePath, true);
                        else if (File.Exists(defaultFallback))
                            File.Copy(defaultFallback, actualImagePath, true);
                    }

                    // Handle game stats (if any enabled and collected.)
                    account.UserStats = _basicStats.GetUserStatsAllGamesMarkup(_currentPlatform.FullName, str);

                    _appData.BasicAccounts.Add(account);
                    continue;
                }

                // TODO: I have no idea what this was for... But the continue skips the section here, right? Or at least there doesn't need to be brackets around it? Lost in my own code here... Whoops.
                if (element is not KeyValuePair<string, string> pair) continue;
                {
                    // Handle account image
                    var (key, value) = pair;
                    account.ImagePath = GetImgPath(platform, key);

                    // Handle game stats (if any enabled and collected.)
                    account.UserStats = _basicStats.GetUserStatsAllGamesMarkup(_currentPlatform.FullName, key);

                    account.AccountId = key;
                    account.DisplayName = value;

                    _appData.BasicAccounts.Add(account);
                }
            }
            await FinaliseAccountList(); // Init context menu & Sorting
        }

        /// <summary>
        /// Finds if file exists as .jpg or .png
        /// </summary>
        /// <param name="platform">Platform name</param>
        /// <param name="user">Username/ID for use in image name</param>
        /// <returns>Image path</returns>
        private string GetImgPath(string platform, string user)
        {
            var imgPath = $"\\img\\profiles\\{platform.ToLowerInvariant()}\\{Globals.GetCleanFilePath(user.Replace("#", "-"))}";
            if (File.Exists("wwwroot\\" + imgPath + ".png")) return imgPath + ".png";
            return imgPath + ".jpg";
        }
        #endregion
    }

    public class LangItem
    {
        public string LangTitle { get; set; }
        public object LangObject { get; set; }

        public LangItem()
        {
            LangTitle = "";
            LangObject = new object();
        }
        public LangItem(string title)
        {
            LangTitle = title;
            LangObject = new object();
        }
        public LangItem(string langTitle, object langObject)
        {
            LangTitle = langTitle;
            LangObject = langObject;
        }
    }
}
