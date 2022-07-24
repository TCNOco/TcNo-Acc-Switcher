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
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.JSInterop;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using TcNo_Acc_Switcher_Server.State;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.Pages.General;

public class GeneralInvocableFuncs
{
    [Inject] private IWindowSettings WindowSettings { get; set; }
    [Inject] private IJSRuntime JsRuntime { get; set; }
    [Inject] private AppState AppState { get; set; }
    [Inject] private ILang Lang { get; set; }
    [Inject] private TemplatedPlatformState TemplatedPlatformState { get; set; }

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
    public void GiSaveOrder(string jsonString)
    {
        Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.GiSaveOrder]  jsonString.length={jsonString.Length}");
        var arr = JArray.Parse(jsonString);
        for (var i = 0; i < arr.Count; i++)
        {
            var plat = WindowSettings.Platforms.FirstOrDefault(x => x.SafeName == arr[i].ToString());
            if (plat is null) continue;
            plat.DisplayIndex = i;
        }

        WindowSettings.Save();
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

    [JSInvokable]
    public string GiLocale(string k) => Lang[k];

    [JSInvokable]
    public string GiLocaleObj(string k, object obj) => Lang[k, obj];

    [JSInvokable]
    public static string GiGetCleanFilePath(string f) => Globals.GetCleanFilePath(f);
}