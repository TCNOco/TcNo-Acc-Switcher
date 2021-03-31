using Microsoft.Win32;
using System;
using System.Collections;
using System.Diagnostics;
using System.IO;
using System.Threading;
using System.Windows;
using System.Windows.Input;
using System.Windows.Navigation;
using TcNo_Acc_Switcher_Server.Pages.Steam;
using TcNo_Acc_Switcher_Globals;

//using System.Windows.Shapes; -- Commented because of clash with System.IO.Path. If causes issues, uncomment.

namespace TcNo_Acc_Switcher_Client
{
    /// <summary>
    /// Interaction logic for ClearLogins.xaml
    /// </summary>
    public partial class ClearLogins
    {
        MainWindow _mw;
        public ClearLogins()
        {
            InitializeComponent();
        }


        public string[] GetFiles(string sourceFolder, string filter, SearchOption searchOption)
        {
            var alFiles = new ArrayList();
            var multipleFilters = filter.Split('|');
            foreach (var fileFilter in multipleFilters)
                alFiles.AddRange(Directory.GetFiles(sourceFolder, fileFilter, searchOption));

            return (string[])alFiles.ToArray(typeof(string));
        }

        private struct PossibleClearArgs
        {
            public string FldName;
            public string FolderName;
            public string FileExtensions;
            public SearchOption SearchOption;
        }
        private readonly object _lockClearFilesOfType = new object();
        private readonly object _lockClearFolder = new object();

        private void ClearFolder(string fldName, string fld)
        {
            var t = new Thread(Task_clearFolder);
            var args = new PossibleClearArgs
            {
                FldName = fldName,
                FolderName = fld
            };
            t.Start(args);
        }
        private void Task_clearFolder(object oin)
        {
            //// Only allow one to be run at a time
            //if (!Monitor.TryEnter(_lockClearFolder)) return;
            //try
            //{
            //    var options = (PossibleClearArgs)oin;
            //    LogPrint(options.FldName);
            //    var count = 0;
            //    if (Directory.Exists(options.FolderName))
            //    {
            //        var d = new DirectoryInfo(options.FolderName);
            //        foreach (var file in d.GetFiles("*.*"))
            //        {
            //            count++;
            //            LogPrint($"Deleting: {file.Name}");
            //            try
            //            {
            //                File.Delete(file.FullName);
            //            }
            //            catch (Exception ex)
            //            {
            //                LogPrint($"ERROR: {ex}");
            //            }
            //        }

            //        foreach (var subDir in d.GetDirectories())
            //        {
            //            count++;
            //            LogPrint($"Deleting: {subDir.Name} subfolder and contents");
            //            try
            //            {
            //                Directory.Delete(subDir.FullName, true);
            //            }
            //            catch (Exception ex)
            //            {
            //                LogPrint($"ERROR: {ex}");
            //            }
            //        }

            //        LogPrint(count == 0 ? $"{options.FolderName} is empty." : "Done.");
            //        CleaningOutput.AppendText(Environment.NewLine);
            //    }
            //    else
            //        LogPrint($"Directory not found: {options.FolderName}");
            //}
            //finally
            //{
            //    Monitor.Exit(_lockClearFolder);
            //}
        }
        // Used for deleting small individual files. Multi-threading not necessary.
        private void ClearFile(string fl)
        {
            //var f = new FileInfo(fl);
            //if (File.Exists(f.FullName))
            //{
            //    LogPrint($"Deleting: {f.Name}");
            //    try
            //    {
            //        File.Delete(f.FullName);
            //    }
            //    catch (Exception ex)
            //    {
            //        LogPrint($"ERROR: {ex}");
            //    }
            //    LogPrint("Done.");
            //    if (fl == "config\\loginusers.vdf")
            //    {
            //        LogPrint("[ Don't forget to clear forgotten account backups as well ]");
            //        CleaningOutput.AppendText(Environment.NewLine);
            //    }
            //    CleaningOutput.AppendText(Environment.NewLine);
            //}
            //else
            //    LogPrint($"{f.Name} was not found.");
        }
        private void ClearFilesOfType(string fldName, string folderName, string fileExtensions, SearchOption searchOption)
        {
            var t = new Thread(Task_clearFilesOfType);
            var args = new PossibleClearArgs
            {
                FldName = fldName,
                FolderName = folderName,
                FileExtensions = fileExtensions,
                SearchOption = searchOption
            };
            t.Start(args);
        }
        private void Task_clearFilesOfType(object oin)
        {
            //// Only allow one to be run at a time
            //if (!Monitor.TryEnter(_lockClearFilesOfType)) return;
            //try
            //{
            //    var options = (PossibleClearArgs)oin;
            //    LogPrint(options.FldName);
            //    if (Directory.Exists(options.FolderName))
            //    {
            //        foreach (var file in GetFiles(options.FolderName, options.FileExtensions, options.SearchOption))
            //        {
            //            LogPrint($"Deleting: {file}");
            //            try
            //            {
            //                File.Delete(file);
            //            }
            //            catch (Exception ex)
            //            {
            //                LogPrint($"ERROR: {ex}");
            //            }
            //        }
            //        this.Dispatcher.Invoke(() =>
            //        {
            //            CleaningOutput.AppendText(Environment.NewLine);
            //        });
            //    }
            //    else
            //        LogPrint($"Directory not found: {options.FolderName}");
            //}
            //finally
            //{
            //    Monitor.Exit(_lockClearFilesOfType);
            //}
        }
        // SSFN very small, so multi-threading isn't needed. Super tiny freeze of UI is OK.
        private void ClearSsfn(string steamDir)
        {
            //var d = new DirectoryInfo(steamDir);
            //var count = 0;
            //foreach (var file in d.GetFiles("ssfn*"))
            //{
            //    count++;
            //    LogPrint($"Deleting: {file.Name}");
            //    try
            //    {
            //        File.Delete(file.FullName);
            //    }
            //    catch (Exception ex)
            //    {
            //        LogPrint($"ERROR: {ex}");
            //    }
            //}
            //if (count > 0)
            //{
            //    LogPrint("Done.");
            //    CleaningOutput.AppendText(Environment.NewLine);
            //}
            //else
            //    LogPrint("No SSFN files found.");
        }
        private void DeleteRegKey(string subKey, string value)
        {
            //using (var key = Registry.CurrentUser.OpenSubKey(subKey, true))
            //{
            //    if (key == null)
            //        LogPrint($"{subKey} does not exist.");
            //    else if (key.GetValue(value) == null)
            //        LogPrint($"{subKey} does not contain {value}");
            //    else
            //    {
            //        LogPrint($"Removing {subKey}\\{value}");
            //        key.DeleteValue(value);
            //        LogPrint("Done.");
            //    }
            //}
        }
        private void BtnClearLogs_Click(object sender, RoutedEventArgs e) => ClearFolder("Clearing logs:", Path.Combine(TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.SteamFolder(), "logs\\"));
        private void BtnClearDumps_Click(object sender, RoutedEventArgs e) => ClearFolder("Clearing dumps:", Path.Combine(TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.SteamFolder(), "dumps\\"));
        private void BtnClearConfigVdf_Click(object sender, RoutedEventArgs e) => ClearFile(Path.Combine(TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.SteamFolder(), "config\\config.vdf"));
        private void BtnClearLoginusersVdf_Click(object sender, RoutedEventArgs e) => ClearFile(Path.Combine(TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.SteamFolder(), "config\\loginusers.vdf"));
        private void btnClearSSFN_Click(object sender, RoutedEventArgs e) => ClearSsfn(TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.SteamFolder());
        private void btnClearRLastName_Click(object sender, RoutedEventArgs e) => DeleteRegKey(@"Software\Valve\Steam", "LastGameNameUsed");
        private void btnClearRAutoLogin_Click(object sender, RoutedEventArgs e) => DeleteRegKey(@"Software\Valve\Steam", "AutoLoginuser");
        private void btnClearRRemember_Click(object sender, RoutedEventArgs e) => DeleteRegKey(@"Software\Valve\Steam", "RememberPassword");
        private void btnClearRUID_Click(object sender, RoutedEventArgs e) => DeleteRegKey(@"Software\Valve\Steam", "PseudoUUID");
        private void BtnClearBackups_Click(object sender, RoutedEventArgs e)
        {
            //LogPrint("Clearing forgotten account backups:");
            //Settings.ClearForgottenBackups();
            //LogPrint("Done.");
            //CleaningOutput.AppendText(Environment.NewLine);
        }
        private void Hyperlink_RequestNavigate(object sender, RequestNavigateEventArgs e)
        {
            Process.Start(new ProcessStartInfo(e.Uri.AbsoluteUri) { UseShellExecute = true });
            e.Handled = true;
        }
        private void BtnKillSteam_Click(object sender, RoutedEventArgs e) => SteamSwitcherFuncs.CloseSteam();

        private void BtnClearHtmlCache_OnClick(object sender, RoutedEventArgs e) =>
            // HTML Cache - %USERPROFILE%\AppData\Local\Steam\htmlcache
            ClearFolder("Clearing htmlcache:", Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Steam\\htmlcache"));


        private void BtnClearUiLogs_OnClick(object sender, RoutedEventArgs e) =>
            // Overlay UI logs -
            //   Steam\GameOverlayUI.exe.log
            //   Steam\GameOverlayRenderer.log
            ClearFilesOfType("Clearing .log & .last from Steam:", TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.SteamFolder(), "*.log|*.last", SearchOption.TopDirectoryOnly);

        private void BtnClearAppCache_OnClick(object sender, RoutedEventArgs e) =>
            // App Cache - Steam\appcache
            ClearFilesOfType("Clearing appcache:", Path.Combine(TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.SteamFolder(), "appcache"), "*.*", SearchOption.TopDirectoryOnly);

        private void BtnClearHttpCache_OnClick(object sender, RoutedEventArgs e) =>
            // HTTP cache - Steam\appcache\httpcache\
            ClearFilesOfType("Clearing appcache\\httpcache:", Path.Combine(TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.SteamFolder(), "appcache\\httpcache"), "*.*", SearchOption.AllDirectories);

        private void BtnClearDepotCache_OnClick(object sender, RoutedEventArgs e) =>
            // Depot - Steam\depotcache\
            ClearFilesOfType("Clearing depotcache:", Path.Combine(TcNo_Acc_Switcher_Server.Pages.Steam.SteamSwitcherFuncs.SteamFolder(), "depotcache"), "*.*", SearchOption.TopDirectoryOnly);
    }
}
