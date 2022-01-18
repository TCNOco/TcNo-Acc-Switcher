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
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Net.Http;
using System.Text;
using System.Threading.Tasks;
using System.Drawing;
using System.Drawing.IconLib;
using System.Runtime.Versioning;
using ShellLink;
using System.Runtime.InteropServices;
using System.Text.RegularExpressions;

namespace TcNo_Acc_Switcher_Globals
{
    public partial class Globals
    {
        #region FILES


        public static string RegexSearchFile(string file, string pattern)
        {
            var m = Regex.Match(File.ReadAllText(file), pattern);
            return m.Success ? m.Value : "";
        }
        public static string RegexSearchFolder(string folder, string pattern, string wildcard = "")
        {
            var result = "";
            // Foreach file in folder (until match):
            foreach (var f in Directory.GetFiles(folder, wildcard))
            {
                result = RegexSearchFile(f, pattern);
                if (result == "") continue;
                return result;
            }

            return "";
        }

        public static bool DeleteFiles(string path, bool throwErr = false)
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

        /// <summary>
        /// Convert byte length of file to string (KB, MB, GB...)
        /// </summary>
        public static string FileLengthToString(long value, int decimalPlaces = 2)
        {
            if (value < 0) { return "-" + FileLengthToString(-value, decimalPlaces); }

            var i = 0;
            var dValue = (decimal)value;
            while (Math.Round(dValue, decimalPlaces) >= 1000)
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
        public static string DownloadString(string uri) => HClient.GetStringAsync(uri).Result;
        public static bool DownloadFile(string url, string path)
        {
            try
            {
                if (!Uri.TryCreate(url, UriKind.Absolute, out _))
                    throw new InvalidOperationException("URI is invalid.");

                var fileBytes = HClient.GetByteArrayAsync(url).Result;
                File.WriteAllBytes(path, fileBytes);
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

        // Overload for below
        public static void CopyFilesRecursive(string inputFolder, string outputFolder) =>
            CopyFilesRecursive(inputFolder, outputFolder, false);
        /// <summary>
        /// Recursively copy files and directories
        /// </summary>
        /// <param name="inputFolder">Folder to copy files recursively from</param>
        /// <param name="outputFolder">Destination folder</param>
        /// <param name="overwrite">Whether to overwrite files or not</param>
        public static void CopyFilesRecursive(string inputFolder, string outputFolder, bool overwrite)
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

        [SupportedOSPlatform("windows")]
        public static void SaveIconFromFile(string path, string output)
        {
            Icon ico;

            if (path.EndsWith("lnk"))
            {
                var shortcutInfo = Shortcut.ReadFromFile(path);
                // Check if points to ico, and save if it does:
                var iconPath = shortcutInfo.ExtraData?.IconEnvironmentDataBlock?.TargetUnicode;
                if (iconPath != null && SaveImageFromIco(iconPath, output)) return;

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
                    if (icoPath != "" && SaveImageFromIco(icoPath, output)) return;
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
                catch (Exception e)
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
            var mi = new MultiIcon();
            mi.Load(Environment.ExpandEnvironmentVariables(ico));
            return SaveImageFromIco(mi, output);
        }

        [SupportedOSPlatform("windows")]
        private static bool SaveImageFromIco(MultiIcon mi, string output)
        {
            try
            {
                var si = mi.FirstOrDefault();
                if (si == null) return false;
                var icon = si.Where(x => x.Size.Height >= 128).OrderBy(x => x.Size.Height).FirstOrDefault();
                var max = si.Max(i => i.Size.Height);
                icon = si.FirstOrDefault(i => i.Size.Height == max);
                Directory.CreateDirectory(Path.GetDirectoryName(output) ?? string.Empty);
                icon?.Transparent.Save(output);
            }
            catch (Exception)
            {
                return false;
            }

            return true;
        }

        // TODO: Use this newly added library to create shortcuts too
        public static string GetShortcutTargetFile(string shortcut) => Shortcut.ReadFromFile(shortcut).LinkTargetIDList.Path;
        #endregion
    }
}
