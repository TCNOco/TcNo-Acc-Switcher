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

        private void btnExit(object sender, RoutedEventArgs e)
        {
            this.Close();
        }

        private void btnMinimize(object sender, RoutedEventArgs e)
        {
            this.WindowState = WindowState.Minimized;
        }
        private void dragWindow(object sender, MouseButtonEventArgs e)
        {
            if (e.LeftButton == MouseButtonState.Pressed)
            {
                this.DragMove();
            }
        }
        private void Window_PreviewKeyDown(object sender, KeyEventArgs e)
        {
            if (e.Key == Key.Escape)
                Close();
        }
        private void logPrint(string text)
        {
            this.Dispatcher.Invoke(() =>
            {
                CleaningOutput.AppendText(text + Environment.NewLine);
                CleaningOutput.ScrollToEnd();
            });
        }


        public string[] getFiles(string SourceFolder, string Filter, System.IO.SearchOption searchOption)
        {
            ArrayList alFiles = new ArrayList();
            string[] MultipleFilters = Filter.Split('|');
            foreach (string FileFilter in MultipleFilters)
                alFiles.AddRange(Directory.GetFiles(SourceFolder, FileFilter, searchOption));

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





        private void clearFolder(string fldName, string fld)
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
                    logPrint(options.fldName);
                    int count = 0;
                    if (Directory.Exists(options.folderName))
                    {
                        DirectoryInfo d = new DirectoryInfo(options.folderName);
                        foreach (var file in d.GetFiles("*.*"))
                        {
                            count++;
                            logPrint($"Deleting: {file.Name}");
                            try
                            {
                                File.Delete(file.FullName);
                            }
                            catch (Exception ex)
                            {
                                logPrint($"ERROR: {ex.ToString()}");
                            }
                        }

                        foreach (var subd in d.GetDirectories())
                        {
                            count++;
                            logPrint($"Deleting: {subd.Name} subfolder and contents");
                            try
                            {
                                Directory.Delete(subd.FullName, true);
                            }
                            catch (Exception ex)
                            {
                                logPrint($"ERROR: {ex.ToString()}");
                            }
                        }
                        if (count == 0)
                            logPrint($"{options.folderName} is empty.");
                        else
                            logPrint("Done.");
                        CleaningOutput.AppendText(Environment.NewLine);
                    }
                    else
                        logPrint($"Directory not found: {options.folderName}");
                }
                finally
                {
                    Monitor.Exit(LockClearFolder);
                }
            }
        }
        // Used for deleting small individual files. Multithreading not nessecary.
        private void clearFile(string fl)
        {
            FileInfo f = new FileInfo(fl);
            if (File.Exists(f.FullName))
            {
                logPrint($"Deleting: {f.Name}");
                try
                {
                    File.Delete(f.FullName);
                }
                catch (Exception ex)
                {
                    logPrint($"ERROR: {ex.ToString()}");
                }
                logPrint("Done.");
                if (fl == "config\\loginusers.vdf")
                {
                    logPrint("[ Don't forget to clear forgotten account backups as well ]");
                    CleaningOutput.AppendText(Environment.NewLine);
                }
                CleaningOutput.AppendText(Environment.NewLine);
            }
            else
            {
                logPrint($"{f.Name} was not found.");
            }
        }
        private void clearFilesOfType(string fldName, string folderName, string fileExtensions, SearchOption searchOption)
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
                    logPrint(options.fldName);
                    int count = 0;
                    if (Directory.Exists(options.folderName))
                    {
                        foreach (var file in getFiles(options.folderName, options.fileExtensions, options.searchOption))
                        {
                            count++;
                            logPrint($"Deleting: {file}");
                            try
                            {
                                File.Delete(file);
                            }
                            catch (Exception ex)
                            {
                                logPrint($"ERROR: {ex.ToString()}");
                            }
                        }
                        this.Dispatcher.Invoke(() =>
                        {
                            CleaningOutput.AppendText(Environment.NewLine);
                        });
                    }
                    else
                        logPrint($"Directory not found: {options.folderName}");
                }
                finally
                {
                    Monitor.Exit(LockClearFilesOfType);
                }
            }
        }
        // SSFN very small, so multithreading isn't needed. Super tiny freeze of UI is OK.
        private void clearSSFN(string steamDIR)
        {
            DirectoryInfo d = new DirectoryInfo(steamDIR);
            int count = 0;
            foreach (var file in d.GetFiles("ssfn*"))
            {
                count++;
                logPrint($"Deleting: {file.Name}");
                try
                {
                    File.Delete(file.FullName);
                }
                catch (Exception ex)
                {
                    logPrint($"ERROR: {ex.ToString()}");
                }
            }
            if (count > 0)
            {
                logPrint("Done.");
                CleaningOutput.AppendText(Environment.NewLine);
            }
            else
                logPrint("No SSFN files found.");
        }
        private void deleteRegKey(string subkey, string value)
        {
            using (RegistryKey key = Registry.CurrentUser.OpenSubKey(subkey, true))
            {
                if (key == null)
                {
                    logPrint($"{subkey} does not exist.");
                }
                else if (key.GetValue(value) == null)
                {
                    logPrint($"{subkey} does not contain {value}");
                }
                else
                {
                    logPrint($"Removing {subkey}\\{value}");
                    key.DeleteValue(value);
                    logPrint("Done.");
                }
            }
        }
        private void btnClearLogs_Click(object sender, RoutedEventArgs e)
        {
            clearFolder("Clearing logs:", Path.Combine(mw.GetSteamDirectory(), "logs\\"));
        }

        private void btnClearDumps_Click(object sender, RoutedEventArgs e)
        {
            clearFolder("Clearing dumps:", Path.Combine(mw.GetSteamDirectory(), "dumps\\"));
        }

        private void btnClearConfigVDF_Click(object sender, RoutedEventArgs e)
        {
            clearFile(Path.Combine(mw.GetSteamDirectory(), "config\\config.vdf"));
        }

        private void btnClearLoginusersVDF_Click(object sender, RoutedEventArgs e)
        {
            clearFile(Path.Combine(mw.GetSteamDirectory(), "config\\loginusers.vdf"));
        }

        private void btnClearSSFN_Click(object sender, RoutedEventArgs e)
        {
            clearSSFN(mw.GetSteamDirectory());
        }

        private void btnClearRLastName_Click(object sender, RoutedEventArgs e)
        {
            deleteRegKey(@"Software\Valve\Steam", "LastGameNameUsed");
        }

        private void btnClearRAutoLogin_Click(object sender, RoutedEventArgs e)
        {
            deleteRegKey(@"Software\Valve\Steam", "AutoLoginuser");
        }

        private void btnClearRRemember_Click(object sender, RoutedEventArgs e)
        {
            deleteRegKey(@"Software\Valve\Steam", "RememberPassword");
        }

        private void btnClearRUID_Click(object sender, RoutedEventArgs e)
        {
            deleteRegKey(@"Software\Valve\Steam", "PseudoUUID");
        }

        private void btnClearBackups_Click(object sender, RoutedEventArgs e)
        {
            logPrint("Clearing forgotten account backups:");
            mw.ClearForgottenBackups();
            logPrint("Done.");
            CleaningOutput.AppendText(Environment.NewLine);
        }
        private void Hyperlink_RequestNavigate(object sender, RequestNavigateEventArgs e)
        {
            Process.Start(new ProcessStartInfo(e.Uri.AbsoluteUri) { UseShellExecute = true });
            e.Handled = true;
        }
        private void btnKillSteam_Click(object sender, RoutedEventArgs e)
        {
            mw.closeSteam();
        }

        private void BtnClearHTMLCache_OnClick(object sender, RoutedEventArgs e)
        {
            // HTML Cache - %USERPROFILE%\AppData\Local\Steam\htmlcache
            clearFolder("Clearing htmlcache:", Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Steam\\htmlcache"));
        }

        private void BtnClearUILogs_OnClick(object sender, RoutedEventArgs e)
        {
            // Overlay UI logs -
            //   Steam\GameOverlayUI.exe.log
            //   Steam\GameOverlayRenderer.log
            clearFilesOfType("Clearing .log & .last from Steam:", mw.GetSteamDirectory(), "*.log|*.last", SearchOption.TopDirectoryOnly);
        }

        private void BtnClearAppCache_OnClick(object sender, RoutedEventArgs e)
        {
            // App Cache - Steam\appcache
            clearFilesOfType("Clearing appcache:", Path.Combine(mw.GetSteamDirectory(), "appcache"),"*.*", SearchOption.TopDirectoryOnly);
        }

        private void BtnClearHTTPCache_OnClick(object sender, RoutedEventArgs e)
        {
            // HTTP cache - Steam\appcache\httpcache\
            clearFilesOfType("Clearing appcache\\httpcache:", Path.Combine(mw.GetSteamDirectory(), "appcache\\httpcache"), "*.*", SearchOption.AllDirectories);
        }

        private void BtnClearDepotCache_OnClick(object sender, RoutedEventArgs e)
        {
            // Depot - Steam\depotcache\
            clearFilesOfType("Clearing depotcache:", Path.Combine(mw.GetSteamDirectory(), "depotcache"), "*.*", SearchOption.TopDirectoryOnly);
        }
    }
}
