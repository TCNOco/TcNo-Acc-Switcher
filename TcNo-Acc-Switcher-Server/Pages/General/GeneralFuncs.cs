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
using System.Collections;
using System.Diagnostics;
using System.IO;
using System.Runtime.Versioning;
using System.Security.Cryptography;
using System.Text.RegularExpressions;
using Microsoft.JSInterop;
using Microsoft.Win32;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;

namespace TcNo_Acc_Switcher_Server.Pages.General
{
    public class GeneralFuncs
    {
        #region PROCESS_OPERATIONS

        /// <summary>
        /// Starts a process with or without Admin
        /// </summary>
        /// <param name="path">Path of process to start</param>
        /// <param name="elevated">Whether the process should start elevated or not</param>
        public static void StartProgram(string path, bool elevated = false)
        {

            if (!elevated)
                Process.Start(new ProcessStartInfo("explorer.exe", path)); // Starts without admin, through Windows explorer.
            else
            {
                var proc = new Process
                {
                    StartInfo =
                    {
                        FileName = path, 
                        UseShellExecute = true, 
                        Verb = "runas"
                    }
                };
                try
                {
                    proc.Start();
                }
                catch (System.ComponentModel.Win32Exception e)
                {
                    if (e.HResult != -2147467259) // Not because it was cancelled by user
                        throw;
                }
            }

        }
        
        public static bool CanKillProcess(string processName)
        {
            bool canKill;
            if (AppSettings.Instance.CurrentlyElevated)
                canKill = true; // Elevated process can kill most other processes.
            else
            {
                if (OperatingSystem.IsWindows())
                    canKill = !ProcessHelper.IsProcessAdmin(processName); // If other process is admin, this process can't kill it (because it's not admin)
                else 
                    canKill = false;
            }

            // Restart self as admin.
            if (!canKill) AppData.ActiveIJsRuntime.InvokeAsync<string>("ShowModal", "notice:RestartAsAdmin");

            return canKill;
        }
        #endregion

        #region FILE_OPERATIONS
        public static string WwwRoot = Path.Join(Path.GetDirectoryName(System.Reflection.Assembly.GetEntryAssembly()?.Location) ?? throw new InvalidOperationException(), "\\wwwroot");
        /// <summary>
        /// Checks if input file is older than 7 days, then deletes if it is
        /// </summary>
        /// <param name="filename">File path to be checked, and possibly deleted</param>
        /// <param name="daysOld">How many days old the file needs to be to be deleted</param>
        /// <returns>Whether file was deleted or not (Outdated or not)</returns>
        public static bool DeletedOutdatedFile(string filename, int daysOld= 7)
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.DeletedOutdatedFile] filename={filename.Substring(filename.Length - 8, 8)}, daysOld={daysOld}");
            if (!File.Exists(filename)) return true;
            if (DateTime.Now.Subtract(File.GetLastWriteTime(filename)).Days <= daysOld) return false;
            File.Delete(filename);
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
                if (!IsValidGdiPlusImage(filename)) // Delete image if is not as valid, working image.
                {
                    File.Delete(filename);
                    return true;
                }
            }
            catch (Exception ex)
            {
                try
                {
                    File.Delete(filename);
                    return true;
                }
                catch (Exception)
                {
                    Console.WriteLine("Empty profile image detected (0 bytes). Can't delete to re-download.\nInfo: \n" + ex);
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
        private static bool IsValidGdiPlusImage(string filename)
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.IsValidGdiPlusImage] filename={filename.Substring(filename.Length - 8, 8)}");
            //From https://stackoverflow.com/questions/8846654/read-image-and-determine-if-its-corrupt-c-sharp
            try
            {
                using var bmp = new System.Drawing.Bitmap(filename);
                return true;
            }
            catch
            {
                return false;
            }
        }

        public static void JsDestNewline(string jsDest)
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.JsDestNewline] jsDest={jsDest}");
            AppData.ActiveIJsRuntime?.InvokeVoidAsync(jsDest, "<br />"); //Newline
        }

        /// <summary>
        /// Deletes a single file
        /// </summary>
        /// <param name="file">(Optional) File string to delete</param>
        /// <param name="fileInfo">(Optional) FileInfo of file to delete</param>
        /// <param name="jsDest">Place to send responses (if any)</param>
        public static void DeleteFile(string file = "", FileInfo fileInfo = null, string jsDest = "")
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.DeleteFile] file={(file != "" ? file : fileInfo?.FullName ?? "")}{(jsDest != "" ? ", jsDest=" + jsDest : "")}");
            var f = fileInfo ?? new FileInfo(file);

            try
            {
                if (!f.Exists) AppData.ActiveIJsRuntime?.InvokeVoidAsync(jsDest, "File not found: " + f.FullName);
                else
                {
                    f.IsReadOnly = false;
                    f.Delete();
                    AppData.ActiveIJsRuntime?.InvokeVoidAsync(jsDest, "Deleted: " + f.FullName);
                }
            }
            catch (Exception e)
            {
                AppData.ActiveIJsRuntime?.InvokeVoidAsync(jsDest, "ERROR: COULDN'T DELETE: " + f.FullName);
                AppData.ActiveIJsRuntime?.InvokeVoidAsync(jsDest, e.ToString());
                JsDestNewline(jsDest);
            }
        }

        /// <summary>
        /// Shorter RecursiveDelete (Sets keep folders to true)
        /// </summary>
        public static void ClearFolder(string folder, string jsDest = "")
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
        public static void RecursiveDelete(DirectoryInfo baseDir, bool keepFolders, string jsDest = "")
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
                DeleteFile(fileInfo: file, jsDest: jsDest);
            }

            if (keepFolders) return;
            baseDir.Delete();
            AppData.ActiveIJsRuntime?.InvokeVoidAsync(jsDest, "Deleting Folder: " + baseDir.FullName);
            JsDestNewline(jsDest);
        }

        /// <summary>
        /// Deletes registry keys
        /// </summary>
        /// <param name="subKey">Subkey to delete</param>
        /// <param name="val">Value to delete</param>
        /// <param name="jsDest">Place to send responses (if any)</param>
        [SupportedOSPlatform("windows")]
        public static void DeleteRegKey(string subKey, string val, string jsDest = "")
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.DeleteRegKey] subKey={subKey}, val={val}, jsDest={jsDest}");
            using var key = Registry.CurrentUser.OpenSubKey(subKey, true);
            if (key == null)
                AppData.ActiveIJsRuntime?.InvokeVoidAsync(jsDest, $"{subKey} does not exist.");
            else if (key.GetValue(val) == null)
                AppData.ActiveIJsRuntime?.InvokeVoidAsync(jsDest, $"{subKey} does not contain {val}");
            else
            {
                AppData.ActiveIJsRuntime?.InvokeVoidAsync(jsDest, $"Removing {subKey}\\{val}");
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
        private static string[] GetFiles(string sourceFolder, string filter, SearchOption searchOption)
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
        public static void ClearFilesOfType(string folder, string extensions, SearchOption so, string jsDest = "")
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.ClearFilesOfType] folder={folder}, extensions={extensions}, jsDest={jsDest}");
            if (!Directory.Exists(folder))
            {
                AppData.ActiveIJsRuntime?.InvokeVoidAsync(jsDest, $"Directory not found: {folder}");
                JsDestNewline(jsDest);
                return;
            }
            foreach (var file in GetFiles(folder, extensions, so))
            {
                AppData.ActiveIJsRuntime?.InvokeVoidAsync(jsDest, $"Deleting: {file}");
                try
                {
                    File.Delete(file);
                }
                catch (Exception ex)
                {
                    AppData.ActiveIJsRuntime?.InvokeVoidAsync(jsDest, $"ERROR: {ex}");
                }
            }
            JsDestNewline(jsDest);
        }

/*
        /// <summary>
        /// Recursively move files and directories
        /// </summary>
        /// <param name="inputFolder">Folder to move files recursively from</param>
        /// <param name="outputFolder">Destination folder</param>
        public static void MoveFilesRecursive(string inputFolder, string outputFolder)
        {
            //Now Create all of the directories
            foreach (var dirPath in Directory.GetDirectories(inputFolder, "*", SearchOption.AllDirectories))
                Directory.CreateDirectory(dirPath.Replace(inputFolder, outputFolder));

            //Move all the files & Replaces any files with the same name
            foreach (var newPath in Directory.GetFiles(inputFolder, "*.*", SearchOption.AllDirectories))
                File.Move(newPath, newPath.Replace(inputFolder, outputFolder), true);
        }
*/

        /// <summary>
        /// Recursively copy files and directories
        /// </summary>
        /// <param name="inputFolder">Folder to copy files recursively from</param>
        /// <param name="outputFolder">Destination folder</param>
        public static void CopyFilesRecursive(string inputFolder, string outputFolder)
        {
            Directory.CreateDirectory(outputFolder);
            //Now Create all of the directories
            foreach (var dirPath in Directory.GetDirectories(inputFolder, "*", SearchOption.AllDirectories))
                Directory.CreateDirectory(dirPath.Replace(inputFolder, outputFolder));

            //Copy all the files & Replaces any files with the same name
            foreach (var newPath in Directory.GetFiles(inputFolder, "*.*", SearchOption.AllDirectories))
                File.Copy(newPath, newPath.Replace(inputFolder, outputFolder), true);
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
        #endregion

        #region SETTINGS
        /// <summary>
        /// Saves input JObject of settings to input file path
        /// </summary>
        /// <param name="file">File path to save JSON string to</param>
        /// <param name="joNewSettings">JObject of settings to be saved</param>
        /// <param name="mergeNewIntoOld">True merges old with new settings, false merges new with old</param>
        public static void SaveSettings(string file, JObject joNewSettings, bool mergeNewIntoOld = false)
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.SaveSettings] file={file}, joNewSettings=hidden, mergeNewIntoOld={mergeNewIntoOld}");
            var sFilename = file.EndsWith(".json") ? file : file + ".json";

            // Create folder if it doesn't exist:
            var folder = Path.GetDirectoryName(file);
            if (folder != "") Directory.CreateDirectory(folder ?? string.Empty);

            // Get existing settings
            var joSettings = new JObject();
            if (File.Exists(sFilename))
                try
                {
                    joSettings = JObject.Parse(File.ReadAllText(sFilename));
                }
                catch (Exception e)
                {
                    Console.WriteLine(e);
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
                    MergeArrayHandling = MergeArrayHandling.Merge
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
            if (folder != "") Directory.CreateDirectory(folder ?? string.Empty);

            File.WriteAllText(sFilename, joOrder.ToString());
        }

        /// <summary>
        /// Loads settings from input file (JSON string to JObject)
        /// </summary>
        /// <param name="file">JSON file to be read</param>
        /// <param name="defaultSettings">(Optional) Default JObject, for merging in missing parameters</param>
        /// <returns>JObject created from file</returns>
        public static JObject LoadSettings(string file, JObject defaultSettings = null)
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.LoadSettings] file={file}, defaultSettings=hidden");
            var sFilename = file.EndsWith(".json") ? file : file + ".json";
            if (!File.Exists(sFilename)) return defaultSettings ?? new JObject();

            var fileSettingsText = File.ReadAllLines(sFilename);
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
                catch (Newtonsoft.Json.JsonReaderException e)
                {
                    if (handledError) // Only try once
                    {
                        Console.WriteLine(e);
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
                    Console.WriteLine(e);
                    throw;
                }

            if (defaultSettings == null) return fileSettings;

            var addedKey = false;
            // Add missing keys from default
            foreach (var kvp in defaultSettings)
            {
                if (fileSettings.ContainsKey(kvp.Key)) continue;
                fileSettings[kvp.Key] = kvp.Value;
                addedKey = true;
            }
            // Save all settings back into file
            if (addedKey) File.WriteAllText(sFilename, fileSettings.ToString());
            return fileSettings;
        }

        //public static JObject SortJObject(JObject joIn)
        //{
        //    return new JObject( joIn.Properties().OrderByDescending(p => p.Name) );
        //}

        #endregion

        #region WINDOW SETTINGS
        private static readonly AppSettings AppSettings = AppSettings.Instance;
        public static bool WindowSettingsValid()
        {
            Globals.DebugWriteLine(@"[Func:General\GeneralFuncs.WindowSettingsValid]");
            AppSettings.LoadFromFile();
            return true;
        }
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
            return input[..lastIndex] + sNew + input.Substring(lastIndexEnd, input.Length - lastIndexEnd);
        }
        #endregion
    }
}
