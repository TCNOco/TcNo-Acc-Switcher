using System;
using System.Collections;
using System.Collections.Generic;
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
        /// <summary>
        /// Checks if input file is older than 7 days, then deletes if it is
        /// </summary>
        /// <param name="filename">File path to be checked, and possibly deleted</param>
        /// <returns>Whether file was deleted or not (Outdated or not)</returns>
        public static bool DeletedOutdatedFile(string filename)
        {
            if (!File.Exists(filename)) return true;
            if (DateTime.Now.Subtract(File.GetLastWriteTime(filename)).Days <= 7) return false;
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
        public static void SaveSettings(string file, JObject joNewSettings)
        {
            var sFilename = file + ".json";

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

            // Merge existing settings with settings from site
            joSettings.Merge(joNewSettings, new JsonMergeSettings
            {
                MergeArrayHandling = MergeArrayHandling.Union
            });

            // Save all settings back into file
            File.WriteAllText(sFilename, joSettings.ToString());
        }

        /// <summary>
        /// Loads settings from input file (JSON string to JObject)
        /// </summary>
        /// <param name="file">JSON file to be read</param>
        /// <returns>JObject created from file</returns>
        public static JObject LoadSettings(string file)
        {
            var sFilename = file + ".json";
            if (File.Exists(sFilename))
            {
                try
                {
                    return JObject.Parse(File.ReadAllText(sFilename));
                }
                catch (Exception e)
                {
                    Console.WriteLine(e);
                }
            }

            return file switch
            {
                "SteamSettings" => SteamSwitcherFuncs.DefaultSettings_Steam(),
                "WindowSettings" => DefaultSettings(),
                _ => new JObject()
            };
        }

        /// <summary>
        /// Returns default settings for program (Just the Window size currently)
        /// </summary>
        public static JObject DefaultSettings() => JObject.Parse(@"WindowSize: ""800, 450""");

        /// <summary>
        /// If input settings from another function are null, load settings from file OR default settings for [Platform]
        /// </summary>
        /// <param name="settings">JObject of settings to be initialized if null</param>
        /// <param name="file">"[Platform]Settings" to be used later in reading file, and setting default if can't</param>
        public static void InitSettingsIfNull(ref JObject settings, string file) => settings ??= GeneralFuncs.LoadSettings(file);

        #endregion
    }
}
