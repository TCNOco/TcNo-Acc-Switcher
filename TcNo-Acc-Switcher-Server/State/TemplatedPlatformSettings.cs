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
using System.IO;
using System.Linq;
using System.Runtime.Versioning;
using Microsoft.AspNetCore.Components;
using Newtonsoft.Json;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using TcNo_Acc_Switcher_Server.State.Classes.Templated;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State;

public class TemplatedPlatformSettings : ITemplatedPlatformSettings
{
    private readonly IStatistics _statistics;
    private readonly ITemplatedPlatformState _templatedPlatformState;

    [JsonProperty(Order = 2)] public bool Admin { get; set; }
    [JsonProperty(Order = 3)] public int TrayAccNumber { get; set; } = 3;
    [JsonProperty(Order = 4)] public bool ForgetAccountEnabled { get; set; }
    [JsonProperty(Order = 5)] public Dictionary<int, string> Shortcuts { get; set; } = new();
    [JsonProperty(Order = 8)] public bool AutoStart { get; set; } = true;
    [JsonProperty(Order = 9)] public bool ShowShortNotes { get; set; } = true;
    [JsonIgnore]
    public bool DesktopShortcut
    {
        get => Shortcut.CheckShortcuts(_templatedPlatformState.CurrentPlatform.Name);
        set => Shortcut.PlatformDesktopShortcut(Environment.GetFolderPath(Environment.SpecialFolder.Desktop),
            _templatedPlatformState.CurrentPlatform.Name, _templatedPlatformState.CurrentPlatform.Identifiers[0], value);
    }
    [JsonIgnore] public int LastAccTimestamp { get; set; }
    [JsonIgnore] public string LastAccName { get; set; } = "";
    [JsonIgnore] public string Exe { get; set; }
    public string ClosingMethod
    {
        get
        {
            if (!string.IsNullOrWhiteSpace(_closingMethod)) return _closingMethod;
            _closingMethod = _templatedPlatformState.CurrentPlatform.Extras.ClosingMethod;
            return _closingMethod;
        }
        set => _closingMethod = value;
    }

    public string StartingMethod
    {
        get
        {
            if (!string.IsNullOrWhiteSpace(_startingMethod)) return _startingMethod;
            _startingMethod = _templatedPlatformState.CurrentPlatform.Extras.StartingMethod;
            return _startingMethod;
        }
        set => _startingMethod = value;
    }
    public string FolderPath
    {
        get
        {
            if (!string.IsNullOrWhiteSpace(_folderPath)) return _folderPath;
            _folderPath = _templatedPlatformState.CurrentPlatform.ExeLocationDefault;
            return _folderPath;
        }
        set => _folderPath = value;
    }

    private Platform _currentPlatform;

    public TemplatedPlatformSettings(IStatistics statistics, ITemplatedPlatformState templatedPlatformState)
    {
        _statistics = statistics;
        _templatedPlatformState = templatedPlatformState;
    }

    public void LoadTemplatedPlatformSettings()
    {
        _currentPlatform = _templatedPlatformState.CurrentPlatform;
        if (File.Exists(_currentPlatform.SettingsFile))
        {
            Globals.LoadSettings(_currentPlatform.SettingsFile, this, false);

            Exe = Path.Join(FolderPath, _currentPlatform.ExeName);
        }
        else
            Exe = Path.Join(_currentPlatform.ExeLocationDefault, _currentPlatform.ExeName);

        // Everything beyond this point needs saved settings loaded for the platform.
        if (OperatingSystem.IsWindows())
        {
            // Add image for main platform button
            SearchAndImportPlatformShortcut();
            // Import shortcuts from start menu, etc.
            ImportShortcuts();
            // Extract images from said shortcuts
            CollectShortcutImages();
        }

        _statistics.SetGameShortcutCount(_templatedPlatformState.CurrentPlatform.SafeName, Shortcuts);
        Save();
    }


    public void Save()
    {
        Globals.SaveJsonFile(_templatedPlatformState.CurrentPlatform.SettingsFile, this, false);
    }

    public void Reset()
    {
        var type = GetType();
        var properties = type.GetProperties();
        foreach (var t in properties)
            t.SetValue(this, null);
        Save();
    }

    /// <summary>
    /// Updates the ForgetAccountEnabled bool in settings file
    /// </summary>
    /// <param name="enabled">Whether will NOT prompt user if they're sure or not</param>
    public void SetForgetAcc(bool enabled)
    {
        if (ForgetAccountEnabled == enabled) return; // Ignore if already set
        ForgetAccountEnabled = enabled;
        Save();
    }
    public void SaveShortcutOrder(Dictionary<int, string> o)
    {
        Shortcuts = o;
        Save();
    }

    public void SetClosingMethod(string method)
    {
        ClosingMethod = method;
        Save();
    }
    public void SetStartingMethod(string method)
    {
        StartingMethod = method;
        Save();
    }

    [JsonProperty("FolderPath", Order = 1)] private string _folderPath = "";
    [JsonProperty("ClosingMethod", Order = 6)] private string _closingMethod = "";
    [JsonProperty("StartingMethod", Order = 7)] private string _startingMethod = "";


    /// <summary>
    /// Search start menu in AppData and ProgramData for requested platform shortcut, and get the image from it.
    /// </summary>
    [SupportedOSPlatform("windows")]
    private void SearchAndImportPlatformShortcut()
    {
        var safeName = _currentPlatform.SafeName;

        // Add image for main platform button:
        if (!_currentPlatform.Extras.ShortcutIncludeMainExe) return;
        var imagePath = Path.Join(_currentPlatform.ShortcutImagePath, safeName + ".png");

        if (File.Exists(imagePath)) return;
        if (_currentPlatform.Extras.SearchStartMenuForIcon)
        {
            var startMenuFiles = Directory.GetFiles(_currentPlatform.ExpandEnvironmentVariables("%StartMenuAppData%"), safeName + ".lnk", SearchOption.AllDirectories);
            var commonStartMenuFiles = Directory.GetFiles(_currentPlatform.ExpandEnvironmentVariables("%StartMenuProgramData%"), safeName + ".lnk", SearchOption.AllDirectories);
            if (startMenuFiles.Length > 0)
                Globals.SaveIconFromFile(startMenuFiles[0], imagePath);
            else if (commonStartMenuFiles.Length > 0)
                Globals.SaveIconFromFile(commonStartMenuFiles[0], imagePath);
            else
                Globals.SaveIconFromFile(Exe, imagePath);
        }
        else
            Globals.SaveIconFromFile(Exe, imagePath);
    }

    /// <summary>
    /// Go through shortcuts in ShortcutFolders and import them, if not ignored already.
    /// </summary>
    private void ImportShortcuts()
    {
        foreach (var sFolder in _currentPlatform.Extras.ShortcutFolders)
        {
            if (sFolder == "") continue;
            // Foreach file in folder
            var desktopShortcutFolder = _currentPlatform.ExpandEnvironmentVariables(sFolder);
            if (!Directory.Exists(desktopShortcutFolder)) continue;
            foreach (var shortcut in new DirectoryInfo(desktopShortcutFolder).GetFiles())
            {
                var fName = shortcut.Name;
                if (_currentPlatform.Extras.ShortcutIgnore.Contains(Globals.RemoveShortcutExt(fName))) continue;

                // Check if in saved shortcuts and If ignored
                if (File.Exists(_currentPlatform.GetShortcutIgnoredPath(fName)))
                {
                    var imagePath = Path.Join(_currentPlatform.ShortcutImagePath, Globals.RemoveShortcutExt(fName) + ".png");
                    if (File.Exists(imagePath)) File.Delete(imagePath);
                    fName = fName.Replace("_ignored", "");
                    if (Shortcuts.ContainsValue(fName))
                        Shortcuts.Remove(Shortcuts.First(e => e.Value == fName).Key);
                    continue;
                }

                Directory.CreateDirectory(_currentPlatform.ShortcutFolder);
                var outputShortcut = Path.Join(_currentPlatform.ShortcutFolder, fName);

                // Exists and is not ignored: Update shortcut
                File.Copy(shortcut.FullName, outputShortcut, true);
                // Organization will be saved in HTML/JS
            }
        }
    }


    /// <summary>
    /// Extract images from shortcuts, and make them accessible to the program.
    /// </summary>
    [SupportedOSPlatform("windows")]
    private void CollectShortcutImages()
    {
        // Now get images for all the shortcuts in the folder, as long as they don't already exist:
        List<string> existingShortcuts = new();
        if (Directory.Exists(_currentPlatform.ShortcutFolder))
            foreach (var f in new DirectoryInfo(_currentPlatform.ShortcutFolder).GetFiles())
            {
                if (f.Name.Contains("_ignored")) continue;
                var imageName = Globals.RemoveShortcutExt(f.Name) + ".png";
                var imagePath = Path.Join(_currentPlatform.ShortcutImagePath, imageName);
                existingShortcuts.Add(f.Name);
                if (!Shortcuts.ContainsValue(f.Name))
                {
                    // Not found in list, so add!
                    var last = 0;
                    foreach (var (k, _) in Shortcuts)
                        if (k > last) last = k;
                    last += 1;
                    Shortcuts.Add(last, f.Name); // Organization added later
                }

                // Extract image and place in wwwroot (Only if not already there):
                if (!File.Exists(imagePath))
                {
                    Globals.SaveIconFromFile(f.FullName, imagePath);
                }
            }

        foreach (var (i, s) in Shortcuts)
        {
            if (!existingShortcuts.Contains(s))
                Shortcuts.Remove(i);
        }
    }
}