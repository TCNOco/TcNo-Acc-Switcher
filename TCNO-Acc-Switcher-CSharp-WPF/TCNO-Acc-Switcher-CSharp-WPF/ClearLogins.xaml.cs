using System;
using System.Collections.Generic;
using System.IO;
using System.Text;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Data;
using System.Windows.Documents;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Media.Imaging;
//using System.Windows.Shapes; -- Commented because of clash with System.IO.Path. If causes issues, uncomment.

namespace TCNO_Acc_Switcher_CSharp_WPF
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
            if (Directory.Exists(fld))
            {
                DirectoryInfo d = new DirectoryInfo(fld);
                foreach (var file in d.GetFiles("*.*"))
                {
                    logPrint($"Deleting: {file.Name}");
                    try
                    {
                        //Directory.Delete(file.FullName);
                    }
                    catch (Exception ex)
                    {
                        logPrint($"ERROR: {ex.ToString()}");
                    }
                }
                logPrint("___Done___");
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
            if (File.Exists(f.FullName)){
                logPrint($"Deleting: {f.Name}");
                try
                {
                    //Directory.Delete(fl.FullName);
                }
                catch (Exception ex)
                {
                    logPrint($"ERROR: {ex.ToString()}");
                }
                logPrint("___Done___");
                CleaningOutput.AppendText(Environment.NewLine);
            }
            else
            {
                logPrint($"File was not found: {f.FullName}");
            }
        }
        private void clearSSFN(string steamDIR)
        {
            DirectoryInfo d = new DirectoryInfo(steamDIR);
            foreach (var file in d.GetFiles("ssfn*"))
            {
                logPrint($"Deleting: {file.Name}");
                try
                {
                    //Directory.Delete(file.FullName);
                }
                catch (Exception ex)
                {
                    logPrint($"ERROR: {ex.ToString()}");
                }
            }
            logPrint("___Done___");
            CleaningOutput.AppendText(Environment.NewLine);
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
            logPrint("[ Don't forget to clear forgotten account backups as well ]");
            CleaningOutput.AppendText(Environment.NewLine);
        }

        private void btnClearSSFN_Click(object sender, RoutedEventArgs e)
        {
            clearSSFN(mw.GetSteamDirectory());
        }

        private void btnClearRLastName_Click(object sender, RoutedEventArgs e)
        {
            logPrint(":");
            logPrint("___Done___");
        }

        private void btnClearRAutoLogin_Click(object sender, RoutedEventArgs e)
        {
            logPrint(":");
            logPrint("___Done___");
        }

        private void btnClearRRemember_Click(object sender, RoutedEventArgs e)
        {
            logPrint(":");
            logPrint("___Done___");
        }

        private void btnClearRUID_Click(object sender, RoutedEventArgs e)
        {
            logPrint(":");
            logPrint("___Done___");
        }

        private void btnClearBackups_Click(object sender, RoutedEventArgs e)
        {
            logPrint("Clearing forgotten account backups:");
            mw.ClearForgottenBackups();
            logPrint("___Done___");
            CleaningOutput.AppendText(Environment.NewLine);
        }
    }
}
