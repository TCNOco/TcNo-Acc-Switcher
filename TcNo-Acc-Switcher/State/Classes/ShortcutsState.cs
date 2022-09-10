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
using System.IO;
using System.Linq;
using Microsoft.Win32;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher.Pages.General.Classes;
using TcNo_Acc_Switcher.State.DataTypes;
using TcNo_Acc_Switcher.State.Interfaces;

namespace TcNo_Acc_Switcher.State.Classes;

public class ShortcutsState
{
    private readonly IWindowSettings _windowSettings;
    private readonly IModals _modals;
    private readonly IToasts _toasts;

    public ShortcutsState(IWindowSettings windowSettings, IModals modals, IToasts toasts)
    {
        _windowSettings = windowSettings;
        _modals = modals;
        _toasts = toasts;
    }


    private readonly string _startMenuFolder = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.Programs), @"TcNo Account Switcher\");
    private readonly string _startMenuPlatformsFolder = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.Programs), @"TcNo Account Switcher\Platforms\");

    public bool DesktopShortcut
    {
        get => File.Exists(Path.Join(Shortcut.Desktop, "TcNo Account Switcher.lnk"));
        set
        {
            Globals.DebugWriteLine($@"[DesktopShortcut-Toggled] shouldExist={value}");
            var s = new Shortcut();
            s.Shortcut_Switcher(Environment.GetFolderPath(Environment.SpecialFolder.Desktop));
            s.ToggleShortcut(value);
        }
    }

    public bool StartMenu
    {
        get => File.Exists(Path.Join(Shortcut.StartMenu, "TcNo Account Switcher.lnk"));
        set
        {
            Globals.DebugWriteLine($@"[StartMenu-Toggled] shouldExist={value}");
            var s = new Shortcut();
            s.Shortcut_Switcher(_startMenuFolder);
            s.ToggleShortcut(value);
            s.Shortcut_Tray(_startMenuFolder);
            s.ToggleShortcut(value);
        }
    }

    public bool StartMenuPlatforms
    {
        get => Directory.Exists(_startMenuPlatformsFolder);
        set
        {
            if (!OperatingSystem.IsWindows()) return;
            Globals.DebugWriteLine(@"[ToggleDesktopShortcuts]");

            // If off, and folder exists: Remove shortcuts.
            if (!value)
            {
                if (Directory.Exists(_startMenuPlatformsFolder)) Directory.Delete(_startMenuPlatformsFolder, true);
                return;
            }

            // Enable shortcuts
            foreach (var platform in _windowSettings.Platforms.Where(x => x.Enabled))
            {
                Shortcut.PlatformDesktopShortcut(_startMenuPlatformsFolder, platform.SafeName, platform.Identifier, true);
            }
        }
    }

    /// <summary>
    /// Toggles whether the TcNo Account Switcher Tray application starts with Windows or not
    /// </summary>
    public bool TrayStartup
    {
        get => File.Exists(Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.Startup), "TcNo Account Switcher - Tray.lnk"));
        set
        {
            Globals.DebugWriteLine($@"[Func:General\Classes\Task.StartWithWindows_Toggle] shouldExist={value}");
            var s = new Shortcut();
            s.Shortcut_Tray(Environment.GetFolderPath(Environment.SpecialFolder.Startup));
            s.ToggleShortcut(value);
        }
    }

    /// <summary>
    /// Check for existence of protocol key in registry (tcno:\\)
    /// </summary>
    public bool ProtocolEnabled
    {
        get
        {
            if (!OperatingSystem.IsWindows()) return false;
            var key = Registry.ClassesRoot.OpenSubKey(@"tcno");
            return key != null && (key.GetValueNames().Contains("URL Protocol"));
        }
        set
        {
            if (!OperatingSystem.IsWindows()) return;
            try
            {
                if (value)
                {
                    // Add
                    using var key = Registry.ClassesRoot.CreateSubKey("tcno");
                    key?.SetValue("URL Protocol", "", RegistryValueKind.String);
                    using var defaultKey = Registry.ClassesRoot.CreateSubKey(@"tcno\Shell\Open\Command");
                    defaultKey?.SetValue("", $"\"{Path.Join(Globals.AppDataFolder, "TcNo-Acc-Switcher.exe")}\" \"%1\"", RegistryValueKind.String);
                    _toasts.ShowToastLang(ToastType.Success, "Toast_ProtocolEnabledTitle", "Toast_ProtocolEnabled");
                }
                else
                {
                    // Remove
                    Registry.ClassesRoot.DeleteSubKeyTree("tcno");
                    _toasts.ShowToastLang(ToastType.Success, "Toast_ProtocolDisabledTitle", "Toast_ProtocolDisabled");
                }
            }
            catch (Exception e)
            {
                _toasts.ShowToastLang(ToastType.Error, "Failed", "Toast_RestartAsAdmin");
                _modals.ShowModal("confirm", ExtraArg.RestartAsAdmin);
            }
        }
    }
}