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
using System.IO.Compression;
using System.Linq;
using System.Net.Http;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Input;
using System.Windows.Media;
using Newtonsoft.Json.Linq;
using TcNo_Acc_Switcher_Globals;
using TcNo_Acc_Switcher_Server;
using TcNo_Acc_Switcher_Server.Pages.General;
using TcNo_Acc_Switcher_Server.State;
using static TcNo_Acc_Switcher_Client.MainWindow;

namespace TcNo_Acc_Switcher_Client;

/// <summary>
/// Interaction logic for App.xaml
/// </summary>
public partial class App
{
    // Create WindowSettings instance. This loads from saved file if available.
    readonly WindowSettings _windowSettings = new();

    protected override void OnExit(ExitEventArgs e)
    {
        _ = NativeFuncs.FreeConsole();
#if DEBUG
        try
        {
#endif
            Mutex.ReleaseMutex();
#if DEBUG
        }
        catch
        {
            // Ignore errors if run in debug mode
        }
#endif
    }

    private static readonly Mutex Mutex = new(true, "{A240C23D-6F45-4E92-9979-11E6CE10A22C}");

    [STAThread]
    protected override async void OnStartup(StartupEventArgs e)
    {
        // Ensure files in documents are available.
        Globals.CreateDataFolder(false);

        // Remove leftover files from previous operations
        CleanupAppFolder();

        Directory.SetCurrentDirectory(Globals.UserDataFolder);

        if (_windowSettings.AlwaysAdmin && !Globals.IsAdministrator) StaticFuncs.RestartAsAdmin();

        // Crash handler
        AppDomain.CurrentDomain.UnhandledException += Globals.CurrentDomain_UnhandledException;

#if DEBUG
        _ = NativeFuncs.AllocConsole();
        NativeFuncs.SetWindowText("Debug console");
        Globals.WriteToLog("Debug Console started");
#endif
        try
        {
            Globals.ClearLogs();
        }
        catch (UnauthorizedAccessException)
        {
            MessageBox.Show("Can't access log.txt in the TcNo Account Switcher directory!",
                "Failed to access files", MessageBoxButton.OK, MessageBoxImage.Error);
            Environment.Exit(4); // The system cannot open the file.
        }

        // Key being held down?
        if ((Keyboard.Modifiers & ModifierKeys.Control) > 0 || (Keyboard.Modifiers & ModifierKeys.Alt) > 0 ||
            (Keyboard.Modifiers & ModifierKeys.Shift) > 0 ||
            (Keyboard.GetKeyStates(Key.Scroll) & KeyStates.Down) != 0)
        {
            // This can be improved. Somehow ignore self, and make sure all processes are killed before self.
            if (Globals.CanKillProcess("TcNo"))
                Globals.KillProcess("TcNo");
        }

        // Single instance:
        IsRunningAlready();

        // See if updater was updated, and move files:
        if (Directory.Exists("newUpdater"))
        {
            try
            {
                Globals.RecursiveDelete("updater", false);
            }
            catch (IOException)
            {
                // Catch first IOException and try to kill the updater, if it's running... Then continue.
                Globals.KillProcess("TcNo-Acc-Switcher-Updater");
                Globals.RecursiveDelete("updater", false);
            }

            Directory.Move("newUpdater", "updater");
        }

        //// Clear WebView2 cache
        //// This is disabled for now in hopes of fixing cache errors in console, causing it to launch with some issues.
        //Globals.ClearWebCache();

        // Show window (Because no command line commands were parsed)
        try
        {
            var mainWindow = new MainWindow(_windowSettings);
            mainWindow.Show();
        }
        catch (FileNotFoundException ex)
        {
            // Check if CEF issue, and download if missing.
            if (!ex.ToString().Contains("CefSharp")) throw;
            TcNo_Acc_Switcher_Server.State.Classes.Updates.AutoStartUpdaterAsAdmin("downloadCEF");
            Environment.Exit(1);
            throw;
        }

        if (!File.Exists("LastError.txt")) return;

        var lastError = await File.ReadAllLinesAsync("LastError.txt");
        lastError = lastError.Skip(1).ToArray();
        // TODO: Work in progress:
        //ShowErrorMessage("Error from last crash", "Last error message:" + Environment.NewLine + string.Join(Environment.NewLine, lastError));
        MessageBox.Show("Last error message:" + Environment.NewLine + string.Join(Environment.NewLine, lastError), "Error from last crash", MessageBoxButton.OK, MessageBoxImage.Error, MessageBoxResult.OK);
        Globals.DeleteFile("LastError.txt");

        Statistics.CrashCount++;
        Statistics.SaveSettings();
    }

    /// <summary>
    /// Cleanup leftover files from previous operations
    /// </summary>
    private static void CleanupAppFolder()
    {
        var backupTemp = Path.Join(Globals.UserDataFolder, "BackupTemp");
        if (Directory.Exists(backupTemp)) Globals.RecursiveDelete(backupTemp, false);
        var restoreTemp = Path.Join(Globals.UserDataFolder, "Restore");
        if (Directory.Exists(restoreTemp)) Globals.RecursiveDelete(restoreTemp, false);
    }

    public SolidColorBrush GetStylesheetColor(string key, string fallback)
    {
        var stylesheetFile = Path.Join(Globals.UserDataFolder, "themes", _windowSettings.ActiveTheme, "style.css");
        var stylesheet = File.ReadAllText(stylesheetFile);
        
        string color;
        try
        {
            var start = stylesheet.IndexOf(key + ":", StringComparison.Ordinal) + key.Length + 1;
            var end = stylesheet.IndexOf(";", start, StringComparison.Ordinal);
            color = stylesheet[start..end];
            color = color.Trim(); // Remove whitespace around variable
        }
        catch (Exception)
        {
            color = "";
        }


        var returnColor = new SolidColorBrush((Color)ColorConverter.ConvertFromString(fallback)!);
        if (!color.StartsWith("#")) return returnColor;
        try
        {
            returnColor = new SolidColorBrush((Color)ColorConverter.ConvertFromString(color)!);
        }
        catch (Exception)
        {
            // Failed to set color
        }
        return returnColor;
    }

    /// <summary>
    /// Shows error and exits program is program is already running
    /// </summary>
    private void IsRunningAlready()
    {
        try
        {
            // Check if program is running, if not: return.
            if (Mutex.WaitOne(TimeSpan.Zero, true)) return;

            // The program is running at this point.
            // If set to minimize to tray, try open it.
            if (_windowSettings.TrayMinimizeNotExit)
            {
                if (NativeFuncs.BringToFront())
                    Environment.Exit(1056); // 1056	An instance of the service is already running.
            }

            // Otherwise: It has probably just closed. Wait a few and try again
            Thread.Sleep(2000); // 2 seconds before just making sure -- Might be an admin restart

            // Ignore other processes running while in DEBUG mode.
            var release = false;
#if RELEASE
release = true;
#endif
            if (Mutex.WaitOne(TimeSpan.Zero, true)) return;
            // ReSharper disable once ConditionIsAlwaysTrueOrFalse
            if (!release) return;
            // Try to show from tray, as user may not know it's hidden there.
            string text;
            if (!NativeFuncs.BringToFront())
            {
                text = "Another TcNo Account Switcher instance has been detected." + Environment.NewLine +
                       "[Something wrong? Hold Hold Alt, Ctrl, Shift or Scroll Lock while starting to close all TcNo processes!]";

                _ = MessageBox.Show(text, "TcNo Account Switcher Notice", MessageBoxButton.OK,
                    MessageBoxImage.Information,
                    MessageBoxResult.OK, MessageBoxOptions.DefaultDesktopOnly);
                Environment.Exit(1056); // 1056	An instance of the service is already running.
            }
            else
            {
                if (!_windowSettings.ShownMinimizedNotification)
                {
                    text = "TcNo Account Switcher was running." + Environment.NewLine +
                           "I've brought it to the top." + Environment.NewLine +
                           "Make sure to check your Windows Task Tray for the icon :)" + Environment.NewLine +
                           "- You can exit it from there too" + Environment.NewLine + Environment.NewLine +
                           "[Something wrong? Hold Alt, Ctrl, Shift or Scroll Lock while starting to close all TcNo processes!]" + Environment.NewLine + Environment.NewLine +
                           "This message only shows once.";

                    _ = MessageBox.Show(text, "TcNo Account Switcher Notice", MessageBoxButton.OK,
                        MessageBoxImage.Information,
                        MessageBoxResult.OK, MessageBoxOptions.DefaultDesktopOnly);

                    _windowSettings.ShownMinimizedNotification = true;
                    _windowSettings.Save();
                }

                Environment.Exit(1056); // 1056	An instance of the service is already running.
            }
        }
        catch (AbandonedMutexException)
        {
            // Just restarted
        }
    }
    

    #region ResizeWindows
    /*
    // https://stackoverflow.com/a/27157947/5165437
    private bool _resizeInProcess;
            private void Resize_Init(object sender, MouseButtonEventArgs e)
            {
                if (sender is not Rectangle senderRect) return;
                _resizeInProcess = true;
                _ = senderRect.CaptureMouse();
            }



            private void Resize_End(object sender, MouseButtonEventArgs e)
            {
                if (sender is not Rectangle senderRect) return;
                _resizeInProcess = false;
                senderRect.ReleaseMouseCapture();
            }

            private void Resizing_Form(object sender, MouseEventArgs e)
            {
                if (!_resizeInProcess) return;
                if (sender is not Rectangle senderRect) return;
                var mainWindow = senderRect.Tag as Window;
                var width = e.GetPosition(mainWindow).X;
                var height = e.GetPosition(mainWindow).Y;
                _ = senderRect.CaptureMouse();
                if (senderRect.Name.ToLowerInvariant().Contains("right"))
                {
                    width += 5;
                    if (width > 0)
                        if (mainWindow != null)
                            mainWindow.Width = width;
                }
                if (senderRect.Name.ToLowerInvariant().Contains("left"))
                {
                    width -= 5;
                    if (mainWindow != null)
                    {
                        mainWindow.Left += width;
                        width = mainWindow.Width - width;
                        if (width > 0)
                        {
                            mainWindow.Width = width;
                        }
                    }
                }
                if (senderRect.Name.ToLowerInvariant().Contains("bottom"))
                {
                    height += 5;
                    if (height > 0)
                        if (mainWindow != null)
                            mainWindow.Height = height;
                }

                if (!senderRect.Name.ToLowerInvariant().Contains("top")) return;
                height -= 5;
                if (mainWindow == null) return;
                mainWindow.Top += height;
                height = mainWindow.Height - height;
                if (height > 0)
                {
                    mainWindow.Height = height;
                }
            }
    */
    #endregion
}