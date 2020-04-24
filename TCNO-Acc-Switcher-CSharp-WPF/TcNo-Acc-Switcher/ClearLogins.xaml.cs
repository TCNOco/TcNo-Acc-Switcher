using Microsoft.Win32;
using System;
using System.Diagnostics;
using System.IO;
using System.Windows;
using System.Windows.Input;
using System.Windows.Navigation;
//using System.Windows.Shapes; -- Commented because of clash with System.IO.Path. If causes issues, uncomment.

namespace TcNo_Acc_Switcher
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
            this.DragMove();
        }
        private void Window_PreviewKeyDown(object sender, KeyEventArgs e)
        {
            if (e.Key == Key.Escape)
                Close();
        }
        private void logPrint(string text)
        {
            CleaningOutput.AppendText(text + Environment.NewLine);
            CleaningOutput.ScrollToEnd();
        }
        private void clearFolder(string fldName, string fld)
        {
            logPrint(fldName);
            int count = 0;
            if (Directory.Exists(fld))
            {
                DirectoryInfo d = new DirectoryInfo(fld);
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
                {
                    logPrint($"{fld} is empty.");
                }
                else
                {
                    logPrint("Done.");
                }
                CleaningOutput.AppendText(Environment.NewLine);
            }
            else
            {
                logPrint($"Directory not found: {fld}");
            }
        }
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
            {
                logPrint("No SSFN files found.");
            }
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
    }
}
