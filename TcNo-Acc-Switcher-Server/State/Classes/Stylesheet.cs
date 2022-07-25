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
using System.IO;
using System.Runtime.CompilerServices;
using System.Runtime.Versioning;
using Microsoft.Win32;
using Newtonsoft.Json;
using SharpScss;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;
using YamlDotNet.Serialization;
using YamlDotNet.Serialization.NamingConventions;

namespace TcNo_Acc_Switcher_Server.State.Classes;

public class Stylesheet : INotifyPropertyChanged
{
    public event PropertyChangedEventHandler? PropertyChanged;
    protected void OnPropertyChanged(string propertyName) => PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));
    protected bool SetField<T>(ref T field, T value, [CallerMemberName] string propertyName = "")
    {
        if (EqualityComparer<T>.Default.Equals(field, value)) return false;
        field = value;
        OnPropertyChanged(propertyName);
        return true;
    }

    private readonly IToasts _toasts;
    private readonly ILang _lang;
    private readonly IWindowSettings _windowSettings;

    public Stylesheet(IToasts toasts, ILang lang, IWindowSettings windowSettings)
    {
        _toasts = toasts;
        _lang = lang;
        _windowSettings = windowSettings;

        StylesheetFile = Path.Join("themes", _windowSettings.ActiveTheme, "style.css");
        StylesheetInfoFile = Path.Join("themes", _windowSettings.ActiveTheme, "info.yaml");

        LoadStylesheetFromFile();
        _windowSettings.PropertyChanged += WindowSettingsPropertyChange;
    }

    private void WindowSettingsPropertyChange(object s, PropertyChangedEventArgs p)
    {
        // While technically updating a few others should also cause a stylesheet update, these are the only important ones as it's ont he Settings page.
        // Others include: StreamerModeEnabled, ShowStreamId, ShowLastLogin
        if (p.PropertyName is "ActiveTheme" or "Rtl" or "Background")
        {
            StylesheetFile = Path.Join("themes", _windowSettings.ActiveTheme, "style.css");
            StylesheetInfoFile = Path.Join("themes", _windowSettings.ActiveTheme, "info.yaml");
            LoadStylesheet();
            PropertyChanged?.Invoke(s, p);
        }
    }

    public string StylesheetCss { get; set; }

    public bool WindowsAccent
    {
        get => _windowsAccent;
        set
        {
            if (!OperatingSystem.IsWindows()) return;
            if (value) SetAccentColor(true);
            else WindowsAccentColor = "";

            SetField(ref _windowsAccent, value);
        }
    }

    public string WindowsAccentColor { get; set; } = "";
    public (float, float, float) WindowsAccentColorHsl { get; set; } = (0, 0, 0);
    public Dictionary<string, string> StylesheetInfo { get; set; }

    private string StylesheetFile { get; set; }
    private string StylesheetInfoFile { get; set; }

    public bool StreamerModeTriggered
    {
        get => _streamerModeTriggered;
        set => SetField(ref _streamerModeTriggered, value);
    }
    private bool _streamerModeTriggered;
    private bool _windowsAccent;

    // Constants
    public static readonly string SettingsFile = "WindowSettings.json";

    [SupportedOSPlatform("windows")]
    public (int, int, int) WindowsAccentColorInt => GetAccentColor();

    /// <summary>
    /// Check if any streaming software is running. Do let me know if you have a program name that you'd like to expand this list with!
    /// It's basically the program's .exe file, but without ".exe".
    /// </summary>
    /// <returns>True when streaming software is running</returns>
    public bool StreamerModeCheck()
    {
        Globals.DebugWriteLine(@"[StreamerModeCheck]");
        if (!_windowSettings.StreamerModeEnabled) return false; // Don't hide anything if disabled.
        StreamerModeTriggered = false;
        foreach (var p in Process.GetProcesses())
        {
            switch (p.ProcessName.ToLowerInvariant())
            {
                case "obs":
                case "obs32":
                case "obs64":
                case "streamlabs obs":
                case "wirecast":
                case "xsplit.core":
                case "xsplit.gamecaster":
                case "twitchstudio":
                    StreamerModeTriggered = true;
                    Globals.WriteToLog($"Streamer mode found: {p.ProcessName}");
                    return true;
            }
        }
        return false;
    }

    public string TryGetStyle(string key)
    {
        try
        {
            return StylesheetInfo[key];
        }
        catch (Exception ex)
        {
            // Try load default theme for values
            var fallback = Path.Join("themes", "Dracula_Cyan", "info.yaml");
            if (File.Exists(fallback))
            {
                var desc = new DeserializerBuilder().WithNamingConvention(HyphenatedNamingConvention.Instance).Build();
                var fallbackSheet = JsonConvert.DeserializeObject<Dictionary<string, string>>(JsonConvert.SerializeObject(desc.Deserialize<object>(string.Join(Environment.NewLine, Globals.ReadAllLines(fallback)))));
                try
                {
                    if (fallbackSheet is not null) return fallbackSheet[key];
                    Globals.WriteToLog("fallback stylesheet was null!");
                    return "";
                }
                catch (Exception exFallback)
                {
                    Globals.WriteToLog($"Could not find {key} in fallback stylesheet Dracula_Cyan", exFallback);
                    return "";
                }
            }

            Globals.WriteToLog($"Could not find {key} in stylesheet, and fallback Dracula_Cyan does not exist", ex);
            return "";
        }
    }

    /// <summary>
    /// Swaps in a requested stylesheet, and loads styles from file.
    /// </summary>
    /// <param name="swapTo">Stylesheet name (without .json) to copy and load</param>
    public void SwapStylesheet(string swapTo)
    {
        try
        {
            if (LoadStylesheetFromFile(swapTo.Replace(" ", "_")))
            {
                _windowSettings.ActiveTheme = swapTo.Replace(" ", "_");
                _windowSettings.Save();
                //navigationManager.NavigateTo(navigationManager.Uri, forceLoad: true);
                return;
            }
        }
        catch (Exception)
        {
            //
        }

        _toasts.ShowToastLang(ToastType.Error, "Error", "Toast_LoadStylesheetFailed");
    }

    /// <summary>
    /// Load stylesheet settings from stylesheet file.
    /// </summary>
    public bool LoadStylesheetFromFile(string styleSheetName = "")
    {
        // This is the first function that's called, and sometimes fails if this is not reset after being changed previously.
        Directory.SetCurrentDirectory(Globals.UserDataFolder);
#if DEBUG
        if (true) // Always generate file in debug mode.
#else
            if (!File.Exists(StylesheetFile))
#endif
        {
            // Check if SCSS file exists.
            var scss = StylesheetFile.Replace("css", "scss");
            if (File.Exists(scss)) GenCssFromScss(scss);
            else
            {
                _windowSettings.ActiveTheme = "Dracula_Cyan";

                if (!File.Exists(StylesheetFile))
                {
                    scss = StylesheetFile.Replace("css", "scss");
                    if (File.Exists(scss)) GenCssFromScss(scss);
                    else throw new Exception(_lang["ThemesNotFound"]);
                }
            }
        }

        try
        {
            LoadStylesheet();
        }
        catch (FileNotFoundException ex)
        {
            // Check if CEF issue, and download if missing.
            if (!ex.ToString().Contains("YamlDotNet")) throw;
            Updates.AutoStartUpdaterAsAdmin("verify"); // This is a static method, so it won't cause circular DI
            Environment.Exit(1);
            throw;
        }

        return true;
    }

    private void GenCssFromScss(string scss)
    {
        ScssResult convertedScss;
        try
        {
            convertedScss = Scss.ConvertFileToCss(scss, new ScssOptions { InputFile = scss, OutputFile = StylesheetFile });
        }
        catch (ScssException e)
        {
            Globals.DebugWriteLine("ERROR in CSS: " + e.Message);
            throw;
        }

        // Convert from SCSS to CSS. The arguments are for "exception reporting", according to the SharpScss Git Repo.
        try
        {
            if (Globals.DeleteFile(StylesheetFile))
            {
                var text = convertedScss.Css;
                File.WriteAllText(StylesheetFile, text);
                if (Globals.DeleteFile(StylesheetFile + ".map"))
                    File.WriteAllText(StylesheetFile + ".map", convertedScss.SourceMap);
            }
            else throw new Exception($"Could not delete StylesheetFile: '{StylesheetFile}'");
        }
        catch (Exception ex)
        {
            // Catches generic errors, as well as not being able to overwrite file errors, etc.
            _toasts.ShowToastLang(ToastType.Error, "Error", "Toast_LoadStylesheetFailed");
            Globals.WriteToLog($"Could not delete stylesheet file: {StylesheetFile}. Could not refresh stylesheet from scss.", ex);
        }

    }

    private void LoadStylesheet()
    {
        // Load new stylesheet
        var desc = new DeserializerBuilder().WithNamingConvention(HyphenatedNamingConvention.Instance).Build();
        StylesheetCss = Globals.ReadAllText(StylesheetFile);

        // Load stylesheet info
        string[] infoText = null;
        if (File.Exists(StylesheetInfoFile)) infoText = Globals.ReadAllLines(StylesheetInfoFile);

        infoText ??= new[] { "name: \"[ERR! info.yml]\"", "accent: \"#00D4FF\"" };
        var newSheet = JsonConvert.DeserializeObject<Dictionary<string, string>>(JsonConvert.SerializeObject(desc.Deserialize<object>(string.Join(Environment.NewLine, infoText))));

        StylesheetInfo = newSheet;

        if (OperatingSystem.IsWindows() && WindowsAccent) SetAccentColor();
    }

    /// <summary>
    /// Returns a list of Stylesheets in the Stylesheet folder.
    /// </summary>
    public string[] GetStyleList()
    {
        var themeList = Directory.GetDirectories("themes");
        for (var i = 0; i < themeList.Length; i++)
        {
            var start = themeList[i].LastIndexOf("\\", StringComparison.Ordinal) + 1;
            themeList[i] = themeList[i][start..].Replace('_', ' ');
        }
        return themeList;
    }
    private static (int, int, int) FromRgb(byte r, byte g, byte b)
    {
        // Adapted from: https://stackoverflow.com/a/4794649/5165437
        var rf = (r / 255f);
        var gf = (g / 255f);
        var bf = (b / 255f);

        var min = Math.Min(Math.Min(rf, gf), bf);
        var max = Math.Max(Math.Max(rf, gf), bf);
        var delta = max - min;

        var h = (float)0;
        var s = (float)0;
        var l = (max + min) / 2.0f;

        if (delta != 0)
        {
            if (l < 0.5f)
                s = delta / (max + min);
            else
                s = delta / (2.0f - max - min);

            if (Math.Abs(rf - max) < 0.01)
                h = (gf - bf) / delta;
            else if (Math.Abs(gf - max) < 0.01)
                h = 2f + (bf - rf) / delta;
            else if (Math.Abs(bf - max) < 0.01)
                h = 4f + (rf - gf) / delta;
        }

        // Rounding and formatting for CSS
        h *= 60;
        s *= 100;
        l *= 100;

        return ((int)Math.Round(h), (int)Math.Round(s), (int)Math.Round(l));
    }

    #region WindowsAccent

    [SupportedOSPlatform("windows")]
    public void SetAccentColor() => SetAccentColor(false);
    [SupportedOSPlatform("windows")]
    public void SetAccentColor(bool userInvoked)
    {
        WindowsAccentColor = GetAccentColorHexString();
        var (r, g, b) = GetAccentColor();
        WindowsAccentColorHsl = FromRgb(r, g, b);

        //if (userInvoked)
        //    NavigationManager.NavigateTo(NavigationManager.Uri, forceLoad: true);
    }

    [SupportedOSPlatform("windows")]
    public static string GetAccentColorHexString()
    {
        var (r, g, b) = GetAccentColor();
        byte[] rgb = { r, g, b };
        return '#' + BitConverter.ToString(rgb).Replace("-", string.Empty);
    }

    [SupportedOSPlatform("windows")]
    public static (byte r, byte g, byte b) GetAccentColor()
    {
        using var dwmKey = Registry.CurrentUser.OpenSubKey(@"Software\Microsoft\Windows\DWM", RegistryKeyPermissionCheck.ReadSubTree);
        const string keyExMsg = "The \"HKCU\\Software\\Microsoft\\Windows\\DWM\" registry key does not exist.";
        if (dwmKey is null) throw new InvalidOperationException(keyExMsg);

        var accentColorObj = dwmKey.GetValue("AccentColor");
        if (accentColorObj is int accentColorDWord)
        {
            return ParseDWordColor(accentColorDWord);
        }

        const string valueExMsg = "The \"HKCU\\Software\\Microsoft\\Windows\\DWM\\AccentColor\" registry key value could not be parsed as an ABGR color.";
        throw new InvalidOperationException(valueExMsg);
    }

    private static (byte r, byte g, byte b) ParseDWordColor(int color)
    {
        byte
            //a = (byte)((color >> 24) & 0xFF),
            b = (byte)((color >> 16) & 0xFF),
            g = (byte)((color >> 8) & 0xFF),
            r = (byte)((color >> 0) & 0xFF);

        return (r, g, b);
    }
    #endregion
}