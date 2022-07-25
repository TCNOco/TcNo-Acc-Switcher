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
using System.Linq;
using System.Net;
using System.Reflection;
using System.Threading;
using Microsoft.AspNetCore.Components;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State.Classes;

public class Updates
{
    private readonly IToasts _toasts;
    private readonly IModals _modals;

    private bool UpdateCheckRan { get; set; }
    public bool PreRenderUpdate { get; set; }
    public bool ShowUpdate { get; set; }
    private bool FirstLaunch { get; set; }

    /// <summary>
    /// (Initializes one time) Checks for update, and submits statistics if enabled.
    /// </summary>
    public Updates(IToasts toasts, IModals modals, IWindowSettings windowSettings, IStatistics statistics)
    {
        _toasts = toasts;
        _modals = modals;
        var windowSettings1 = windowSettings;

        if (!FirstLaunch) return;
        FirstLaunch = false;
        // Check for update in another thread
        // Also submit statistics, if enabled
        new Thread(CheckForUpdate).Start();
        if (windowSettings1.CollectStats && windowSettings1.ShareAnonymousStats)
            new Thread(statistics.UploadStats).Start();
    }

    /// <summary>
    /// Checks for an update
    /// </summary>
    public async void CheckForUpdate()
    {
        if (UpdateCheckRan) return;
        UpdateCheckRan = true;

        try
        {
#if DEBUG
            var latestVersion = Globals.DownloadString("https://tcno.co/Projects/AccSwitcher/api?debug&v=" + Globals.Version);
#else
                var latestVersion = Globals.DownloadString("https://tcno.co/Projects/AccSwitcher/api?v=" + Globals.Version);
#endif
            if (CheckLatest(latestVersion)) return;
            // Show notification
            try
            {
                ShowUpdate = true;
            }
            catch (Exception)
            {
                PreRenderUpdate = true;
            }
        }
        catch (Exception e) when (e is WebException or AggregateException)
        {
            if (File.Exists("WindowSettings.json"))
            {
                try
                {
                    var o = JObject.Parse(Globals.ReadAllText("WindowSettings.json"));
                    if (o.ContainsKey("LastUpdateCheckFail"))
                    {
                        if (!(DateTime.TryParseExact((string)o["LastUpdateCheckFail"], "yyyy-MM-dd HH:mm:ss.fff",
                                  CultureInfo.InvariantCulture, DateTimeStyles.None, out var timediff) &&
                              DateTime.Now.Subtract(timediff).Days >= 1)) return;
                    }

                    // Has not shown error today
                    _toasts.ShowToastLang(ToastType.Error, "Toast_UpdateCheckFail", 15000);
                    o["LastUpdateCheckFail"] = DateTime.Now.ToString("yyyy-MM-dd HH:mm:ss.fff");
                    await File.WriteAllTextAsync("WindowSettings.json", o.ToString());
                }
                catch (JsonException je)
                {
                    Globals.WriteToLog("Could not interpret <User Data>\\WindowSettings.json.", je);
                    _toasts.ShowToastLang(ToastType.Error, "Toast_UserDataLoadFail", 15000);
                    File.Move("WindowSettings.json", "WindowSettings.bak.json", true);
                }
            }
            Globals.WriteToLog(@"Could not reach https://tcno.co/ to check for updates.", e);
        }
    }

    /// <summary>
    /// Checks whether the program version is equal to or newer than the servers
    /// </summary>
    /// <param name="latest">Latest version provided by server</param>
    /// <returns>True when the program is up-to-date or ahead</returns>
    private bool CheckLatest(string latest)
    {
        latest = latest.Replace("\r", "").Replace("\n", "");
        if (DateTime.TryParseExact(latest, "yyyy-MM-dd_mm", null, DateTimeStyles.None, out var latestDate))
        {
            if (DateTime.TryParseExact(Globals.Version, "yyyy-MM-dd_mm", null, DateTimeStyles.None, out var currentDate))
            {
                if (latestDate.Equals(currentDate) || currentDate.Subtract(latestDate) > TimeSpan.Zero) return true;
            }
            else
                Globals.WriteToLog($"Unable to convert '{latest}' to a date and time.");
        }
        else
            Globals.WriteToLog($"Unable to convert '{latest}' to a date and time.");
        return false;
    }

    public void UpdateNow()
    {
        try
        {
            switch (UpdateNowNoToasts())
            {
                case UpdateResponse.RestartAsAdmin:
                    _modals.ShowModal("confirm", ExtraArg.RestartAsAdmin);
                    break;
                case UpdateResponse.VerifyFail:
                    _toasts.ShowToastLang(ToastType.Error, "Toast_UpdateVerifyFail");
                    break;
                case UpdateResponse.Success:
                    break;
            }
        }
        catch (Exception e)
        {
            _toasts.ShowToastLang(ToastType.Error, "Toast_FailedUpdateCheck");
            Globals.WriteToLog("Failed to check for updates:" + e);
        }
        Directory.SetCurrentDirectory(Globals.UserDataFolder);
    }

    public enum UpdateResponse
    {
        RestartAsAdmin,
        VerifyFail,
        Success
    }

    /// <summary>
    /// Verify updater files and start update
    /// </summary>
    public UpdateResponse UpdateNowNoToasts()
    {
        if (Globals.InstalledToProgramFiles() && !Globals.IsAdministrator || !Globals.HasFolderAccess(Globals.AppDataFolder))
            return UpdateResponse.RestartAsAdmin;

        Directory.SetCurrentDirectory(Globals.AppDataFolder);
        // Download latest hash list
        var hashFilePath = Path.Join(Globals.UserDataFolder, "hashes.json");
        Globals.DownloadFile("https://tcno.co/Projects/AccSwitcher/latest/hashes.json", hashFilePath);

        // Verify updater files
        var verifyDictionary = JsonConvert.DeserializeObject<Dictionary<string, string>>(Globals.ReadAllText(hashFilePath));
        if (verifyDictionary == null)
            return UpdateResponse.VerifyFail;

        var updaterDict = verifyDictionary.Where(pair => pair.Key.StartsWith("updater")).ToDictionary(pair => pair.Key, pair => pair.Value);

        // Download and replace broken files
        Globals.RecursiveDelete("newUpdater", false);
        foreach (var (key, value) in updaterDict)
        {
            if (key == null) continue;
            if (File.Exists(key) && value == Globals.GetFileMd5(key))
                continue;
            Globals.DownloadFile("https://tcno.co/Projects/AccSwitcher/latest/" + key.Replace('\\', '/'), key);
        }

        AutoStartUpdaterAsAdmin();
        Directory.SetCurrentDirectory(Globals.UserDataFolder);

        return UpdateResponse.Success;
    }

    public static void AutoStartUpdaterAsAdmin(string args = "")
    {
        // Run updater
        if (Globals.InstalledToProgramFiles() || !Globals.HasFolderAccess(Globals.AppDataFolder))
        {
            StartUpdaterAsAdmin(args);
        }
        else
        {
            _ = Process.Start(new ProcessStartInfo(Path.Join(Globals.AppDataFolder, @"updater\\TcNo-Acc-Switcher-Updater.exe")) { UseShellExecute = true, Arguments = args });
            Environment.Exit(1);
        }
    }

    private static void StartUpdaterAsAdmin(string args = "")
    {
        var exeLocation = Path.GetDirectoryName(Assembly.GetEntryAssembly()?.Location) ?? Environment.CurrentDirectory;
        Directory.SetCurrentDirectory(exeLocation);

        var proc = new ProcessStartInfo
        {
            WorkingDirectory = exeLocation,
            FileName = "updater\\TcNo-Acc-Switcher-Updater.exe",
            Arguments = args,
            UseShellExecute = true,
            Verb = "runas"
        };

        try
        {
            _ = Process.Start(proc);
            Environment.Exit(1);
        }
        catch (Exception ex)
        {
            Globals.WriteToLog(@"This program must be run as an administrator!" + Environment.NewLine + ex);
            Environment.Exit(1);
        }
    }
}