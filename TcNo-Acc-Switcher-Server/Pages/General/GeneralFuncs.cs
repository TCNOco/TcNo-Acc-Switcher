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
using System.Drawing;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Net.Http;
using System.Reflection;
using System.Runtime.Versioning;
using System.Security.Cryptography;
using System.Text.RegularExpressions;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.AspNetCore.WebUtilities;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;

namespace TcNo_Acc_Switcher_Server.Pages.General
{
    public class GeneralFuncs
    {

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
                Modals.ShowModal("confirm", Modals.ExtraArg.RestartAsAdmin);
            return false;
        }

        public static async Task<bool> CloseProcesses(string procName, string closingMethod)
        {
            if (!OperatingSystem.IsWindows()) return false;
            Globals.DebugWriteLine(@"Closing: " + procName);
            if (!CanKillProcess(procName, closingMethod)) return false;
            Globals.KillProcess(procName, closingMethod);

            return await WaitForClose(procName);
        }
        public static async Task<bool> CloseProcesses(List<string> procNames, string closingMethod)
        {
            if (!OperatingSystem.IsWindows()) return false;
            Globals.DebugWriteLine(@"Closing: " + string.Join(", ", procNames));
            if (!CanKillProcess(procNames, closingMethod)) return false;
            Globals.KillProcess(procNames, closingMethod);

            return await WaitForClose(procNames, closingMethod);
        }

        /// <summary>
        /// Waits for a program to close, and returns true if not running anymore.
        /// </summary>
        /// <param name="procName">Name of process to lookup</param>
        /// <returns>Whether it was closed before this function returns or not.</returns>
        public static async Task<bool> WaitForClose(string procName)
        {
            if (!OperatingSystem.IsWindows()) return false;
            var timeout = 0;
            while (Globals.ProcessHelper.IsProcessRunning(procName) && timeout < 10)
            {
                timeout++;
                await AppData.InvokeVoidAsync("updateStatus", Lang["Status_WaitingForClose", new { processName = procName, timeout, timeLimit = "10" }]);
                Thread.Sleep(1000);
            }

            if (timeout == 10)
                await GeneralInvocableFuncs.ShowToast("error", Lang["CouldNotCloseX", new { x = procName }], Lang["Error"], "toastarea");

            return timeout != 10; // Returns true if timeout wasn't reached.
        }
        public static async Task<bool> WaitForClose(List<string> procNames, string closingMethod)
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
                    await AppData.InvokeVoidAsync("updateStatus", Lang["Status_WaitingForMultipleClose", new { processName = procToClose[0], count = appCount, timeout, timeLimit = "10" }]);
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
            await GeneralInvocableFuncs.ShowToast("error", Lang["CouldNotCloseX", new { x = string.Join(", ", leftOvers.ToArray()) }], Lang["Error"], "toastarea");
            return false; // Returns true if timeout wasn't reached.
        }

        /// <summary>
        /// Restart the TcNo Account Switcher as Admin
        /// Launches either the Server or main exe, depending on what's currently running.
        /// </summary>
        public static void RestartAsAdmin(string args = "")
        {
            var fileName = "TcNo-Acc-Switcher_main.exe";
            if (!AppData.TcNoClientApp) fileName = Assembly.GetEntryAssembly()?.Location.Replace(".dll", ".exe") ?? "TcNo-Acc-Switcher-Server_main.exe";
            else
            {
                // Is client app, but could be developing >> No _main just yet.
                if (!File.Exists(Path.Join(Globals.AppDataFolder, fileName)) && File.Exists(Path.Join(Globals.AppDataFolder, "TcNo-Acc-Switcher.exe")))
                    fileName = Path.Combine(Globals.AppDataFolder, "TcNo-Acc-Switcher.exe");
            }

            var proc = new ProcessStartInfo
            {
                WorkingDirectory = Globals.AppDataFolder,
                FileName = fileName,
                UseShellExecute = true,
                Arguments = args,
                Verb = "runas"
            };
            try
            {
                _ = Process.Start(proc);
                Environment.Exit(0);
            }
            catch (Exception ex)
            {
                Globals.WriteToLog(@"This program must be run as an administrator!" + Environment.NewLine + ex);
                Environment.Exit(0);
            }
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
        public static async Task<JToken> ReadJsonFile(string path)
        {
            Globals.DebugWriteLine($@"[Func:General\GeneralFuncs.ReadJsonFile] path={path}");
            JToken jToken             = null;

            if (Globals.TryReadJsonFile(path, ref jToken)) return jToken;

            await GeneralInvocableFuncs.ShowToast("error", Lang["CouldNotReadFile", new { file = path }], renderTo: "toastarea");
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

        public class CrowdinResponse
        {
            [JsonProperty("ProofReaders")]
            public SortedDictionary<string, string> ProofReaders { get; set; }

            [JsonProperty("Translators")]
            public List<string> Translators { get; set; }
        }


        /// <summary>
        /// Returns an object with a list of all translators, and proofreaders with their languages.
        /// </summary>
        public static CrowdinResponse CrowdinList()
        {
            try
            {
                var html = new HttpClient().GetStringAsync(
                    "https://tcno.co/Projects/AccSwitcher/api/crowdinNew/").Result;
                var resp = JsonConvert.DeserializeObject<CrowdinResponse>(html);
                if (resp is null)
                    return new CrowdinResponse();

                resp.Translators.Sort();

                var expandedProofreaders = new SortedDictionary<string, string>();
                foreach (var proofReader in resp?.ProofReaders)
                {
                    expandedProofreaders.Add(proofReader.Key,
                        string.Join(", ",
                            proofReader.Value.Split(',').Select(lang => new CultureInfo(lang).DisplayName)
                                .ToList()));
                }

                resp.ProofReaders = expandedProofreaders;
                return resp;
            }
            catch (Exception e)
            {
                // Handle website not loading or JObject not loading properly
                Globals.WriteToLog("Failed to load Crowdin users", e);
                _ = GeneralInvocableFuncs.ShowToast("error", Lang["Crowdin_Fail"], renderTo: "toastarea");
                return new CrowdinResponse();
            }
        }
        #endregion

        #region SWITCHER_FUNCTIONS

        public static async Task HandleFirstRender(bool firstRender, string platform)
        {
            if (firstRender)
            {
                AppData.WindowTitle = Lang["Title_AccountsList", new { platform }];
                // Handle Streamer Mode notification
                if (AppSettings.StreamerModeEnabled && AppSettings.StreamerModeTriggered)
                    await GeneralInvocableFuncs.ShowToast("info", Lang["Toast_StreamerModeHint"], Lang["Toast_StreamerModeTitle"], "toastarea");

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
                        await BasicSwitcherFuncs.LoadProfiles();
                        Data.Settings.Basic.SaveSettings();
                        break;
                }

                // Handle queries and invoke status "Ready"
                await HandleQueries();
                await AppData.InvokeVoidAsync("updateStatus", Lang["Done"]);
            }
        }

        /// <summary>
        /// For handling queries in URI
        /// </summary>
        public static async Task<bool> HandleQueries()
        {
            Globals.DebugWriteLine(@"[JSInvoke:General\GeneralFuncs.HandleQueries]");
            var uri = AppData.ActiveNavMan.ToAbsoluteUri(AppData.ActiveNavMan.Uri);
            // Clear cache reload
            var queries = QueryHelpers.ParseQuery(uri.Query);
            // cacheReload handled in JS

            // Toast
            if (!queries.TryGetValue("toast_type", out var toastType) ||
                !queries.TryGetValue("toast_title", out var toastTitle) ||
                !queries.TryGetValue("toast_message", out var toastMessage)) return true;
            for (var i = 0; i < toastType.Count; i++)
            {
                try
                {
                    await GeneralInvocableFuncs.ShowToast(toastType[i], toastMessage[i], toastTitle[i], "toastarea");
                    await AppData.InvokeVoidAsync("removeUrlArgs", "toast_type,toast_title,toast_message");
                }
                catch (TaskCanceledException e)
                {
                    Globals.WriteToLog(e.ToString());
                }
            }

            return true;
        }


        public static async Task<string> ExportAccountList()
        {
            var platform = AppSettings.GetPlatform(AppData.SelectedPlatform);
            Globals.DebugWriteLine(@$"[Func:Pages\General\GeneralInvocableFuncs.GiExportAccountList] platform={platform}");
            if (!Directory.Exists(Path.Join("LoginCache", platform.SafeName)))
            {
                await GeneralInvocableFuncs.ShowToast("error", Lang["Toast_AddAccountsFirst"], Lang["Toast_AddAccountsFirstTitle"], "toastarea");
                return "";
            }

            var s = CultureInfo.CurrentCulture.TextInfo.ListSeparator; // Different regions use different separators in csv files.

            await BasicStats.SetCurrentPlatform(platform.Name);

            List<string> allAccountsTable = new();
            if (platform.Name == "Steam")
            {
                // Add headings and separator for programs like Excel
                allAccountsTable.Add($"SEP={s}");
                allAccountsTable.Add($"Account name:{s}Community name:{s}SteamID:{s}VAC status:{s}Last login:{s}Saved profile image:{s}Stats game:{s}Stat name:{s}Stat value:");

                AppData.SteamUsers = await SteamSwitcherFuncs.GetSteamUsers(Data.Settings.Steam.LoginUsersVdf());
                // Load cached ban info
                SteamSwitcherFuncs.LoadCachedBanInfo();

                foreach (var su in AppData.SteamUsers)
                {
                    var banInfo = "";
                    if (su.Vac && su.Limited) banInfo += "VAC + Limited";
                    else banInfo += (su.Vac ? "VAC" : "") + (su.Limited ? "Limited" : "");

                    var imagePath = Path.GetFullPath($"{Data.Settings.Steam.SteamImagePath + su.SteamId}.jpg");

                    allAccountsTable.Add(su.AccName + s +
                                         su.Name + s +
                                         su.SteamId + s +
                                         banInfo + s +
                                         Globals.UnixTimeStampToDateTime(su.LastLogin) + s +
                                         (File.Exists(imagePath) ? imagePath : "Missing from disk") + s +
                                         BasicStats.GetGameStatsString(su.SteamId, s));
                }
            }
            else
            {
                // Add headings and separator for programs like Excel
                allAccountsTable.Add($"SEP={s}");
                // Platform does not have specific details other than usernames saved.
                allAccountsTable.Add($"Account name:{s}Stats game:{s}Stat name:{s}Stat value:");
                foreach (var accDirectory in Directory.GetDirectories(Path.Join("LoginCache", platform.SafeName)))
                {
                    allAccountsTable.Add(Path.GetFileName(accDirectory) + s +
                                         BasicStats.GetGameStatsString(accDirectory, s, true));
                }
            }

            var outputFolder = Path.Join("wwwroot", "Exported");
            _ = Directory.CreateDirectory(outputFolder);

            var outputFile = Path.Join(outputFolder, platform + ".csv");
            await File.WriteAllLinesAsync(outputFile, allAccountsTable).ConfigureAwait(false);
            return Path.Join("Exported", platform + ".csv");
        }

        #endregion
    }
}
