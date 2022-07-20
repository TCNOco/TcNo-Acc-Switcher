using System;
using System.IO;
using Microsoft.AspNetCore.Components;
using Microsoft.Win32;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server.Data;
using TcNo_Acc_Switcher_Server.Pages.General.Classes;
using TcNo_Acc_Switcher_Server.Shared.Toast;

namespace TcNo_Acc_Switcher_Server.Shared
{
    public partial class SharedSettings
    {
        public void Button_StartTray()
        {
            var res = NativeFuncs.StartTrayIfNotRunning();
            switch (res)
            {
                case "Started Tray":
                    AData.ShowToastLang(ToastType.Success, "Toast_TrayStarted");
                    break;
                case "Already running":
                    AData.ShowToastLang(ToastType.Info, "Toast_TrayRunning");
                    break;
                case "Tray users not found":
                    AData.ShowToastLang(ToastType.Error, "Toast_TrayUsersMissing");
                    break;
                default:
                    AData.ShowToastLang(ToastType.Error, "Toast_TrayFail");
                    break;
            }
        }

        /// <summary>
        /// Toggle protocol functionality in Windows
        /// </summary>
        public void Button_ToggleProtocol()
        {
            if (!OperatingSystem.IsWindows()) return;

            try
            {
                if (!SharedStaticFuncs.Protocol_IsEnabled())
                {
                    // Add
                    using var key = Registry.ClassesRoot.CreateSubKey("tcno");
                    key?.SetValue("URL Protocol", "", RegistryValueKind.String);
                    using var defaultKey = Registry.ClassesRoot.CreateSubKey(@"tcno\Shell\Open\Command");
                    defaultKey?.SetValue("", $"\"{Path.Join(Globals.AppDataFolder, "TcNo-Acc-Switcher.exe")}\" \"%1\"", RegistryValueKind.String);
                    AData.ShowToastLang(ToastType.Success, "Toast_ProtocolEnabledTitle", "Toast_ProtocolEnabled");
                }
                else
                {
                    // Remove
                    Registry.ClassesRoot.DeleteSubKeyTree("tcno");
                    AData.ShowToastLang(ToastType.Success, "Toast_ProtocolDisabledTitle", "Toast_ProtocolDisabled");
                }
                AppSettings.Instance.ProtocolEnabled = SharedStaticFuncs.Protocol_IsEnabled();
            }
            catch (UnauthorizedAccessException)
            {
                AData.ShowToastLang(ToastType.Error, "Failed", "Toast_RestartAsAdmin");
                ModalData.ShowModal("confirm", ModalData.ExtraArg.RestartAsAdmin);
            }
        }

        /// <summary>
        /// Create shortcuts in Start Menu
        /// </summary>
        /// <param name="platforms">true creates Platforms folder & drops shortcuts, otherwise only places main program & tray shortcut</param>
        public void StartMenu_Toggle(bool platforms)
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.StartMenu_Toggle]");
            if (platforms)
            {
                var platformsFolder = Path.Join(Shortcut.StartMenu, "Platforms");
                if (Directory.Exists(platformsFolder)) Globals.RecursiveDelete(Path.Join(Shortcut.StartMenu, "Platforms"), false);
                else if (!AppSettings.Instance.StartMenuPlatforms) return;

                _ = Directory.CreateDirectory(platformsFolder);
                foreach (var platform in AppSettings.Platforms)
                {
                    Shortcut.PlatformDesktopShortcut(platformsFolder, platform.Name, platform.Identifier, true);
                }
            }
            // Only create these shortcuts of requested, by setting platforms to false.
            var s = new Shortcut();
            _ = s.Shortcut_Switcher(Shortcut.StartMenu);
            s.ToggleShortcut(!AppSettings.Instance.StartMenu, false);

            _ = s.Shortcut_Tray(Shortcut.StartMenu);
            s.ToggleShortcut(!AppSettings.Instance.StartMenu, false);
        }

        /// <summary>
        /// Toggle the main Desktop shortcut
        /// </summary>
        public void DesktopShortcut_Toggle()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.DesktopShortcut_Toggle]");
            var s = new Shortcut();
            _ = s.Shortcut_Switcher(Shortcut.Desktop);
            s.ToggleShortcut(!AppSettings.Instance.StartMenu);
        }

        /// <summary>
        /// Toggle using the Windows highlight as in-app highlight
        /// </summary>
        public void WindowsAccent_Toggle()
        {
            if (!OperatingSystem.IsWindows()) return;
            if (!StylesheetSettings.WindowsAccent)
                StylesheetSettings.SetAccentColor(true);
            else
                StylesheetSettings.WindowsAccentColor = "";

            StylesheetSettings.NotifyDataChanged();
        }

        /// <summary>
        /// Sets the active browser
        /// </summary>
        public void SetActiveBrowser(string browser)
        {
            AppSettings.Instance.ActiveBrowser = browser;
            AData.ShowToastLang(ToastType.Info, "Notice", "Toast_RestartRequired");
        }

        /// <summary>
        /// Toggle whether the program minimizes to the start menu on exit
        /// </summary>
        public void TrayMinimizeNotExit_Toggle()
        {
            Globals.DebugWriteLine(@"[Func:Data\Settings\Steam.TrayMinimizeNotExit_Toggle]");
            if (AppSettings.Instance.TrayMinimizeNotExit) return;
            AData.ShowToastLang(ToastType.Info, "Toast_TrayPosition", 15000);
            AData.ShowToastLang(ToastType.Info, "Toast_TrayHint", 15000);
        }

        public static void AutoStart_Toggle() => Shortcut.StartWithWindows_Toggle(!AppSettings.Instance.TrayStartup);
    }
}
