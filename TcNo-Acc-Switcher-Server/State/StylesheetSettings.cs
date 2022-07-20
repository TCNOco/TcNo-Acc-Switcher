using System;
using System.Collections.Generic;
using System.IO;
using System.Runtime.Versioning;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.Win32;
using Newtonsoft.Json;
using SharpScss;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Shared.Toast;
using TcNo_Acc_Switcher_Server.State.Interfaces;
using YamlDotNet.Serialization;
using YamlDotNet.Serialization.NamingConventions;

namespace TcNo_Acc_Switcher_Server.State;

public class StylesheetSettings : IStylesheetSettings
{
    [Inject] private AppData AData { get; set; }

    public string ActiveTheme { get; set; } = "";
    public bool Rtl { get; set; } // AppSettings.Rtl
    public string BackgroundPath { get; set; } = ""; // AppSettings.BackgroundPath
    public bool StreamerModeEnabled { get; set; } // AppSettings.StreamerModeEnabled
    public bool StreamerModeTriggered { get; set; } // AppSettings.StreamerModeTriggered
    public Action AutoStartUpdaterAsAdmin { get; set; } // Set to: AppSettings.AutoStartUpdaterAsAdmin("verify");

    public StylesheetSettings()
    {
        // For now, while APpSettings isn't a proper singleton:
        // Load settings from file.
        if (File.Exists("WindowSettings.json")) JsonConvert.PopulateObject(File.ReadAllText("WindowSettings.json"), this);
        AutoStartUpdaterAsAdmin = () => AppSettings.Instance.AutoStartUpdaterAsAdmin("verify");
        if (LoadStylesheetFromFile())
            NotifyDataChanged();
    }
    public void Load(AppSettings appSettings)
    {
        ActiveTheme = appSettings.ActiveTheme;
        Rtl = appSettings.Rtl;
        BackgroundPath = appSettings.Background;
        StreamerModeEnabled = appSettings.StreamerModeEnabled;
        StreamerModeTriggered = appSettings.StreamerModeTriggered;
        AutoStartUpdaterAsAdmin = () => appSettings.AutoStartUpdaterAsAdmin("verify");
    }

    public event Action Updated;
    public void NotifyDataChanged() => Updated?.Invoke();

    private string Stylesheet;

    public bool WindowsAccent { get; set; }

    public string WindowsAccentColor { get; set; }

    private (float, float, float) WindowsAccentColorHsl = (0, 0, 0);

    public Dictionary<string, string> StylesheetInfo { get; set; }


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

    /// <summary>
    /// Swaps in a requested stylesheet, and loads styles from file.
    /// </summary>
    /// <param name="swapTo">Stylesheet name (without .json) to copy and load</param>
    public void SwapStylesheet(string swapTo)
    {
        ActiveTheme = swapTo.Replace(" ", "_");
        try
        {
            if (LoadStylesheetFromFile())
                NotifyDataChanged();
        }
        catch (Exception)
        {
            AData.ShowToastLang(ToastType.Error, "Error", "Toast_LoadStylesheetFailed");
        }
    }

    /// <summary>
    /// Load stylesheet settings from stylesheet file.
    /// </summary>
    private bool LoadStylesheetFromFile()
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
                ActiveTheme = "Dracula_Cyan";

                if (!File.Exists(StylesheetFile))
                {
                    scss = StylesheetFile.Replace("css", "scss");
                    if (File.Exists(scss)) GenCssFromScss(scss);
                    else AData.ShowToastLang(ToastType.Error, "ThemesNotFound");
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
            new Task(AutoStartUpdaterAsAdmin).Start();
            Environment.Exit(1);
            throw;
        }

        return true;
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
            AData.ShowToastLang(ToastType.Error, "Error", "Toast_LoadStylesheetFailed");
            Globals.WriteToLog($"Could not delete stylesheet file: {StylesheetFile}. Could not refresh stylesheet from scss.", ex);
        }

    }

    private void LoadStylesheet()
    {
        // Load new stylesheet
        var desc = new DeserializerBuilder().WithNamingConvention(HyphenatedNamingConvention.Instance).Build();
        Stylesheet = Globals.ReadAllText(StylesheetFile);

        // Load stylesheet info
        string[] infoText = null;
        if (File.Exists(StylesheetInfoFile)) infoText = Globals.ReadAllLines(StylesheetInfoFile);

        infoText ??= new[] { "name: \"[ERR! info.yml]\"", "accent: \"#00D4FF\"" };
        var newSheet = JsonConvert.DeserializeObject<Dictionary<string, string>>(JsonConvert.SerializeObject(desc.Deserialize<object>(string.Join(Environment.NewLine, infoText))));

        StylesheetInfo = newSheet;

        if (OperatingSystem.IsWindows() && WindowsAccent) SetAccentColor();
    }

    /// <summary>
    /// Convert RGB values to HSL
    /// </summary>
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

        NotifyDataChanged();
    }

    [SupportedOSPlatform("windows")]
    private static string GetAccentColorHexString()
    {
        var (r, g, b) = GetAccentColor();
        byte[] rgb = { r, g, b };
        return '#' + BitConverter.ToString(rgb).Replace("-", string.Empty);
    }

    [SupportedOSPlatform("windows")]
    private static (byte r, byte g, byte b) GetAccentColor()
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

    [SupportedOSPlatform("windows")]
    private static (int, int, int) WindowsAccentColorInt => GetAccentColor();

    private string StylesheetFile => Path.Join("themes", ActiveTheme, "style.css");
    private string StylesheetInfoFile => Path.Join("themes", ActiveTheme, "info.yaml");

    /// <summary>
    /// Returns a block of CSS text to be used on the page. Used to hide or show certain things in certain ways, in components that aren't being added through Blazor.
    /// </summary>
    private string GetCssBlock() => ".streamerCensor { display: " + (StreamerModeEnabled && StreamerModeTriggered ? "none!important" : "block") + "}";

    public MarkupString GetStylesheetMarkupString()
    {
        var style = Stylesheet;

        if (OperatingSystem.IsWindows() && WindowsAccent)
        {
            var start = style.IndexOf("--accent:", StringComparison.Ordinal);
            var end = style.IndexOf(";", start, StringComparison.Ordinal) - start;
            style = style.Replace(style.Substring(start, end), "");

            var (h, s, l) = WindowsAccentColorHsl;
            var (r, g, b) = WindowsAccentColorInt;
            style = $":root {{ --accentHS: {h}, {s}%; --accentL: {l}%; --accent: {WindowsAccentColor}}}\n\n; --accentInt: {r}, {g}, {b}" + style;
        }

        if (Rtl)
            style = "@import url(css/rtl.min.css);\n" + style;

        if (BackgroundPath != "")
            style += ".programMain {background: url(" + BackgroundPath + ")!important;background-size:cover!important;}";

        return new MarkupString(style);
    }

    #endregion
}