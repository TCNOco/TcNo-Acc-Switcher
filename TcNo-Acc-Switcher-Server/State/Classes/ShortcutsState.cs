using System;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Components;
using Microsoft.Win32;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using TcNo_Acc_Switcher_Server.State.DataTypes;
using TcNo_Acc_Switcher_Server.State.Interfaces;

namespace TcNo_Acc_Switcher_Server.State.Classes
{
    public class ShortcutsState
    {
        [Inject] private Modals Modals { get; set; }
        [Inject] private IAppState AppState { get; set; }
        [Inject] private Toasts Toasts { get; set; }

        private readonly string _startMenuFolder = Path.Join(Environment.GetFolderPath(Environment.SpecialFolder.Programs), @"TcNo Account Switcher\");

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
            }
        }

        public bool StartMenuPlatforms
        {
            get => Directory.Exists(Path.Join(Shortcut.StartMenu, "Platforms"));
            set
            {
                if (!OperatingSystem.IsWindows()) return;
                Globals.DebugWriteLine(@"[ToggleDesktopShortcuts]");

                // If off, and folder exists: Remove shortcuts.
                if (!value)
                {
                    if (!Directory.Exists(_startMenuFolder)) return;
                    foreach (var f in Directory.GetFiles(_startMenuFolder))
                        if (!f.EndsWith("TcNo Account Switcher.lnk")) Globals.DeleteFile(f); // Delete everything but main shortcut
                    return;
                }

                // TODO: Else: Create shortcuts.
                // foreach platform

                //foreach (var platform in AppSettings.Platforms)
                //{
                //    Shortcut.PlatformDesktopShortcut(_startMenuFolder, platform.Name, platform.Identifier, true);
                //}
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
                        Toasts.ShowToastLang(ToastType.Success, "Toast_ProtocolEnabledTitle", "Toast_ProtocolEnabled");
                    }
                    else
                    {
                        // Remove
                        Registry.ClassesRoot.DeleteSubKeyTree("tcno");
                        Toasts.ShowToastLang(ToastType.Success, "Toast_ProtocolDisabledTitle", "Toast_ProtocolDisabled");
                    }
                }
                catch (Exception e)
                {
                    Toasts.ShowToastLang(ToastType.Error, "Failed", "Toast_RestartAsAdmin");
                    Modals.ShowModal("confirm", ExtraArg.RestartAsAdmin);
                }
            }
        }

        public ShortcutsState() { }
    }
}
