using System;
using System.Collections.Generic;
using Microsoft.AspNetCore.Components;
using TcNo_Acc_Switcher_Server.Data;

namespace TcNo_Acc_Switcher_Server.State.Interfaces;

public interface IStylesheetSettings
{
    string ActiveTheme { get; set; }
    bool Rtl { get; set; } // AppSettings.Rtl
    string BackgroundPath { get; set; } // AppSettings.BackgroundPath
    bool StreamerModeEnabled { get; set; } // AppSettings.StreamerModeEnabled
    bool StreamerModeTriggered { get; set; } // AppSettings.StreamerModeTriggered
    Action AutoStartUpdaterAsAdmin { get; set; } // Set to: AppSettings.AutoStartUpdaterAsAdmin("verify");
    void Load(AppSettings appSettings);
    event Action Updated;
    void NotifyDataChanged();
    bool WindowsAccent { get; set; }
    string WindowsAccentColor { get; set; }
    Dictionary<string, string> StylesheetInfo { get; set; }

    /// <summary>
    /// Returns a list of Stylesheets in the Stylesheet folder.
    /// </summary>
    string[] GetStyleList();

    /// <summary>
    /// Swaps in a requested stylesheet, and loads styles from file.
    /// </summary>
    /// <param name="swapTo">Stylesheet name (without .json) to copy and load</param>
    void SwapStylesheet(string swapTo);

    string TryGetStyle(string key);
    void SetAccentColor();
    void SetAccentColor(bool userInvoked);
    MarkupString GetStylesheetMarkupString();
}