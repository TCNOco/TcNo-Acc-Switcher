using Microsoft.Win32;
using System;
using System.Collections;
using System.Diagnostics;
using System.IO;
using System.Threading;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Input;
using System.Windows.Navigation;
using TcNo_Acc_Switcher_Globals;

//using System.Windows.Shapes; -- Commented because of clash with System.IO.Path. If causes issues, uncomment.

namespace TcNo_Acc_Switcher_Steam
{
    /// <summary>
    /// Interaction logic for ClearLogins.xaml
    /// </summary>
    public partial class ClearLogins : Window
    {
        MainWindow mw;
        public ClearLogins()
        {
            InitializeComponent();
        }
        public void ShareMainWindow(MainWindow imw)
        {
            mw = imw;
        }

        private void BtnExit(object sender, RoutedEventArgs e)
        {
            Globals.WindowHandling.BtnExit(sender, e, this);
        }
        private void BtnMinimize(object sender, RoutedEventArgs e)
        {
            Globals.WindowHandling.BtnMinimize(sender, e, this);
        }
        private void DragWindow(object sender, MouseButtonEventArgs e)
        {
            Globals.WindowHandling.DragWindow(sender, e, this);
        }
        private void Window_PreviewKeyDown(object sender, KeyEventArgs e)
        {
            if (e.Key == Key.Escape)
                Close();
        }
        private void LogPrint(string text)
        {
            this.Dispatcher.Invoke(() =>
            {
                CleaningOutput.AppendText(text + Environment.NewLine);
                CleaningOutput.ScrollToEnd();
            });
        }


        public string[] GetFiles(string sourceFolder, string filter, System.IO.SearchOption searchOption)
        {
            ArrayList alFiles = new ArrayList();
            string[] multipleFilters = filter.Split('|');
            foreach (string fileFilter in multipleFilters)
                alFiles.AddRange(Directory.GetFiles(sourceFolder, fileFilter, searchOption));

            return (string[])alFiles.ToArray(typeof(string));
        }
        struct PossibleClearArgs
        {
            public string fldName;
            public string folderName;
            public string fileExtensions;
            public SearchOption searchOption;
        }
        private readonly object LockClearFilesOfType = new object();
        private readonly object LockClearFolder = new object();





        private void ClearFolder(string fldName, string fld)
        {
            Thread t = new Thread(new ParameterizedThreadStart(Task_clearFolder));
            PossibleClearArgs args = new PossibleClearArgs
            {
                fldName = fldName,
                folderName = fld
            };
            t.Start(args);
        }
        private void Task_clearFolder(object oin)
        {
            // Only allow one to be run at a time
            if (Monitor.TryEnter(LockClearFolder))
            {
                try
                {
                    PossibleClearArgs options = (PossibleClearArgs)oin;
                    LogPrint(options.fldName);
                    int count = 0;
                    if (Directory.Exists(options.folderName))
                    {
                        DirectoryInfo d = new DirectoryInfo(options.folderName);
                        foreach (var file in d.GetFiles("*.*"))
                        {
                            count++;
                            LogPrint($"Deleting: {file.Name}");
                            try
                            {
                                File.Delete(file.FullName);
                            }
                            catch (Exception ex)
                            {
                                LogPrint($"ERROR: {ex.ToString()}");
                            }
                        }

                        foreach (var subd in d.GetDirectories())
                        {
                            count++;
                            LogPrint($"Deleting: {subd.Name} subfolder and contents");
                            try
                            {
                                Directory.Delete(subd.FullName, true);
                            }
                            catch (Exception ex)
                            {
                                LogPrint($"ERROR: {ex.ToString()}");
                            }
                        }
                        if (count == 0)
                            LogPrint($"{options.folderName} is empty.");
                        else
                            LogPrint("Done.");
                        CleaningOutput.AppendText(Environment.NewLine);
                    }
                    else
                        LogPrint($"Directory not found: {options.folderName}");
                }
                finally
                {
                    Monitor.Exit(LockClearFolder);
                }
            }
        }
        // Used for deleting small individual files. Multithreading not nessecary.
        private void ClearFile(string fl)
        {
            FileInfo f = new FileInfo(fl);
            if (File.Exists(f.FullName))
            {
                LogPrint($"Deleting: {f.Name}");
                try
                {
                    File.Delete(f.FullName);
                }
                catch (Exception ex)
                {
                    LogPrint($"ERROR: {ex.ToString()}");
                }
                LogPrint("Done.");
                if (fl == "config\\loginusers.vdf")
                {
                    LogPrint("[ Don't forget to clear forgotten account backups as well ]");
                    CleaningOutput.AppendText(Environment.NewLine);
                }
                CleaningOutput.AppendText(Environment.NewLine);
            }
            else
            {
                LogPrint($"{f.Name} was not found.");
            }
        }
        private void ClearFilesOfType(string fldName, string folderName, string fileExtensions, SearchOption searchOption)
        {
            Thread t = new Thread(new ParameterizedThreadStart(Task_clearFilesOfType));
            PossibleClearArgs args = new PossibleClearArgs
            {
                fldName = fldName,
                folderName = folderName,
                fileExtensions = fileExtensions,
                searchOption = searchOption
            };
            t.Start(args);
        }
        private void Task_clearFilesOfType(object oin)
        {
            // Only allow one to be run at a time
            if (Monitor.TryEnter(LockClearFilesOfType))
            {
                try
                {
                    PossibleClearArgs options = (PossibleClearArgs)oin;
                    LogPrint(options.fldName);
                    int count = 0;
                    if (Directory.Exists(options.folderName))
                    {
                        foreach (var file in GetFiles(options.folderName, options.fileExtensions, options.searchOption))
                        {
                            count++;
                            LogPrint($"Deleting: {file}");
                            try
                            {
                                File.Delete(file);
                            }
                            catch (Exception ex)
                            {
                                LogPrint($"ERROR: {ex.ToString()}");
                            }
                        }
                        this.Dispatcher.Invoke(() =>
                        {
                            CleaningOutput.AppendText(Environment.NewLine);
                        });
                    }
                    else
                        LogPrint($"Directory not found: {options.folderName}");
                }
                finally
                {
                    Monitor.Exit(LockClearFilesOfType);
                }
            }
        }
        // SSFN very small, so multithreading isn't needed. Super tiny freeze of UI is OK.
        private void ClearSsfn(string steamDir)
        {
            DirectoryInfo d = new DirectoryInfo(steamDir);
            int count = 0;
            foreach (var file in d.GetFiles("ssfn*"))
            {
                count++;
                LogPrint($"Deleting: {file.Name}");
                try
                {
                    File.Delete(file.FullName);
                }
                catch (Exception ex)
                {
                    LogPrint($"ERROR: {ex.ToString()}");
                }
            }
            if (count > 0)
            {
                LogPrint("Done.");
                CleaningOutput.AppendText(Environment.NewLine);
            }
            else
                LogPrint("No SSFN files found.");
        }
        private void DeleteRegKey(string subkey, string value)
        {
            using (RegistryKey key = Registry.CurrentUser.OpenSubKey(subkey, true))
            {
                if (key == null)
                {
                    LogPrint($"{subkey} does not exist.");
                }
                else if (key.GetValue(value) == null)
                {
                    LogPrint($"{subkey} does not contain {value}");
                }
                else
                {
                    LogPrint($"Removing {subkey}\\{value}");
                    key.DeleteValue(value);
                    LogPrint("Done.");
                }
            }
        }
        private void btnClearLogs_Click(object sender, RoutedEventArgs e)
        {
            ClearFolder("Clearing logs:", Path.Combine(mw.GetSteamDirectory(), "logs\\"));
        }

        private void btnClearDumps_Click(object sender, RoutedEventArgs e)
        {
            ClearFolder("Clearing dumps:", Path.Combine(mw.GetSteamDirectory(), "dumps\\"));
        }

        private void btnClearConfigVDF_Click(object sender, RoutedEventArgs e)
        {
            ClearFile(Path.Combine(mw.GetSteamDirectory(), "config\\config.vdf"));
        }

        private void btnClearLoginusersVDF_Click(object sender, RoutedEventArgs e)
        {
            ClearFile(Path.Combine(mw.GetSteamDirectory(), "config\\loginusers.vdf"));
        }

        private void btnClearSSFN_Click(object sender, RoutedEventArgs e)
        {
            ClearSsfn(mw.GetSteamDirectory());
        }

        private void btnClearRLastName_Click(object sender, RoutedEventArgs e)
        {
            DeleteRegKey(@"Software\Valve\Steam", "LastGameNameUsed");
        }

        private void btnClearRAutoLogin_Click(object sender, RoutedEventArgs e)
        {
            DeleteRegKey(@"Software\Valve\Steam", "AutoLoginuser");
        }

        private void btnClearRRemember_Click(object sender, RoutedEventArgs e)
        {
            DeleteRegKey(@"Software\Valve\Steam", "RememberPassword");
        }

        private void btnClearRUID_Click(object sender, RoutedEventArgs e)
        {
            DeleteRegKey(@"Software\Valve\Steam", "PseudoUUID");
        }

        private void btnClearBackups_Click(object sender, RoutedEventArgs e)
        {
            LogPrint("Clearing forgotten account backups:");
            mw.ClearForgottenBackups();
            LogPrint("Done.");
            CleaningOutput.AppendText(Environment.NewLine);
        }
        private void Hyperlink_RequestNavigate(object sender, RequestNavigateEventArgs e)
        {
            Process.Start(new ProcessStartInfo(e.Uri.AbsoluteUri) { UseShellExecute = true });
            e.Handled = true;
        }
        private void btnKillSteam_Click(object sender, RoutedEventArgs e)
        {
            mw.CloseSteam();
        }

        private void BtnClearHTMLCache_OnClick(object sender, RoutedEventArgs e)
        {
            // HTML Cache - %USERPROFILE%\AppData\Local\Steam\htmlcache
            ClearFolder("Clearing htmlcache:", Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Steam\\htmlcache"));
        }

        private void BtnClearUILogs_OnClick(object sender, RoutedEventArgs e)
        {
            // Overlay UI logs -
            //   Steam\GameOverlayUI.exe.log
            //   Steam\GameOverlayRenderer.log
            ClearFilesOfType("Clearing .log & .last from Steam:", mw.GetSteamDirectory(), "*.log|*.last", SearchOption.TopDirectoryOnly);
        }

        private void BtnClearAppCache_OnClick(object sender, RoutedEventArgs e)
        {
            // App Cache - Steam\appcache
            ClearFilesOfType("Clearing appcache:", Path.Combine(mw.GetSteamDirectory(), "appcache"),"*.*", SearchOption.TopDirectoryOnly);
        }

        private void BtnClearHTTPCache_OnClick(object sender, RoutedEventArgs e)
        {
            // HTTP cache - Steam\appcache\httpcache\
            ClearFilesOfType("Clearing appcache\\httpcache:", Path.Combine(mw.GetSteamDirectory(), "appcache\\httpcache"), "*.*", SearchOption.AllDirectories);
        }

        private void BtnClearDepotCache_OnClick(object sender, RoutedEventArgs e)
        {
            // Depot - Steam\depotcache\
            ClearFilesOfType("Clearing depotcache:", Path.Combine(mw.GetSteamDirectory(), "depotcache"), "*.*", SearchOption.TopDirectoryOnly);
        }
    }
}
