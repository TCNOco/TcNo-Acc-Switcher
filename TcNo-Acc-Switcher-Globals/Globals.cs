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
using System.ComponentModel;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Net;
using System.Net.Http;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Runtime.Versioning;
using System.Security.AccessControl;
using System.Security.Cryptography;
using System.Security.Principal;
using System.Text;
using System.Text.RegularExpressions;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Win32;
using Newtonsoft.Json;

namespace TcNo_Acc_Switcher_Globals
{
    public partial class Globals
    {
#pragma warning disable CA2211 // Non-constant fields should not be visible - This is necessary due to it being a launch parameter.
        public static bool VerboseMode;
#pragma warning restore CA2211 // Non-constant fields should not be visible
        public static readonly string Version = "2022-02-10_00";

        #region INITIALISATION

        private static string _userDataFolder = "";
        public static string UserDataFolder
        {
            get
            {
                if (!string.IsNullOrEmpty(_userDataFolder)) return _userDataFolder;
                // Has not yet been initialized
                // Check if set to something different
                _userDataFolder = File.Exists(Path.Join(AppDataFolder, "userdata_path.txt"))
                    ? ReadAllLines(Path.Join(AppDataFolder, "userdata_path.txt"))[0].Trim()
                    : Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),
                        "TcNo Account Switcher\\");

                return _userDataFolder;
            }
            set
            {
                _userDataFolder = value;
                try
                {
                    File.WriteAllText(Path.Join(AppDataFolder, "userdata_path.txt"), value);
                }
                catch (Exception)
                {
                    // Failed to write to file.
                    // This setter will be unused for now at least, so this isn't really necessary.
                }
            }
        }

        public static string AppDataFolder =>
            Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location) ?? string.Empty;

        public static string OriginalWwwroot => Path.Join(AppDataFolder, "originalwwwroot");

        public static bool InstalledToProgramFiles()
        {
            var progFiles = Environment.GetFolderPath(Environment.SpecialFolder.ProgramFiles);
            var progFilesX86 = Environment.GetFolderPath(Environment.SpecialFolder.ProgramFilesX86);
            return AppDataFolder.Contains(progFiles) || AppDataFolder.Contains(progFilesX86);
        }

        public static bool HasFolderAccess(string folderPath)
        {
            try
            {
                using var fs = File.Create(Path.Combine(folderPath, Path.GetRandomFileName()), 1, FileOptions.DeleteOnClose);
                return true;
            }
            catch
            {
                return false;
            }
        }

        /// <summary>
        /// Move pre 2021-07-05 Documents userdata folder into AppData.
        /// </summary>
	    private static void OldDocumentsToAppData()
        {
            // Check to see if folder still located in My Documents (from Pre 2021-07-05)
            var oldDocuments = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.MyDocuments), "TcNo Account Switcher\\");
            if (Directory.Exists(oldDocuments)) CopyFilesRecursive(oldDocuments, UserDataFolder, true);
            RecursiveDelete(oldDocuments, false);
        }

        /// <summary>
        /// Creates data folder in AppData
        /// </summary>
        /// <param name="overwrite">Whether files should be overwritten or not (Update)</param>
        public static void CreateDataFolder(bool overwrite)
        {
            OldDocumentsToAppData();

            var wwwroot = Directory.Exists(Path.Join(AppDataFolder, "wwwroot"))
                ? Path.Join(AppDataFolder, "wwwroot")
                : Path.Join(AppDataFolder, "originalwwwroot");
            if (!Directory.Exists(UserDataFolder)) _ = Directory.CreateDirectory(UserDataFolder);
            else
            {
                var appFilesHash = GetDataFolderMd5(wwwroot) + GetDataFolderMd5(Path.Join(AppDataFolder, "themes"));
                var userFilesHash = "";

                if (Directory.Exists(Path.Join(UserDataFolder, "wwwroot")) && Directory.Exists(Path.Join(UserDataFolder, "themes")))
                {
                    userFilesHash = GetDataFolderMd5(Path.Join(UserDataFolder, "wwwroot")) +
                                    GetDataFolderMd5(Path.Join(UserDataFolder, "themes"));
                }

                if (appFilesHash != userFilesHash)
                    overwrite = true;
            }

            // Initialise folder:
            InitWwwroot(wwwroot, overwrite);
            InitFolder("themes", overwrite);
        }

        public static void ClearWebCache()
        {
            var cache = Path.Join(UserDataFolder, "EBWebView\\Default\\Cache");
            var codeCache = Path.Join(UserDataFolder, "EBWebView\\Default\\Code Cache");
            try
            {
                 RecursiveDelete(cache, true);
                 RecursiveDelete(codeCache, true);
            }
            catch (Exception)
            {
                // Clearing cache isn't REQUIRED but it's a nice-to-have.
            }
        }

        /// <summary>
        /// Recursively copies directories from install dir to documents dir.
        /// </summary>
        /// <param name="f">Folder to recursively copy</param>
        /// <param name="overwrite">Whether files should be overwritten anyways</param>
        private static void InitFolder(string f, bool overwrite)
        {
            if (overwrite || !Directory.Exists(Path.Join(UserDataFolder, f)))
                CopyFilesRecursive(Path.Join(AppDataFolder, f), Path.Join(UserDataFolder, f));
        }
        private static void InitWwwroot(string root, bool overwrite)
        {
            if (Directory.Exists(root))
                CopyFilesRecursive(root, Path.Join(UserDataFolder, "wwwroot"), overwrite);
        }

        #endregion

        private static readonly Random Rnd = new();
        public static string RandomString(int length) => new(Enumerable.Repeat("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", length).Select(s => s[Rnd.Next(s.Length)]).ToArray());

        public static string GetSha256HashString(string text) => string.IsNullOrEmpty(text)
            ? string.Empty
            : SHA512.Create().ComputeHash(Encoding.UTF8.GetBytes(text))
                .Aggregate("", (current, x) => current + $"{x:x2}");

        public static string GetUnixTime()
        {
            return ((int)DateTime.Now.Subtract(new DateTime(1970, 1, 1)).TotalSeconds).ToString();
        }

        /// <summary>
        /// Replaces the input regex string with an 'expanded' regex, if it's an enum
        /// </summary>
        public static string ExpandRegex(string regex)
        {
            Dictionary<string, string> regexDictionary = new(){
                { "EMAIL_REGEX", "(?:[a-z0-9!#$%&'*+/=?^_`{|}~-]+(?:\\.[a-z0-9!#$%&'*+/=?^_`{|}~-]+)*|\"(?:[\\x01-\\x08\\x0b\\x0c\\x0e-\\x1f\\x21\\x23-\\x5b\\x5d-\\x7f]|\\\\[\\x01-\\x09\\x0b\\x0c\\x0e-\\x7f])*\")@(?:(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\\.)+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?|\\[(?:(?:(2(5[0-5]|[0-4][0-9])|1[0-9][0-9]|[1-9]?[0-9]))\\.){3}(?:(2(5[0-5]|[0-4][0-9])|1[0-9][0-9]|[1-9]?[0-9])|[a-z0-9-]*[a-z0-9]:(?:[\\x01-\\x08\\x0b\\x0c\\x0e-\\x1f\\x21-\\x5a\\x53-\\x7f]|\\\\[\\x01-\\x09\\x0b\\x0c\\x0e-\\x7f])+)\\])" },
                { "WIN_FILEPATH_REGEX", @"[a-zA-Z]:[\\\/](?:[a-zA-Z0-9]+[\\\/])*([a-zA-Z0-9]+\.[a-zA-Z]*)" }
            };

            return regexDictionary.ContainsKey(regex) ? regexDictionary[regex] : regex;
        }
    }
}
