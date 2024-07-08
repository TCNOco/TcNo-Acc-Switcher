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
using System.IO;
using System.Linq;
using System.Security.Cryptography;
using System.Text;
using System.Text.RegularExpressions;

namespace TcNo_Acc_Switcher_Globals
{
    public partial class Globals
    {
        #region FOLDERS
        /// <summary>
        ///  Gets a hash for the provided Data folder (Ignores specific folders in it).
        /// </summary>
        /// <param name="path">Directory to hash</param>
        /// <returns>Hash string</returns>
        public static string GetDataFolderMd5(string path)
        {
            if (!Directory.Exists(path)) return "ENOTFOUND";
            // assuming you want to include nested folders
            var files = Directory.GetFiles(path, "*.*", SearchOption.AllDirectories).OrderBy(p => p).ToList();

            var md5 = MD5.Create();
            byte[] lastBytes = null;
            foreach (var file in files)
            {
                if (lastBytes != null)
                {
                    _ = md5.TransformBlock(lastBytes, 0, lastBytes.Length, lastBytes, 0);
                }

                if (file.Contains("profiles")) // Ignore user-customisable folders.
                    continue;

                var pathBytes = Encoding.UTF8.GetBytes(file[(path.Length + 1)..].ToLower());
                _ = md5.TransformBlock(pathBytes, 0, pathBytes.Length, pathBytes, 0);

                try
                {
                    lastBytes = File.ReadAllBytes(file);
                }
                catch (IOException)
                {
                    // File is in use
                }
            }

            if (lastBytes != null)
                _ = md5.TransformFinalBlock(lastBytes, 0, lastBytes.Length);

            return BitConverter.ToString(md5.Hash ?? Array.Empty<byte>()).Replace("-", "").ToLower();
        }
        /// <summary>
        /// Gets a file's MD5 Hash
        /// </summary>
        /// <param name="filePath">Path to file to get hash of</param>
        /// <returns></returns>
        public static string GetFileMd5(string filePath)
        {
            if (!File.Exists(filePath)) return "ENOTFOUND";
            using var md5 = MD5.Create();
            using var stream = File.OpenRead(filePath);
            return stream.Length != 0 ? BitConverter.ToString(md5.ComputeHash(stream)).Replace("-", "").ToLowerInvariant() : "0";
        }

        public static bool RecursiveDelete(string baseDir, bool keepFolders, bool throwOnError = false) =>
            RecursiveDelete(new DirectoryInfo(baseDir), keepFolders, throwOnError);
        public static bool RecursiveDelete(DirectoryInfo baseDir, bool keepFolders, bool throwOnError = false)
        {
            if (!baseDir.Exists)
                return true;

            try
            {
                foreach (var dir in baseDir.EnumerateDirectories())
                {
                    RecursiveDelete(dir, keepFolders);
                }
                var files = baseDir.GetFiles();
                foreach (var file in files)
                {
                    if (!file.Exists) continue;
                    file.IsReadOnly = false;
                    try
                    {
                        file.Delete();
                    }
                    catch (Exception e)
                    {
                        if (throwOnError)
                        {
                            WriteToLog("Recursive Delete could not delete file.", e);
                        }
                    }
                }

                if (keepFolders) return true;
                try
                {
                    baseDir.Delete();
                }
                catch (Exception e)
                {
                    if (throwOnError)
                    {
                        WriteToLog("Recursive Delete could not delete folder.", e);
                    }
                }
                return true;
            }
            catch (UnauthorizedAccessException e)
            {
                WriteToLog("RecursiveDelete failed", e);
                return false;
            }
        }

        /// <summary>
        /// Remove illegal characters from file path string
        /// </summary>
        public static string GetCleanFilePath(string f)
        {
            var regexSearch = new string(Path.GetInvalidFileNameChars()) + new string(Path.GetInvalidPathChars());
            var r = new Regex($"[{Regex.Escape(regexSearch)}]");
            return r.Replace(f, "");
        }
        // Adapted from: https://docs.microsoft.com/en-us/dotnet/standard/io/how-to-copy-directories?
        public static void CopyDirectory(string sourceDir, string destinationDir, bool recursive)
        {
            var dir = new DirectoryInfo(sourceDir);
            if (!dir.Exists) return;;

            var dirs = dir.GetDirectories();
            Directory.CreateDirectory(destinationDir);
            foreach (var file in dir.GetFiles())
            {
                var targetFilePath = Path.Combine(destinationDir, file.Name);
                file.CopyTo(targetFilePath);
            }

            if (!recursive) return;
            foreach (var subDir in dirs)
            {
                var newDestinationDir = Path.Combine(destinationDir, subDir.Name);
                CopyDirectory(subDir.FullName, newDestinationDir, true);
            }
        }

        #endregion
    }
}
