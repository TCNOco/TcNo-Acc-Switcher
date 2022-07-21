using System;
using System.IO;
using System.Linq;
using Microsoft.Win32;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;

namespace TcNo_Acc_Switcher_Server.State.Classes
{
    public class ShortcutsState
    {
        public bool DesktopShortcut
        {
            get => File.Exists(Path.Join(Shortcut.Desktop, "TcNo Account Switcher.lnk"));
            set
            {
                Globals.DebugWriteLine($@"[Func:General\Classes\Task.StartWithWindows_Toggle] shouldExist={value}");
                var s = new Shortcut();
                _ = s.Shortcut_Switcher(Environment.GetFolderPath(Environment.SpecialFolder.Desktop));
                s.ToggleShortcut(value);
            }
        }

        public bool StartMenu
        {
            get => File.Exists(Path.Join(Shortcut.StartMenu, "TcNo Account Switcher.lnk"));
            set
            {

            }
        }

        public bool StartMenuPlatforms
        {
            get => Directory.Exists(Path.Join(Shortcut.StartMenu, "Platforms"));
            set;
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
                _ = s.Shortcut_Tray(Environment.GetFolderPath(Environment.SpecialFolder.Startup));
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

            }
        }

        public ShortcutsState() { }
    }
}
