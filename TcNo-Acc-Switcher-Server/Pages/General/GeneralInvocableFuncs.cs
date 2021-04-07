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
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Server.Pages.Steam;

using Microsoft.Win32;
using System.Windows;
using Microsoft.AspNetCore.Components;
using Microsoft.AspNetCore.WebUtilities;


namespace TcNo_Acc_Switcher_Server.Pages.General
{
    public class GeneralInvocableFuncs
    {
        private static readonly Data.Settings.Steam Steam = Data.Settings.Steam.Instance;
        /// <summary>
        /// JS function handler for saving settings from Settings GUI page into [Platform]Settings.json file
        /// </summary>
        /// <param name="file">Platform specific filename (has .json appended later)</param>
        /// <param name="jsonString">JSON String to be saved to file, from GUI</param>
        [JSInvokable]
        public static void GiSaveSettings(string file, string jsonString)
        {
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
            var settings = GeneralFuncs.LoadSettings(file);
            settings["Path"] = path;
            GeneralFuncs.SaveSettings(file, settings);
        }

        [JSInvokable]
        public static Task<string> GiConfirmAction(string action, bool value)
        {
            Console.WriteLine(action);
            Console.WriteLine(value);

            if (action.StartsWith("AcceptForgetSteamAcc:"))
            {
                string steamId = action.Split(":")[1];
                Steam.UpdateSteamForgetAcc(true);
                SteamSwitcherFuncs.ForgetAccount(steamId);
                return Task.FromResult("refresh");
            }

            if (!value) Task.FromResult("");
            switch (action)
            {
                case "ClearSteamBackups": SteamSwitcherFuncs.ClearForgotten_Confirmed();
                    break;
            }

            return Task.FromResult("");
        }

        /// <summary>
        /// Opens a link in user's browser through Shell
        /// </summary>
        /// <param name="link">URL string</param>
        [JSInvokable]
        public static void OpenLinkInBrowser(string link)
        {
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
        /// <param name="jsRuntime">JS Runtime where JS function is run</param>
        /// <param name="args">Argument string, containing a command to be handled later by modal</param>
        /// <returns></returns>
        public static async Task ShowModal(IJSRuntime jsRuntime, string args)
        {
            await jsRuntime.InvokeAsync<string>("ShowModal", args);
        }

        /// <summary>
        /// JS function handler for showing Toast message.
        /// </summary>
        /// <param name="jsRuntime">JS Runtime where JS function is run</param>
        /// <param name="toastType">success, info, warning, error</param>
        /// <param name="toastMessage">Message to be shown in toast</param>
        /// <param name="toastTitle">(Optional) Title to be shown in toast (Empty doesn't show any title)</param>
        /// <param name="renderTo">(Optional) Part of the document to append the toast to (Empty = Default, document.body)</param>
        /// <param name="duration">(Optional) Duration to show the toast before fading</param>
        /// <returns></returns>
        public static async Task ShowToast(IJSRuntime jsRuntime, string toastType, string toastMessage, string toastTitle = "", string renderTo = "body", int duration = 5000)
        {
            await jsRuntime.InvokeVoidAsync($"window.notification.new", new { type = toastType, title = toastTitle, message = toastMessage, renderTo = renderTo, duration = duration });
        }
        
        /// <summary>
        /// For handling queries in URI
        /// </summary>
        /// <param name="navMan">Navigation Manager to get URI from</param>
        /// <param name="jsr">JSRuntime to interact with webpage</param>
        public static async void HandleQueries(NavigationManager navMan, IJSRuntime jsr)
        {
            var uri = navMan.ToAbsoluteUri(navMan.Uri);
            // Clear cache reload
            var queries = QueryHelpers.ParseQuery(uri.Query);
            // cacheReload handled in JS

            //Modal
            if (queries.TryGetValue("modal", out var modalValue))
                foreach (var stringValue in modalValue) await ShowModal(jsr, Uri.UnescapeDataString(stringValue));

            // Toast
            if (queries.TryGetValue("toast_type", out var toastType) &&
                queries.TryGetValue("toast_title", out var toastTitle) &&
                queries.TryGetValue("toast_message", out var toastMessage))
            {
                for (var i = 0; i < toastType.Count; i++)
                {
                    await GeneralInvocableFuncs.ShowToast(jsr, toastType[i], toastMessage[i], toastTitle[i], "toastarea");
                }
            }
        }
    }
}
