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
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.BattleNet;
using TcNo_Acc_Switcher_Server.Pages.Discord;
using TcNo_Acc_Switcher_Server.Pages.Epic;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using TcNo_Acc_Switcher_Server.Pages.Origin;
using TcNo_Acc_Switcher_Server.Pages.Riot;
using TcNo_Acc_Switcher_Server.Pages.Ubisoft;
using Task = System.Threading.Tasks.Task;


namespace TcNo_Acc_Switcher_Server.Pages.General
{
    public class GeneralInvocableFuncs
    {
        private static readonly Data.Settings.Discord Discord = Data.Settings.Discord.Instance;
        private static readonly Data.Settings.Epic Epic = Data.Settings.Epic.Instance;
        private static readonly Data.Settings.Steam Steam = Data.Settings.Steam.Instance;
        private static readonly Data.Settings.Origin Origin = Data.Settings.Origin.Instance;
        private static readonly Data.Settings.Ubisoft Ubisoft = Data.Settings.Ubisoft.Instance;
        private static readonly Data.Settings.BattleNet BattleNet = Data.Settings.BattleNet.Instance;
        private static readonly Data.Settings.Riot Riot = Data.Settings.Riot.Instance;

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
            return Task.FromResult(File.Exists(file) ? File.ReadAllText(file) : "");
        }

        /// <summary>
        /// JS function handler for for updates to platform's path in settings file from modal GUI
        /// </summary>
        /// <param name="file">Platform specific filename (has .json appended later)</param>
        /// <param name="path">New platform path string</param>
        [JSInvokable]
        public static void GiUpdatePath(string file, string path)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.GiUpdatePath] file={file}, path={path}");
            var settings = GeneralFuncs.LoadSettings(file);
            settings["FolderPath"] = path;
            GeneralFuncs.SaveSettings(file, settings);
            switch (file)
            {
                case "BattleNetSettings":
                    BattleNet.FolderPath = path;
                    break;
                case "EpicSettings":
                    Epic.FolderPath = path;
                    break;
                case "SteamSettings":
                    Steam.FolderPath = path;
                    break;
                case "OriginSettings":
                    Origin.FolderPath = path;
                    break;
                case "UbisoftSettings":
                    Ubisoft.FolderPath = path;
                    break;
            }
        }

        [JSInvokable]
        public static Task<string> GiConfirmAction(string action, bool value)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.GiConfirmAction] action={action.Split(":")[0]}, value={value}");
            if (!value) return Task.FromResult("");

            var split = action.Split(":");
            if (split.Length > 1)
            {
                var accName = split[1];

                if (action.StartsWith("AcceptForgetDiscordAcc:"))
                {
	                Discord.SetForgetAcc(true);
	                DiscordSwitcherFuncs.ForgetAccount(accName);
	                return Task.FromResult("refresh");
                }

                if (action.StartsWith("AcceptForgetEpicAcc:"))
                {
	                Epic.SetForgetAcc(true);
	                EpicSwitcherFuncs.ForgetAccount(accName);
	                return Task.FromResult("refresh");
                }

                if (action.StartsWith("AcceptForgetSteamAcc:"))
                {
                    Steam.SetForgetAcc(true);
                    SteamSwitcherFuncs.ForgetAccount(accName);
                    return Task.FromResult("refresh");
                }

                if (action.StartsWith("AcceptForgetOriginAcc:"))
                {
                    Origin.SetForgetAcc(true);
                    OriginSwitcherFuncs.ForgetAccount(accName);
                    return Task.FromResult("refresh");
                }

                if (action.StartsWith("AcceptForgetUbisoftAcc:"))
                {
                    Ubisoft.SetForgetAcc(true);
                    UbisoftSwitcherFuncs.ForgetAccount(accName);
                    return Task.FromResult("refresh");
                }

                if (action.StartsWith("AcceptForgetBattleNetAcc:"))
                {
                    BattleNet.SetForgetAcc(true);
                    BattleNetSwitcherFuncs.ForgetAccount(accName);
                    return Task.FromResult("refresh");
                }

                if (action.StartsWith("AcceptForgetRiotAcc:"))
                {
                    Riot.SetForgetAcc(true);
                    RiotSwitcherFuncs.ForgetAccount(accName);
                    return Task.FromResult("refresh");
                }
            }
            switch (action)
            {
                case "ClearSteamBackups": SteamSwitcherFuncs.ClearForgotten_Confirmed();
                    break;
                case "ClearBattleNetIgnored": BattleNetSwitcherFuncs.ClearIgnored_Confirmed();
                    break;
                case "RestartAsAdmin": 
                    break;
            }

            return Task.FromResult("");
        }


        [JSInvokable]
        public static Task<string> GiGetVersion() => Task.FromResult(Globals.Version);

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
            Process.Start(ps);
        }

        /// <summary>
        /// JS function handler for running showModal JS function, with input arguments.
        /// </summary>
        /// <param name="args">Argument string, containing a command to be handled later by modal</param>
        /// <returns></returns>
        public static bool ShowModal(string args)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.ShowModal] args={args}");
            return AppData.InvokeVoidAsync("showModal", args);
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
        public static bool ShowToast(string toastType, string toastMessage, string toastTitle = "", string renderTo = "body", int duration = 5000)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.ShowToast] type={toastType}, message={toastMessage}, title={toastTitle}, renderTo={renderTo}, duration={duration}");
            return AppData.InvokeVoidAsync("window.notification.new", new { type = toastType, title = toastTitle, message = toastMessage, renderTo, duration }); ;
        }
        
        /// <summary>
        /// JS function handler for changing selected username on a platform
        /// </summary>
        /// <param name="id">Unique identifier for account</param>
        /// <param name="reqName">Requested new username</param>
        /// <param name="platform">Platform to change username for unique id</param>
        [JSInvokable]
        public static void ChangeUsername(string id, string reqName, string platform)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.ChangeUsername] id:hidden, reqName:hidden, platform:{platform}");
            switch (platform)
			{
				case "BattleNet":
					BattleNetSwitcherFuncs.ChangeBTag(id, reqName);
					break;
				case "Discord":
					DiscordSwitcherFuncs.ChangeUsername(id, reqName);
					break;
				case "Epic":
                    EpicSwitcherFuncs.ChangeUsername(id, reqName);
                    break;
                case "Riot":
                    RiotSwitcherFuncs.ChangeUsername(id, reqName);
                    break;
                case "Origin":
                    OriginSwitcherFuncs.ChangeUsername(id, reqName, true);
                    break;
                case "Ubisoft":
                    UbisoftSwitcherFuncs.SetUsername(id, reqName, true);
                    break;
            }
        }


        /// <summary>
        /// Creates a shortcut to start the Account Switcher, and swap to the account related to provided SteamID.
        /// </summary>
        /// <param name="page">The account switcher the user is on</param>
        /// <param name="accId">ID of account to swap to</param>
        /// <param name="accName">Account name of account to swap to</param>
        /// <param name="args">(Optional) arguments for shortcut</param>
        [JSInvokable]
        public static void CreateShortcut(string page, string accId, string accName, string args = "")
        {
            Globals.DebugWriteLine(@"[JSInvoke:General\GeneralInvocableFuncs.CreateShortcut]");
            var platform = page;
            page = page.ToLowerInvariant();
            if (args.Length > 0 && args[0] != ':') args = $" {args}"; // Add a space before arguments if doesn't start with ':'
            var platformName = $"Switch to {accName} [{platform}]";
            var originalAccId = accId;
            switch (page)
            {
                case "steam":
                {
                    var ePersonaState = -1;
                    if (args.Length == 2) _ = int.TryParse(args[1].ToString(), out ePersonaState);
                    platformName = $"Switch to {accName} {(args.Length > 0 ? $"({SteamSwitcherFuncs.PersonaStateToString(ePersonaState)})" : "")} [{platform}]";
                    break;
                    }
                case "riot":
                    accId = accId.Replace("#", "-");
                    originalAccId = accId;
                    break;
                case "discord":
                    accId = accId.Replace("#", "-");
	                break;
			}

            var fgImg = Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\{page}\\{accId}.jpg");
            if (!File.Exists(fgImg)) fgImg = Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\profiles\\{page}\\{accId}.png");
            if (!File.Exists(fgImg))
            {
	            ShowToast("error", "Failed to find users' image.", "Can not create shortcut", "toastarea");
	            return;
            }

            var s = new Shortcut();
            s.Shortcut_Platform(
                Shortcut.Desktop, 
                platformName,
                $"+{page[0]}:{originalAccId}{args}",
                $"Switch to {accName} [{platform}] in TcNo Account Switcher", 
                true);
            s.CreateCombinedIcon(
                Path.Join(GeneralFuncs.WwwRoot(), $"\\img\\platform\\{page}.png"),
                fgImg, 
                $"{accId}.ico");
            s.TryWrite();

            _ = AppSettings.Instance.StreamerModeTriggered
                ? ShowToast("success", "Shortcut created!", "Success", "toastarea")
                : ShowToast("success", $"For: {accName}", "Shortcut created!", "toastarea");
        }

        [JSInvokable]
        public static void GiCreatePlatformShortcut(string platform)
        {
            Globals.DebugWriteLine(@$"[Func:Pages\General\GeneralInvocableFuncs.GiCreatePlatformShortcut] platform={platform}");
            var s = new Shortcut();
            s.Shortcut_Platform(Shortcut.Desktop, platform, platform.ToLowerInvariant());
            s.ToggleShortcut(true);
        }

        [JSInvokable]
        public static async Task<string> GiExportAccountList(string platform)
        {
	        Globals.DebugWriteLine(@$"[Func:Pages\General\GeneralInvocableFuncs.GiExportAccountList] platform={platform}");
	        if (!Directory.Exists(Path.Join("LoginCache", platform)))
	        {
		        ShowToast("error", "Open the platform from the list and add a few accounts first",
			        "No saved accounts", "toastarea");
		        return "";
	        }

	        var s = System.Globalization.CultureInfo.CurrentCulture.TextInfo.ListSeparator; // Different regions use different separators in csv files.

	        List<string> allAccountsTable = new();
	        if (platform == "Steam")
	        {
		        // Add headings and separator for programs like Excel
                allAccountsTable.Add("SEP=,");
                allAccountsTable.Add("Account name:,Community name:,SteamID:,VAC status:,Last login:,Saved profile image:");

		        var userAccounts = SteamSwitcherFuncs.GetSteamUsers(Steam.LoginUsersVdf());
		        var vacStatusList = new List<SteamSwitcherFuncs.VacStatus>();
		        var loadedVacCache = SteamSwitcherFuncs.LoadVacInfo(ref vacStatusList);

				foreach (var ua in userAccounts)
				{
					var VacInfo = "";
					// Get VAC/Limited info
					if (loadedVacCache)
						foreach (var vsi in vacStatusList.Where(vsi => vsi.SteamId == ua.SteamId))
						{
							if (vsi.Vac && vsi.Ltd) VacInfo += "VAC + Limited";
							else VacInfo += (vsi.Vac ? "VAC" : "") + (vsi.Ltd ? "Limited" : "");
							break;
						}
					else
					{
						VacInfo += "N/A";
					}

					var imagePath = Path.GetFullPath($"{Steam.SteamImagePath + ua.SteamId}.jpg");
					allAccountsTable.Add(ua.AccName + s + 
					                     ua.Name + s + 
					                     ua.SteamId + s + 
					                     VacInfo + s + 
					                     SteamSwitcherFuncs.UnixTimeStampToDateTime(ua.LastLogin) + s + 
                                         (File.Exists(imagePath) ? imagePath : "Missing from disk"));
                }
	        }
            else if (platform == "BattleNet")
			{
				// Add headings and separator for programs like Excel
				allAccountsTable.Add("SEP=,");
				allAccountsTable.Add("Email:,BattleTag:,Overwatch Support SR:,Overwatch DPS SR:,Overwatch Tank SR:,Saved profile image:");

                await BattleNetSwitcherFuncs.LoadProfiles();

                foreach (var ba in BattleNet.Accounts)
                {
	                var imagePath = Path.GetFullPath($"wwwroot\\img\\profiles\\battlenet\\{ba.Email}.png");
	                allAccountsTable.Add(ba.Email + s +
	                                     ba.BTag + s +
	                                     (ba.OwSupportSr != 0 ? ba.OwSupportSr : "") + s +
	                                     (ba.OwDpsSr != 0 ? ba.OwDpsSr : "") + s +
	                                     (ba.OwTankSr != 0 ? ba.OwTankSr : "") + s +
	                                     (File.Exists(imagePath) ? imagePath : "Missing from disk"));
                }
			}
            else
			{
				// Platform does not have specific details other than usernames saved.
				allAccountsTable.Add("Account name:");
				foreach (var accDirectory in Directory.GetDirectories(Path.Join("LoginCache", platform)))
				{
					allAccountsTable.Add(Path.GetFileName(accDirectory));
				}
			}

	        var outputFolder = Path.Join("wwwroot", "Exported");
	        Directory.CreateDirectory(outputFolder);

	        var outputFile = Path.Join(outputFolder, platform + ".csv");
	        await File.WriteAllLinesAsync(outputFile, allAccountsTable).ConfigureAwait(false);
	        return Path.Join("Exported", platform + ".csv");
        }

        /// <summary>
        /// 
        /// </summary>
        /// <param name="platform"></param>
        /// <returns>
        /// 0 = Does not have a password set
        /// 1 = Has a password set
        /// 2 = Password already inputted
        /// </returns>
        [JSInvokable]
        public static int GiCheckPlatformPassword(string platform)
        {
	        if (!File.Exists(Path.Join(Globals.UserDataFolder, "LoginCache", platform, "pass"))) return 0;

            // Check if password entered (Will not be empty if correct)
            return platform switch
            {
	            "Discord" => (string.IsNullOrEmpty(Data.Settings.Discord.Instance.Password) ? 1 : 2),
	            _ => 1
            };
        }

        [JSInvokable]
        public static bool GiVerifyPlatformPassword(string platform, string password)
        {
	        var passFolder = Path.Join(Globals.UserDataFolder, "LoginCache", platform);
	        Directory.CreateDirectory(passFolder);
            var passFile = Path.Join(passFolder, "pass");
	        var hashedPass = Globals.GetSha256HashString(password);

            if (File.Exists(passFile))
            {
	            if (hashedPass != File.ReadAllText(passFile)) return false; // Check if password matches the one in the file
            }
            else
            {
	            // Save password, as this is the first time.
	            File.WriteAllText(passFile, hashedPass);
			}
            
			// Passwords match. Handle password for each switcher
			switch (platform)
            {
	            case "Discord":
		            Data.Settings.Discord.Instance.Password = password;
		            break;
			}
            
			return true;
        }
    }
}
