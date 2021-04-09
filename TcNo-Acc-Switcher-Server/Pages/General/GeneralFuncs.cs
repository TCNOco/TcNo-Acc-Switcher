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
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Microsoft.Win32;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Server.Pages.Steam;

namespace TcNo_Acc_Switcher_Server.Pages.General
{
    public class GeneralFuncs
    {
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

        public static void JsDestNewline(IJSRuntime js, string jsDest)
        {
            js?.InvokeVoidAsync(jsDest, "<br />"); //Newline
        }

        /// <summary>
        /// Deletes a single file
        /// </summary>
        /// <param name="file">(Optional) File string to delete</param>
        /// <param name="fileInfo">(Optional) FileInfo of file to delete</param>
        /// <param name="js">JSRuntime to send progress to</param>
        /// <param name="jsDest">Place to send responses (if any)</param>
        public static void DeleteFile(string file = "", FileInfo fileInfo = null, IJSRuntime js = null, string jsDest = "")
        {
             var f = fileInfo ?? new FileInfo(file);

            try
            {
                if (!f.Exists) js?.InvokeVoidAsync(jsDest, "File not found: " + f.FullName);
                else
                {
                    f.IsReadOnly = false;
                    f.Delete();
                    js?.InvokeVoidAsync(jsDest, "Deleted: " + f.FullName);
                }
            }
            catch (Exception e)
            {
                js?.InvokeVoidAsync(jsDest, "ERROR: COULDN'T DELETE: " + f.FullName);
                js?.InvokeVoidAsync(jsDest, e.ToString());
                JsDestNewline(js, jsDest);
            }
        }

        /// <summary>
        /// Shorter RecursiveDelete (Sets keep folders to true)
        /// </summary>
        public static void ClearFolder(string folder, IJSRuntime js = null, string jsDest = "")
        {
            RecursiveDelete(new DirectoryInfo(folder), true, js, jsDest);
        }

        /// <summary>
        /// Recursively delete files in folders (Choose to keep or delete folders too)
        /// </summary>
        /// <param name="baseDir">Folder to start working inwards from (as DirectoryInfo)</param>
        /// <param name="keepFolders">Set to False to delete folders as well as files</param>
        /// <param name="js">JSRuntime to send progress to</param>
        /// <param name="jsDest">Place to send responses (if any)</param>
        public static void RecursiveDelete(DirectoryInfo baseDir, bool keepFolders, IJSRuntime js = null, string jsDest = "")
        {
            if (!baseDir.Exists)
                return;

            foreach (var dir in baseDir.EnumerateDirectories())
            {
                RecursiveDelete(dir, keepFolders, js, jsDest);
            }
            var files = baseDir.GetFiles();
            foreach (var file in files)
            {
                DeleteFile(fileInfo: file, js: js, jsDest: jsDest);
            }

            if (keepFolders) return;
            baseDir.Delete();
            js?.InvokeVoidAsync(jsDest, "Deleting Folder: " + baseDir.FullName);
            JsDestNewline(js, jsDest);
        }

        /// <summary>
        /// Deletes registry keys
        /// </summary>
        /// <param name="subKey">Subkey to delete</param>
        /// <param name="val">Value to delete</param>
        /// <param name="js">JSRuntime to send progress to</param>
        /// <param name="jsDest">Place to send responses (if any)</param>
        public static void DeleteRegKey(string subKey, string val, IJSRuntime js = null, string jsDest = "")
        {
            using var key = Registry.CurrentUser.OpenSubKey(subKey, true);
            if (key == null)
                js?.InvokeVoidAsync(jsDest, $"{subKey} does not exist.");
            else if (key.GetValue(val) == null)
                js?.InvokeVoidAsync(jsDest, $"{subKey} does not contain {val}");
            else
            {
                js?.InvokeVoidAsync(jsDest, $"Removing {subKey}\\{val}");
                key.DeleteValue(val);
            }
            JsDestNewline(js, jsDest);
        }

        /// <summary>
        /// Returns a string array of files in a folder, based on a SearchOption.
        /// </summary>
        /// <param name="sourceFolder">Folder to search for files in</param>
        /// <param name="filter">Filter for files in folder</param>
        /// <param name="searchOption">Option: ie: Subfolders, TopLevel only etc.</param>
        private static string[] GetFiles(string sourceFolder, string filter, SearchOption searchOption)
        {
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
        /// <param name="so">SeachOption of where to look for files</param>
        /// <param name="js">JSRuntime to send progress to</param>
        /// <param name="jsDest">Place to send responses (if any)</param>
        public static void ClearFilesOfType(string folder, string extensions, SearchOption so, IJSRuntime js = null, string jsDest = "")
        {
            if (!Directory.Exists(folder))
            {
                js?.InvokeVoidAsync(jsDest, $"Directory not found: {folder}");
                JsDestNewline(js, jsDest);
                return;
            }
            foreach (var file in GetFiles(folder, extensions, so))
            {
                js?.InvokeVoidAsync(jsDest, $"Deleting: {file}");
                try
                {
                    File.Delete(file);
                }
                catch (Exception ex)
                {
                    js?.InvokeVoidAsync(jsDest, $"ERROR: {ex.ToString()}");
                }
            }
            JsDestNewline(js, jsDest);
        }
        #endregion

        #region SETTINGS
        /// <summary>
        /// Saves input JObject of settings to input file path
        /// </summary>
        /// <param name="file">File path to save JSON string to</param>
        /// <param name="joNewSettings">JObject of settings to be saved</param>
        /// <param name="reverse">True merges old with new settings, false merges new with old</param>
        public static void SaveSettings(string file, JObject joNewSettings, bool reverse = false)
        {
            var sFilename = file.EndsWith(".json") ? file : file + ".json";

            // Get existing settings
            var joSettings = new JObject();
            try
            {
                joSettings = JObject.Parse(File.ReadAllText(sFilename));
            }
            catch (Exception e)
            {
                Console.WriteLine(e);
            }

            if (reverse)
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
                    MergeArrayHandling = MergeArrayHandling.Union
                });
                // Save all settings back into file
                File.WriteAllText(sFilename, joSettings.ToString());
            }
        }

        /// <summary>
        /// Loads settings from input file (JSON string to JObject)
        /// </summary>
        /// <param name="file">JSON file to be read</param>
        /// <returns>JObject created from file</returns>
        public static JObject LoadSettings(string file)
        {
            var sFilename = file.EndsWith(".json") ? file : file + ".json";
            if (File.Exists(sFilename)) 
            {
                try
                {
                    return JObject.Parse(File.ReadAllText(sFilename));
                }
                catch (Exception e)
                {
                    // ignored
                }
            }
            return file switch
            {
                //"SteamSettings.json" => Data.Settings.Steam.DefaultSettings(), Removed the default for this
                "WindowSettings" => DefaultSettings(),
                _ => new JObject()
            };
        }

        /// <summary>
        /// Returns default settings for program (Just the Window size currently)
        /// </summary>
        public static JObject DefaultSettings() => JObject.Parse(@"WindowSize: ""800, 450""");

        #endregion

        #region WINDOW SETTINGS
        private static readonly Data.AppSettings AppSettings = Data.AppSettings.Instance;
        public static bool WindowSettingsValid()
        {
            AppSettings.LoadFromFile();
            return true;
        }
        #endregion
    }
}
