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
using System.Threading.Tasks;
using Microsoft.JSInterop;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;

namespace TcNo_Acc_Switcher_Server.Pages.General;

public class GeneralInvocableFuncs
{
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
    public static void GiSaveOrder(string jsonString)
    {
        Globals.DebugWriteLine($@"[JSInvoke:General\GeneralInvocableFuncs.GiSaveOrder]  jsonString.length={jsonString.Length}");
        var arr = JArray.Parse(jsonString);
        for (var i = 0; i < arr.Count; i++)
        {
            var plat = WindowSettings.Platforms.FirstOrDefault(x => x.SafeName == arr[i].ToString());
            if (plat is null) continue;
            plat.DisplayIndex = i;
        }

        WindowSettings.SaveSettings();
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
        return await JsRuntime.InvokeVoidAsync("showModal", args);
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
        return await JsRuntime.InvokeVoidAsync("window.notification.new", new { type = toastType, title = toastTitle, message = toastMessage, renderTo, duration });
    }

    /// <summary>
    /// Creates a shortcut to start the Account Switcher, and swap to the account related.
    /// </summary>
    /// <param name="args">(Optional) arguments for shortcut</param>
    public static async Task CreateShortcut(string args = "")
    {
        if (!OperatingSystem.IsWindows()) return;
        Globals.DebugWriteLine(@"[JSInvoke:General\GeneralInvocableFuncs.CreateShortcut]");
        if (args.Length > 0 && args[0] != ':') args = $" {args}"; // Add a space before arguments if doesn't start with ':'
        string platformName;
        var primaryPlatformId = "" + AppState.Switcher.CurrentSwitcher[0];
        var bgImg = Path.Join(Globals.WwwRoot, $"\\img\\platform\\{AppState.Switcher.CurrentSwitcherSafe}.svg");
        string currentPlatformImgPath, currentPlatformImgPathOverride;
        switch (AppState.Switcher.CurrentSwitcher)
        {
            case "Steam":
                currentPlatformImgPath = Path.Join(Globals.WwwRoot, "\\img\\platform\\Steam.svg");
                currentPlatformImgPathOverride = Path.Join(Globals.WwwRoot, "\\img\\platform\\Steam.png");
                var ePersonaState = -1;
                if (args.Length == 2) _ = int.TryParse(args[1].ToString(), out ePersonaState);
                platformName = $"Switch to {AppState.Switcher.SelectedAccount.DisplayName} {(args.Length > 0 ? $"({SteamSwitcherFuncs.PersonaStateToString(ePersonaState)})" : "")} [{AppState.Switcher.CurrentSwitcher}]";
                break;
            default:
                currentPlatformImgPath = Path.Join(Globals.WwwRoot, $"\\img\\platform\\{CurrentPlatform.SafeName}.svg");
                currentPlatformImgPathOverride = Path.Join(Globals.WwwRoot, $"\\img\\platform\\{CurrentPlatform.SafeName}.png");
                primaryPlatformId = CurrentPlatform.PrimaryId;
                platformName = $"Switch to {AppState.Switcher.SelectedAccount.DisplayName} [{AppState.Switcher.CurrentSwitcher}]";
                break;
        }

        if (File.Exists(currentPlatformImgPathOverride))
            bgImg = currentPlatformImgPathOverride;
        else if (File.Exists(currentPlatformImgPath))
            bgImg = currentPlatformImgPath;
        else if (File.Exists(Path.Join(Globals.WwwRoot, "\\img\\BasicDefault.png")))
            bgImg = Path.Join(Globals.WwwRoot, "\\img\\BasicDefault.png");


        var fgImg = Path.Join(Globals.WwwRoot, $"\\img\\profiles\\{AppState.Switcher.CurrentSwitcherSafe}\\{AppState.Switcher.SelectedAccountId}.jpg");
        if (!File.Exists(fgImg)) fgImg = Path.Join(Globals.WwwRoot, $"\\img\\profiles\\{AppState.Switcher.CurrentSwitcherSafe}\\{AppState.Switcher.SelectedAccountId}.png");
        if (!File.Exists(fgImg))
        {
            await ShowToast("error", Lang["Toast_CantFindImage"], Lang["Toast_CantCreateShortcut"], "toastarea");
            return;
        }

        var s = new Shortcut();
        _ = s.Shortcut_Platform(
            Shortcut.Desktop,
            platformName,
            $"+{primaryPlatformId}:{AppState.Switcher.SelectedAccountId}{args}",
            $"Switch to {AppState.Switcher.SelectedAccount.DisplayName} [{AppState.Switcher.CurrentSwitcher}] in TcNo Account Switcher",
            true);
        await s.CreateCombinedIcon(bgImg, fgImg, $"{AppState.Switcher.SelectedAccountId}.ico");
        s.TryWrite();

        if (WindowSettings.StreamerModeTriggered)
            await ShowToast("success", Lang["Toast_ShortcutCreated"], Lang["Success"], "toastarea");
        else
            await ShowToast("success", Lang["ForName", new { name = AppState.Switcher.SelectedAccount.DisplayName }], Lang["Toast_ShortcutCreated"], "toastarea");
    }

    [JSInvokable]
    public static string PlatformUserModalCopyText() => CurrentPlatform.GetUserModalCopyText;
    [JSInvokable]
    public static string PlatformHintText() => CurrentPlatform.GetUserModalHintText();

    [JSInvokable]
    public static string GiLocale(string k) => Lang[k];

    [JSInvokable]
    public static string GiLocaleObj(string k, object obj) => Lang[k, obj];

    [JSInvokable]
    public static string GiGetCleanFilePath(string f) => Globals.GetCleanFilePath(f);
}