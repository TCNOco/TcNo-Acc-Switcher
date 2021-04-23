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
using System.Diagnostics;
using System.IO;
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using Microsoft.AspNetCore.WebUtilities;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.BattleNet;
using TcNo_Acc_Switcher_Server.Pages.Origin;
using TcNo_Acc_Switcher_Server.Pages.Ubisoft;


namespace TcNo_Acc_Switcher_Server.Pages.General
{
    public class GeneralInvocableFuncs
    {
        private static readonly Data.Settings.Steam Steam = Data.Settings.Steam.Instance;
        private static readonly Data.Settings.Origin Origin = Data.Settings.Origin.Instance;
        private static readonly Data.Settings.Ubisoft Ubisoft = Data.Settings.Ubisoft.Instance;
        private static readonly Data.Settings.BattleNet BattleNet = Data.Settings.BattleNet.Instance;

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
                case "SteamSettings":
                    Steam.FolderPath = path;
                    break;
                case "OriginSettings":
                    Origin.FolderPath = path;
                    break;
                case "UbisoftSettings":
                    Origin.FolderPath = path;
                    break;
            }
        }

        [JSInvokable]
        public static Task<string> GiConfirmAction(string action, bool value)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.GiConfirmAction] action={action}, value={value}");
            Console.WriteLine(action);
            Console.WriteLine(value);
            if (!value) return Task.FromResult("");

            if (action.StartsWith("AcceptForgetSteamAcc:"))
            {
                var steamId = action.Split(":")[1];
                Steam.SetForgetAcc(true);
                SteamSwitcherFuncs.ForgetAccount(steamId);
                return Task.FromResult("refresh");
            }

            if (action.StartsWith("AcceptForgetOriginAcc:"))
            {
                var accName = action.Split(":")[1];
                Origin.SetForgetAcc(true);
                OriginSwitcherFuncs.ForgetAccount(accName);
                return Task.FromResult("refresh");
            }

            if (action.StartsWith("AcceptForgetUbisoftAcc:"))
            {
                var accName = action.Split(":")[1];
                Ubisoft.SetForgetAcc(true);
                UbisoftSwitcherFuncs.ForgetAccount(accName);
                return Task.FromResult("refresh");
            }

            if (action.StartsWith("AcceptForgetBattleNetAcc:"))
            {
                var accName = action.Split(":")[1];
                BattleNet.SetForgetAcc(true);
                BattleNetSwitcherFuncs.ForgetAccount(accName);
                return Task.FromResult("refresh");
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
        public static Task<string> GiGetVersion() => Task.FromResult(AppSettings.Instance.Version);

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
        /// JS function handler for running ShowModal JS function, with input arguments.
        /// </summary>
        /// <param name="args">Argument string, containing a command to be handled later by modal</param>
        /// <returns></returns>
        public static async Task ShowModal(string args)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.ShowModal] args={args}");
            await AppData.ActiveIJsRuntime.InvokeAsync<string>("ShowModal", args);
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
        public static async Task ShowToast(string toastType, string toastMessage, string toastTitle = "", string renderTo = "body", int duration = 5000)
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.ShowToast] type={toastType}, message={toastMessage}, title={toastTitle}, renderTo={renderTo}, duration={duration}");
            await AppData.ActiveIJsRuntime.InvokeVoidAsync($"window.notification.new", new { type = toastType, title = toastTitle, message = toastMessage, renderTo, duration });
        }
        
        /// <summary>
        /// For handling queries in URI
        /// </summary>
        public static async void HandleQueries()
        {
            Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.HandleQueries]");
            var uri = AppData.ActiveNavMan.ToAbsoluteUri(AppData.ActiveNavMan.Uri);
            // Clear cache reload
            var queries = QueryHelpers.ParseQuery(uri.Query);
            // cacheReload handled in JS

            //Modal
            if (queries.TryGetValue("modal", out var modalValue))
                foreach (var stringValue in modalValue) await ShowModal(Uri.UnescapeDataString(stringValue));

            // Toast
            if (!queries.TryGetValue("toast_type", out var toastType) ||
                !queries.TryGetValue("toast_title", out var toastTitle) ||
                !queries.TryGetValue("toast_message", out var toastMessage)) return;
            for (var i = 0; i < toastType.Count; i++)
            {
                try
                {
                    await ShowToast(toastType[i], toastMessage[i], toastTitle[i], "toastarea");
                }
                catch (TaskCanceledException e)
                {
                    Console.WriteLine(e);
                }
            }
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
            Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.ChangeUsername] id:{id}, reqName:{reqName}, platform:{platform}");
            switch (platform)
            {
                case "BattleNet":
                    BattleNetSwitcherFuncs.ChangeBTag(id, reqName);
                    break;
                case "Origin":
                    OriginSwitcherFuncs.ChangeUsername(id, reqName, true);
                    break;
                case "Ubisoft":
                    UbisoftSwitcherFuncs.SetUsername(id, reqName, true);
                    break;
            }
        }
    }
}
