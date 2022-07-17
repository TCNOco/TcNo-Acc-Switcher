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
using System.Globalization;
using System.IO;
using System.Net.Http;
using System.Reflection;
using System.Runtime.Versioning;
using System.Threading.Tasks;
using System.Web;
using Microsoft.JSInterop;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.Basic;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using BasicSettings = TcNo_Acc_Switcher_Server.Data.Settings.Basic;
using SteamSettings = TcNo_Acc_Switcher_Server.Data.Settings.Steam;

namespace TcNo_Acc_Switcher_Server.Pages.General
{
    public class GeneralInvocableFuncs
    {
        private static readonly Lang Lang = Lang.Instance;

        /// <summary>
        /// JS function handler for saving settings from Settings GUI page into [Platform]Settings.json file
        /// </summary>
        /// <param name="file">Platform specific filename (has .json appended later)</param>
        /// <param name="jsonString">JSON String to be saved to file, from GUI</param>
        [JSInvokable]
        public static void GiSaveSettings(string file, string jsonString)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.GiSaveSettings] file={file}, jsonString.length={jsonString.Length}");
            GeneralFuncs.SaveSettings(file, JObject.Parse(jsonString));
        }

        [JSInvokable]
        public static void GiSaveOrder(string file, string jsonString)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.GiSaveOrder] file={file}, jsonString.length={jsonString.Length}");
            GeneralFuncs.SaveOrder(file, JArray.Parse(jsonString));
        }

        /// <summary>
        /// JS function handler for returning JObject of settings from [Platform]Settings.json file
        /// </summary>
        /// <param name="file">Platform specific filename (has .json appended later)</param>
        /// <returns>JObject of settings, to be loaded into GUI</returns>
        [JSInvokable]
        public static Task GiLoadSettings(string file)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.GiLoadSettings] file={file}");
            return Task.FromResult(GeneralFuncs.LoadSettings(file).ToString());
        }

        /// <summary>
        /// JS function handler for returning string contents of a *.* file
        /// </summary>
        /// <param name="file">Name of file to be read and contents returned in string format</param>
        /// <returns>string of file contents</returns>
        [JSInvokable]
        public static Task GiFileReadAllText(string file)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.GiFileReadAllText] file={file}");
            return Task.FromResult(File.Exists(file) ? Globals.ReadAllText(file) : "");
        }

        /// <summary>
        /// Opens a link in user's browser through Shell
        /// </summary>
        /// <param name="link">URL string</param>
        [JSInvokable]
        public static void OpenLinkInBrowser(string link)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.OpenLinkInBrowser] link={link}");
            var ps = new ProcessStartInfo(link)
            {
                UseShellExecute = true,
                Verb = "open"
            };
            _ = Process.Start(ps);
        }

        /// <summary>
        /// JS function handler for running showModal JS function, with input arguments.
        /// </summary>
        /// <param name="args">Argument string, containing a command to be handled later by modal</param>
        /// <returns></returns>
        public static async Task<bool> ShowModal(string args)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.ShowModal] args={args}");
            return await AppData.InvokeVoidAsync("showModal", args);
        }

        /// <summary>
        /// JS function handler for showing Toast message.
        /// </summary>
        /// <param name="toastType">success, info, warning, error</param>
        /// <param name="toastMessage">Message to be shown in toast</param>
        /// <param name="toastTitle">(Optional) Title to be shown in toast (Empty doesn't show any title)</param>
        /// <param name="renderTo">(Optional) Part of the document to append the toast to (Empty = Default, document.body)</param>
        /// <param name="duration">(Optional) Duration to show the toast before fading</param>
        /// <returns></returns>
        public static async Task<bool> ShowToast(string toastType, string toastMessage, string toastTitle = "", string renderTo = "body", int duration = 5000)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.ShowToast] type={toastType}, message={toastMessage}, title={toastTitle}, renderTo={renderTo}, duration={duration}");
            return await AppData.InvokeVoidAsync("window.notification.new", new { type = toastType, title = toastTitle, message = toastMessage, renderTo, duration });
        }

        /// <summary>
        /// Creates a shortcut to start the Account Switcher, and swap to the account related.
        /// </summary>
        /// <param name="args">(Optional) arguments for shortcut</param>
        [JSInvokable]
        [SupportedOSPlatform("windows")]
        public static async Task CreateShortcut(string args = "")
        {
            Globals.DebugWriteLine(@"[JSInvoke:General\GeneralInvocableFuncs.CreateShortcut]");
            if (args.Length > 0 && args[0] != ':') args = $" {args}"; // Add a space before arguments if doesn't start with ':'
            string platformName;
            var primaryPlatformId = "" + AppData.CurrentSwitcher[0];
            var bgImg = Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\platform\\{AppData.CurrentSwitcherSafe}.svg");
            string currentPlatformImgPath, currentPlatformImgPathOverride;
            switch (AppData.CurrentSwitcher)
            {
                case "Steam":
                    currentPlatformImgPath = Path.Join(GeneralFuncs.WwwRoot(), "\\img\\platform\\Steam.svg");
                    currentPlatformImgPathOverride = Path.Join(GeneralFuncs.WwwRoot(), "\\img\\platform\\Steam.png");
                    var ePersonaState = -1;
                    if (args.Length == 2) _ = int.TryParse(args[1].ToString(), out ePersonaState);
                    platformName = $"Switch to {AppData.SelectedAccount.DisplayName} {(args.Length > 0 ? $"({SteamSwitcherFuncs.PersonaStateToString(ePersonaState)})" : "")} [{AppData.CurrentSwitcher}]";
                    break;
                default:
                    currentPlatformImgPath = Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\platform\\{CurrentPlatform.SafeName}.svg");
                    currentPlatformImgPathOverride = Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\platform\\{CurrentPlatform.SafeName}.png");
                    primaryPlatformId = CurrentPlatform.PrimaryId;
                    platformName = $"Switch to {AppData.SelectedAccount.DisplayName} [{AppData.CurrentSwitcher}]";
                    break;
            }

            if (File.Exists(currentPlatformImgPathOverride))
                bgImg = currentPlatformImgPathOverride;
            else if (File.Exists(currentPlatformImgPath))
                bgImg = currentPlatformImgPath;
            else if (File.Exists(Path.Join(GeneralFuncs.WwwRoot(), "\\img\\BasicDefault.png")))
                bgImg = Path.Join(GeneralFuncs.WwwRoot(), "\\img\\BasicDefault.png");


            var fgImg = Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\{AppData.CurrentSwitcherSafe}\\{AppData.SelectedAccountId}.jpg");
            if (!File.Exists(fgImg)) fgImg = Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\{AppData.CurrentSwitcherSafe}\\{AppData.SelectedAccountId}.png");
            if (!File.Exists(fgImg))
            {
                await ShowToast("error", Lang["Toast_CantFindImage"], Lang["Toast_CantCreateShortcut"], "toastarea");
                return;
            }

            var s = new Shortcut();
            _ = s.Shortcut_Platform(
                Shortcut.Desktop,
                platformName,
                $"+{primaryPlatformId}:{AppData.SelectedAccountId}{args}",
                $"Switch to {AppData.SelectedAccount.DisplayName} [{AppData.CurrentSwitcher}] in TcNo Account Switcher",
                true);
            await s.CreateCombinedIcon(bgImg, fgImg, $"{AppData.SelectedAccountId}.ico");
            s.TryWrite();

            if (AppSettings.StreamerModeTriggered)
                await ShowToast("success", Lang["Toast_ShortcutCreated"], Lang["Success"], "toastarea");
            else
                await ShowToast("success", Lang["ForName", new { name = AppData.SelectedAccount.DisplayName }], Lang["Toast_ShortcutCreated"], "toastarea");
        }

        [JSInvokable]
        public static async Task GiCreatePlatformShortcut(string platform)
        {
            Globals.DebugWriteLine(@$"[Func:Pages\General\GeneralInvocableFuncs.GiCreatePlatformShortcut] platform={platform}");
            var platId = platform.ToLowerInvariant();
            platform = BasicPlatforms.PlatformFullName(platform); // If it's a basic platform

            var s = new Shortcut();
            _ = s.Shortcut_Platform(Shortcut.Desktop, platform, platId);
            s.ToggleShortcut(true);

            await ShowToast("success", Lang["Toast_ShortcutCreated"], Lang["Success"], "toastarea");
        }

        [JSInvokable]
        public static async Task<string> GiExportAccountList(string platform)
        {
            Globals.DebugWriteLine(@$"[Func:Pages\General\GeneralInvocableFuncs.GiExportAccountList] platform={platform}");
            platform = BasicPlatforms.PlatformFullName(platform);
            if (!Directory.Exists(Path.Join("LoginCache", platform)))
            {
                await ShowToast("error", Lang["Toast_AddAccountsFirst"], Lang["Toast_AddAccountsFirstTitle"], "toastarea");
                return "";
            }

            var s = CultureInfo.CurrentCulture.TextInfo.ListSeparator; // Different regions use different separators in csv files.

            await BasicStats.SetCurrentPlatform(platform);

            List<string> allAccountsTable = new();
            if (platform == "Steam")
            {
                // Add headings and separator for programs like Excel
                allAccountsTable.Add($"SEP={s}");
                allAccountsTable.Add($"Account name:{s}Community name:{s}SteamID:{s}VAC status:{s}Last login:{s}Saved profile image:{s}Stats:");

                AppData.SteamUsers = await SteamSwitcherFuncs.GetSteamUsers(SteamSettings.LoginUsersVdf());
                // Load cached ban info
                SteamSwitcherFuncs.LoadCachedBanInfo();

                foreach (var su in AppData.SteamUsers)
                {
                    var banInfo = "";
                    if (su.Vac && su.Limited) banInfo += "VAC + Limited";
                    else banInfo += (su.Vac ? "VAC" : "") + (su.Limited ? "Limited" : "");

                    var imagePath = Path.GetFullPath($"{SteamSettings.SteamImagePath + su.SteamId}.jpg");

                    allAccountsTable.Add(su.AccName + s +
                                         su.Name + s +
                                         su.SteamId + s +
                                         banInfo + s +
                                         SteamSwitcherFuncs.UnixTimeStampToDateTime(su.LastLogin) + s +
                                         (File.Exists(imagePath) ? imagePath : "Missing from disk") + s +
                                         BasicStats.GetGameStatsString(su.SteamId));
                }
            }
            else
            {
                // Platform does not have specific details other than usernames saved.
                allAccountsTable.Add($"Account name:{s}Stats:");
                foreach (var accDirectory in Directory.GetDirectories(Path.Join("LoginCache", platform)))
                {
                    allAccountsTable.Add(Path.GetFileName(accDirectory) + s +
                                         BasicStats.GetGameStatsString(accDirectory));
                }
            }

            var outputFolder = Path.Join("wwwroot", "Exported");
            _ = Directory.CreateDirectory(outputFolder);

            var outputFile = Path.Join(outputFolder, platform + ".csv");
            await File.WriteAllLinesAsync(outputFile, allAccountsTable).ConfigureAwait(false);
            return Path.Join("Exported", platform + ".csv");
        }

        [JSInvokable]
        public static string PlatformUserModalCopyText() => CurrentPlatform.GetUserModalCopyText;
        [JSInvokable]
        public static string PlatformHintText() => CurrentPlatform.GetUserModalHintText();

        [JSInvokable]
        public static string GiLocale(string k) => Lang.Instance[k];

        [JSInvokable]
        public static string GiLocaleObj(string k, object obj) => Lang.Instance[k, obj];


        [JSInvokable]
        public static string GiCurrentBasicPlatform(string platform)
        {
            if (platform == "Basic")
                return CurrentPlatform.FullName;
            return BasicPlatforms.PlatformExists(platform)
                ? BasicPlatforms.PlatformFullName(platform)
                : platform;
        }

        [JSInvokable]
        public static string GiCurrentBasicPlatformExe(string platform)
        {
            // EXE name from current platform by name:
            if (platform == "Basic")
                return CurrentPlatform.ExeName;
            return BasicPlatforms.PlatformExists(platform)
                ? BasicPlatforms.GetExeNameFromPlatform(platform)
                : platform;
        }

        [JSInvokable]
        public static string GiGetCleanFilePath(string f) => Globals.GetCleanFilePath(f);

        [JSInvokable]
        public static void ImportNewImage(string o)
        {
            var f = JObject.Parse(o);
            var imageDest = Path.Join(Globals.UserDataFolder, "wwwroot", HttpUtility.UrlDecode(f.Value<string>("dest")));
            Globals.CopyFile(f.Value<string>("path"), imageDest);
            AppData.ReloadPage();
        }
    }
}
