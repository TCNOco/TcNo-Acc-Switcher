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
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Runtime.Versioning;
using System.Security.Cryptography;
using System.Security.Principal;
using System.Text.RegularExpressions;
using System.Threading.Tasks;
using Microsoft.AspNetCore.WebUtilities;
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
        public static void StartProgram(string path, bool elevated) => StartProgram(path, elevated, "");
        /// <summary>
        /// Starts a process with or without Admin
        /// </summary>
        /// <param name="path">Path of process to start</param>
        /// <param name="elevated">Whether the process should start elevated or not</param>
        /// <param name="args">Arguments to pass into the program</param>
        public static void StartProgram(string path, bool elevated, string args)
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
                        Arguments = args,
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
        
        public static bool CanKillProcess(string processName, bool showModal = true)
        {
            // Checks whether program is running as Admin or not
            var currentlyElevated = false;
            if (OperatingSystem.IsWindows())
            {
                var securityIdentifier = WindowsIdentity.GetCurrent().Owner;
                if (securityIdentifier is not null) currentlyElevated = securityIdentifier.IsWellKnown(WellKnownSidType.BuiltinAdministratorsSid);
            }

            bool canKill;
            if (currentlyElevated)
                canKill = true; // Elevated process can kill most other processes.
            else
            {
                if (OperatingSystem.IsWindows())
                    canKill = !ProcessHelper.IsProcessAdmin(processName); // If other process is admin, this process can't kill it (because it's not admin)
                else 
                    canKill = false;
            }

            // Restart self as admin.
            if (!canKill && showModal) _ = GeneralInvocableFuncs.ShowModal("notice:RestartAsAdmin");

            return canKill;
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
			while (ProcessHelper.IsProcessRunning(procName) && timeout < 10)
			{
				timeout++;
				AppData.InvokeVoidAsync("updateStatus", $"Waiting for {procName} to close ({timeout}/10 seconds)");
                System.Threading.Thread.Sleep(1000);
			}

            if (timeout == 10)
	            GeneralInvocableFuncs.ShowToast("error", $"Could not close {procName}!", "Error", "toastarea");

            return timeout != 10; // Returns true if timeout wasn't reached.
		}
		#endregion

		#region FILE_OPERATIONS

		public static string WwwRoot()
        {
            return Path.Join(Globals.UserDataFolder, "\\wwwroot");
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
	            if (File.Exists(filename) && !IsValidGdiPlusImage(filename)) // Delete image if is not as valid, working image.
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
            if (string.IsNullOrEmpty(jsDest)) return;
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.JsDestNewline] jsDest={jsDest}");
            AppData.InvokeVoidAsync(jsDest, "<br />"); //Newline
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
                if (f is {Exists: false} && !string.IsNullOrEmpty(jsDest)) AppData.InvokeVoidAsync(jsDest, "File not found: " + f.FullName);
                else
                {
	                if (f == null) return;
	                f.IsReadOnly = false;
	                f.Delete();
                    if (!string.IsNullOrEmpty(jsDest))
						AppData.InvokeVoidAsync(jsDest, "Deleted: " + f.FullName);
                }
            }
            catch (Exception e)
            {
                if (string.IsNullOrEmpty(jsDest)) return;
                if (f != null) AppData.InvokeVoidAsync(jsDest, "ERROR: COULDN'T DELETE: " + f.FullName);
                else AppData.InvokeVoidAsync(jsDest, "ERROR: COULDN'T DELETE UNDEFINED FILE");
                AppData.InvokeVoidAsync(jsDest, e.ToString());
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

        // Overload for below
        public static void RecursiveDelete(DirectoryInfo baseDir, bool keepFolders) => RecursiveDelete(baseDir, keepFolders, "");

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
            if (!string.IsNullOrEmpty(jsDest)) AppData.InvokeVoidAsync(jsDest, "Deleting Folder: " + baseDir.FullName);
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
	            AppData.InvokeVoidAsync(jsDest, $"{subKey} does not exist.");
            else if (key.GetValue(val) == null)
	            AppData.InvokeVoidAsync(jsDest, $"{subKey} does not contain {val}");
            else
            {
	            AppData.InvokeVoidAsync(jsDest, $"Removing {subKey}\\{val}");
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
	            AppData.InvokeVoidAsync(jsDest, $"Directory not found: {folder}");
                JsDestNewline(jsDest);
                return;
            }
            foreach (var file in GetFiles(folder, extensions, so))
            {
	            AppData.InvokeVoidAsync(jsDest, $"Deleting: {file}");
                try
                {
                    File.Delete(file);
                }
                catch (Exception ex)
                {
                    AppData.InvokeVoidAsync(jsDest, $"ERROR: {ex}");
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
        /// <summary>
        /// Saves input JObject of settings to input file path
        /// </summary>
        /// <param name="file">File path to save JSON string to</param>
        /// <param name="joNewSettings">JObject of settings to be saved</param>
        /// <param name="mergeNewIntoOld">True merges old with new settings, false merges new with old</param>
        public static void SaveSettings(string file, JObject joNewSettings, bool mergeNewIntoOld)
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
            if (folder != "") Directory.CreateDirectory(folder ?? string.Empty);

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
                        Globals.WriteToLog(e.ToString());
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

        public static async System.Threading.Tasks.Task HandleFirstRender(bool firstRender, string platform)
        {
            AppData.Instance.WindowTitle = $"TcNo Account Switcher - {platform}";
            if (firstRender)
            {
                // Handle Streamer Mode notification
                if (AppSettings.Instance.StreamerModeEnabled && AppSettings.Instance.StreamerModeTriggered)
                    _ = GeneralInvocableFuncs.ShowToast("info", "Private info is hidden! - See settings", "Streamer mode", "toastarea");

                // Handle loading accounts for specific platforms
                // - Init file if it doesn't exist, or isn't fully initialised (adds missing settings when true)
                if (platform == null) return;
                switch (platform)
                {
                    case "BattleNet":
                        await BattleNet.BattleNetSwitcherFuncs.LoadProfiles();
                        Data.Settings.BattleNet.Instance.SaveSettings(!File.Exists(Data.Settings.BattleNet.SettingsFile));
                        break;

                    case "Discord":
	                    Discord.DiscordSwitcherFuncs.LoadProfiles();
	                    Data.Settings.Discord.Instance.SaveSettings(!File.Exists(Data.Settings.Discord.SettingsFile));
	                    break;

                    case "Epic Games":
                        Epic.EpicSwitcherFuncs.LoadProfiles();
                        Data.Settings.Epic.Instance.SaveSettings(!File.Exists(Data.Settings.Epic.SettingsFile));
                        break;

                    case "Origin":
                        Origin.OriginSwitcherFuncs.LoadProfiles();
                        Data.Settings.Origin.Instance.SaveSettings(!File.Exists(Data.Settings.Origin.SettingsFile));
                        break;

                    case "Riot Games":
                        Riot.RiotSwitcherFuncs.LoadProfiles();
                        Data.Settings.Riot.Instance.SaveSettings(!File.Exists(Data.Settings.Riot.SettingsFile));
                        break;

                    case "Steam":
                        await Steam.SteamSwitcherFuncs.LoadProfiles();
                        Data.Settings.Steam.Instance.SaveSettings(!File.Exists(Data.Settings.Steam.SettingsFile));
                        break;

                    case "Ubisoft":
                        await Ubisoft.UbisoftSwitcherFuncs.LoadProfiles();
                        Data.Settings.Ubisoft.Instance.SaveSettings(!File.Exists(Data.Settings.Ubisoft.SettingsFile));
                        break;
                }
                
                // Handle queries and invoke status "Ready"
                HandleQueries();
                AppData.InvokeVoidAsync("updateStatus", "Ready");
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
                foreach (var stringValue in modalValue) GeneralInvocableFuncs.ShowModal(Uri.UnescapeDataString(stringValue));

            // Toast
            if (!queries.TryGetValue("toast_type", out var toastType) ||
                !queries.TryGetValue("toast_title", out var toastTitle) ||
                !queries.TryGetValue("toast_message", out var toastMessage)) return true;
            for (var i = 0; i < toastType.Count; i++)
            {
                try
                {
                    GeneralInvocableFuncs.ShowToast(toastType[i], toastMessage[i], toastTitle[i], "toastarea");
                    AppData.InvokeVoidAsync("removeUrlArgs", "toast_type,toast_title,toast_message");
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
