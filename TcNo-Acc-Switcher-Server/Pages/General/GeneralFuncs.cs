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
using System.Drawing;
using System.IO;
using System.Runtime.Versioning;
using System.Text.RegularExpressions;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Pages.General;

public class GeneralFuncs
{


    #region FILE_OPERATIONS

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
}