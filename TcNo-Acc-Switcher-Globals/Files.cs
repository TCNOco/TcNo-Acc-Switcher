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
using System.Drawing;
using System.Drawing.IconLib;
using System.IO;
using System.Linq;
using System.Net.Http;
using System.Runtime.Versioning;
using System.Text.RegularExpressions;
using System.Threading.Tasks;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using SevenZip;
using ShellLink;

namespace TcNo_Acc_Switcher_Globals
{
    public partial class Globals
    {
        public static bool ReplaceVarInJsonFile<T>(string path, string selector, T replaceWith)
        {
            try
            {
                JToken jToken = null;
                if (!TryReadJsonFile(path, ref jToken)) return false;

                // It is a good idea to check what kind of variable it is to replace it with it's default... Though most of the time a "" will suffice here.
                //var originalValue = js.SelectToken(selector);
                var newJs = jToken                .ReplacePath(selector, replaceWith); // Using JsonExtensions.ReplacePath in Globals\Extensions.cs
                SaveJsonFile(path, newJs);
            }
            catch (Exception e)
            {
                WriteToLog("Failed to ReplaceVarInJsonFile", e);
                return false;
            }

            return true;
        }

        /// <summary>
        /// Tries to read JSON file. Returns whether it was successful or not.
        /// </summary>
        /// <param name="path">Path of JSON file</param>
        /// <param name="jToken">Ref to JToken to edit.</param>
        /// <returns></returns>
        public static bool TryReadJsonFile(string path, ref JToken jToken)
        {
            DebugWriteLine($@"[Func:General\GeneralFuncs.ReadJsonFile] path={path}");
            if (!File.Exists(path)) return false;
            try
            {
                using var file = File.OpenText(path);
                using var reader = new JsonTextReader(file);
                jToken =                 JToken.ReadFrom(reader);
            }
            catch (Exception e)
            {
                WriteToLog("Could not JSON read file: " + path, e);
                return false;
            }

            return true;
        }

        /// <summary>
        /// Saves a JToken into a file. Either with, or without indentation (none by default).
        /// </summary>
        public static void SaveJsonFile(string path, JToken jo, bool formatted = true)
        {
            File.WriteAllText(path, JsonConvert.SerializeObject(jo, formatted ? Formatting.Indented : Formatting.None));
        }






        #region FILES

        /// <summary>
        /// Checks if a file is accessable with current permissions
        /// </summary>
        public static bool NeedsAdminForFileAccess(string filepath)
        {
            try
            {
                using FileStream fs = new(filepath, FileMode.Open);
                return false;
            }
            catch (UnauthorizedAccessException)
            {
                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }
        /// <summary>
        /// Expands custom environment variables.
        /// </summary>
        /// <param name="path"></param>
        /// <returns></returns>
        public static string ExpandEnvironmentVariables(string path)
        {
            var variables = new Dictionary<string, string>
            {
                { "%TCNO_UserData%", Globals.UserDataFolder },
                { "%TCNO_AppData%", Globals.AppDataFolder },
                { "%Desktop%", Environment.GetFolderPath(Environment.SpecialFolder.Desktop) },
                { "%Documents%", Environment.GetFolderPath(Environment.SpecialFolder.MyDocuments) },
                { "%Music%", Environment.GetFolderPath(Environment.SpecialFolder.MyMusic) },
                { "%Pictures%", Environment.GetFolderPath(Environment.SpecialFolder.MyPictures) },
                { "%Videos%", Environment.GetFolderPath(Environment.SpecialFolder.MyVideos) },
                { "%StartMenu%", Environment.GetFolderPath(Environment.SpecialFolder.StartMenu) },
                { "%StartMenuProgramData%", Environment.ExpandEnvironmentVariables(Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.CommonStartMenu), "\\Programs")) },
                { "%StartMenuAppData%", Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.StartMenu), "\\Programs") }
            };

            foreach (var (k, v) in variables)
                path = path.Replace(k, v);

            return Environment.ExpandEnvironmentVariables(path);
        }
        public static bool FileOrDirectoryExists(string p) => (Directory.Exists(p) || File.Exists(p));

        public static bool IsFolder(string path) => FileOrDirectoryExists(path) && File.GetAttributes(path).HasFlag(FileAttributes.Directory);
        public static bool IsFile(string path) => FileOrDirectoryExists(path) && !File.GetAttributes(path).HasFlag(FileAttributes.Directory);

        public static bool CopyFile(string source, string dest, bool overwrite = true)
        {
            if (string.IsNullOrWhiteSpace(source) || string.IsNullOrWhiteSpace(dest))
            {
                WriteToLog("Failed to copy file! Either path is empty or invalid! From: " + source + ", To: " + dest);
                return false;
            }

            if (!File.Exists(source)) return false;
            if (File.Exists(dest) && !overwrite) return false;

            // Try copy the file normally - This will fail if in use
            var dirName = Path.GetDirectoryName(dest);
            if (!string.IsNullOrWhiteSpace(dirName)) // This could be a file in the working directory, instead of a file in a folder -> No need to create folder if exists.
                Directory.CreateDirectory(dirName);

            try
            {
                File.Copy(source, dest, overwrite);
                return true;
            }
            catch (Exception e)
            {
                // Try another method to copy.
                if (e.HResult == -2147024864) // File in use
                {
                    try
                    {
                        if (File.Exists(dest)) File.Delete(dest);
                        using var inputFile = new FileStream(source, FileMode.Open, FileAccess.Read, FileShare.ReadWrite);
                        using var outputFile = new FileStream(dest, FileMode.Create);
                        var buffer = new byte[0x10000];
                        int bytes;

                        while ((bytes = inputFile.Read(buffer, 0, buffer.Length)) > 0)
                            outputFile.Write(buffer, 0, bytes);
                        return true;
                    }
                    catch (Exception exception)
                    {
                        WriteToLog("Failed to copy file! From: " + source + ", To: " + dest, exception);
                    }
                }
            }
            return false;
        }


        public static string RegexSearchFile(string file, string pattern)
        {
            var m = Regex.Match(File.ReadAllText(file), pattern);
            return m.Success ? m.Value : "";
        }
        public static string RegexSearchFolder(string folder, string pattern, string wildcard = "")
        {
            // Foreach file in folder (until match):
            foreach (var f in Directory.GetFiles(folder, wildcard))
            {
                var result = RegexSearchFile(f, pattern);
                if (result == "") continue;
                return result;
            }

            return "";
        }

        public static bool DeleteFiles(string path)
        {
            return Directory.GetFiles(path).Aggregate(false, (current, f) => DeleteFile(f, true) || current);
        }
        public static bool DeleteFile(string path, bool throwErr = false)
        {
            try
            {
                if (File.Exists(path))
                    File.Delete(path);
                return true;
            }
            catch (Exception e)
            {
                WriteToLog($"Could not delete ({MessageFromHResult(e.HResult)}): {path}");
                if (throwErr)
                    throw;
                return false;
            }
        }

        private static readonly string[] SizeSuffixes =
            { "bytes", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB" };

        public static string FolderSizeString(string path) => FileLengthToString(FolderSize(path));
        public static long FolderSize(string path) => FolderSize(new DirectoryInfo(path));
        public static long FolderSize(DirectoryInfo d)
        {
            var fis = d.GetFiles();
            var size = fis.Sum(fi => fi.Length);
            var dis = d.GetDirectories();
            size += dis.Sum(FolderSize);

            return size;

        }

        public static string FileSizeString(string f)
        {
            if (!File.Exists(f)) return "ERR: NOT FOUND";
            var fi = new FileInfo(f);
            return FileSizeString(fi);
        }
        public static string FileSizeString(FileInfo fi) => FileLengthToString(fi.Length);

        /// <summary>
        /// Convert byte length of file to string (KB, MB, GB...)
        /// </summary>
        public static string FileLengthToString(long value, int decimalPlaces = 2)
        {
            if (value < 0) { return "-" + FileLengthToString(-value, decimalPlaces); }

            var i = 0;
            var dValue = (decimal)value;
            while (Math.Round(dValue, decimalPlaces) >= 1000 && i < SizeSuffixes.Length - 1)
            {
                dValue /= 1024;
                i++;
            }

            return string.Format("{0:n" + decimalPlaces + "} {1}", dValue, SizeSuffixes[i]);
        }


        public static bool IsDirectoryEmpty(string path)
        {
            return !Directory.EnumerateFileSystemEntries(path).Any();
        }

        private static readonly HttpClient HClient = new();

        public static string DownloadString(string uri)
        {
            try
            {
                return HClient.GetStringAsync(uri).Result;
            }
            catch (Exception e)
            {
                WriteToLog($"Failed to download string from: {uri}", e);
                return "";
            }
        }
        public static bool DownloadFile(string url, string path)
        {
            try
            {
                HClient.DefaultRequestHeaders.Add("User-Agent", "TcNo Account Switcher");

                if (!Uri.TryCreate(url, UriKind.Absolute, out _))
                    throw new InvalidOperationException("URI is invalid.");

                var fileBytes = HClient.GetByteArrayAsync(url).Result;
                if (path.Contains('\\')) Directory.CreateDirectory(Path.GetDirectoryName(path)!);
                File.WriteAllBytes(path, fileBytes);
            }
            catch (Exception)
            {
                return false;
            }
            return true;
        }
        public static async Task<bool> DownloadFileAsync(string url, string path)
        {
            try
            {
                if (!Uri.TryCreate(url, UriKind.Absolute, out _))
                    throw new InvalidOperationException("URI is invalid.");

                var fileBytes = HClient.GetByteArrayAsync(url).Result;
                if (path.Contains('\\')) Directory.CreateDirectory(Path.GetDirectoryName(path)!);
                await File.WriteAllBytesAsync(path, fileBytes);
            }
            catch (Exception)
            {
                return false;
            }
            return true;
        }

        /// <summary>
        /// A replacement for File.ReadAllText() that doesn't crash if a file is in use.
        /// </summary>
        /// <param name="f">File to be read</param>
        /// <returns>string of content</returns>
        public static string ReadAllText(string f)
        {
            using var fs = new FileStream(f, FileMode.Open, FileAccess.Read, FileShare.ReadWrite);
            using var tr = new StreamReader(fs);
            return tr.ReadToEnd();
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

        /// <summary>
        /// Recursively copy files and directories
        /// </summary>
        /// <param name="inputFolder">Folder to copy files recursively from</param>
        /// <param name="outputFolder">Destination folder</param>
        /// <param name="overwrite">Whether to overwrite files or not</param>
        /// <param name="throwOnError">When false, error is only logged (default)</param>
        public static bool CopyFilesRecursive(string inputFolder, string outputFolder, bool overwrite = true, bool throwOnError = false)
        {
            try
            {
                _ = Directory.CreateDirectory(outputFolder);
                outputFolder = outputFolder.EndsWith("\\") ? outputFolder : outputFolder + "\\";
                //Now Create all of the directories
                foreach (var dirPath in Directory.GetDirectories(inputFolder, "*", SearchOption.AllDirectories))
                    _ = Directory.CreateDirectory(dirPath.Replace(inputFolder, outputFolder));

                //Copy all the files & Replaces any files with the same name
                foreach (var newPath in Directory.GetFiles(inputFolder, "*.*", SearchOption.AllDirectories))
                {
                    var dest = newPath.Replace(inputFolder, outputFolder);
                    if (!overwrite && File.Exists(dest)) continue;

                    File.Copy(newPath, dest, true);
                }
            }
            catch (Exception e)
            {
                WriteToLog($"Failed to CopyFilesRecursive: {inputFolder} -> {outputFolder} (Overwrite {overwrite})", e);
                if (throwOnError) throw;
                return false;
            }

            return true;
        }

        /// <summary>
        /// Recursively copy files and directories - With a filter for file types
        /// </summary>
        /// <param name="inputFolder">Folder to copy files recursively from</param>
        /// <param name="outputFolder">Destination folder</param>
        /// <param name="overwrite">Whether to overwrite files or not</param>
        /// <param name="fileTypes">List of file types to include or exclude</param>
        /// <param name="include">True: include files from file types ONLY, or FALSE: Exclude any file type matches</param>
        /// <param name="throwOnError"></param>
        public static bool CopyFilesRecursive(string inputFolder, string outputFolder, bool overwrite, List<string> fileTypes, bool include, bool throwOnError = false)
        {
            try
            {
                _ = Directory.CreateDirectory(outputFolder);
                outputFolder = outputFolder.EndsWith("\\") ? outputFolder : outputFolder + "\\";
                //Now Create all of the directories
                foreach (var dirPath in Directory.GetDirectories(inputFolder, "*", SearchOption.AllDirectories))
                    _ = Directory.CreateDirectory(dirPath.Replace(inputFolder, outputFolder));

                //Copy all the files & Replaces any files with the same name
                foreach (var newPath in Directory.GetFiles(inputFolder, "*.*", SearchOption.AllDirectories))
                {
                    var fileTypesContains = fileTypes.Contains(Path.GetExtension(newPath));
                    // Filter:
                    if (include && !fileTypesContains)
                    {
                        // Include only in list, and wasn't found in list
                        continue;
                    }

                    if (!include && fileTypesContains)
                    {
                        // Exclude if in list, and was found in list
                        continue;
                    }

                    var dest = newPath.Replace(inputFolder, outputFolder);
                    if (!overwrite && File.Exists(dest)) continue;

                    File.Copy(newPath, dest, true);
                }
            }
            catch (Exception e)
            {
                WriteToLog($"Failed to CopyFilesRecursive: {inputFolder} -> {outputFolder} (Overwrite {overwrite}, File types: {string.Join(',', fileTypes)}, Include {include})", e);
                if (throwOnError) throw;
                return false;
            }

            return true;
        }

        public static void CompressFolder(string folder, string output)
        {
            // Get file dictionary
            Dictionary<string, string> files = new();
            foreach (var f in Directory.GetFiles(folder, "*.*", SearchOption.AllDirectories))
            {
                var destFolder = f.Replace(folder + "\\", "");
                files.Add(destFolder, f);
            }

            if (!output.EndsWith(".7z"))
                output += ".7z";

            // Compress file
            SevenZipBase.SetLibraryPath(Path.Combine(AppDataFolder, Environment.Is64BitProcess ? "x64" : "x86", "7z.dll"));

            var szc = new SevenZipCompressor
            {
                CompressionLevel = CompressionLevel.Normal,
                ArchiveFormat = OutArchiveFormat.SevenZip,
                CompressionMethod = CompressionMethod.Default,
                CompressionMode = CompressionMode.Create
            };
            szc.CustomParameters.Add("mt", "on"); // Multi-threading ON
            szc.CustomParameters.Add("s", "off"); // Solid mode OFF

            szc.CompressFileDictionary(files, output);
        }

        public static void DecompressZip(string zipPath, string output)
        {
            SevenZipBase.SetLibraryPath(Path.Combine(AppDataFolder, Environment.Is64BitProcess ? "x64" : "x86", "7z.dll"));
            using var file = new SevenZip.SevenZipExtractor(zipPath);
            file.ExtractArchive(output);
        }
/*
        public static void DecompressZip(Stream zipData, string output)
        {
            SevenZipBase.SetLibraryPath(Path.Combine(AppDataFolder, Environment.Is64BitProcess ? "x64" : "x86", "7z.dll"));
            using var file = new SevenZip.SevenZipExtractor(zipData);
            file.ExtractArchive(output);
        }
*/

        /// <summary>
        /// Returns icon from specified file.
        /// </summary>
        [SupportedOSPlatform("windows")]
        public static Icon ExtractIconFromFilePath(string path)
        {
            var result = (Icon)null;
            try
            {
                result = Icon.ExtractAssociatedIcon(path);
            }
            catch (Exception)
            {
                // Cannot get image
            }

            return result;
        }

        /// <summary>
        /// Saves icon from specified shortcut.
        /// </summary>
        /// <param name="path">Path to icon file (lnk, url or exe)</param>
        /// <param name="output">Path to save .ico</param>
        [SupportedOSPlatform("windows")]
        public static bool SaveIconFromFile(string path, string output)
        {
            // Check if file exists, otherwise return false.
            if (!File.Exists(path))
                return false;

            Icon ico;

            if (path.EndsWith("lnk"))
            {
                var shortcutInfo = Shortcut.ReadFromFile(path);
                // Check if points to ico, and save if it does:
                var iconPath = shortcutInfo.ExtraData?.IconEnvironmentDataBlock?.TargetUnicode;
                if (iconPath != null && SaveImageFromIco(iconPath, output)) return true;

                // Otherwise go to main EXE and get icon from that
                path = GetShortcutTargetFile(path);
                ico = IconExtractor.ExtractIconFromExecutable(path);
                if (!SaveImageFromIco(ico, output))
                    SaveIconFromExtension("lnk", output); // Fallback
            }
            else if (path.EndsWith("url"))
            {
                // There is probably a real way of doing this, but this works.
                var urlLines = File.ReadAllLines(path);
                foreach (var l in urlLines)
                {
                    if (!l.StartsWith("IconFile")) continue;
                    var icoPath = l.Split("=")[1];
                    if (icoPath != "" && SaveImageFromIco(icoPath, output)) return true;
                }
                // OTHERWISE:
                // Get default icon for ext from that extension thing
                SaveIconFromExtension("url", output); // Fallback
            }
            else if (path.EndsWith("exe"))
            {
                try
                {
                    ico = IconExtractor.ExtractIconFromExecutable(path);
                }
                catch (Exception)
                {
                    ico = ExtractIconFromFilePath(path);
                }
                SaveImageFromIco(ico, output);
            }
            else
            {
                ico = ExtractIconFromFilePath(path);
                SaveImageFromIco(ico, output);
            }

            return true;
        }

        /// <summary>
        /// Gets and returns shortcut's Target
        /// </summary>
        /// <param name="shortcutPath">Path to .lnk shortcut file</param>
        [SupportedOSPlatform("windows")]
        public static string GetShortcutTarget(string shortcutPath = null)
        {
            if (string.IsNullOrEmpty(shortcutPath) || !File.Exists(shortcutPath))
                return null;

            if (!shortcutPath.EndsWith(".lnk", StringComparison.OrdinalIgnoreCase))
                return null;

            try
            {
                var shortcutInfo = Shortcut.ReadFromFile(shortcutPath);

                string targetPath = shortcutInfo.LinkTargetIDList.Path;

                if (!string.IsNullOrEmpty(targetPath))
                    return targetPath;
                else
                    return null;
            }
            catch (Exception ex)
            {
                Console.WriteLine($"Error retrieving shortcut target (Path: {shortcutPath}): {ex.Message}");
                return null;
            }
        }

        // Consider this a fallback for when the image can not be extracted from shortcuts.
        // Will usually be Chrome, IE or any other logo - But could also just be a blank page.
        [SupportedOSPlatform("windows")]
        private static void SaveIconFromExtension(string ext, string output)
        {
            var img = IconExtractor.GetPngFromExtension(ext, IconSizes.Jumbo);
            img?.Save(output);
        }

        [SupportedOSPlatform("windows")]
        private static bool SaveImageFromIco(Icon ico, string output)
        {
            var memoryStream = new MemoryStream();
            ico.Save(memoryStream);
            memoryStream.Seek(0, SeekOrigin.Begin);

            var mi = new MultiIcon();
            mi.Load(memoryStream);
            return SaveImageFromIco(mi, output);
        }

        [SupportedOSPlatform("windows")]
        private static bool SaveImageFromIco(string ico, string output)
        {
            var iconPath = Environment.ExpandEnvironmentVariables(ico);
            if (!File.Exists(iconPath)) return false;
            var mi = new MultiIcon();
            mi.Load(iconPath);
            return SaveImageFromIco(mi, output);
        }

        [SupportedOSPlatform("windows")]
        private static bool SaveImageFromIco(MultiIcon mi, string output)
        {
            try
            {
                var si = mi.FirstOrDefault();
                if (si == null) return false;
                var max = si.Max(i => i.Size.Height);
                var icon = si.FirstOrDefault(i => i.Size.Height == max);
                Directory.CreateDirectory(Path.GetDirectoryName(output) ?? string.Empty);
                icon?.Transparent.Save(output);
            }
            catch (Exception)
            {
                return false;
            }

            return true;
        }
        /// <summary>
        /// Removes @import url("http..."); lines from text. For the Offline Mode.
        /// </summary>
        public static string RemoveHttpImports(string input)
        {
            string pattern = @"^\s*@import\s+url\(""http[^""]*""\);\s*$";

            string result = Regex.Replace(input, pattern, "/* import removed in offline mode */", RegexOptions.Multiline | RegexOptions.IgnoreCase);

            result = Regex.Replace(result, @"^\s*$\n|\r", string.Empty, RegexOptions.Multiline);

            return result;
        }

        // TODO: Use this newly added library to create shortcuts too
        public static string GetShortcutTargetFile(string shortcut) => Shortcut.ReadFromFile(shortcut).LinkTargetIDList.Path;
        #endregion
    }
}
