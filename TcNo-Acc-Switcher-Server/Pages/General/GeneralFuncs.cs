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
using System.Collections;
using System.Collections.Generic;
using System.Drawing;
using System.IO;
using System.Linq;
using System.Runtime.Versioning;
using System.Security.Cryptography;
using System.Text.RegularExpressions;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.AspNetCore.WebUtilities;
using Microsoft.Win32;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.Basic;
using TcNo_Acc_Switcher_Server.Pages.Steam;

namespace TcNo_Acc_Switcher_Server.Pages.General
{
    public class GeneralFuncs
    {
        private static readonly Lang Lang = Lang.Instance;

        #region PROCESS_OPERATIONS

        //public static bool CanKillProcess(List<string> procNames) => procNames.Aggregate(true, (current, s) => current & CanKillProcess(s));
        public static bool CanKillProcess(List<string> procNames, string closingMethod = "Combined", bool showModal = true)
        {
            var canKillAll = true;
            foreach (var procName in procNames)
            {
                if (procName.StartsWith("SERVICE:") && closingMethod == "TaskKill") continue; // Ignore explicit services when using TaskKill - Admin isn't ALWAYS needed. Eg: Steam.
                canKillAll = canKillAll && CanKillProcess(procName, closingMethod);
            }

            return canKillAll;
        }

        public static bool CanKillProcess(string processName, string closingMethod = "Combined", bool showModal = true)
        {
            if (processName.StartsWith("SERVICE:") && closingMethod == "TaskKill") return true; // Ignore explicit services when using TaskKill - Admin isn't ALWAYS needed. Eg: Steam.
            if (processName.StartsWith("SERVICE:")) // Services need admin to close (as far as I understand)
                processName = processName[8..].Split(".exe")[0];


            // Restart self as if can't close admin.
            if (Globals.CanKillProcess(processName)) return true;
            if (showModal)
                _ = GeneralInvocableFuncs.ShowModal("notice:RestartAsAdmin");
            return false;
        }

        public static bool CloseProcesses(string procName, string closingMethod)
        {
            if (!OperatingSystem.IsWindows()) return false;
            Globals.DebugWriteLine(@"Closing: " + procName);
            if (!CanKillProcess(procName, closingMethod)) return false;
            Globals.KillProcess(procName, closingMethod);

            return WaitForClose(procName);
        }
        public static bool CloseProcesses(List<string> procNames, string closingMethod)
        {
            if (!OperatingSystem.IsWindows()) return false;
            Globals.DebugWriteLine(@"Closing: " + string.Join(", ", procNames));
            if (!CanKillProcess(procNames, closingMethod)) return false;
            Globals.KillProcess(procNames, closingMethod);

            return WaitForClose(procNames, closingMethod);
        }

        /// <summary>
        /// Waits for a program to close, and returns true if not running anymore.
        /// </summary>
        /// <param name="procName">Name of process to lookup</param>
        /// <returns>Whether it was closed before this function returns or not.</returns>
        public static bool WaitForClose(string procName)
        {
            if (!OperatingSystem.IsWindows()) return false;
            var timeout = 0;
            while (Globals.ProcessHelper.IsProcessRunning(procName) && timeout < 10)
            {
                timeout++;
                _ = AppData.InvokeVoidAsync("updateStatus", Lang["Status_WaitingForClose", new { processName = procName, timeout, timeLimit = "10" }]);
                Thread.Sleep(1000);
            }

            if (timeout == 10)
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["CouldNotCloseX", new { x = procName }], Lang["Error"], "toastarea");

            return timeout != 10; // Returns true if timeout wasn't reached.
        }
        public static bool WaitForClose(List<string> procNames, string closingMethod)
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
                    _ = AppData.InvokeVoidAsync("updateStatus", Lang["Status_WaitingForMultipleClose", new { processName = procToClose[0], count = appCount, timeout, timeLimit = "10" }]);
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
            _ = GeneralInvocableFuncs.ShowToast("error", Lang["CouldNotCloseX", new { x = string.Join(", ", leftOvers.ToArray()) }], Lang["Error"], "toastarea");
            return false; // Returns true if timeout wasn't reached.
        }
        #endregion

        #region FILE_OPERATIONS

        /// <summary>
        /// Remove requested account from account switcher (Generic file/ids.json operation)
        /// </summary>
        /// <param name="accName">Basic account name</param>
        /// <param name="platform">Platform string (file safe)</param>
        /// <param name="accNameIsId">Whether the accName is the unique ID in ids.json (false by default)</param>
        public static bool ForgetAccount_Generic(string accName, string platform, bool accNameIsId = false)
        {
            Globals.DebugWriteLine(@"[Func:General\GeneralSwitcherFuncs.ForgetAccount_Generic] Forgetting account: hidden, Platform: " + platform);

            // Remove ID from list of ids
            var idsFile = Path.Join(Globals.UserDataFolder, $"LoginCache\\{platform}\\ids.json");
            var orderFile = Path.Join(Globals.UserDataFolder, $"LoginCache\\{platform}\\order.json");
            var ogAccName = accName;

            if (File.Exists(idsFile))
            {
                var allIds = ReadDict(idsFile);
                
                if (accNameIsId)
                {
                    var accId = accName;
                    accName = allIds[accName];
                    _ = allIds.Remove(accId);
                }
                else
                    _ = allIds.Remove(allIds.Single(x => x.Value == accName).Key);

                File.WriteAllText(idsFile, JsonConvert.SerializeObject(allIds));
            }

            if (File.Exists(orderFile))
            {

                Task.Run(async () =>
                {
                    var savedOrder = JsonConvert.DeserializeObject<List<string>>(await File.ReadAllTextAsync(orderFile).ConfigureAwait(false));
                    savedOrder.Remove(ogAccName);
                    File.WriteAllText(orderFile, JsonConvert.SerializeObject(savedOrder));
                }).Wait();
            }

            // Remove cached files
            Globals.RecursiveDelete(Path.Join(Globals.UserDataFolder, $"LoginCache\\{platform}\\{accName}"), false);

            // Remove image
            Globals.DeleteFile(Path.Join(WwwRoot(), $"\\img\\profiles\\{platform}\\{Globals.GetCleanFilePath(ogAccName)}.jpg"));

            // Remove from Tray
            Globals.RemoveTrayUser(platform, accName); // Add to Tray list
            return true;
        }

        /// <summary>
        /// Read all ids from requested platform file
        /// </summary>
        /// <param name="dictPath">Full *.json file path (file safe)</param>
        /// <param name="isBasic"></param>
        public static Dictionary<string, string> ReadDict(string dictPath, bool isBasic = false)
        {
            Globals.DebugWriteLine(@"[Func:General\GeneralSwitcherFuncs.ReadDict]");
            var s = JsonConvert.SerializeObject(new Dictionary<string, string>());
            if (!File.Exists(dictPath))
            {
                if (isBasic && !Globals.IsDirectoryEmpty(Path.GetDirectoryName(dictPath)))
                {
                    _ = GeneralInvocableFuncs.ShowToast("error", Lang["Toast_RegSaveMissing"], Lang["Error"], "toastarea");
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

        public static void SaveDict(Dictionary<string, string> dict, string path, bool deleteIfEmpty = false)
        {
            Globals.DebugWriteLine(@"[Func:General\GeneralSwitcherFuncs.SaveDict]");
            if (path == null) return;
            var outText = JsonConvert.SerializeObject(dict);
            if (outText.Length < 4 && File.Exists(path))
                Globals.DeleteFile(path);
            else
                File.WriteAllText(path, outText);
        }


        public static string WwwRoot()
        {
            return Path.Join(Globals.UserDataFolder, "wwwroot");
        }

        // Overload for below
        public static bool DeletedOutdatedFile(string filename) => DeletedOutdatedFile(filename, 0);

        /// <summary>
        /// Checks if input file is older than 7 days, then deletes if it is
        /// </summary>
        /// <param name="filename">File path to be checked, and possibly deleted</param>
        /// <param name="daysOld">How many days old the file needs to be to be deleted</param>
        /// <returns>Whether file was deleted or not (Outdated or not)</returns>
        public static bool DeletedOutdatedFile(string filename, int daysOld)
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.DeletedOutdatedFile] filename={filename.Substring(filename.Length - 8, 8)}, daysOld={daysOld}");
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
        public static bool DeletedInvalidImage(string filename)
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.DeletedInvalidImage] filename={filename.Substring(filename.Length - 8, 8)}");
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
        private static bool IsValidGdiPlusImage(string filename)
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.IsValidGdiPlusImage] filename={filename.Substring(filename.Length - 8, 8)}");
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

        public static void JsDestNewline(string jsDest)
        {
            if (string.IsNullOrEmpty(jsDest)) return;
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.JsDestNewline] jsDest={jsDest}");
            _ = AppData.InvokeVoidAsync(jsDest, "<br />"); //Newline
        }

        // Overload for below
        public static void DeleteFile(string file, string jsDest) => DeleteFile(new FileInfo(file), jsDest);

        /// <summary>
        /// Deletes a single file
        /// </summary>
        /// <param name="f">(Optional) FileInfo of file to delete</param>
        /// <param name="jsDest">Place to send responses (if any)</param>
        public static void DeleteFile(FileInfo f, string jsDest)
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.DeleteFile] file={f?.FullName ?? ""}{(jsDest != "" ? ", jsDest=" + jsDest : "")}");
            try
            {
                if (f is { Exists: false } && !string.IsNullOrEmpty(jsDest)) _ = AppData.InvokeVoidAsync(jsDest, "File not found: " + f.FullName);
                else
                {
                    if (f == null) return;
                    f.IsReadOnly = false;
                    f.Delete();
                    if (!string.IsNullOrEmpty(jsDest))
                        _ = AppData.InvokeVoidAsync(jsDest, "Deleted: " + f.FullName);
                }
            }
            catch (Exception e)
            {
                if (string.IsNullOrEmpty(jsDest)) return;
                _ = AppData.InvokeVoidAsync(jsDest,
                    f != null ? Lang["CouldntDeleteX", new {x = f.FullName}] : Lang["CouldntDeleteUndefined"]);
                _ = AppData.InvokeVoidAsync(jsDest, e.ToString());
                JsDestNewline(jsDest);
            }
        }

        // Overload for below
        public static void ClearFolder(string folder) => ClearFolder(folder, "");

        /// <summary>
        /// Shorter RecursiveDelete (Sets keep folders to true)
        /// </summary>
        public static void ClearFolder(string folder, string jsDest)
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.ClearFolder] folder={folder}, jsDest={jsDest}");
            RecursiveDelete(new DirectoryInfo(folder), true, jsDest);
        }

        /// <summary>
        /// Recursively delete files in folders (Choose to keep or delete folders too)
        /// </summary>
        /// <param name="baseDir">Folder to start working inwards from (as DirectoryInfo)</param>
        /// <param name="keepFolders">Set to False to delete folders as well as files</param>
        /// <param name="jsDest">Place to send responses (if any)</param>
        public static void RecursiveDelete(DirectoryInfo baseDir, bool keepFolders, string jsDest)
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.RecursiveDelete] baseDir={baseDir.Name}, jsDest={jsDest}");
            if (!baseDir.Exists)
                return;

            foreach (var dir in baseDir.EnumerateDirectories())
            {
                RecursiveDelete(dir, keepFolders, jsDest);
            }
            var files = baseDir.GetFiles();
            foreach (var file in files)
            {
                DeleteFile(file, jsDest);
            }

            if (keepFolders) return;
            baseDir.Delete();
            if (!string.IsNullOrEmpty(jsDest)) _ = AppData.InvokeVoidAsync(jsDest, Lang["DeletingFolder"] + baseDir.FullName);
            JsDestNewline(jsDest);
        }

        /// <summary>
        /// Deletes registry keys
        /// </summary>
        /// <param name="subKey">Subkey to delete</param>
        /// <param name="val">Value to delete</param>
        /// <param name="jsDest">Place to send responses (if any)</param>
        [SupportedOSPlatform("windows")]
        public static void DeleteRegKey(string subKey, string val, string jsDest)
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.DeleteRegKey] subKey={subKey}, val={val}, jsDest={jsDest}");
            using var key = Registry.CurrentUser.OpenSubKey(subKey, true);
            if (key == null)
                _ = AppData.InvokeVoidAsync(jsDest, Lang["Reg_DoesntExist", new { subKey }]);
            else if (key.GetValue(val) == null)
                _ = AppData.InvokeVoidAsync(jsDest, Lang["Reg_DoesntContain", new { subKey, val }]);
            else
            {
                _ = AppData.InvokeVoidAsync(jsDest, Lang["Reg_Removing", new { subKey, val }]);
                key.DeleteValue(val);
            }
            JsDestNewline(jsDest);
        }

        /// <summary>
        /// Returns a string array of files in a folder, based on a SearchOption.
        /// </summary>
        /// <param name="sourceFolder">Folder to search for files in</param>
        /// <param name="filter">Filter for files in folder</param>
        /// <param name="searchOption">Option: ie: Sub-folders, TopLevel only etc.</param>
        private static IEnumerable<string> GetFiles(string sourceFolder, string filter, SearchOption searchOption)
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.GetFiles] sourceFolder={sourceFolder}, filter={filter}");
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
        public static void ClearFilesOfType(string folder, string extensions, SearchOption so, string jsDest)
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.ClearFilesOfType] folder={folder}, extensions={extensions}, jsDest={jsDest}");
            if (!Directory.Exists(folder))
            {
                _ = AppData.InvokeVoidAsync(jsDest, Lang["DirectoryNotFound", new { folder }]);
                JsDestNewline(jsDest);
                return;
            }
            foreach (var file in GetFiles(folder, extensions, so))
            {
                _ = AppData.InvokeVoidAsync(jsDest, Lang["DeletingFile", new { file }]);
                try
                {
                    Globals.DeleteFile(file);
                }
                catch (Exception ex)
                {
                    _ = AppData.InvokeVoidAsync(jsDest, Lang["ErrorDetails", new { ex = Globals.MessageFromHResult(ex.HResult) }]);
                }
            }
            JsDestNewline(jsDest);
        }

        /// <summary>
        /// Gets a file's MD5 Hash
        /// </summary>
        /// <param name="filePath">Path to file to get hash of</param>
        /// <returns></returns>
        public static string GetFileMd5(string filePath)
        {
            using var md5 = MD5.Create();
            using var stream = File.OpenRead(filePath);
            return stream.Length != 0 ? BitConverter.ToString(md5.ComputeHash(stream)).Replace("-", "").ToLowerInvariant() : "0";
        }

        public static string ReadOnlyReadAllText(string f)
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
        public static string FileSizeString(double len)
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
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.SaveSettings] file={file}, joNewSettings=hidden, mergeNewIntoOld={mergeNewIntoOld}");
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
        public static void SaveOrder(string file, JArray joOrder)
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.SaveOrder] file={file}, joOrder=hidden");
            var sFilename = file.EndsWith(".json") ? file : file + ".json";

            // Create folder if it doesn't exist:
            var folder = Path.GetDirectoryName(file);
            if (folder != "") _ = Directory.CreateDirectory(folder ?? string.Empty);

            File.WriteAllText(sFilename, joOrder.ToString());
        }

        // Overload for below
        public static JObject LoadSettings(string file) => LoadSettings(file, null);

        /// <summary>
        /// Loads settings from input file (JSON string to JObject)
        /// </summary>
        /// <param name="file">JSON file to be read</param>
        /// <param name="defaultSettings">(Optional) Default JObject, for merging in missing parameters</param>
        /// <returns>JObject created from file</returns>
        public static JObject LoadSettings(string file, JObject defaultSettings)
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.LoadSettings] file={file}, defaultSettings=hidden");
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
        public static JToken ReadJsonFile(string path)
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.ReadJsonFile] path={path}");
            JToken jToken             = null;

            if (Globals.TryReadJsonFile(path, ref jToken)) return jToken;

            _ = GeneralInvocableFuncs.ShowToast("error", Lang["CouldNotReadFile", new { file = path }], renderTo: "toastarea");
            return new JObject();

        }

        //public static JObject SortJObject(JObject joIn)
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
        public static string ReplaceLast(string input, string sOld, string sNew)
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
        public static string EscapeText(string text)
        {
            return text.Replace("&", "&amp;")
                .Replace("<", "&lt;")
                .Replace(">", "&gt;")
                .Replace("\"", "&#34;")
                .Replace("'", "&#39;")
                .Replace("/", "&#47;");
        }
        #endregion

        #region SWITCHER_FUNCTIONS

        public static async Task HandleFirstRender(bool firstRender, string platform)
        {
            AppData.Instance.WindowTitle = Lang["Title_AccountsList", new { platform }];
            if (firstRender)
            {
                // Handle Streamer Mode notification
                if (AppSettings.StreamerModeEnabled && AppSettings.StreamerModeTriggered)
                    _ = GeneralInvocableFuncs.ShowToast("info", Lang["Toast_StreamerModeHint"], Lang["Toast_StreamerModeTitle"], "toastarea");

                // Handle loading accounts for specific platforms
                // - Init file if it doesn't exist, or isn't fully initialised (adds missing settings when true)
                switch (platform)
                {
                    case null:
                        return;

                    case "Steam":
                        await SteamSwitcherFuncs.LoadProfiles();
                        Data.Settings.Steam.SaveSettings();
                        break;

                    default:
                        BasicSwitcherFuncs.LoadProfiles();
                        Data.Settings.Basic.SaveSettings();
                        break;
                }

                // Handle queries and invoke status "Ready"
                _ = HandleQueries();
                _ = AppData.InvokeVoidAsync("updateStatus", Lang["Done"]);
            }
        }

        /// <summary>
        /// For handling queries in URI
        /// </summary>
        public static bool HandleQueries()
        {
            Globals.DebugWriteLine(@"[JSInvoke:General\GeneralFuncs.HandleQueries]");
            var uri = AppData.ActiveNavMan.ToAbsoluteUri(AppData.ActiveNavMan.Uri);
            // Clear cache reload
            var queries = QueryHelpers.ParseQuery(uri.Query);
            // cacheReload handled in JS

            //Modal
            if (queries.TryGetValue("modal", out var modalValue))
                foreach (var stringValue in modalValue) _ = GeneralInvocableFuncs.ShowModal(Uri.UnescapeDataString(stringValue));

            // Toast
            if (!queries.TryGetValue("toast_type", out var toastType) ||
                !queries.TryGetValue("toast_title", out var toastTitle) ||
                !queries.TryGetValue("toast_message", out var toastMessage)) return true;
            for (var i = 0; i < toastType.Count; i++)
            {
                try
                {
                    _ = GeneralInvocableFuncs.ShowToast(toastType[i], toastMessage[i], toastTitle[i], "toastarea");
                    _ = AppData.InvokeVoidAsync("removeUrlArgs", "toast_type,toast_title,toast_message");
                }
                catch (TaskCanceledException e)
                {
                    Globals.WriteToLog(e.ToString());
                }
            }

            return true;
        }


        #endregion
    }
}
